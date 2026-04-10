package integration

import (
	collectorsvc "content-hub/collector/service"
	"content-hub/domain"
	"content-hub/infra/sqlite"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectorBridgeMainline(t *testing.T) {
	provider, err := sqlite.NewProvider(filepath.Join(t.TempDir(), "content-hub.db"))
	require.NoError(t, err)
	defer provider.Close()

	source := domain.NewCollectorSource("baidu", "百度热搜")
	require.NoError(t, provider.CollectorSourceRepo().Create(t.Context(), source))
	run := domain.NewCollectorRun("manual")
	require.NoError(t, provider.CollectorRunRepo().Create(t.Context(), run))
	entry := domain.NewCollectorEntry(run.ID, source.ID, "7492239302142358563", "SpaceX completes third orbital refueling test", "https://top.baidu.com/board?tab=realtime")
	require.NoError(t, provider.CollectorEntryRepo().Create(t.Context(), entry))

	article := domain.NewCollectorArticle(entry.ID, run.ID, source.ID, "7492239302142358563", "SpaceX completes third orbital refueling test", "https://top.baidu.com/board?tab=realtime")
	article.Summary = "Mission milestone summary"
	article.Body = "Long-form normalized article body"
	article.MetadataJSON, err = json.Marshal(map[string]any{"collector": map[string]any{"stage": "detail"}})
	require.NoError(t, err)
	require.NoError(t, provider.CollectorArticleRepo().Create(t.Context(), article))

	bridge := collectorsvc.NewBridgeService(provider.CollectorArticleRepo(), provider.WorkspaceRepo())
	first, err := bridge.PushToWorkspace(t.Context(), article.ID)
	require.NoError(t, err)
	second, err := bridge.PushToWorkspace(t.Context(), article.ID)
	require.NoError(t, err)

	assert.Equal(t, first.WorkspaceArticleID, second.WorkspaceArticleID)
	assert.Equal(t, article.ID, first.CollectorArticleID)
	workspace, err := provider.WorkspaceRepo().GetByID(t.Context(), first.WorkspaceArticleID)
	require.NoError(t, err)
	assert.Equal(t, article.Title, workspace.Title)
	assert.Equal(t, article.Summary, workspace.Summary)
	assert.Equal(t, "collector", workspace.Source.SourceType)
	assert.Equal(t, article.SourceID, workspace.Source.Platform)
	assert.Equal(t, article.CanonicalURL, workspace.Source.URL)
	assert.Equal(t, article.ID, workspace.Metadata["collector_article_id"])
	assert.Equal(t, article.EntryID, workspace.Metadata["collector_entry_id"])
	assert.Equal(t, article.RunID, workspace.Metadata["collector_run_id"])
	assert.Equal(t, article.SourceID, workspace.Metadata["collector_source_id"])

	stored, err := provider.CollectorArticleRepo().GetByID(t.Context(), article.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.CollectorArticleBridgeSucceeded, stored.BridgeStatus)
	assert.Equal(t, first.WorkspaceArticleID, stored.WorkspaceID)

	workspaceItems, err := provider.WorkspaceRepo().List(t.Context(), nil)
	require.NoError(t, err)
	assert.Len(t, workspaceItems, 1)
}
