package integration

import (
	"content-hub/collector/httpclient"
	"content-hub/collector/plugin"
	"content-hub/collector/plugin/sources"
	collectorsvc "content-hub/collector/service"
	"content-hub/domain"
	"content-hub/infra/sqlite"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectorHotlistMainline_RunOncePersistsEntries(t *testing.T) {
	provider, err := sqlite.NewProvider(filepath.Join(t.TempDir(), "content-hub.db"))
	require.NoError(t, err)
	defer provider.Close()

	registry := plugin.NewRegistry()
	require.NoError(t, registry.Register(newIntegrationHotlistBaiduPlugin(t)))

	registrySvc := collectorsvc.NewSourceRegistryService(provider.CollectorSourceRepo(), registry)
	require.NoError(t, registrySvc.Sync(t.Context()))

	runSvc := collectorsvc.NewRunService(provider.CollectorSourceRepo(), provider.CollectorRunRepo(), provider.CollectorEntryRepo(), registry)
	result, err := runSvc.RunHotlist(t.Context(), "scheduled")
	require.NoError(t, err)

	assert.Equal(t, domain.CollectorRunSucceeded, result.Status)
	assert.Equal(t, 1, result.SourceCount)
	assert.Equal(t, 1, result.SuccessfulSources)
	assert.Zero(t, result.FailedSources)
	assert.Equal(t, 2, result.EntryCount)

	detail, err := runSvc.GetRun(t.Context(), result.RunID)
	require.NoError(t, err)
	require.Len(t, detail.SourceRuns, 1)
	assert.Equal(t, domain.CollectorSourceRunSucceeded, detail.SourceRuns[0].Status)
	assert.Equal(t, domain.CollectorStageHotlist, detail.SourceRuns[0].Stage)
	assert.Equal(t, 2, detail.SourceRuns[0].DiscoveredCount)
	assert.Equal(t, 2, detail.SourceRuns[0].StoredCount)

	entries, err := provider.CollectorEntryRepo().ListByRunID(t.Context(), result.RunID)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "baidu", entries[0].SourceID)
	assert.Equal(t, domain.CollectorEntryPendingDetail, entries[0].Status)
	assert.NotEmpty(t, entries[0].NormalizedJSON)
	assert.NotEmpty(t, entries[0].RawJSON)
	assert.Equal(t, "7492239302142358563", entries[0].ExternalID)
	assert.Equal(t, "SpaceX completes third orbital refueling test", entries[0].Title)
}

func newIntegrationHotlistBaiduPlugin(t *testing.T) plugin.SourcePlugin {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RequestURI() != "/api/board?platform=wise&tab=realtime" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, err := os.ReadFile(filepath.Join("..", "testdata", "collector", "fixtures", "baidu-hotlist.json"))
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)

	client, err := httpclient.New(httpclient.Options{BaseURL: server.URL})
	require.NoError(t, err)
	return sources.NewBaiduWithClient(client)
}
