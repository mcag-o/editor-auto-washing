package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
)

type TemplateService struct {
	repo repo.TemplateRepo
}

func NewTemplateService(r repo.TemplateRepo) *TemplateService {
	return &TemplateService{repo: r}
}

func (s *TemplateService) CreateTemplate(ctx context.Context, category, name, content string) (*domain.TemplateAsset, error) {
	t, err := domain.NewTemplateAsset(category, name, content)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *TemplateService) GetTemplate(ctx context.Context, id string) (*domain.TemplateAsset, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *TemplateService) ListTemplates(ctx context.Context, category string) ([]domain.TemplateAsset, error) {
	return s.repo.List(ctx, category)
}

func (s *TemplateService) ListCategories(ctx context.Context) ([]string, error) {
	return s.repo.ListCategories(ctx)
}

func (s *TemplateService) UpdateTemplate(ctx context.Context, id, content string) (*domain.TemplateAsset, error) {
	if err := s.repo.Update(ctx, id, content); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *TemplateService) DeleteTemplate(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
