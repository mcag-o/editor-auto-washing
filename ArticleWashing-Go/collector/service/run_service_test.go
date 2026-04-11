package service_test

import (
	"content-hub/collector/httpclient"
	"content-hub/collector/plugin"
	"content-hub/collector/plugin/sources"
	collectorruntime "content-hub/collector/runtime"
	collectorsvc "content-hub/collector/service"
	"content-hub/domain"
	"content-hub/infra/config"
	"content-hub/infra/sqlite"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunService_RunHotlistCreatesRunSourceRunsAndEntries(t *testing.T) {
	provider := newCollectorProvider(t)
	registry := plugin.NewRegistry()
	require.NoError(t, registry.Register(&stubSourcePlugin{
		sourceID:    "stub-hotlist",
		displayName: "Stub Hotlist",
		hotlist: []plugin.HotEntry{
			{SourceID: "stub-hotlist", ExternalID: "entry-1", Title: "Entry One", CanonicalURL: "https://example.com/entry-1"},
			{SourceID: "stub-hotlist", ExternalID: "entry-2", Title: "Entry Two", CanonicalURL: "https://example.com/entry-2"},
		},
	}))

	registrySvc := collectorsvc.NewSourceRegistryService(provider.CollectorSourceRepo(), registry)
	require.NoError(t, registrySvc.Sync(t.Context()))

	runSvc := collectorsvc.NewRunService(provider.CollectorSourceRepo(), provider.CollectorRunRepo(), provider.CollectorEntryRepo(), registry)

	result, err := runSvc.RunHotlist(t.Context(), "manual")

	require.NoError(t, err)
	assert.Equal(t, "manual", result.Trigger)
	assert.Equal(t, domain.CollectorRunSucceeded, result.Status)
	assert.Equal(t, 1, result.SourceCount)
	assert.Equal(t, 2, result.EntryCount)
	assert.Equal(t, 1, result.SuccessfulSources)
	assert.Zero(t, result.FailedSources)

	runs, err := runSvc.ListRuns(t.Context(), 10)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, result.RunID, runs[0].ID)
	assert.Equal(t, domain.CollectorRunSucceeded, runs[0].Status)

	detail, err := runSvc.GetRun(t.Context(), result.RunID)
	require.NoError(t, err)
	assert.Equal(t, result.RunID, detail.Run.ID)
	require.Len(t, detail.SourceRuns, 1)
	assert.Equal(t, domain.CollectorSourceRunSucceeded, detail.SourceRuns[0].Status)
	assert.Equal(t, 2, detail.SourceRuns[0].DiscoveredCount)
	assert.Equal(t, 2, detail.SourceRuns[0].StoredCount)

	entries, err := provider.CollectorEntryRepo().ListByRunID(t.Context(), result.RunID)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, domain.CollectorEntryPendingDetail, entries[0].Status)
}

func TestRunService_UsesPersistedCookieSecretRefForSourcePlugin(t *testing.T) {
	provider := newCollectorProvider(t)
	source := domain.NewCollectorSource("weibo", "微博热搜")
	source.AuthMode = domain.CollectorAuthModeCookie
	source.CookieSecretRef = "env.WEIBO_COOKIE_ALT"
	source.HeadersJSON = []byte(`{"X-Source-Header":"from-source"}`)
	require.NoError(t, provider.CollectorSourceRepo().Create(t.Context(), source))

	registry := plugin.NewRegistry()
	require.NoError(t, registry.Register(newRunServiceWeiboPlugin(t, "SUB=alt-cookie", "weibo-hotlist.json", map[string]string{"X-Plugin-Header": "from-plugin"})))

	cfg := config.DefaultConfig()
	runSvc := collectorsvc.NewRunServiceWithRuntime(provider.CollectorSourceRepo(), provider.CollectorRunRepo(), provider.CollectorEntryRepo(), registry, cfg, secretResolverStub{"env.WEIBO_COOKIE_ALT": "SUB=alt-cookie"})

	result, err := runSvc.RunHotlist(t.Context(), "manual")

	require.NoError(t, err)
	assert.Equal(t, domain.CollectorRunSucceeded, result.Status)
	assert.Equal(t, 2, result.EntryCount)

	entries, err := provider.CollectorEntryRepo().ListByRunID(t.Context(), result.RunID)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "Mars colony launch date set", entries[0].Title)
}

func TestRunService_FailsWhenRuntimeAuthSecretMissing(t *testing.T) {
	provider := newCollectorProvider(t)
	source := domain.NewCollectorSource("weibo", "微博热搜")
	source.AuthMode = domain.CollectorAuthModeCookie
	source.CookieSecretRef = "env.WEIBO_COOKIE_ALT"
	require.NoError(t, provider.CollectorSourceRepo().Create(t.Context(), source))

	registry := plugin.NewRegistry()
	require.NoError(t, registry.Register(newRunServiceWeiboPlugin(t, "SUB=alt-cookie", "weibo-hotlist.json", map[string]string{"X-Plugin-Header": "from-plugin"})))

	cfg := config.DefaultConfig()
	runSvc := collectorsvc.NewRunServiceWithRuntime(provider.CollectorSourceRepo(), provider.CollectorRunRepo(), provider.CollectorEntryRepo(), registry, cfg, secretResolverStub{})

	result, err := runSvc.RunHotlist(t.Context(), "manual")

	require.NoError(t, err)
	assert.Equal(t, domain.CollectorRunFailed, result.Status)
	assert.Equal(t, 0, result.EntryCount)

	detail, err := runSvc.GetRun(t.Context(), result.RunID)
	require.NoError(t, err)
	require.Len(t, detail.SourceRuns, 1)
	assert.Contains(t, detail.SourceRuns[0].ErrorMessage, "env.WEIBO_COOKIE_ALT")
	assert.Contains(t, detail.SourceRuns[0].ErrorMessage, "not found")
	assert.Equal(t, domain.CollectorSourceRunFailed, detail.SourceRuns[0].Status)
}

type secretResolverStub map[string]string

func (s secretResolverStub) Resolve(ref string) (string, error) {
	value, ok := s[ref]
	if !ok {
		return "", collectorruntime.ErrSecretNotFound(ref)
	}
	return value, nil
}

func newRunServiceWeiboPlugin(t *testing.T, expectedCookie string, fixture string, defaultHeaders map[string]string) plugin.SourcePlugin {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, expectedCookie, r.Header.Get("Cookie"))
		for key, value := range defaultHeaders {
			assert.Equal(t, value, r.Header.Get(key))
		}
		assert.Equal(t, "from-source", r.Header.Get("X-Source-Header"))
		body, err := os.ReadFile(filepath.Join("..", "..", "testdata", "collector", "fixtures", fixture))
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)

	client, err := httpclient.New(httpclient.Options{BaseURL: server.URL, DefaultHeaders: defaultHeaders})
	require.NoError(t, err)
	return sources.NewWeiboWithClient(client)
}

func newCollectorProvider(t *testing.T) *sqlite.Provider {
	t.Helper()
	dbPath := fmt.Sprintf("%s/test_collector_service_%d.db", t.TempDir(), os.Getpid())
	provider, err := sqlite.NewProvider(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	return provider
}

type stubSourcePlugin struct {
	sourceID    string
	displayName string
	hotlist     []plugin.HotEntry
	health      plugin.SourceHealth
	healthErr   error
}

func (p *stubSourcePlugin) SourceID() string { return p.sourceID }

func (p *stubSourcePlugin) DisplayName() string { return p.displayName }

func (p *stubSourcePlugin) Aliases() []string { return nil }

func (p *stubSourcePlugin) FetchHotlist(_ context.Context, _ plugin.FetchHotlistRequest) ([]plugin.HotEntry, error) {
	return append([]plugin.HotEntry(nil), p.hotlist...), nil
}

func (p *stubSourcePlugin) FetchArticle(_ context.Context, _ plugin.FetchArticleRequest) (*plugin.RawArticle, error) {
	return nil, plugin.ErrArticleFetchNotSupported(p.sourceID)
}

func (p *stubSourcePlugin) NormalizeHotEntry(raw any) (plugin.HotEntry, error) {
	entry, _ := raw.(plugin.HotEntry)
	return entry, nil
}

func (p *stubSourcePlugin) NormalizeArticle(any) (*plugin.NormalizedArticle, error) {
	return nil, plugin.ErrArticleFetchNotSupported(p.sourceID)
}

func (p *stubSourcePlugin) HealthCheck(_ context.Context) (plugin.SourceHealth, error) {
	health := p.health
	if health.SourceID == "" {
		health.SourceID = p.sourceID
	}
	if health.CheckedAt.IsZero() {
		health.CheckedAt = time.Now().UTC()
	}
	if health.Code == "" {
		if p.healthErr == nil {
			health.OK = true
			health.Code = plugin.HealthCodeHealthy
		} else {
			health.Code = plugin.HealthCodeUnavailable
		}
	}
	if p.healthErr != nil {
		health.OK = false
		health.Message = p.healthErr.Error()
	}
	return health, p.healthErr
}

func (p *stubSourcePlugin) Capabilities() plugin.SourceCapabilities {
	return plugin.SourceCapabilities{SupportsHotlist: true, SupportsArticle: false, AuthModes: []string{domain.CollectorAuthModeNone}}
}
