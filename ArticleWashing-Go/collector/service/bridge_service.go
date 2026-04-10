package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"errors"
	"fmt"
	"time"
)

type BridgeResult struct {
	CollectorArticleID string
	WorkspaceArticleID string
	Created            bool
}

type BridgeService struct {
	articles  repo.CollectorArticleRepo
	workspace repo.WorkspaceRepo
}

func NewBridgeService(articles repo.CollectorArticleRepo, workspace repo.WorkspaceRepo) *BridgeService {
	return &BridgeService{articles: articles, workspace: workspace}
}

func (s *BridgeService) PushToWorkspace(ctx context.Context, articleID string) (*BridgeResult, error) {
	article, err := s.articles.GetByID(ctx, articleID)
	if err != nil {
		return nil, err
	}
	workspaceID := article.ID
	if article.WorkspaceID != "" {
		existing, err := s.workspace.GetByID(ctx, article.WorkspaceID)
		if err == nil {
			return &BridgeResult{CollectorArticleID: article.ID, WorkspaceArticleID: existing.ID, Created: false}, nil
		}
		if !isWorkspaceNotFound(err) {
			return nil, fmt.Errorf("lookup bridged workspace article: %w", err)
		}
	}
	existing, err := s.workspace.GetByID(ctx, workspaceID)
	if err == nil {
		article.WorkspaceID = workspaceID
		article.BridgeStatus = domain.CollectorArticleBridgeSucceeded
		article.UpdatedAt = time.Now().UTC()
		if updateErr := s.articles.Update(ctx, article); updateErr != nil {
			return nil, fmt.Errorf("update collector article bridge state: %w", updateErr)
		}
		return &BridgeResult{CollectorArticleID: article.ID, WorkspaceArticleID: existing.ID, Created: false}, nil
	}
	if !isWorkspaceNotFound(err) {
		return nil, fmt.Errorf("lookup workspace article: %w", err)
	}
	workspaceArticle := domain.NewArticleWorkspaceRecord(workspaceID, article.Title, article.Summary, domain.ArticleWorkspaceSource{SourceType: "collector", Platform: article.SourceID, URL: article.CanonicalURL}, map[string]any{
		"collector_article_id": article.ID,
		"collector_entry_id":   article.EntryID,
		"collector_run_id":     article.RunID,
		"collector_source_id":  article.SourceID,
	})
	workspaceArticle.LifecycleHistory = []domain.ArticleWorkspaceLifecycleEntry{{Status: domain.ArticleWorkspaceStatusImported, Notes: "created from collector article", CreatedAt: workspaceArticle.CreatedAt}}
	if err := s.workspace.Create(ctx, workspaceArticle); err != nil {
		article.BridgeStatus = domain.CollectorArticleBridgeFailed
		article.UpdatedAt = time.Now().UTC()
		_ = s.articles.Update(ctx, article)
		return nil, err
	}
	article.WorkspaceID = workspaceID
	article.BridgeStatus = domain.CollectorArticleBridgeSucceeded
	article.UpdatedAt = time.Now().UTC()
	if err := s.articles.Update(ctx, article); err != nil {
		article.WorkspaceID = ""
		article.BridgeStatus = domain.CollectorArticleBridgeFailed
		article.UpdatedAt = time.Now().UTC()
		articleUpdateErr := s.articles.Update(ctx, article)
		workspaceDeleteErr := deleteWorkspaceIfPossible(ctx, s.workspace, workspaceID)
		if workspaceDeleteErr == nil {
			workspaceDeleteErr = fmt.Errorf("workspace article %s removed after failed collector link update", workspaceID)
		}
		return nil, combineErrors(fmt.Errorf("workspace article created but collector article link update failed: %w", err), articleUpdateErr, workspaceDeleteErr)
	}
	return &BridgeResult{CollectorArticleID: article.ID, WorkspaceArticleID: workspaceID, Created: true}, nil
}

func isWorkspaceNotFound(err error) bool {
	var appErr *domain.AppError
	return errors.As(err, &appErr) && appErr.Code == domain.ErrNotFound
}

func deleteWorkspaceIfPossible(ctx context.Context, workspace repo.WorkspaceRepo, id string) error {
	if workspace == nil {
		return nil
	}
	if err := workspace.Delete(ctx, id); err != nil && !isWorkspaceNotFound(err) {
		return err
	}
	return nil
}
