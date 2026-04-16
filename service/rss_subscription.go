package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
)

type RSSSubscriptionService struct {
	repo repo.RSSSubscriptionRepo
}

func NewRSSSubscriptionService(r repo.RSSSubscriptionRepo) *RSSSubscriptionService {
	return &RSSSubscriptionService{repo: r}
}

func (s *RSSSubscriptionService) Create(ctx context.Context, sub *domain.RSSSubscription) (*domain.RSSSubscription, error) {
	if sub == nil {
		return nil, domain.NewValidationErr("subscription is required", nil)
	}
	if err := sub.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, sub); err != nil {
		return nil, err
	}
	return sub, nil
}

func (s *RSSSubscriptionService) Get(ctx context.Context, id string) (*domain.RSSSubscription, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *RSSSubscriptionService) List(ctx context.Context) ([]domain.RSSSubscription, error) {
	return s.repo.List(ctx)
}

func (s *RSSSubscriptionService) Update(ctx context.Context, sub *domain.RSSSubscription) (*domain.RSSSubscription, error) {
	if sub == nil {
		return nil, domain.NewValidationErr("subscription is required", nil)
	}
	if err := sub.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, sub); err != nil {
		return nil, err
	}
	return sub, nil
}

func (s *RSSSubscriptionService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
