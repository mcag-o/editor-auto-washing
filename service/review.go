package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"fmt"
)

type ReviewService struct {
	repo       repo.ReviewRepo
	workspaces repo.WorkspaceRepo
}

func NewReviewService(r repo.ReviewRepo, workspaces repo.WorkspaceRepo) *ReviewService {
	return &ReviewService{repo: r, workspaces: workspaces}
}

func (s *ReviewService) CreateReview(ctx context.Context, articleID string, assetIDs []string, publishProfile string) (*domain.ReviewTask, error) {
	r := domain.NewReviewTask(articleID, assetIDs, publishProfile)
	if err := s.repo.Create(ctx, r); err != nil {
		return nil, err
	}
	if s.workspaces != nil {
		if err := s.workspaces.TransitionStatus(ctx, articleID, domain.ArticleWorkspaceStatusReviewPending, "review created"); err != nil {
			if rollbackErr := s.repo.Delete(ctx, r.ID); rollbackErr != nil {
				return nil, fmt.Errorf("%w: %v", err, rollbackErr)
			}
			return nil, err
		}
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
	review, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateStatus(ctx, id, domain.ReviewStatusApproved, reviewer, notes); err != nil {
		return nil, err
	}
	if s.workspaces != nil {
		if err := s.workspaces.TransitionStatus(ctx, review.ArticleID, domain.ArticleWorkspaceStatusApproved, notesOrDefault(notes, "review approved")); err != nil {
			if rollbackErr := s.repo.UpdateStatus(ctx, id, domain.ReviewStatusPending, review.Reviewer, review.Notes); rollbackErr != nil {
				return nil, fmt.Errorf("%w: %v", err, rollbackErr)
			}
			return nil, err
		}
	}
	return s.repo.GetByID(ctx, id)
}

func (s *ReviewService) RejectReview(ctx context.Context, id, reviewer, notes string) (*domain.ReviewTask, error) {
	review, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateStatus(ctx, id, domain.ReviewStatusRejected, reviewer, notes); err != nil {
		return nil, err
	}
	if s.workspaces != nil {
		if err := s.workspaces.TransitionStatus(ctx, review.ArticleID, domain.ArticleWorkspaceStatusReviewRejected, notesOrDefault(notes, "review rejected")); err != nil {
			if rollbackErr := s.repo.UpdateStatus(ctx, id, domain.ReviewStatusPending, review.Reviewer, review.Notes); rollbackErr != nil {
				return nil, fmt.Errorf("%w: %v", err, rollbackErr)
			}
			return nil, err
		}
	}
	return s.repo.GetByID(ctx, id)
}

func notesOrDefault(notes, fallback string) string {
	if notes != "" {
		return notes
	}
	return fallback
}
