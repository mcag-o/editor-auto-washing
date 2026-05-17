package service

import (
	"content-hub/domain"
	"content-hub/infra/memory"
	"content-hub/pkg/repo"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewServiceCreateApproveRejectAndList(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("draft-1", "Draft title", "", domain.ArticleWorkspaceSource{}, nil)
	workspace.Status = domain.ArticleWorkspaceStatusRendered
	workspace.StatusHistory = []string{domain.ArticleWorkspaceStatusImported, domain.ArticleWorkspaceStatusDraft, domain.ArticleWorkspaceStatusRendered}
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))
	svc := NewReviewService(provider.ReviewRepo(), provider.WorkspaceRepo())

	review, err := svc.CreateReview(t.Context(), "draft-1", []string{"asset-1"}, "wechat-main")
	require.NoError(t, err)
	assert.Equal(t, domain.ReviewStatusPending, review.Status)
	updatedWorkspace, err := provider.WorkspaceRepo().GetByID(t.Context(), "draft-1")
	require.NoError(t, err)
	assert.Equal(t, domain.ArticleWorkspaceStatusReviewPending, updatedWorkspace.Status)

	approved, err := svc.ApproveReview(t.Context(), review.ID, "alice", "ship it")
	require.NoError(t, err)
	assert.Equal(t, domain.ReviewStatusApproved, approved.Status)
	assert.Equal(t, "alice", approved.Reviewer)
	updatedWorkspace, err = provider.WorkspaceRepo().GetByID(t.Context(), "draft-1")
	require.NoError(t, err)
	assert.Equal(t, domain.ArticleWorkspaceStatusApproved, updatedWorkspace.Status)
	listed, err := svc.ListReviews(t.Context(), "draft-1")
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, review.ID, listed[0].ID)
	assert.Equal(t, domain.ReviewStatusApproved, listed[0].Status)
}

func TestReviewServiceRejectTransitionsWorkspaceToReviewRejected(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("draft-2", "Draft title", "", domain.ArticleWorkspaceSource{}, nil)
	workspace.Status = domain.ArticleWorkspaceStatusRendered
	workspace.StatusHistory = []string{domain.ArticleWorkspaceStatusImported, domain.ArticleWorkspaceStatusDraft, domain.ArticleWorkspaceStatusRendered}
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))
	svc := NewReviewService(provider.ReviewRepo(), provider.WorkspaceRepo())

	review, err := svc.CreateReview(t.Context(), "draft-2", []string{"asset-1"}, "wechat-main")
	require.NoError(t, err)

	rejected, err := svc.RejectReview(t.Context(), review.ID, "bob", "needs work")
	require.NoError(t, err)
	assert.Equal(t, domain.ReviewStatusRejected, rejected.Status)
	assert.Equal(t, "bob", rejected.Reviewer)
	updatedWorkspace, err := provider.WorkspaceRepo().GetByID(t.Context(), "draft-2")
	require.NoError(t, err)
	assert.Equal(t, domain.ArticleWorkspaceStatusReviewRejected, updatedWorkspace.Status)
}

func TestReviewServiceDoesNotPersistReviewWhenWorkspaceTransitionFailsOnCreate(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("draft-3", "Draft title", "", domain.ArticleWorkspaceSource{}, nil)
	workspace.Status = domain.ArticleWorkspaceStatusRendered
	workspace.StatusHistory = []string{domain.ArticleWorkspaceStatusImported, domain.ArticleWorkspaceStatusDraft, domain.ArticleWorkspaceStatusRendered}
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))
	svc := NewReviewService(provider.ReviewRepo(), failingWorkspaceRepoForReview{base: provider.WorkspaceRepo(), failOnStatus: domain.ArticleWorkspaceStatusReviewPending, err: errors.New("workspace transition failed")})

	review, err := svc.CreateReview(t.Context(), "draft-3", []string{"asset-1"}, "wechat-main")
	require.Error(t, err)
	assert.Nil(t, review)
	listed, listErr := provider.ReviewRepo().ListByArticle(t.Context(), "draft-3")
	require.NoError(t, listErr)
	assert.Empty(t, listed)
	updatedWorkspace, getErr := provider.WorkspaceRepo().GetByID(t.Context(), "draft-3")
	require.NoError(t, getErr)
	assert.Equal(t, domain.ArticleWorkspaceStatusRendered, updatedWorkspace.Status)
}

func TestReviewServiceReportsRollbackFailureWhenCreateCompensationFails(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("draft-5", "Draft title", "", domain.ArticleWorkspaceSource{}, nil)
	workspace.Status = domain.ArticleWorkspaceStatusRendered
	workspace.StatusHistory = []string{domain.ArticleWorkspaceStatusImported, domain.ArticleWorkspaceStatusDraft, domain.ArticleWorkspaceStatusRendered}
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))
	reviewRepo := reviewRepoWithDeleteFailure{base: provider.ReviewRepo(), deleteErr: errors.New("review delete rollback failed")}
	svc := NewReviewService(reviewRepo, failingWorkspaceRepoForReview{base: provider.WorkspaceRepo(), failOnStatus: domain.ArticleWorkspaceStatusReviewPending, err: errors.New("workspace transition failed")})

	review, err := svc.CreateReview(t.Context(), "draft-5", []string{"asset-1"}, "wechat-main")
	require.Error(t, err)
	assert.Nil(t, review)
	assert.Contains(t, err.Error(), "workspace transition failed")
	assert.Contains(t, err.Error(), "review delete rollback failed")
}

func TestReviewServiceRollsBackApproveWhenWorkspaceTransitionFails(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("draft-4", "Draft title", "", domain.ArticleWorkspaceSource{}, nil)
	workspace.Status = domain.ArticleWorkspaceStatusRendered
	workspace.StatusHistory = []string{domain.ArticleWorkspaceStatusImported, domain.ArticleWorkspaceStatusDraft, domain.ArticleWorkspaceStatusRendered}
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))
	baseSvc := NewReviewService(provider.ReviewRepo(), provider.WorkspaceRepo())
	review, err := baseSvc.CreateReview(t.Context(), "draft-4", []string{"asset-1"}, "wechat-main")
	require.NoError(t, err)
	svc := NewReviewService(provider.ReviewRepo(), failingWorkspaceRepoForReview{base: provider.WorkspaceRepo(), failOnStatus: domain.ArticleWorkspaceStatusApproved, err: errors.New("workspace transition failed")})

	approved, err := svc.ApproveReview(t.Context(), review.ID, "alice", "ok")
	require.Error(t, err)
	assert.Nil(t, approved)
	currentReview, getErr := provider.ReviewRepo().GetByID(t.Context(), review.ID)
	require.NoError(t, getErr)
	assert.Equal(t, domain.ReviewStatusPending, currentReview.Status)
	updatedWorkspace, wsErr := provider.WorkspaceRepo().GetByID(t.Context(), "draft-4")
	require.NoError(t, wsErr)
	assert.Equal(t, domain.ArticleWorkspaceStatusReviewPending, updatedWorkspace.Status)
}

func TestReviewServiceReportsRollbackFailureWhenApproveCompensationFails(t *testing.T) {
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("draft-6", "Draft title", "", domain.ArticleWorkspaceSource{}, nil)
	workspace.Status = domain.ArticleWorkspaceStatusRendered
	workspace.StatusHistory = []string{domain.ArticleWorkspaceStatusImported, domain.ArticleWorkspaceStatusDraft, domain.ArticleWorkspaceStatusRendered}
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))
	baseSvc := NewReviewService(provider.ReviewRepo(), provider.WorkspaceRepo())
	review, err := baseSvc.CreateReview(t.Context(), "draft-6", []string{"asset-1"}, "wechat-main")
	require.NoError(t, err)
	reviewRepo := reviewRepoWithConditionalUpdateFailure{base: provider.ReviewRepo(), failStatus: domain.ReviewStatusPending, updateErr: errors.New("review rollback update failed")}
	svc := NewReviewService(reviewRepo, failingWorkspaceRepoForReview{base: provider.WorkspaceRepo(), failOnStatus: domain.ArticleWorkspaceStatusApproved, err: errors.New("workspace transition failed")})

	approved, err := svc.ApproveReview(t.Context(), review.ID, "alice", "ok")
	require.Error(t, err)
	assert.Nil(t, approved)
	assert.Contains(t, err.Error(), "workspace transition failed")
	assert.Contains(t, err.Error(), "review rollback update failed")
}

func TestPublishGateServiceRejectsUnapprovedReviewAndRecordsAttempt(t *testing.T) {
	provider := memory.NewProvider()
	reviewSvc := NewReviewService(provider.ReviewRepo(), provider.WorkspaceRepo())
	publisher := &publishProviderStub{}
	gate := NewPublishGateService(provider.ReviewRepo(), provider.AssetRepo(), provider.DraftRepo(), provider.PublishRepo(), provider.WorkspaceRepo(), map[string]PublisherProvider{"wechat": publisher})

	draft := domain.NewArticleDraft("daily")
	draft.Headline["title"] = "Draft title"
	require.NoError(t, provider.DraftRepo().Create(t.Context(), draft))
	workspace := newRenderedWorkspaceRecord(draft.ID, "Draft title")
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))
	asset := domain.NewRenderedAssetRecord(draft.ID, "wechat", "html", "daily", "<html></html>", "")
	asset.Status = domain.AssetStatusReady
	require.NoError(t, provider.AssetRepo().Create(t.Context(), asset))
	review, err := reviewSvc.CreateReview(t.Context(), draft.ID, []string{asset.AssetID}, "wechat-main")
	require.NoError(t, err)

	result, err := gate.PublishReview(t.Context(), review.ID)
	require.Error(t, err)
	require.NotNil(t, result)
	require.Empty(t, result.Records)
	assert.False(t, result.Success)
	assert.False(t, result.Partial)
	assert.Contains(t, err.Error(), "review must be approved")
	assert.Empty(t, publisher.requests)

	history, histErr := provider.PublishRepo().ListByArticle(t.Context(), draft.ID)
	require.NoError(t, histErr)
	require.Len(t, history, 1)
	assert.False(t, history[0].Success)
	assert.Equal(t, review.ID, history[0].ReviewID)
	assert.Equal(t, asset.AssetID, history[0].AssetID)
	assert.Contains(t, history[0].Message, "review must be approved")
	updatedWorkspace, err := provider.WorkspaceRepo().GetByID(t.Context(), draft.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ArticleWorkspaceStatusReviewPending, updatedWorkspace.Status)
}

func TestPublishGateServicePublishesApprovedReadyAssetsAndRecordsResults(t *testing.T) {
	provider := memory.NewProvider()
	reviewSvc := NewReviewService(provider.ReviewRepo(), provider.WorkspaceRepo())
	publisher := &publishProviderStub{result: &domain.PublishResult{Success: true, Platform: "wechat", Message: "published", Metadata: map[string]any{"remote_id": "wx-1"}}}
	gate := NewPublishGateService(provider.ReviewRepo(), provider.AssetRepo(), provider.DraftRepo(), provider.PublishRepo(), provider.WorkspaceRepo(), map[string]PublisherProvider{"wechat": publisher})

	draft := domain.NewArticleDraft("daily")
	draft.Headline["title"] = "Draft title"
	require.NoError(t, provider.DraftRepo().Create(t.Context(), draft))
	workspace := newRenderedWorkspaceRecord(draft.ID, "Draft title")
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))
	asset := domain.NewRenderedAssetRecord(draft.ID, "wechat", "html", "daily", "<html><body>ok</body></html>", "")
	asset.Status = domain.AssetStatusReady
	require.NoError(t, provider.AssetRepo().Create(t.Context(), asset))
	review, err := reviewSvc.CreateReview(t.Context(), draft.ID, []string{asset.AssetID}, "wechat-main")
	require.NoError(t, err)
	_, err = reviewSvc.ApproveReview(t.Context(), review.ID, "alice", "looks good")
	require.NoError(t, err)

	outcome, err := gate.PublishReview(t.Context(), review.ID)
	require.NoError(t, err)
	require.NotNil(t, outcome)
	require.Len(t, outcome.Records, 1)
	assert.True(t, outcome.Success)
	assert.True(t, outcome.WorkspaceSynced)
	assert.True(t, outcome.Records[0].Success)
	assert.Equal(t, draft.ID, outcome.Records[0].ArticleID)
	assert.Equal(t, review.ID, outcome.Records[0].ReviewID)
	assert.Equal(t, asset.AssetID, outcome.Records[0].AssetID)
	require.Len(t, publisher.requests, 1)
	assert.Equal(t, "Draft title", publisher.requests[0].Title)
	assert.Equal(t, asset.Content, publisher.requests[0].HTMLContent)

	history, histErr := gate.History(t.Context(), draft.ID)
	require.NoError(t, histErr)
	require.Len(t, history, 1)
	assert.True(t, history[0].Success)
	assert.Equal(t, "wx-1", history[0].Metadata["remote_id"])
	updatedWorkspace, err := provider.WorkspaceRepo().GetByID(t.Context(), draft.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ArticleWorkspaceStatusPublished, updatedWorkspace.Status)
}

func TestPublishGateServiceReturnsRecordFailureForBlockedPublishAttempt(t *testing.T) {
	provider := memory.NewProvider()
	reviewSvc := NewReviewService(provider.ReviewRepo(), provider.WorkspaceRepo())
	recordErr := errors.New("record write failed")
	gate := NewPublishGateService(provider.ReviewRepo(), provider.AssetRepo(), provider.DraftRepo(), &publishRepoRecordFailStub{err: recordErr}, provider.WorkspaceRepo(), map[string]PublisherProvider{"wechat": &publishProviderStub{}})

	draft := domain.NewArticleDraft("daily")
	draft.Headline["title"] = "Draft title"
	require.NoError(t, provider.DraftRepo().Create(t.Context(), draft))
	workspace := newRenderedWorkspaceRecord(draft.ID, "Draft title")
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))
	asset := domain.NewRenderedAssetRecord(draft.ID, "wechat", "html", "daily", "<html></html>", "")
	require.NoError(t, provider.AssetRepo().Create(t.Context(), asset))
	review, err := reviewSvc.CreateReview(t.Context(), draft.ID, []string{asset.AssetID}, "wechat-main")
	require.NoError(t, err)

	outcome, err := gate.PublishReview(t.Context(), review.ID)
	require.ErrorIs(t, err, recordErr)
	require.NotNil(t, outcome)
	require.Empty(t, outcome.Records)
}

func TestPublishGateServiceReturnsRecordFailureForProviderErrorAttempt(t *testing.T) {
	provider := memory.NewProvider()
	reviewSvc := NewReviewService(provider.ReviewRepo(), provider.WorkspaceRepo())
	recordErr := errors.New("record write failed")
	gate := NewPublishGateService(provider.ReviewRepo(), provider.AssetRepo(), provider.DraftRepo(), &publishRepoRecordFailStub{err: recordErr}, provider.WorkspaceRepo(), map[string]PublisherProvider{"wechat": &publishProviderStub{err: errors.New("provider publish failed")}})

	draft := domain.NewArticleDraft("daily")
	draft.Headline["title"] = "Draft title"
	require.NoError(t, provider.DraftRepo().Create(t.Context(), draft))
	workspace := newRenderedWorkspaceRecord(draft.ID, "Draft title")
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))
	asset := domain.NewRenderedAssetRecord(draft.ID, "wechat", "html", "daily", "<html></html>", "")
	asset.Status = domain.AssetStatusReady
	require.NoError(t, provider.AssetRepo().Create(t.Context(), asset))
	review, err := reviewSvc.CreateReview(t.Context(), draft.ID, []string{asset.AssetID}, "wechat-main")
	require.NoError(t, err)
	_, err = reviewSvc.ApproveReview(t.Context(), review.ID, "alice", "ok")
	require.NoError(t, err)

	outcome, err := gate.PublishReview(t.Context(), review.ID)
	require.ErrorIs(t, err, recordErr)
	require.NotNil(t, outcome)
	require.Empty(t, outcome.Records)
}

func TestPublishGateServiceRejectsNonReadyAssetAndKeepsApprovedWorkspaceStatus(t *testing.T) {
	provider := memory.NewProvider()
	reviewSvc := NewReviewService(provider.ReviewRepo(), provider.WorkspaceRepo())
	gate := NewPublishGateService(provider.ReviewRepo(), provider.AssetRepo(), provider.DraftRepo(), provider.PublishRepo(), provider.WorkspaceRepo(), map[string]PublisherProvider{"wechat": &publishProviderStub{}})

	draft := domain.NewArticleDraft("daily")
	draft.Headline["title"] = "Draft title"
	require.NoError(t, provider.DraftRepo().Create(t.Context(), draft))
	workspace := newRenderedWorkspaceRecord(draft.ID, "Draft title")
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))
	asset := domain.NewRenderedAssetRecord(draft.ID, "wechat", "html", "daily", "<html></html>", "")
	asset.Status = "processing"
	require.NoError(t, provider.AssetRepo().Create(t.Context(), asset))
	review, err := reviewSvc.CreateReview(t.Context(), draft.ID, []string{asset.AssetID}, "wechat-main")
	require.NoError(t, err)
	_, err = reviewSvc.ApproveReview(t.Context(), review.ID, "alice", "ok")
	require.NoError(t, err)

	outcome, err := gate.PublishReview(t.Context(), review.ID)
	require.Error(t, err)
	require.NotNil(t, outcome)
	require.Empty(t, outcome.Records)
	assert.Equal(t, asset.AssetID, outcome.FailedAssetID)
	assert.Contains(t, err.Error(), "asset is not ready")
	history, histErr := gate.History(t.Context(), draft.ID)
	require.NoError(t, histErr)
	require.Len(t, history, 1)
	assert.False(t, history[0].Success)
	updatedWorkspace, err := provider.WorkspaceRepo().GetByID(t.Context(), draft.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ArticleWorkspaceStatusApproved, updatedWorkspace.Status)
}

func TestPublishGateServiceRejectsMissingPublisherAndKeepsApprovedWorkspaceStatus(t *testing.T) {
	provider := memory.NewProvider()
	reviewSvc := NewReviewService(provider.ReviewRepo(), provider.WorkspaceRepo())
	gate := NewPublishGateService(provider.ReviewRepo(), provider.AssetRepo(), provider.DraftRepo(), provider.PublishRepo(), provider.WorkspaceRepo(), map[string]PublisherProvider{})

	draft := domain.NewArticleDraft("daily")
	draft.Headline["title"] = "Draft title"
	require.NoError(t, provider.DraftRepo().Create(t.Context(), draft))
	workspace := newRenderedWorkspaceRecord(draft.ID, "Draft title")
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))
	asset := domain.NewRenderedAssetRecord(draft.ID, "wechat", "html", "daily", "<html></html>", "")
	require.NoError(t, provider.AssetRepo().Create(t.Context(), asset))
	review, err := reviewSvc.CreateReview(t.Context(), draft.ID, []string{asset.AssetID}, "wechat-main")
	require.NoError(t, err)
	_, err = reviewSvc.ApproveReview(t.Context(), review.ID, "alice", "ok")
	require.NoError(t, err)

	outcome, err := gate.PublishReview(t.Context(), review.ID)
	require.Error(t, err)
	require.NotNil(t, outcome)
	require.Empty(t, outcome.Records)
	assert.Equal(t, asset.AssetID, outcome.FailedAssetID)
	assert.Contains(t, err.Error(), "unknown publisher")
	history, histErr := gate.History(t.Context(), draft.ID)
	require.NoError(t, histErr)
	require.Len(t, history, 1)
	assert.False(t, history[0].Success)
	updatedWorkspace, err := provider.WorkspaceRepo().GetByID(t.Context(), draft.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ArticleWorkspaceStatusApproved, updatedWorkspace.Status)
}

func TestPublishGateServiceReturnsPartialResultsAndHistoryOnMultiAssetFailure(t *testing.T) {
	provider := memory.NewProvider()
	reviewSvc := NewReviewService(provider.ReviewRepo(), provider.WorkspaceRepo())
	publisher := &publishProviderSequenceStub{results: []*domain.PublishResult{{Success: true, Platform: "wechat", Message: "published asset 1", Metadata: map[string]any{"remote_id": "wx-1"}}}, errAt: 1, err: errors.New("second publish failed")}
	gate := NewPublishGateService(provider.ReviewRepo(), provider.AssetRepo(), provider.DraftRepo(), provider.PublishRepo(), provider.WorkspaceRepo(), map[string]PublisherProvider{"wechat": publisher})

	draft := domain.NewArticleDraft("daily")
	draft.Headline["title"] = "Draft title"
	require.NoError(t, provider.DraftRepo().Create(t.Context(), draft))
	workspace := newRenderedWorkspaceRecord(draft.ID, "Draft title")
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))
	assetOne := domain.NewRenderedAssetRecord(draft.ID, "wechat", "html", "daily", "<html>one</html>", "")
	assetTwo := domain.NewRenderedAssetRecord(draft.ID, "wechat", "html", "daily", "<html>two</html>", "")
	require.NoError(t, provider.AssetRepo().Create(t.Context(), assetOne))
	require.NoError(t, provider.AssetRepo().Create(t.Context(), assetTwo))
	review, err := reviewSvc.CreateReview(t.Context(), draft.ID, []string{assetOne.AssetID, assetTwo.AssetID}, "wechat-main")
	require.NoError(t, err)
	_, err = reviewSvc.ApproveReview(t.Context(), review.ID, "alice", "ok")
	require.NoError(t, err)

	outcome, err := gate.PublishReview(t.Context(), review.ID)
	require.Error(t, err)
	require.NotNil(t, outcome)
	require.Len(t, outcome.Records, 1)
	assert.True(t, outcome.Partial)
	assert.False(t, outcome.Success)
	assert.True(t, outcome.Records[0].Success)
	assert.Equal(t, assetOne.AssetID, outcome.Records[0].AssetID)
	assert.Equal(t, assetTwo.AssetID, outcome.FailedAssetID)
	assert.Contains(t, err.Error(), "second publish failed")
	history, histErr := gate.History(t.Context(), draft.ID)
	require.NoError(t, histErr)
	require.Len(t, history, 2)
	assert.Equal(t, assetTwo.AssetID, history[0].AssetID)
	assert.False(t, history[0].Success)
	assert.Equal(t, assetOne.AssetID, history[1].AssetID)
	assert.True(t, history[1].Success)
	updatedWorkspace, err := provider.WorkspaceRepo().GetByID(t.Context(), draft.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ArticleWorkspaceStatusApproved, updatedWorkspace.Status)
}

func TestPublishGateServicePreservesPublishRecordsWhenWorkspacePublishTransitionFails(t *testing.T) {
	provider := memory.NewProvider()
	reviewSvc := NewReviewService(provider.ReviewRepo(), provider.WorkspaceRepo())
	publisher := &publishProviderStub{result: &domain.PublishResult{Success: true, Platform: "wechat", Message: "published", Metadata: map[string]any{"remote_id": "wx-1"}}}
	publishRepo := &memoryBackedPublishRepoWithDelete{records: map[string]*domain.PublishRecord{}}
	gate := NewPublishGateService(provider.ReviewRepo(), provider.AssetRepo(), provider.DraftRepo(), publishRepo, failingWorkspaceRepoForReview{base: provider.WorkspaceRepo(), failOnStatus: domain.ArticleWorkspaceStatusPublished, err: errors.New("workspace publish transition failed")}, map[string]PublisherProvider{"wechat": publisher})

	draft := domain.NewArticleDraft("daily")
	draft.Headline["title"] = "Draft title"
	require.NoError(t, provider.DraftRepo().Create(t.Context(), draft))
	workspace := newRenderedWorkspaceRecord(draft.ID, "Draft title")
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))
	asset := domain.NewRenderedAssetRecord(draft.ID, "wechat", "html", "daily", "<html>one</html>", "")
	require.NoError(t, provider.AssetRepo().Create(t.Context(), asset))
	review, err := reviewSvc.CreateReview(t.Context(), draft.ID, []string{asset.AssetID}, "wechat-main")
	require.NoError(t, err)
	_, err = reviewSvc.ApproveReview(t.Context(), review.ID, "alice", "ok")
	require.NoError(t, err)

	outcome, err := gate.PublishReview(t.Context(), review.ID)
	require.Error(t, err)
	require.NotNil(t, outcome)
	assert.True(t, outcome.Success)
	assert.False(t, outcome.WorkspaceSynced)
	require.Len(t, outcome.Records, 1)
	history, histErr := publishRepo.ListByArticle(t.Context(), draft.ID)
	require.NoError(t, histErr)
	require.Len(t, history, 1)
	assert.True(t, history[0].Success)
	updatedWorkspace, wsErr := provider.WorkspaceRepo().GetByID(t.Context(), draft.ID)
	require.NoError(t, wsErr)
	assert.Equal(t, domain.ArticleWorkspaceStatusApproved, updatedWorkspace.Status)
	assert.Contains(t, err.Error(), "workspace publish transition failed")
}

func TestPublishGateServiceReturnsExplicitPartialPublishOutcome(t *testing.T) {
	provider := memory.NewProvider()
	reviewSvc := NewReviewService(provider.ReviewRepo(), provider.WorkspaceRepo())
	publisher := &publishProviderSequenceStub{results: []*domain.PublishResult{{Success: true, Platform: "wechat", Message: "published asset 1", Metadata: map[string]any{"remote_id": "wx-1"}}}, errAt: 1, err: errors.New("second publish failed")}
	gate := NewPublishGateService(provider.ReviewRepo(), provider.AssetRepo(), provider.DraftRepo(), provider.PublishRepo(), provider.WorkspaceRepo(), map[string]PublisherProvider{"wechat": publisher})

	draft := domain.NewArticleDraft("daily")
	draft.Headline["title"] = "Draft title"
	require.NoError(t, provider.DraftRepo().Create(t.Context(), draft))
	workspace := newRenderedWorkspaceRecord(draft.ID, "Draft title")
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))
	assetOne := domain.NewRenderedAssetRecord(draft.ID, "wechat", "html", "daily", "<html>one</html>", "")
	assetTwo := domain.NewRenderedAssetRecord(draft.ID, "wechat", "html", "daily", "<html>two</html>", "")
	require.NoError(t, provider.AssetRepo().Create(t.Context(), assetOne))
	require.NoError(t, provider.AssetRepo().Create(t.Context(), assetTwo))
	review, err := reviewSvc.CreateReview(t.Context(), draft.ID, []string{assetOne.AssetID, assetTwo.AssetID}, "wechat-main")
	require.NoError(t, err)
	_, err = reviewSvc.ApproveReview(t.Context(), review.ID, "alice", "ok")
	require.NoError(t, err)

	outcome, err := gate.PublishReview(t.Context(), review.ID)
	require.Error(t, err)
	require.NotNil(t, outcome)
	assert.True(t, outcome.Partial)
	assert.False(t, outcome.Success)
	assert.Len(t, outcome.Records, 1)
	assert.Equal(t, assetOne.AssetID, outcome.Records[0].AssetID)
	assert.Equal(t, assetTwo.AssetID, outcome.FailedAssetID)
}

type publishProviderStub struct {
	requests []domain.PublishRequest
	result   *domain.PublishResult
	err      error
}

func (s *publishProviderStub) Publish(_ context.Context, req domain.PublishRequest) (*domain.PublishResult, error) {
	s.requests = append(s.requests, req)
	if s.err != nil {
		return nil, s.err
	}
	if s.result != nil {
		return s.result, nil
	}
	return &domain.PublishResult{Success: true, Platform: req.Platform, Message: "published"}, nil
}

func (s *publishProviderStub) Platforms() []string {
	return []string{"wechat"}
}

type publishRepoRecordFailStub struct{ err error }

func (s *publishRepoRecordFailStub) Record(_ context.Context, r *domain.PublishRecord) error {
	return s.err
}

func (s *publishRepoRecordFailStub) ListByArticle(_ context.Context, articleID string) ([]domain.PublishRecord, error) {
	return nil, nil
}

func (s *publishRepoRecordFailStub) Delete(_ context.Context, id string) error {
	return nil
}

type publishProviderSequenceStub struct {
	results []*domain.PublishResult
	errAt   int
	err     error
	count   int
}

func (s *publishProviderSequenceStub) Publish(_ context.Context, req domain.PublishRequest) (*domain.PublishResult, error) {
	if s.count == s.errAt {
		s.count++
		return nil, s.err
	}
	result := s.results[s.count]
	s.count++
	if result == nil {
		return &domain.PublishResult{Success: true, Platform: req.Platform, Message: "published"}, nil
	}
	return result, nil
}

func (s *publishProviderSequenceStub) Platforms() []string {
	return []string{"wechat"}
}

func newRenderedWorkspaceRecord(id, title string) *domain.ArticleWorkspaceRecord {
	workspace := domain.NewArticleWorkspaceRecord(id, title, "", domain.ArticleWorkspaceSource{}, nil)
	workspace.Status = domain.ArticleWorkspaceStatusRendered
	workspace.StatusHistory = []string{domain.ArticleWorkspaceStatusImported, domain.ArticleWorkspaceStatusDraft, domain.ArticleWorkspaceStatusRendered}
	return workspace
}

func newApprovedWorkspaceRecord(id, title string) *domain.ArticleWorkspaceRecord {
	workspace := newRenderedWorkspaceRecord(id, title)
	workspace.Status = domain.ArticleWorkspaceStatusApproved
	workspace.StatusHistory = append(workspace.StatusHistory, domain.ArticleWorkspaceStatusReviewPending, domain.ArticleWorkspaceStatusApproved)
	return workspace
}

type failingWorkspaceRepoForReview struct {
	base         repo.WorkspaceRepo
	failOnStatus string
	err          error
	failed       *bool
}

func (r failingWorkspaceRepoForReview) Create(ctx context.Context, w *domain.ArticleWorkspaceRecord) error {
	return r.base.Create(ctx, w)
}

func (r failingWorkspaceRepoForReview) Update(ctx context.Context, w *domain.ArticleWorkspaceRecord) error {
	return r.base.Update(ctx, w)
}

func (r failingWorkspaceRepoForReview) GetByID(ctx context.Context, id string) (*domain.ArticleWorkspaceRecord, error) {
	return r.base.GetByID(ctx, id)
}

func (r failingWorkspaceRepoForReview) List(ctx context.Context, status *string) ([]domain.ArticleWorkspaceRecord, error) {
	return r.base.List(ctx, status)
}

func (r failingWorkspaceRepoForReview) ListByIngestionID(ctx context.Context, ingestionID string) ([]domain.ArticleWorkspaceRecord, error) {
	return r.base.ListByIngestionID(ctx, ingestionID)
}

func (r failingWorkspaceRepoForReview) TransitionStatus(ctx context.Context, id string, newStatus, notes string) error {
	if r.failed != nil && !*r.failed && newStatus == r.failOnStatus {
		*r.failed = true
		return r.err
	}
	if r.failed == nil && newStatus == r.failOnStatus {
		return r.err
	}
	return r.base.TransitionStatus(ctx, id, newStatus, notes)
}

func (r failingWorkspaceRepoForReview) Delete(ctx context.Context, id string) error {
	return r.base.Delete(ctx, id)
}

type reviewRepoWithDeleteFailure struct {
	base      repo.ReviewRepo
	deleteErr error
}

func (r reviewRepoWithDeleteFailure) Create(ctx context.Context, review *domain.ReviewTask) error {
	return r.base.Create(ctx, review)
}

func (r reviewRepoWithDeleteFailure) GetByID(ctx context.Context, id string) (*domain.ReviewTask, error) {
	return r.base.GetByID(ctx, id)
}

func (r reviewRepoWithDeleteFailure) ListByArticle(ctx context.Context, articleID string) ([]domain.ReviewTask, error) {
	return r.base.ListByArticle(ctx, articleID)
}

func (r reviewRepoWithDeleteFailure) UpdateStatus(ctx context.Context, id, status, reviewer, notes string) error {
	return r.base.UpdateStatus(ctx, id, status, reviewer, notes)
}

func (r reviewRepoWithDeleteFailure) Delete(ctx context.Context, id string) error {
	return r.deleteErr
}

type reviewRepoWithConditionalUpdateFailure struct {
	base       repo.ReviewRepo
	failStatus string
	updateErr  error
}

func (r reviewRepoWithConditionalUpdateFailure) Create(ctx context.Context, review *domain.ReviewTask) error {
	return r.base.Create(ctx, review)
}

func (r reviewRepoWithConditionalUpdateFailure) GetByID(ctx context.Context, id string) (*domain.ReviewTask, error) {
	return r.base.GetByID(ctx, id)
}

func (r reviewRepoWithConditionalUpdateFailure) ListByArticle(ctx context.Context, articleID string) ([]domain.ReviewTask, error) {
	return r.base.ListByArticle(ctx, articleID)
}

func (r reviewRepoWithConditionalUpdateFailure) UpdateStatus(ctx context.Context, id, status, reviewer, notes string) error {
	if status == r.failStatus {
		return r.updateErr
	}
	return r.base.UpdateStatus(ctx, id, status, reviewer, notes)
}

func (r reviewRepoWithConditionalUpdateFailure) Delete(ctx context.Context, id string) error {
	return r.base.Delete(ctx, id)
}

type memoryBackedPublishRepoWithDelete struct {
	records map[string]*domain.PublishRecord
}

func (r *memoryBackedPublishRepoWithDelete) Record(ctx context.Context, rec *domain.PublishRecord) error {
	if r.records == nil {
		r.records = map[string]*domain.PublishRecord{}
	}
	cp := *rec
	r.records[rec.ID] = &cp
	return nil
}

func (r *memoryBackedPublishRepoWithDelete) ListByArticle(ctx context.Context, articleID string) ([]domain.PublishRecord, error) {
	result := []domain.PublishRecord{}
	for _, record := range r.records {
		if record.ArticleID == articleID {
			result = append(result, *record)
		}
	}
	return result, nil
}

func (r *memoryBackedPublishRepoWithDelete) Delete(ctx context.Context, id string) error {
	if _, ok := r.records[id]; !ok {
		return domain.NewNotFoundErr("publish_record", id)
	}
	delete(r.records, id)
	return nil
}
