package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
)

type TemplateDefinitionService struct {
	repo repo.TemplateDefinitionRepo
}

func NewTemplateDefinitionService(r repo.TemplateDefinitionRepo) *TemplateDefinitionService {
	return &TemplateDefinitionService{repo: r}
}

func (s *TemplateDefinitionService) Upsert(ctx context.Context, template *domain.TemplateDefinition) error {
	return s.repo.Upsert(ctx, template)
}

func (s *TemplateDefinitionService) GetByID(ctx context.Context, id string) (*domain.TemplateDefinition, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *TemplateDefinitionService) List(ctx context.Context, limit int) ([]domain.TemplateDefinition, error) {
	return s.repo.List(ctx, limit)
}
