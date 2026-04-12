package integration

import (
	"content-hub/domain"
	"content-hub/infra/sqlite"
	"content-hub/service"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewPublishMainline(t *testing.T) {
	root := t.TempDir()
	provider, err := sqlite.NewProvider(filepath.Join(root, "content-hub.db"))
	require.NoError(t, err)
	defer provider.Close()

	draft := domain.NewArticleDraft("daily")
	draft.Meta["title"] = "Mainline Draft"
	require.NoError(t, provider.DraftRepo().Create(t.Context(), draft))
	workspace := domain.NewArticleWorkspaceRecord(draft.ID, "Mainline Draft", "", domain.ArticleWorkspaceSource{}, nil)
	workspace.Status = domain.ArticleWorkspaceStatusRendered
	workspace.StatusHistory = []string{domain.ArticleWorkspaceStatusImported, domain.ArticleWorkspaceStatusDraft, domain.ArticleWorkspaceStatusRendered}
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))
	asset := domain.NewRenderedAssetRecord(draft.ID, "wechat", "html", "daily", "<html><body>mainline</body></html>", filepath.Join(root, "rendered", "draft.html"))
	asset.Status = domain.AssetStatusReady
	asset.Metadata["artifact_path"] = asset.ArtifactPath
	require.NoError(t, os.MkdirAll(filepath.Dir(asset.ArtifactPath), 0o755))
	require.NoError(t, os.WriteFile(asset.ArtifactPath, []byte(asset.Content), 0o644))
	require.NoError(t, provider.AssetRepo().Create(t.Context(), asset))

	reviewSvc := service.NewReviewService(provider.ReviewRepo(), provider.WorkspaceRepo())
	review, err := reviewSvc.CreateReview(t.Context(), draft.ID, []string{asset.AssetID}, "wechat-main")
	require.NoError(t, err)
	_, err = reviewSvc.ApproveReview(t.Context(), review.ID, "alice", "approved")
	require.NoError(t, err)

	gate := service.NewPublishGateService(provider.ReviewRepo(), provider.AssetRepo(), provider.DraftRepo(), provider.PublishRepo(), provider.WorkspaceRepo(), map[string]service.PublisherProvider{"wechat": integrationPublishProvider{}})
	outcome, err := gate.PublishReview(t.Context(), review.ID)
	require.NoError(t, err)
	require.NotNil(t, outcome)
	require.Len(t, outcome.Records, 1)
	assert.True(t, outcome.Success)
	assert.True(t, outcome.Records[0].Success)

	history, err := provider.PublishRepo().ListByArticle(t.Context(), draft.ID)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, review.ID, history[0].ReviewID)
	assert.Equal(t, asset.AssetID, history[0].AssetID)
	assert.Equal(t, "integration-remote", history[0].Metadata["remote_id"])
}

type integrationPublishProvider struct{}

func (integrationPublishProvider) Publish(_ context.Context, req domain.PublishRequest) (*domain.PublishResult, error) {
	return &domain.PublishResult{Success: true, Platform: req.Platform, Message: fmt.Sprintf("published %s", req.Title), Metadata: map[string]any{"remote_id": "integration-remote"}}, nil
}

func (integrationPublishProvider) Platforms() []string { return []string{"wechat"} }
