package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
)

type PromptRegistry struct {
	repo repo.PromptTemplateRepo
}

func NewPromptRegistry(r repo.PromptTemplateRepo) *PromptRegistry {
	return &PromptRegistry{repo: r}
}

func (r *PromptRegistry) Get(ctx context.Context, key, version string) (*domain.PromptTemplate, error) {
	return r.repo.Get(ctx, key, version)
}
