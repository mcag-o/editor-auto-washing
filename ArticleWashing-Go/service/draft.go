package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
)

type DraftService struct {
	repo repo.DraftRepo
}

func NewDraftService(r repo.DraftRepo) *DraftService {
	return &DraftService{repo: r}
}

func (s *DraftService) CreateDraft(ctx context.Context, template string) (*domain.ArticleDraft, error) {
	d := domain.NewArticleDraft(template)
	if err := s.repo.Create(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

func (s *DraftService) GetDraft(ctx context.Context, id string) (*domain.ArticleDraft, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *DraftService) ListDrafts(ctx context.Context, status string) ([]domain.ArticleDraft, error) {
	return s.repo.List(ctx, &status)
}

func (s *DraftService) UpdateDraft(ctx context.Context, id string, fn func(*domain.ArticleDraft)) error {
	return s.repo.Update(ctx, id, fn)
}
