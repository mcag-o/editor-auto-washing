package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
)

type ReviewService struct {
	repo repo.ReviewRepo
}

func NewReviewService(r repo.ReviewRepo) *ReviewService {
	return &ReviewService{repo: r}
}

func (s *ReviewService) CreateReview(ctx context.Context, articleID string, assetIDs []string, publishProfile string) (*domain.ReviewTask, error) {
	r := domain.NewReviewTask(articleID, assetIDs, publishProfile)
	if err := s.repo.Create(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

func (s *ReviewService) GetReview(ctx context.Context, id string) (*domain.ReviewTask, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ReviewService) ListReviews(ctx context.Context, articleID string) ([]domain.ReviewTask, error) {
	return s.repo.ListByArticle(ctx, articleID)
}

func (s *ReviewService) ApproveReview(ctx context.Context, id, reviewer, notes string) (*domain.ReviewTask, error) {
	if err := s.repo.UpdateStatus(ctx, id, "approved", reviewer, notes); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *ReviewService) RejectReview(ctx context.Context, id, reviewer, notes string) (*domain.ReviewTask, error) {
	if err := s.repo.UpdateStatus(ctx, id, "rejected", reviewer, notes); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}
