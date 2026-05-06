package service

import (
	"content-hub/domain"
	"content-hub/pkg/id"
	"context"
	"fmt"
	"strings"
	"time"
)

type rewriteRunner interface {
	Run(ctx context.Context, req RewriteRunRequest) (*domain.RewritePipelineRun, error)
}

type rewriteResumeRunner interface {
	Resume(ctx context.Context, rewriteRunID string, title string) (*domain.RewritePipelineRun, error)
}

type ArticleIntakeResult struct {
	WorkspaceArticle *domain.ArticleWorkspaceRecord
	RewriteRun       *domain.RewritePipelineRun
	DraftID          string
}

type ArticleIntakeService struct {
	workspaces workspaceArticleWriter
	rewrite    rewriteRunner
	workflows  workflowDefinitionReader
}

type workflowDefinitionReader interface {
	GetByID(context.Context, string) (*domain.WorkflowDefinition, error)
}

type workspaceArticleWriter interface {
	Create(ctx context.Context, record *domain.ArticleWorkspaceRecord) error
}

func NewArticleIntakeService(workspaces workspaceArticleWriter, rewrite rewriteRunner) *ArticleIntakeService {
	return &ArticleIntakeService{workspaces: workspaces, rewrite: rewrite}
}

func NewArticleIntakeServiceWithWorkflows(workspaces workspaceArticleWriter, rewrite rewriteRunner, workflows workflowDefinitionReader) *ArticleIntakeService {
	return &ArticleIntakeService{workspaces: workspaces, rewrite: rewrite, workflows: workflows}
}

func (s *ArticleIntakeService) Intake(ctx context.Context, article domain.IntakeArticle) (*domain.ArticleWorkspaceRecord, error) {
	result, err := s.IntakeResult(ctx, article)
	if err != nil {
		if result == nil {
			return nil, err
		}
		return result.WorkspaceArticle, err
	}
	return result.WorkspaceArticle, nil
}

func (s *ArticleIntakeService) IntakeIntoWorkspace(ctx context.Context, workspaceArticleID string, article domain.IntakeArticle) (*domain.ArticleWorkspaceRecord, error) {
	result, err := s.IntakeResultIntoWorkspace(ctx, strings.TrimSpace(workspaceArticleID), article)
	if err != nil {
		if result == nil {
			return nil, err
		}
		return result.WorkspaceArticle, err
	}
	return result.WorkspaceArticle, nil
}

func (s *ArticleIntakeService) IntakeResult(ctx context.Context, article domain.IntakeArticle) (*ArticleIntakeResult, error) {
	return s.intake(ctx, "", article)
}

func (s *ArticleIntakeService) IntakeResultIntoWorkspace(ctx context.Context, workspaceArticleID string, article domain.IntakeArticle) (*ArticleIntakeResult, error) {
	return s.intake(ctx, strings.TrimSpace(workspaceArticleID), article)
}

func (s *ArticleIntakeService) ResumeResult(ctx context.Context, rewriteRunID string, article domain.IntakeArticle) (*SourceProcessingRewriteResult, error) {
	if err := article.Validate(); err != nil {
		return nil, err
	}
	if s.rewrite == nil {
		return nil, domain.NewInternalErr("article intake service is not configured", nil)
	}
	resumeRunner, ok := s.rewrite.(rewriteResumeRunner)
	if !ok {
		return nil, domain.NewInternalErr("article intake resume is not configured", nil)
	}
	run, err := resumeRunner.Resume(ctx, strings.TrimSpace(rewriteRunID), article.Title)
	if err != nil {
		return nil, fmt.Errorf("resume rewrite orchestrator: %w", err)
	}
	if run == nil {
		return nil, domain.NewInternalErr("article intake resume did not return a rewrite run", nil)
	}
	if strings.TrimSpace(run.WorkspaceArticleID) == "" {
		return nil, domain.NewInternalErr("article intake resume did not return a workspace article id", nil)
	}
	if strings.TrimSpace(run.ID) == "" {
		return nil, domain.NewInternalErr("article intake resume did not return a rewrite run id", nil)
	}
	draftID := strings.TrimSpace(run.FinalDraftID)
	if draftID == "" {
		return nil, domain.NewInternalErr("article intake resume did not return a draft id", nil)
	}
	return &SourceProcessingRewriteResult{
		WorkspaceArticleID: strings.TrimSpace(run.WorkspaceArticleID),
		DraftID:            draftID,
		RewriteRunID:       strings.TrimSpace(run.ID),
	}, nil
}

func (s *ArticleIntakeService) intake(ctx context.Context, workspaceArticleID string, article domain.IntakeArticle) (*ArticleIntakeResult, error) {
	if err := article.Validate(); err != nil {
		return nil, err
	}
	if s.workspaces == nil || s.rewrite == nil {
		return nil, domain.NewInternalErr("article intake service is not configured", nil)
	}

	workspace := &domain.ArticleWorkspaceRecord{ID: workspaceArticleID}
	if workspaceArticleID == "" {
		workspaceID := id.New()
		workspace = domain.NewArticleWorkspaceRecord(workspaceID, article.Title, article.Summary, domain.ArticleWorkspaceSource{
			SourceType: article.SourceType,
			URL:        article.OriginalURL,
		}, buildIntakeWorkspaceMetadata(article))
		workspace.LifecycleHistory = []domain.ArticleWorkspaceLifecycleEntry{{
			Status:    domain.ArticleWorkspaceStatusImported,
			Notes:     "created from rss intake",
			CreatedAt: workspace.CreatedAt,
		}}

		if err := s.workspaces.Create(ctx, workspace); err != nil {
			return nil, fmt.Errorf("create workspace article: %w", err)
		}
	}

	rewriteMetadata, err := s.buildRewriteMetadata(ctx, article)
	if err != nil {
		return nil, err
	}

	run, err := s.rewrite.Run(ctx, RewriteRunRequest{
		WorkspaceArticleID: workspace.ID,
		CollectorArticleID: article.ExternalID,
		Title:              article.Title,
		TargetType:         article.TargetType,
		SourceProfile:      article.SourceProfile,
		Version:            normalizeRewriteProfileVersion(article.RewriteProfileVersion),
		Metadata:           rewriteMetadata,
	})
	result := &ArticleIntakeResult{WorkspaceArticle: workspace}
	if run != nil {
		result.RewriteRun = run
		result.DraftID = strings.TrimSpace(run.FinalDraftID)
	}
	if err != nil {
		return result, fmt.Errorf("run rewrite orchestrator: %w", err)
	}

	return result, nil
}

func (s *ArticleIntakeService) buildRewriteMetadata(ctx context.Context, article domain.IntakeArticle) (map[string]any, error) {
	metadata := buildIntakeRewriteMetadata(article)
	if len(metadata) == 0 {
		return nil, nil
	}
	workflowID, _ := metadata["workflow_template_id"].(string)
	workflowID = strings.TrimSpace(workflowID)
	if workflowID == "" || s.workflows == nil {
		return metadata, nil
	}
	workflow, err := s.workflows.GetByID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("load workflow template %s: %w", workflowID, err)
	}
	overrides, err := deriveWorkflowStageOverrides(workflow)
	if err != nil {
		return nil, fmt.Errorf("derive workflow template %s execution config: %w", workflowID, err)
	}
	if len(overrides) == 0 {
		return metadata, nil
	}
	metadata[workflowStageOverridesMetadataKey] = overrides
	for stageName, override := range overrides {
		if strings.TrimSpace(override.NodeID) != "" {
			metadata["workflow_node_"+stageName] = override.NodeID
		}
		if strings.TrimSpace(override.PromptRef) != "" {
			if _, exists := metadata[workflowPromptRefMetadataKey]; !exists {
				metadata[workflowPromptRefMetadataKey] = override.PromptRef
			}
		}
	}
	return metadata, nil
}

func buildIntakeRewriteMetadata(article domain.IntakeArticle) map[string]any {
	if len(article.Metadata) == 0 {
		return nil
	}
	metadata := map[string]any{}
	for key, value := range article.Metadata {
		metadata[key] = value
	}
	return metadata
}

func buildIntakeWorkspaceMetadata(article domain.IntakeArticle) map[string]any {
	metadata := map[string]any{}
	for key, value := range article.Metadata {
		metadata[key] = value
	}
	metadata["collector_article_id"] = strings.TrimSpace(article.ExternalID)
	metadata["rss_external_id"] = strings.TrimSpace(article.ExternalID)
	metadata["rss_guid"] = strings.TrimSpace(article.ExternalID)
	metadata["rss_subscription_id"] = strings.TrimSpace(article.SubscriptionID)
	metadata["rss_original_url"] = strings.TrimSpace(article.OriginalURL)
	metadata["source_body"] = article.Body
	if article.PublishedAt != nil {
		metadata["rss_published_at"] = article.PublishedAt.UTC().Format(time.RFC3339)
	}
	if len(article.Tags) > 0 {
		metadata["rss_tags"] = append([]string(nil), article.Tags...)
	}
	if strings.TrimSpace(article.Author) != "" {
		metadata["rss_author"] = strings.TrimSpace(article.Author)
	}
	return metadata
}

func normalizeRewriteProfileVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "latest"
	}
	return version
}
