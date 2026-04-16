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

type ArticleIntakeService struct {
	workspaces workspaceArticleWriter
	rewrite    rewriteRunner
}

type workspaceArticleWriter interface {
	Create(ctx context.Context, record *domain.ArticleWorkspaceRecord) error
}

func NewArticleIntakeService(workspaces workspaceArticleWriter, rewrite rewriteRunner) *ArticleIntakeService {
	return &ArticleIntakeService{workspaces: workspaces, rewrite: rewrite}
}

func (s *ArticleIntakeService) Intake(ctx context.Context, article domain.IntakeArticle) (*domain.ArticleWorkspaceRecord, error) {
	return s.intake(ctx, "", article)
}

func (s *ArticleIntakeService) IntakeIntoWorkspace(ctx context.Context, workspaceArticleID string, article domain.IntakeArticle) (*domain.ArticleWorkspaceRecord, error) {
	return s.intake(ctx, strings.TrimSpace(workspaceArticleID), article)
}

func (s *ArticleIntakeService) intake(ctx context.Context, workspaceArticleID string, article domain.IntakeArticle) (*domain.ArticleWorkspaceRecord, error) {
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

	if _, err := s.rewrite.Run(ctx, RewriteRunRequest{
		WorkspaceArticleID: workspace.ID,
		CollectorArticleID: article.ExternalID,
		Title:              article.Title,
		TargetType:         article.TargetType,
		SourceProfile:      article.SourceProfile,
		Version:            normalizeRewriteProfileVersion(article.RewriteProfileVersion),
	}); err != nil {
		return workspace, fmt.Errorf("run rewrite orchestrator: %w", err)
	}

	return workspace, nil
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
