package service_test

import (
	"content-hub/domain"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"content-hub/collector/httpclient"
	"content-hub/collector/plugin"
	"content-hub/collector/plugin/sources"
	collectorsvc "content-hub/collector/service"
	"content-hub/infra/config"
	"content-hub/pkg/repo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSourceRegistryHealthSurfacesAuthFailuresDistinctly(t *testing.T) {
	provider := newCollectorProvider(t)
	registry := plugin.NewRegistry()
	require.NoError(t, registry.Register(&stubSourcePlugin{
		sourceID:    "weibo",
		displayName: "微博热搜",
		health: plugin.SourceHealth{
			SourceID:  "weibo",
			OK:        false,
			Code:      plugin.HealthCodeAuthExpired,
			Message:   "cookie expired for env.WEIBO_COOKIE",
			CheckedAt: time.Now().UTC(),
		},
	}))

	svc := collectorsvc.NewSourceRegistryService(provider.CollectorSourceRepo(), registry)
	require.NoError(t, svc.Sync(t.Context()))

	statuses, err := svc.Health(t.Context())

	require.NoError(t, err)
	require.Len(t, statuses, 1)
	assert.False(t, statuses[0].Health.OK)
	assert.Equal(t, plugin.HealthCodeAuthExpired, statuses[0].Health.Code)
	assert.Contains(t, statuses[0].Health.Message, "cookie expired")
}

func TestSourceRegistryHealthMarksMissingPluginAsUnavailable(t *testing.T) {
	provider := newCollectorProvider(t)
	mustCreateSource(t, provider.CollectorSourceRepo(), "orphan", "Orphan Source")

	svc := collectorsvc.NewSourceRegistryService(provider.CollectorSourceRepo(), plugin.NewRegistry())

	statuses, err := svc.Health(t.Context())

	require.NoError(t, err)
	require.Len(t, statuses, 1)
	assert.Equal(t, plugin.HealthCodeUnavailable, statuses[0].Health.Code)
	assert.False(t, statuses[0].Health.OK)
}

func mustCreateSource(t *testing.T, sources repo.CollectorSourceRepo, id, displayName string) {
	t.Helper()
	source := domain.NewCollectorSource(id, displayName)
	require.NoError(t, sources.Create(t.Context(), source))
}

func TestSourceRegistryHealthPreservesUnavailableCodeFromPluginErrors(t *testing.T) {
	provider := newCollectorProvider(t)
	registry := plugin.NewRegistry()
	require.NoError(t, registry.Register(&stubSourcePlugin{
		sourceID:    "broken",
		displayName: "Broken Source",
		health: plugin.SourceHealth{
			SourceID:  "broken",
			OK:        false,
			Code:      plugin.HealthCodeUnavailable,
			CheckedAt: time.Now().UTC(),
		},
		healthErr: errors.New("upstream unavailable"),
	}))

	svc := collectorsvc.NewSourceRegistryService(provider.CollectorSourceRepo(), registry)
	require.NoError(t, svc.Sync(t.Context()))

	statuses, err := svc.Health(t.Context())

	require.NoError(t, err)
	require.Len(t, statuses, 1)
	assert.Equal(t, plugin.HealthCodeUnavailable, statuses[0].Health.Code)
	assert.Contains(t, statuses[0].Health.Message, "upstream unavailable")
}

func TestSourceRegistryHealthUsesPersistedCookieSecretRef(t *testing.T) {
	provider := newCollectorProvider(t)
	source := domain.NewCollectorSource("weibo", "微博热搜")
	source.AuthMode = domain.CollectorAuthModeCookie
	source.CookieSecretRef = "env.WEIBO_COOKIE_ALT"
	require.NoError(t, provider.CollectorSourceRepo().Create(t.Context(), source))

	pluginUnderTest := newRegistryWeiboPlugin(t, "SUB=alt-cookie", http.StatusOK, "weibo-hotlist.json")
	registry := plugin.NewRegistry()
	require.NoError(t, registry.Register(pluginUnderTest))

	svc := collectorsvc.NewSourceRegistryService(provider.CollectorSourceRepo(), registry)

	statuses, err := svc.Health(t.Context())

	require.NoError(t, err)
	require.Len(t, statuses, 1)
	assert.True(t, statuses[0].Health.OK)
	assert.Equal(t, plugin.HealthCodeHealthy, statuses[0].Health.Code)
	assert.Equal(t, "weibo", statuses[0].SourceID)
	assert.Equal(t, "微博热搜", statuses[0].DisplayName)
	assert.True(t, statuses[0].Enabled)
	assert.True(t, statuses[0].Capabilities.SupportsHotlist)
	assert.False(t, statuses[0].Capabilities.SupportsArticle)
	assert.Equal(t, []string{domain.CollectorAuthModeCookie}, statuses[0].Capabilities.AuthModes)
	assert.False(t, statuses[0].Health.CheckedAt.IsZero())
}

func TestSourceRegistrySyncPersistsPlaceholderMetadata(t *testing.T) {
	provider := newCollectorProvider(t)
	registry := plugin.NewRegistry()
	require.NoError(t, registry.Register(sources.NewPlaceholder(configStubSourceDefinition(), "zhihu")))

	svc := collectorsvc.NewSourceRegistryService(provider.CollectorSourceRepo(), registry)
	require.NoError(t, svc.Sync(t.Context()))

	stored, err := provider.CollectorSourceRepo().GetByID(t.Context(), "zhihu")
	require.NoError(t, err)
	assert.False(t, stored.Enabled)
	assert.Equal(t, domain.CollectorAuthModeNone, stored.AuthMode)
	assert.JSONEq(t, `{"source_type":"json-api","source_url":"https://www.zhihu.com/api/v3/explore/guest/feeds","status":"placeholder","goal":"补齐知乎热榜实现与后续详情正文抽取","placeholder_required":true,"supports_article":true}`, string(stored.OptionsJSON))
	assert.Contains(t, string(stored.HeadersJSON), "{}")
	assert.Contains(t, stored.Metadata["migration_reference"], "DataCollection/src/platforms/zhihu.js")
	assert.Contains(t, stored.Metadata["todo"], "实现列表字段标准化")
}

func newRegistryWeiboPlugin(t *testing.T, customCookie string, statusCode int, fixture string) plugin.SourcePlugin {
	t.Helper()
	client := newRegistryCookieClient(t, statusCode, fixture, customCookie)
	return sources.NewWeiboWithClient(client, "env.WEIBO_COOKIE", sources.SecretResolverFunc(func(ref string) (string, error) {
		switch ref {
		case "env.WEIBO_COOKIE_ALT":
			return customCookie, nil
		case "env.WEIBO_COOKIE":
			return "", nil
		default:
			return "", nil
		}
	}))
}

func newRegistryCookieClient(t *testing.T, statusCode int, fixture string, expectedCookie string) *httpclient.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if expectedCookie != "" {
			assert.Equal(t, expectedCookie, r.Header.Get("Cookie"))
		}
		body, err := os.ReadFile(filepath.Join("..", "..", "testdata", "collector", "fixtures", fixture))
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)

	client, err := httpclient.New(httpclient.Options{BaseURL: server.URL})
	require.NoError(t, err)
	return client
}

func configStubSourceDefinition() config.CollectorSourceDef {
	return config.CollectorSourceDef{
		DisplayName:         "知乎热榜",
		Aliases:             []string{"zhihu"},
		SourceType:          "json-api",
		SourceURL:           "https://www.zhihu.com/api/v3/explore/guest/feeds",
		Enabled:             false,
		ScheduleEnabled:     false,
		IntervalMinutes:     30,
		TimeoutMS:           10000,
		HotlistLimit:        50,
		DetailFetchEnabled:  false,
		Concurrency:         1,
		AuthMode:            domain.CollectorAuthModeNone,
		Status:              "placeholder",
		Goal:                "补齐知乎热榜实现与后续详情正文抽取",
		Todo:                []string{"实现列表字段标准化", "确认详情抓取接口或页面回源方式"},
		Notes:               []string{"适合作为下一批重点迁移平台之一。"},
		MigrationReference:  "DataCollection/src/platforms/zhihu.js",
		SupportsArticle:     true,
		PlaceholderRequired: true,
		Headers:             map[string]string{},
	}
}
