package service

import (
	"content-hub/domain"
	"content-hub/pkg/id"
	"content-hub/pkg/repo"
	"context"
)

type IngestionService struct {
	repo repo.IngestionRepo
}

func NewIngestionService(r repo.IngestionRepo) *IngestionService {
	return &IngestionService{repo: r}
}

func (s *IngestionService) Record(ctx context.Context, t string, payload map[string]any) error {
	rec := &domain.IngestionRecord{
		ID:         id.New(),
		SourceType: t,
		Payload:    payload,
		Status:     "received",
	}
	return s.repo.Record(ctx, rec)
}

func (s *IngestionService) List(ctx context.Context, t string) ([]domain.IngestionRecord, error) {
	return s.repo.List(ctx, t)
}
