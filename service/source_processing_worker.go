package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	defaultSourceProcessingTargetType    = "wechat-longform"
	defaultSourceProcessingSourceProfile = "sspai"
	defaultSourceProcessingPlatform      = "wechat"
)

type sourceProcessingRewriteEntryPoint interface {
	Intake(ctx context.Context, article domain.IntakeArticle) (*domain.ArticleWorkspaceRecord, error)
	IntakeIntoWorkspace(ctx context.Context, workspaceArticleID string, article domain.IntakeArticle) (*domain.ArticleWorkspaceRecord, error)
}

type sourceProcessingRenderRunner interface {
	Render(ctx context.Context, workspaceArticleID, draftID string) error
}

type SourceProcessingRewriteResult struct {
	WorkspaceArticleID string
	DraftID            string
	RewriteRunID       string
}

type SourceProcessingWorker struct {
	repo    repo.SourceDocumentRepo
	rewrite sourceProcessingWorkerRewriteRunner
	render  sourceProcessingRenderRunner
}

type sourceProcessingWorkerRewriteRunner interface {
	Run(ctx context.Context, doc *domain.SourceDocument) (*SourceProcessingRewriteResult, error)
}

func NewSourceProcessingWorker(repo repo.SourceDocumentRepo, rewrite sourceProcessingWorkerRewriteRunner, render sourceProcessingRenderRunner) *SourceProcessingWorker {
	return &SourceProcessingWorker{repo: repo, rewrite: rewrite, render: render}
}

func (w *SourceProcessingWorker) Process(ctx context.Context, doc *domain.SourceDocument) error {
	if err := w.validate(); err != nil {
		return err
	}
	loaded, err := w.loadClaimed(ctx, doc)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	loaded.Status = domain.SourceDocumentStatusProcessing
	loaded.ProcessingStartedAt = &now
	loaded.CompletedAt = nil
	loaded.ErrorSummary = ""
	if err := w.repo.Update(ctx, loaded); err != nil {
		return fmt.Errorf("mark source document processing: %w", err)
	}

	result, err := w.rewrite.Run(ctx, loaded)
	if err != nil {
		return w.fail(ctx, loaded, err)
	}
	loaded.WorkspaceArticleID = strings.TrimSpace(result.WorkspaceArticleID)
	loaded.RewriteRunID = strings.TrimSpace(result.RewriteRunID)

	if err := w.render.Render(ctx, loaded.WorkspaceArticleID, strings.TrimSpace(result.DraftID)); err != nil {
		return w.fail(ctx, loaded, err)
	}

	completedAt := time.Now().UTC()
	loaded.Status = domain.SourceDocumentStatusCompleted
	loaded.CompletedAt = &completedAt
	loaded.ErrorSummary = ""
	if err := w.repo.Update(ctx, loaded); err != nil {
		return fmt.Errorf("mark source document completed: %w", err)
	}
	return nil
}

func (w *SourceProcessingWorker) validate() error {
	if w.repo == nil || w.rewrite == nil || w.render == nil {
		return domain.NewInternalErr("source processing worker is not configured", nil)
	}
	return nil
}

func (w *SourceProcessingWorker) loadClaimed(ctx context.Context, doc *domain.SourceDocument) (*domain.SourceDocument, error) {
	if doc == nil {
		return nil, domain.NewValidationErr("source document is required", nil)
	}
	if strings.TrimSpace(doc.ID) == "" {
		return nil, domain.NewValidationErr("source document id is required", nil)
	}
	loaded, err := w.repo.GetByID(ctx, doc.ID)
	if err != nil {
		return nil, fmt.Errorf("load source document: %w", err)
	}
	if err := loaded.Validate(); err != nil {
		return nil, err
	}
	if loaded.Status != domain.SourceDocumentStatusClaimed {
		return nil, domain.NewValidationErr("source document must be claimed before processing", nil)
	}
	return loaded, nil
}

func (w *SourceProcessingWorker) fail(ctx context.Context, doc *domain.SourceDocument, cause error) error {
	failedAt := time.Now().UTC()
	doc.Status = domain.SourceDocumentStatusFailed
	doc.CompletedAt = &failedAt
	doc.ErrorSummary = cause.Error()
	if err := w.repo.Update(ctx, doc); err != nil {
		return fmt.Errorf("%w: mark source document failed: %v", cause, err)
	}
	return cause
}

type ArticleIntakeSourceProcessingRewriteRunner struct {
	intake sourceProcessingRewriteEntryPoint
	repo   repo.SourceDocumentRepo
}

func NewArticleIntakeSourceProcessingRewriteRunner(intake sourceProcessingRewriteEntryPoint, repo repo.SourceDocumentRepo) *ArticleIntakeSourceProcessingRewriteRunner {
	return &ArticleIntakeSourceProcessingRewriteRunner{intake: intake, repo: repo}
}

func (r *ArticleIntakeSourceProcessingRewriteRunner) Run(ctx context.Context, doc *domain.SourceDocument) (*SourceProcessingRewriteResult, error) {
	if r.intake == nil || r.repo == nil {
		return nil, domain.NewInternalErr("source processing rewrite runner is not configured", nil)
	}
	if doc == nil {
		return nil, domain.NewValidationErr("source document is required", nil)
	}

	article := domain.IntakeArticle{
		ExternalID:            doc.ID,
		SourceType:            fallbackSourceDocumentSourceType(doc.SourceType),
		Title:                 strings.TrimSpace(doc.Title),
		Body:                  doc.Body,
		Summary:               strings.TrimSpace(doc.Summary),
		OriginalURL:           sourceDocumentOriginalURL(doc),
		TargetType:            sourceDocumentTargetType(doc),
		SourceProfile:         sourceDocumentSourceProfile(doc),
		RewriteProfileVersion: sourceDocumentRewriteProfileVersion(doc),
		Metadata:              sourceDocumentIntakeMetadata(doc),
	}

	workspaceID := strings.TrimSpace(doc.WorkspaceArticleID)
	var (
		workspace *domain.ArticleWorkspaceRecord
		err       error
	)
	if workspaceID == "" {
		workspace, err = r.intake.Intake(ctx, article)
	} else {
		workspace, err = r.intake.IntakeIntoWorkspace(ctx, workspaceID, article)
	}
	if err != nil {
		return nil, err
	}

	stored, err := r.repo.GetByID(ctx, doc.ID)
	if err != nil {
		return nil, fmt.Errorf("reload source document after rewrite: %w", err)
	}
	if strings.TrimSpace(stored.WorkspaceArticleID) == "" {
		stored.WorkspaceArticleID = workspace.ID
	}

	result := &SourceProcessingRewriteResult{
		WorkspaceArticleID: strings.TrimSpace(stored.WorkspaceArticleID),
		DraftID:            strings.TrimSpace(stored.WorkspaceArticleID),
		RewriteRunID:       strings.TrimSpace(stored.RewriteRunID),
	}
	if result.WorkspaceArticleID == "" {
		result.WorkspaceArticleID = workspace.ID
	}
	if result.DraftID == "" {
		result.DraftID = workspace.ID
	}
	return result, nil
}

type FormattingPipelineSourceProcessingRenderRunner struct {
	renderer            *FormattingPipelineService
	defaultPlatform     string
	defaultTemplateName string
}

func NewFormattingPipelineSourceProcessingRenderRunner(renderer *FormattingPipelineService, defaultPlatform, defaultTemplateName string) *FormattingPipelineSourceProcessingRenderRunner {
	return &FormattingPipelineSourceProcessingRenderRunner{
		renderer:            renderer,
		defaultPlatform:     strings.TrimSpace(defaultPlatform),
		defaultTemplateName: strings.TrimSpace(defaultTemplateName),
	}
}

func (r *FormattingPipelineSourceProcessingRenderRunner) Render(ctx context.Context, _ string, draftID string) error {
	if r.renderer == nil {
		return domain.NewInternalErr("source processing render runner is not configured", nil)
	}
	if strings.TrimSpace(draftID) == "" {
		return domain.NewValidationErr("draft id is required for render", nil)
	}
	_, err := r.renderer.Render(ctx, strings.TrimSpace(draftID), fallbackSourceProcessingPlatform(r.defaultPlatform), r.defaultTemplateName)
	return err
}

func fallbackSourceDocumentSourceType(value string) string {
	if strings.TrimSpace(value) == "" {
		return "folder"
	}
	return strings.TrimSpace(value)
}

func sourceDocumentOriginalURL(doc *domain.SourceDocument) string {
	if doc == nil {
		return ""
	}
	if value, ok := doc.Metadata["original_url"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	if strings.TrimSpace(doc.ArchivedPath) != "" {
		return strings.TrimSpace(doc.ArchivedPath)
	}
	return strings.TrimSpace(doc.OriginalPath)
}

func sourceDocumentTargetType(doc *domain.SourceDocument) string {
	if doc == nil {
		return defaultSourceProcessingTargetType
	}
	if value, ok := doc.Metadata["target_type"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return defaultSourceProcessingTargetType
}

func sourceDocumentSourceProfile(doc *domain.SourceDocument) string {
	if doc == nil {
		return defaultSourceProcessingSourceProfile
	}
	if value, ok := doc.Metadata["source_profile"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return defaultSourceProcessingSourceProfile
}

func sourceDocumentRewriteProfileVersion(doc *domain.SourceDocument) string {
	if doc == nil {
		return "latest"
	}
	if value, ok := doc.Metadata["rewrite_profile_version"].(string); ok {
		return normalizeRewriteProfileVersion(value)
	}
	return "latest"
}

func sourceDocumentIntakeMetadata(doc *domain.SourceDocument) map[string]any {
	metadata := map[string]any{}
	if doc == nil {
		return metadata
	}
	for key, value := range doc.Metadata {
		metadata[key] = value
	}
	metadata["source_document_id"] = doc.ID
	metadata["source_document_path"] = strings.TrimSpace(doc.OriginalPath)
	metadata["source_document_archived_path"] = strings.TrimSpace(doc.ArchivedPath)
	metadata["source_document_hash"] = strings.TrimSpace(doc.Hash)
	return metadata
}

func fallbackSourceProcessingPlatform(platform string) string {
	if strings.TrimSpace(platform) == "" {
		return defaultSourceProcessingPlatform
	}
	return strings.TrimSpace(platform)
}
