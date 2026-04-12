package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
)

type WorkspaceArticleService struct {
	repo repo.WorkspaceRepo
}

func NewWorkspaceArticleService(r repo.WorkspaceRepo) *WorkspaceArticleService {
	return &WorkspaceArticleService{repo: r}
}

func (s *WorkspaceArticleService) CreateArticle(ctx context.Context, id, title string) (*domain.ArticleWorkspaceRecord, error) {
	w := domain.NewArticleWorkspaceRecord(id, title, "", domain.ArticleWorkspaceSource{}, nil)
	w.Status = domain.ArticleWorkspaceStatusDraft
	w.StatusHistory = []string{domain.ArticleWorkspaceStatusDraft}
	w.LifecycleHistory = []domain.ArticleWorkspaceLifecycleEntry{{
		Status:    domain.ArticleWorkspaceStatusDraft,
		Notes:     "workspace article created",
		CreatedAt: w.CreatedAt,
	}}
	if err := s.repo.Create(ctx, w); err != nil {
		return nil, err
	}
	return w, nil
}

func (s *WorkspaceArticleService) GetArticle(ctx context.Context, id string) (*domain.ArticleWorkspaceRecord, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *WorkspaceArticleService) ListArticles(ctx context.Context, status string) ([]domain.ArticleWorkspaceRecord, error) {
	return s.repo.List(ctx, &status)
}

func (s *WorkspaceArticleService) TransitionArticle(ctx context.Context, id, newStatus, notes string) (*domain.ArticleWorkspaceRecord, error) {
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateWorkspaceTransition(current.Status, newStatus); err != nil {
		return nil, err
	}

	if err := s.repo.TransitionStatus(ctx, id, newStatus, notes); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}
