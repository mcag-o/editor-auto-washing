package service

import (
	"content-hub/domain"
	"content-hub/pkg/id"
	"content-hub/pkg/repo"
	"context"
	"fmt"
	"strings"
	"time"
)

type rewriteRunner interface {
	Run(ctx context.Context, req RewriteRunRequest) (*domain.RewritePipelineRun, error)
}

type ArticleIntakeService struct {
	items      repo.RSSItemRepo
	workspaces repo.WorkspaceRepo
	rewrite    rewriteRunner
}

func NewArticleIntakeService(items repo.RSSItemRepo, workspaces repo.WorkspaceRepo, rewrite rewriteRunner) *ArticleIntakeService {
	return &ArticleIntakeService{items: items, workspaces: workspaces, rewrite: rewrite}
}

func (s *ArticleIntakeService) Intake(ctx context.Context, article domain.IntakeArticle) error {
	if err := article.Validate(); err != nil {
		return err
	}
	if s.items == nil || s.workspaces == nil || s.rewrite == nil {
		return domain.NewInternalErr("article intake service is not configured", nil)
	}

	workspaceID := id.New()
	workspace := domain.NewArticleWorkspaceRecord(workspaceID, article.Title, article.Summary, domain.ArticleWorkspaceSource{
		SourceType: article.SourceType,
		URL:        article.OriginalURL,
	}, buildIntakeWorkspaceMetadata(article))
	workspace.LifecycleHistory = []domain.ArticleWorkspaceLifecycleEntry{{
		Status:    domain.ArticleWorkspaceStatusImported,
		Notes:     "created from rss intake",
		CreatedAt: workspace.CreatedAt,
	}}

	if err := s.workspaces.Create(ctx, workspace); err != nil {
		return fmt.Errorf("create workspace article: %w", err)
	}

	if _, err := s.rewrite.Run(ctx, RewriteRunRequest{
		WorkspaceArticleID: workspace.ID,
		CollectorArticleID: article.ExternalID,
		Title:              article.Title,
		TargetType:         article.TargetType,
		SourceProfile:      article.SourceProfile,
		Version:            normalizeRewriteProfileVersion(article.RewriteProfileVersion),
	}); err != nil {
		return fmt.Errorf("run rewrite orchestrator: %w", err)
	}

	if err := s.markRSSItemImported(ctx, article, workspace.ID); err != nil {
		return err
	}

	return nil
}

func (s *ArticleIntakeService) markRSSItemImported(ctx context.Context, article domain.IntakeArticle, workspaceID string) error {
	item, err := s.items.FindDuplicate(ctx, domain.RSSDuplicateKey{
		SubscriptionID: article.SubscriptionID,
		GUID:           article.ExternalID,
		Link:           article.OriginalURL,
	})
	if err != nil {
		return fmt.Errorf("lookup rss item record: %w", err)
	}
	if item == nil {
		return nil
	}

	item.Status = domain.RSSItemStatusImported
	importedAt := time.Now().UTC()
	item.ImportedAt = &importedAt
	item.WorkspaceArticleID = workspaceID
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}
	item.Metadata["source_type"] = article.SourceType
	if article.PublishedAt != nil {
		publishedAt := *article.PublishedAt
		item.PublishedAt = &publishedAt
	}

	if err := s.items.Update(ctx, item); err != nil {
		return fmt.Errorf("update rss item record: %w", err)
	}
	return nil
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
