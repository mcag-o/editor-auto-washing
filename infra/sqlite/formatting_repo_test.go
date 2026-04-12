package sqlite

import (
	"content-hub/domain"
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFormattingProvider(t *testing.T) *Provider {
	t.Helper()
	dbPath := fmt.Sprintf("%s/test_formatting_%d.db", t.TempDir(), os.Getpid())
	provider, err := NewProvider(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	return provider
}

func TestDraftRepoCreateAndGetByID(t *testing.T) {
	provider := newFormattingProvider(t)
	draft := domain.NewArticleDraft("daily-intelligence")
	draft.Meta["title"] = "市场快讯"
	draft.Meta["digest"] = "摘要"
	draft.Meta["author"] = "编辑部"

	require.NoError(t, provider.DraftRepo().Create(context.Background(), draft))

	stored, err := provider.DraftRepo().GetByID(context.Background(), draft.ID)
	require.NoError(t, err)
	assert.Equal(t, draft.ID, stored.ID)
	assert.Equal(t, "市场快讯", stored.Meta["title"])
}

func TestAssetRepoCreateAndGetByID(t *testing.T) {
	provider := newFormattingProvider(t)
	asset := domain.NewRenderedAssetRecord("draft-1", "wechat", "html", "daily-intelligence", "<html></html>", "/tmp/out.html")
	asset.Metadata["artifact_path"] = "/tmp/out.html"
	asset.Metadata["warnings"] = []string{"meta.thumb_media_id is missing"}

	require.NoError(t, provider.AssetRepo().Create(context.Background(), asset))

	stored, err := provider.AssetRepo().GetByID(context.Background(), asset.AssetID)
	require.NoError(t, err)
	assert.Equal(t, asset.AssetID, stored.AssetID)
	assert.Equal(t, "/tmp/out.html", stored.Metadata["artifact_path"])
	assert.Equal(t, "draft-1", stored.ArticleID)
}
