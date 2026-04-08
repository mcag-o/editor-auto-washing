package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"fmt"
)

type WorkspaceArticleService struct {
	repo repo.WorkspaceRepo
}

func NewWorkspaceArticleService(r repo.WorkspaceRepo) *WorkspaceArticleService {
	return &WorkspaceArticleService{repo: r}
}

func (s *WorkspaceArticleService) CreateArticle(ctx context.Context, id, title string) (*domain.WorkspaceArticle, error) {
	w := domain.NewWorkspaceArticle(id, title)
	if err := s.repo.Create(ctx, w); err != nil {
		return nil, err
	}
	return w, nil
}

func (s *WorkspaceArticleService) GetArticle(ctx context.Context, id string) (*domain.WorkspaceArticle, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *WorkspaceArticleService) ListArticles(ctx context.Context, status string) ([]domain.WorkspaceArticle, error) {
	return s.repo.List(ctx, &status)
}

func (s *WorkspaceArticleService) TransitionArticle(ctx context.Context, id, newStatus, notes string) (*domain.WorkspaceArticle, error) {
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	valid, ok := domain.ValidWorkspaceStatusTransitions[current.Status]
	if !ok {
		return nil, domain.NewConflictErr("unknown status: " + current.Status)
	}
	found := false
	for _, st := range valid {
		if st == newStatus {
			found = true
			break
		}
	}
	if !found {
		return nil, domain.NewConflictErr(fmt.Sprintf("invalid transition: %s → %s", current.Status, newStatus))
	}

	if err := s.repo.TransitionStatus(ctx, id, newStatus, notes); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}
