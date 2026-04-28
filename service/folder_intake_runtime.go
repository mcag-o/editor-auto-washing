package service

import (
	"content-hub/domain"
	"context"
	"path/filepath"
)

const defaultFolderIntakeConcurrency = 1

type SourceDocumentParser func(path string) (*ParsedSourceDocument, error)

type FolderIntakeRuntime struct {
	Parser             SourceDocumentParser
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
}

type importRunReader interface {
	GetByID(ctx context.Context, id string) (*domain.ImportRun, error)
	List(ctx context.Context, limit int) ([]domain.ImportRun, error)
}

func BuildFolderIntakeRuntime(repos *RuntimeRepos) (*FolderIntakeRuntime, error) {
	if repos == nil {
		return nil, domain.NewInternalErr("folder intake runtime repos are required", nil)
	}

	archiveDir := filepath.Join(repos.RenderedDir, "source-documents")
	importService := NewSourceDocumentImportService(repos.SourceDocumentRepo, archiveDir)
	scanner := NewFolderScanner(importService)
	rewriteAssembly := buildRewriteAssembly(repos)
	articleIntakeService := NewArticleIntakeService(repos.WorkspaceRepo, rewriteAssembly.orchestrator)
	renderer := NewFormattingPipelineService(repos.DraftRepo, repos.AssetRepo, repos.WorkspaceRepo, repos.Formatter).WithRenderedDir(repos.RenderedDir)
	rewriteRunner := NewArticleIntakeSourceProcessingRewriteRunner(articleIntakeService)
	renderRunner := NewFormattingPipelineSourceProcessingRenderRunner(renderer, "")
	worker := NewSourceProcessingWorker(repos.SourceDocumentRepo, rewriteRunner, renderRunner)
	scheduler := NewSourceProcessingScheduler(repos.SourceDocumentRepo, worker, defaultFolderIntakeConcurrency, "folder-intake-runtime")

	return &FolderIntakeRuntime{
		Parser:             ParseSourceDocument,
		ImportService:      importService,
		Scanner:            scanner,
		Worker:             worker,
		Scheduler:          scheduler,
		SourceDocumentRepo: repos.SourceDocumentRepo,
		ImportRunRepo:      repos.ImportRunRepo,
	}, nil
}
