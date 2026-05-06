package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
)

type WorkflowTemplateService struct {
	repo repo.WorkflowDefinitionRepo
}

func NewWorkflowTemplateService(r repo.WorkflowDefinitionRepo) *WorkflowTemplateService {
	return &WorkflowTemplateService{repo: r}
}

func (s *WorkflowTemplateService) Create(ctx context.Context, workflow *domain.WorkflowDefinition) error {
	return s.repo.Create(ctx, workflow)
}

func (s *WorkflowTemplateService) Update(ctx context.Context, workflow *domain.WorkflowDefinition) error {
	return s.repo.Update(ctx, workflow)
}

func (s *WorkflowTemplateService) Upsert(ctx context.Context, workflow *domain.WorkflowDefinition) error {
	return s.repo.Upsert(ctx, workflow)
}

func (s *WorkflowTemplateService) GetByID(ctx context.Context, id string) (*domain.WorkflowDefinition, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *WorkflowTemplateService) List(ctx context.Context, limit int) ([]domain.WorkflowDefinition, error) {
	return s.repo.List(ctx, limit)
}

func (s *WorkflowTemplateService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
