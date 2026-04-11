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
	collectorruntime "content-hub/collector/runtime"
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
	source.HeadersJSON = []byte(`{"X-Source-Header":"from-source"}`)
	require.NoError(t, provider.CollectorSourceRepo().Create(t.Context(), source))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "SUB=alt-cookie", r.Header.Get("Cookie"))
		assert.Equal(t, "from-plugin", r.Header.Get("X-Plugin-Header"))
		assert.Equal(t, "from-source", r.Header.Get("X-Source-Header"))
		body, err := os.ReadFile(filepath.Join("..", "..", "testdata", "collector", "fixtures", "weibo-hotlist.json"))
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)

	pluginUnderTest := newRegistryWeiboPluginWithBaseURL(t, "http://127.0.0.1:1")
	registry := plugin.NewRegistry()
	require.NoError(t, registry.Register(pluginUnderTest))

	cfg := config.DefaultConfig()
	cfg.Collector.HTTPClients["runtime_weibo_client"] = config.HTTPClientProfile{Headers: map[string]string{"X-Plugin-Header": "from-plugin"}}
	weibo := cfg.Collector.Sources["weibo"]
	weibo.SourceURL = server.URL
	weibo.HTTPClient = "runtime_weibo_client"
	cfg.Collector.Sources["weibo"] = weibo
	svc := collectorsvc.NewSourceRegistryServiceWithRuntime(provider.CollectorSourceRepo(), registry, cfg, registrySecretResolverStub{"env.WEIBO_COOKIE_ALT": "SUB=alt-cookie"})

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

func TestSourceRegistryHealthFailsWhenRuntimeAuthSecretMissing(t *testing.T) {
	provider := newCollectorProvider(t)
	source := domain.NewCollectorSource("weibo", "微博热搜")
	source.AuthMode = domain.CollectorAuthModeCookie
	source.CookieSecretRef = "env.WEIBO_COOKIE_ALT"
	require.NoError(t, provider.CollectorSourceRepo().Create(t.Context(), source))

	registry := plugin.NewRegistry()
	require.NoError(t, registry.Register(newRegistryWeiboPlugin(t, "SUB=alt-cookie", http.StatusOK, "weibo-hotlist.json", map[string]string{"X-Plugin-Header": "from-plugin"})))

	cfg := config.DefaultConfig()
	svc := collectorsvc.NewSourceRegistryServiceWithRuntime(provider.CollectorSourceRepo(), registry, cfg, registrySecretResolverStub{})

	statuses, err := svc.Health(t.Context())

	require.NoError(t, err)
	require.Len(t, statuses, 1)
	assert.False(t, statuses[0].Health.OK)
	assert.Equal(t, plugin.HealthCodeUnavailable, statuses[0].Health.Code)
	assert.Contains(t, statuses[0].Health.Message, "env.WEIBO_COOKIE_ALT")
	assert.Contains(t, statuses[0].Health.Message, "not found")
}

func TestSourceRegistryHealthUsesRuntimeConfiguredClientForNonAuthSources(t *testing.T) {
	provider := newCollectorProvider(t)
	source := domain.NewCollectorSource("baidu", "百度热搜")
	source.Enabled = true
	require.NoError(t, provider.CollectorSourceRepo().Create(t.Context(), source))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"cards":[{"content":[{"word":"OpenAI","query":"AI","url":"https://example.com/openai","hotScore":"123","rank":1,"newsId":"n1"}]}]}}`))
	}))
	t.Cleanup(server.Close)

	registry := plugin.NewRegistry()
	require.NoError(t, registry.Register(newRegistryBaiduPluginWithBaseURL(t, "http://127.0.0.1:1")))

	cfg := config.DefaultConfig()
	baidu := cfg.Collector.Sources["baidu"]
	baidu.SourceURL = server.URL + "/api/board?platform=wise&tab=realtime"
	cfg.Collector.Sources["baidu"] = baidu

	svc := collectorsvc.NewSourceRegistryServiceWithRuntime(provider.CollectorSourceRepo(), registry, cfg, registrySecretResolverStub{})

	statuses, err := svc.Health(t.Context())

	require.NoError(t, err)
	require.Len(t, statuses, 1)
	assert.True(t, statuses[0].Health.OK)
	assert.Equal(t, plugin.HealthCodeHealthy, statuses[0].Health.Code)
}

type registrySecretResolverStub map[string]string

func (s registrySecretResolverStub) Resolve(ref string) (string, error) {
	value, ok := s[ref]
	if !ok {
		return "", collectorruntime.ErrSecretNotFound(ref)
	}
	return value, nil
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

func TestSourceRegistrySyncPersistsResolvedRetryMetadata(t *testing.T) {
	provider := newCollectorProvider(t)
	registry, err := collectorsvc.NewRegistryFromCollectorConfig(config.DefaultConfig().Collector)
	require.NoError(t, err)

	svc := collectorsvc.NewSourceRegistryService(provider.CollectorSourceRepo(), registry)
	require.NoError(t, svc.Sync(t.Context()))

	stored, err := provider.CollectorSourceRepo().GetByID(t.Context(), "zhihu")
	require.NoError(t, err)
	assert.JSONEq(t, `{"max_attempts":3,"wait":"500ms","max_wait":"5s"}`, string(stored.RetryPolicyJSON))
	assert.JSONEq(t, `{"detail_fetch_enabled":false,"goal":"补齐知乎热榜实现与后续详情正文抽取","placeholder_required":true,"source_type":"json-api","source_url":"https://www.zhihu.com/api/v3/explore/guest/feeds?limit=30&ws_qiangzhisafe=0","status":"placeholder","supports_article":true}`, string(stored.OptionsJSON))
}

func TestSourceRegistrySyncPersistsResolvedAuthModeFromAuthProfile(t *testing.T) {
	provider := newCollectorProvider(t)
	cfg := config.DefaultConfig().Collector
	cfg.AuthProfiles["runtime_header"] = config.AuthProfileConfig{Mode: domain.CollectorAuthModeHeader, HeaderName: "Authorization", HeaderValuePrefix: "Bearer "}
	zhihu := cfg.Sources["zhihu"]
	zhihu.AuthProfile = "runtime_header"
	zhihu.AuthMode = ""
	zhihu.HeaderSecretRef = "env.ZHIHU_TOKEN"
	cfg.Sources["zhihu"] = zhihu

	registry, err := collectorsvc.NewRegistryFromCollectorConfig(cfg)
	require.NoError(t, err)

	svc := collectorsvc.NewSourceRegistryService(provider.CollectorSourceRepo(), registry)
	require.NoError(t, svc.Sync(t.Context()))

	stored, err := provider.CollectorSourceRepo().GetByID(t.Context(), "zhihu")
	require.NoError(t, err)
	assert.Equal(t, domain.CollectorAuthModeHeader, stored.AuthMode)
	assert.Equal(t, "env.ZHIHU_TOKEN", stored.HeaderSecretRef)
	assert.Empty(t, stored.CookieSecretRef)
}

func newRegistryWeiboPlugin(t *testing.T, customCookie string, statusCode int, fixture string, defaultHeaders map[string]string) plugin.SourcePlugin {
	t.Helper()
	client := newRegistryCookieClient(t, statusCode, fixture, customCookie, defaultHeaders)
	return sources.NewWeiboWithClient(client)
}

func newRegistryWeiboPluginWithBaseURL(t *testing.T, baseURL string) plugin.SourcePlugin {
	t.Helper()
	client, err := httpclient.New(httpclient.Options{BaseURL: baseURL})
	require.NoError(t, err)
	return sources.NewWeiboWithClient(client)
}

func newRegistryBaiduPluginWithBaseURL(t *testing.T, baseURL string) plugin.SourcePlugin {
	t.Helper()
	client, err := httpclient.New(httpclient.Options{BaseURL: baseURL})
	require.NoError(t, err)
	return sources.NewBaiduWithClient(client)
}

func newRegistryCookieClient(t *testing.T, statusCode int, fixture string, expectedCookie string, defaultHeaders map[string]string) *httpclient.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if expectedCookie != "" {
			assert.Equal(t, expectedCookie, r.Header.Get("Cookie"))
		}
		for key, value := range defaultHeaders {
			assert.Equal(t, value, r.Header.Get(key))
		}
		assert.Equal(t, "from-source", r.Header.Get("X-Source-Header"))
		body, err := os.ReadFile(filepath.Join("..", "..", "testdata", "collector", "fixtures", fixture))
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)

	client, err := httpclient.New(httpclient.Options{BaseURL: server.URL, DefaultHeaders: defaultHeaders})
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
