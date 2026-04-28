package service

import (
	"content-hub/domain"
	"context"
	"strings"
)

type SourceDocumentParser func(path string) (*ParsedSourceDocument, error)

type FolderIntakeConfig struct {
	WatchDir              string
	ArchiveDir            string
	Concurrency           int
	TargetType            string
	SourceProfile         string
	RenderPlatform        string
	RewriteProfileVersion string
}

type FolderIntakeRuntime struct {
	Parser             SourceDocumentParser
	WatchDir           string
	ArchiveDir         string
	ImportService      *SourceDocumentImportService
	Scanner            *FolderScanner
	Worker             *SourceProcessingWorker
	Scheduler          *SourceProcessingScheduler
	SourceDocumentRepo sourceDocumentStatusReader
	ImportRunRepo      importRunReader
}

type sourceDocumentStatusReader interface {
	ListByStatus(ctx context.Context, status string, limit int) ([]domain.SourceDocument, error)
	GetByID(ctx context.Context, id string) (*domain.SourceDocument, error)
	Update(ctx context.Context, doc *domain.SourceDocument) error
}

type importRunReader interface {
	GetByID(ctx context.Context, id string) (*domain.ImportRun, error)
	List(ctx context.Context, limit int) ([]domain.ImportRun, error)
}

func BuildFolderIntakeRuntime(repos *RuntimeRepos, cfg FolderIntakeConfig) (*FolderIntakeRuntime, error) {
	if repos == nil {
		return nil, domain.NewInternalErr("folder intake runtime repos are required", nil)
	}
	watchDir := strings.TrimSpace(cfg.WatchDir)
	if watchDir == "" {
		return nil, domain.NewValidationErr("folder intake watch directory is required", nil)
	}
	archiveDir := strings.TrimSpace(cfg.ArchiveDir)
	if archiveDir == "" {
		return nil, domain.NewValidationErr("folder intake archive directory is required", nil)
	}
	if cfg.Concurrency <= 0 {
		return nil, domain.NewValidationErr("folder intake processing concurrency must be greater than zero", nil)
	}

	importService := NewSourceDocumentImportService(repos.SourceDocumentRepo, archiveDir)
	scannerImporter := folderIntakeImporterWithDefaults(
		importService,
		repos.SourceDocumentRepo,
		cfg,
	)
	scanner := NewFolderScanner(scannerImporter)
	rewriteAssembly := buildRewriteAssembly(repos)
	articleIntakeService := NewArticleIntakeService(repos.WorkspaceRepo, rewriteAssembly.orchestrator)
	renderer := NewFormattingPipelineService(repos.DraftRepo, repos.AssetRepo, repos.WorkspaceRepo, repos.Formatter).WithRenderedDir(repos.RenderedDir)
	rewriteRunner := NewArticleIntakeSourceProcessingRewriteRunner(articleIntakeService)
	renderRunner := NewFormattingPipelineSourceProcessingRenderRunner(renderer, "")
	worker := NewSourceProcessingWorker(repos.SourceDocumentRepo, rewriteRunner, renderRunner)
	scheduler := NewSourceProcessingScheduler(repos.SourceDocumentRepo, worker, cfg.Concurrency, "folder-intake-runtime")

	return &FolderIntakeRuntime{
		Parser:             ParseSourceDocument,
		WatchDir:           watchDir,
		ArchiveDir:         archiveDir,
		ImportService:      importService,
		Scanner:            scanner,
		Worker:             worker,
		Scheduler:          scheduler,
		SourceDocumentRepo: repos.SourceDocumentRepo,
		ImportRunRepo:      repos.ImportRunRepo,
	}, nil
}

type folderIntakeImporter interface {
	ImportFile(ctx context.Context, path string) (*domain.SourceDocument, error)
}

type folderIntakeDefaultingImporter struct {
	base     folderIntakeImporter
	repo     sourceDocumentStatusReader
	metadata map[string]string
}

func folderIntakeImporterWithDefaults(base folderIntakeImporter, repo sourceDocumentStatusReader, cfg FolderIntakeConfig) folderIntakeImporter {
	metadata := map[string]string{}
	if value := strings.TrimSpace(cfg.TargetType); value != "" {
		metadata["target_type"] = value
	}
	if value := strings.TrimSpace(cfg.SourceProfile); value != "" {
		metadata["source_profile"] = value
	}
	if value := strings.TrimSpace(cfg.RenderPlatform); value != "" {
		metadata["render_platform"] = value
	}
	if value := strings.TrimSpace(cfg.RewriteProfileVersion); value != "" {
		metadata["rewrite_profile_version"] = value
	}
	if len(metadata) == 0 {
		return base
	}
	return &folderIntakeDefaultingImporter{base: base, repo: repo, metadata: metadata}
}

func (i *folderIntakeDefaultingImporter) ImportFile(ctx context.Context, path string) (*domain.SourceDocument, error) {
	doc, err := i.base.ImportFile(ctx, path)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, nil
	}
	if doc.Metadata == nil {
		doc.Metadata = map[string]any{}
	}
	for key, value := range i.metadata {
		doc.Metadata[key] = value
	}
	if err := i.repo.Update(ctx, doc); err != nil {
		return nil, err
	}
	return doc, nil
}
