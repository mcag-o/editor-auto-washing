package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
)

type RewriteProfileRegistry struct {
	repo repo.RewritePipelineProfileRepo
}

func NewRewriteProfileRegistry(r repo.RewritePipelineProfileRepo) *RewriteProfileRegistry {
	return &RewriteProfileRegistry{repo: r}
}

func (r *RewriteProfileRegistry) Resolve(ctx context.Context, targetType, sourceProfile, version string) (*domain.RewritePipelineProfile, error) {
	return r.repo.Get(ctx, targetType, sourceProfile, version)
}
