package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
)

type PublishService struct {
	repo       repo.PublishRepo
	publishers map[string]repo.PublisherProvider
}

func NewPublishService(r repo.PublishRepo, publishers map[string]repo.PublisherProvider) *PublishService {
	return &PublishService{repo: r, publishers: publishers}
}

func (s *PublishService) Publish(ctx context.Context, req domain.PublishRequest) (*domain.PublishResult, error) {
	p, ok := s.publishers[req.Platform]
	if !ok {
		return nil, domain.NewValidationErr("unknown publisher: "+req.Platform, nil)
	}
	result, err := p.Publish(ctx, req)
	if err == nil {
		record := domain.NewPublishRecord(req.Title, req.Platform, result.Success, result.Message, result.Metadata)
		s.repo.Record(ctx, record)
	}
	return result, err
}

func (s *PublishService) GetHistory(ctx context.Context, title string) ([]domain.PublishRecord, error) {
	return s.repo.ListByArticle(ctx, title)
}

func (s *PublishService) Platforms() []string {
	platforms := make([]string, 0, len(s.publishers))
	for k := range s.publishers {
		platforms = append(platforms, k)
	}
	return platforms
}
