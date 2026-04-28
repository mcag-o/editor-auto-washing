package service

import (
	"content-hub/domain"
	"context"
	"strings"
)

type SourceDocumentParser func(path string) (*ParsedSourceDocument, error)

type FolderIntakeConfig struct {
	WatchDir    string
	ArchiveDir  string
	Concurrency int
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
	scanner := NewFolderScanner(importService)
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
