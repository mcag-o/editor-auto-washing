package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"fmt"
	"strings"
	"time"
)

type sourceProcessingRewriteEntryPoint interface {
	IntakeResult(ctx context.Context, article domain.IntakeArticle) (*ArticleIntakeResult, error)
	IntakeResultIntoWorkspace(ctx context.Context, workspaceArticleID string, article domain.IntakeArticle) (*ArticleIntakeResult, error)
	ResumeResult(ctx context.Context, rewriteRunID string, article domain.IntakeArticle) (*SourceProcessingRewriteResult, error)
}

type sourceProcessingRenderRunner interface {
	Render(ctx context.Context, workspaceArticleID, draftID string, doc *domain.SourceDocument) error
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

	if err := w.render.Render(ctx, loaded.WorkspaceArticleID, strings.TrimSpace(result.DraftID), loaded); err != nil {
		return w.fail(ctx, loaded, err)
	}

	// Automated folder intake stops after draft materialization and render.
	// Review and publish remain optional/manual follow-up steps.
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
	if err := validateSourceProcessingMetadata(loaded); err != nil {
		return nil, err
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
}

func NewArticleIntakeSourceProcessingRewriteRunner(intake sourceProcessingRewriteEntryPoint) *ArticleIntakeSourceProcessingRewriteRunner {
	return &ArticleIntakeSourceProcessingRewriteRunner{intake: intake}
}

func (r *ArticleIntakeSourceProcessingRewriteRunner) Run(ctx context.Context, doc *domain.SourceDocument) (*SourceProcessingRewriteResult, error) {
	if r.intake == nil {
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
		TargetType:            requiredSourceDocumentMetadataString(doc, "target_type"),
		SourceProfile:         requiredSourceDocumentMetadataString(doc, "source_profile"),
		RewriteProfileVersion: sourceDocumentRewriteProfileVersion(doc),
		Metadata:              sourceDocumentIntakeMetadata(doc),
	}
	if rewriteRunID := strings.TrimSpace(doc.RewriteRunID); rewriteRunID != "" {
		// Resume now relies on the rewrite run's persisted checkpoint state.
		return r.intake.ResumeResult(ctx, rewriteRunID, article)
	}

	workspaceID := strings.TrimSpace(doc.WorkspaceArticleID)
	var (
		result *ArticleIntakeResult
		err    error
	)
	if workspaceID == "" {
		result, err = r.intake.IntakeResult(ctx, article)
	} else {
		result, err = r.intake.IntakeResultIntoWorkspace(ctx, workspaceID, article)
	}
	if err != nil {
		return nil, err
	}
	if result == nil || result.WorkspaceArticle == nil {
		return nil, domain.NewInternalErr("source processing rewrite runner did not return a workspace article", nil)
	}
	if result.RewriteRun == nil || strings.TrimSpace(result.RewriteRun.ID) == "" {
		return nil, domain.NewInternalErr("source processing rewrite runner did not return a rewrite run id", nil)
	}
	draftID := strings.TrimSpace(result.DraftID)
	if draftID == "" {
		return nil, domain.NewInternalErr("source processing rewrite runner did not return a draft id", nil)
	}
	return &SourceProcessingRewriteResult{
		WorkspaceArticleID: strings.TrimSpace(result.WorkspaceArticle.ID),
		DraftID:            draftID,
		RewriteRunID:       strings.TrimSpace(result.RewriteRun.ID),
	}, nil
}

type FormattingPipelineSourceProcessingRenderRunner struct {
	renderer            *FormattingPipelineService
	defaultTemplateName string
}

func NewFormattingPipelineSourceProcessingRenderRunner(renderer *FormattingPipelineService, defaultTemplateName string) *FormattingPipelineSourceProcessingRenderRunner {
	return &FormattingPipelineSourceProcessingRenderRunner{
		renderer:            renderer,
		defaultTemplateName: strings.TrimSpace(defaultTemplateName),
	}
}

func (r *FormattingPipelineSourceProcessingRenderRunner) Render(ctx context.Context, _ string, draftID string, doc *domain.SourceDocument) error {
	if r.renderer == nil {
		return domain.NewInternalErr("source processing render runner is not configured", nil)
	}
	if strings.TrimSpace(draftID) == "" {
		return domain.NewValidationErr("draft id is required for render", nil)
	}
	_, err := r.renderer.Render(ctx, strings.TrimSpace(draftID), requiredSourceDocumentMetadataString(doc, "render_platform"), r.defaultTemplateName)
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

func validateSourceProcessingMetadata(doc *domain.SourceDocument) error {
	for _, key := range []string{"target_type", "source_profile", "render_platform"} {
		if requiredSourceDocumentMetadataString(doc, key) == "" {
			return domain.NewValidationErr(fmt.Sprintf("source document metadata %s is required", key), nil)
		}
	}
	return nil
}

func requiredSourceDocumentMetadataString(doc *domain.SourceDocument, key string) string {
	if doc == nil || doc.Metadata == nil {
		return ""
	}
	value, ok := doc.Metadata[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}
