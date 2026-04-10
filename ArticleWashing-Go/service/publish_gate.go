package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"fmt"
	"strings"
)

type PublisherProvider = repo.PublisherProvider

type PublishGateService struct {
	reviews    repo.ReviewRepo
	assets     repo.AssetRepo
	drafts     repo.DraftRepo
	publishes  repo.PublishRepo
	workspaces repo.WorkspaceRepo
	publishers map[string]repo.PublisherProvider
}

func NewPublishGateService(reviews repo.ReviewRepo, assets repo.AssetRepo, drafts repo.DraftRepo, publishes repo.PublishRepo, workspaces repo.WorkspaceRepo, publishers map[string]repo.PublisherProvider) *PublishGateService {
	return &PublishGateService{reviews: reviews, assets: assets, drafts: drafts, publishes: publishes, workspaces: workspaces, publishers: publishers}
}

func (s *PublishGateService) PublishReview(ctx context.Context, reviewID string) (*domain.PublishOutcome, error) {
	review, err := s.reviews.GetByID(ctx, reviewID)
	if err != nil {
		return nil, err
	}
	draft, err := s.drafts.GetByID(ctx, review.ArticleID)
	if err != nil {
		return nil, err
	}
	articleTitle := strings.TrimSpace(draft.DisplayTitle())
	if articleTitle == "" {
		articleTitle = review.ArticleID
	}
	outcome := &domain.PublishOutcome{Records: make([]domain.PublishRecord, 0, len(review.AssetIDs))}
	if review.Status != domain.ReviewStatusApproved {
		record := domain.NewPublishRecord(review.ArticleID, articleTitle, review.ID, firstAssetID(review.AssetIDs), "", false, "review must be approved before publishing", nil)
		if err := s.recordAttempt(ctx, record, domain.NewConflictErr("review must be approved before publishing")); err != nil {
			return outcome, err
		}
		return outcome, domain.NewConflictErr("review must be approved before publishing")
	}

	for _, assetID := range review.AssetIDs {
		asset, assetErr := s.assets.GetByID(ctx, assetID)
		if assetErr != nil {
			outcome.FailedAssetID = assetID
			outcome.Partial = len(outcome.Records) > 0
			record := domain.NewPublishRecord(review.ArticleID, articleTitle, review.ID, assetID, "", false, assetErr.Error(), nil)
			if err := s.recordAttempt(ctx, record, assetErr); err != nil {
				return outcome, err
			}
			return outcome, assetErr
		}
		if asset.Status != domain.AssetStatusReady {
			outcome.FailedAssetID = assetID
			outcome.Partial = len(outcome.Records) > 0
			record := domain.NewPublishRecord(review.ArticleID, articleTitle, review.ID, assetID, asset.Platform, false, "asset is not ready", nil)
			if err := s.recordAttempt(ctx, record, domain.NewConflictErr("asset is not ready")); err != nil {
				return outcome, err
			}
			return outcome, domain.NewConflictErr("asset is not ready")
		}
		publisher, ok := s.publishers[asset.Platform]
		if !ok {
			outcome.FailedAssetID = assetID
			outcome.Partial = len(outcome.Records) > 0
			record := domain.NewPublishRecord(review.ArticleID, articleTitle, review.ID, assetID, asset.Platform, false, "unknown publisher: "+asset.Platform, nil)
			if err := s.recordAttempt(ctx, record, domain.NewValidationErr("unknown publisher: "+asset.Platform, nil)); err != nil {
				return outcome, err
			}
			return outcome, domain.NewValidationErr("unknown publisher: "+asset.Platform, nil)
		}
		publishReq := domain.PublishRequest{
			Platform:    asset.Platform,
			Title:       articleTitle,
			HTMLContent: asset.Content,
			Metadata: map[string]any{
				"article_id":      review.ArticleID,
				"review_id":       review.ID,
				"asset_id":        asset.AssetID,
				"publish_profile": review.PublishProfile,
			},
		}
		publishResult, publishErr := publisher.Publish(ctx, publishReq)
		if publishErr != nil {
			outcome.FailedAssetID = assetID
			outcome.Partial = len(outcome.Records) > 0
			record := domain.NewPublishRecord(review.ArticleID, articleTitle, review.ID, assetID, asset.Platform, false, publishErr.Error(), nil)
			if err := s.recordAttempt(ctx, record, publishErr); err != nil {
				return outcome, err
			}
			return outcome, publishErr
		}
		record := domain.NewPublishRecord(review.ArticleID, articleTitle, review.ID, assetID, asset.Platform, publishResult.Success, publishResult.Message, publishResult.Metadata)
		if err := s.publishes.Record(ctx, record); err != nil {
			return outcome, err
		}
		outcome.Records = append(outcome.Records, *record)
	}
	outcome.Success = len(outcome.Records) == len(review.AssetIDs)
	if outcome.Success && len(outcome.Records) > 0 && s.workspaces != nil {
		if err := s.workspaces.TransitionStatus(ctx, review.ArticleID, domain.ArticleWorkspaceStatusPublished, "publish completed"); err != nil {
			outcome.WorkspaceSynced = false
			return outcome, err
		}
		outcome.WorkspaceSynced = true
	}
	return outcome, nil
}

func (s *PublishGateService) History(ctx context.Context, articleID string) ([]domain.PublishRecord, error) {
	return s.publishes.ListByArticle(ctx, articleID)
}

func firstAssetID(assetIDs []string) string {
	if len(assetIDs) == 0 {
		return ""
	}
	return assetIDs[0]
}

func (s *PublishGateService) recordAttempt(ctx context.Context, record *domain.PublishRecord, cause error) error {
	if err := s.publishes.Record(ctx, record); err != nil {
		if cause == nil {
			return err
		}
		return fmt.Errorf("%w: %v", err, cause)
	}
	return nil
}
