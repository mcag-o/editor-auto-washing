package service

import (
	collectorruntime "content-hub/collector/runtime"
	"content-hub/domain"
	"content-hub/infra/config"
	"content-hub/infra/memory"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildCollectorRuntime_ExposesTask4Services(t *testing.T) {
	provider := memory.NewProvider()
	repos := &RuntimeRepos{
		CollectorSourceRepo:    provider.CollectorSourceRepo(),
		CollectorRunRepo:       provider.CollectorRunRepo(),
		CollectorEntryRepo:     provider.CollectorEntryRepo(),
		CollectorArticleRepo:   provider.CollectorArticleRepo(),
		CollectorAttemptRepo:   provider.CollectorAttemptRepo(),
		CollectorSchedulerRepo: provider.CollectorSchedulerRepo(),
		WorkspaceRepo:          provider.WorkspaceRepo(),
	}

	runtime, err := BuildCollectorRuntime(t.Context(), repos, time.Minute)

	require.NoError(t, err)
	assert.NotNil(t, runtime.ArticleFetchService)
	assert.NotNil(t, runtime.BridgeService)
	assert.NotNil(t, runtime.RunService)
	assert.NotNil(t, runtime.RegistryService)
	assert.NotNil(t, runtime.SchedulerService)
}

func TestBuildCollectorRuntime_SyncsTwentyTwoCollectorSources(t *testing.T) {
	provider := memory.NewProvider()
	repos := &RuntimeRepos{
		CollectorSourceRepo:    provider.CollectorSourceRepo(),
		CollectorRunRepo:       provider.CollectorRunRepo(),
		CollectorEntryRepo:     provider.CollectorEntryRepo(),
		CollectorArticleRepo:   provider.CollectorArticleRepo(),
		CollectorAttemptRepo:   provider.CollectorAttemptRepo(),
		CollectorSchedulerRepo: provider.CollectorSchedulerRepo(),
		WorkspaceRepo:          provider.WorkspaceRepo(),
	}

	_, err := BuildCollectorRuntime(t.Context(), repos, time.Minute)
	require.NoError(t, err)

	sources, err := repos.CollectorSourceRepo.ListAll(t.Context())
	require.NoError(t, err)
	assert.Len(t, sources, 22)
	var sourceIDs []string
	for _, source := range sources {
		sourceIDs = append(sourceIDs, source.ID)
	}
	assert.Contains(t, sourceIDs, "zhihu")
	assert.Contains(t, sourceIDs, "xueqiu")
	assert.Contains(t, sourceIDs, "hackernews")
	assert.Contains(t, sourceIDs, "36kr")
}

func TestBuildCollectorRuntime_UsesConfigDrivenPoliciesForAllSources(t *testing.T) {
	provider := memory.NewProvider()
	repos := &RuntimeRepos{
		CollectorSourceRepo:    provider.CollectorSourceRepo(),
		CollectorRunRepo:       provider.CollectorRunRepo(),
		CollectorEntryRepo:     provider.CollectorEntryRepo(),
		CollectorArticleRepo:   provider.CollectorArticleRepo(),
		CollectorAttemptRepo:   provider.CollectorAttemptRepo(),
		CollectorSchedulerRepo: provider.CollectorSchedulerRepo(),
		WorkspaceRepo:          provider.WorkspaceRepo(),
	}

	runtime, err := BuildCollectorRuntime(t.Context(), repos, time.Minute)

	require.NoError(t, err)
	sources, err := repos.CollectorSourceRepo.ListAll(t.Context())
	require.NoError(t, err)
	assert.Len(t, sources, 22)
	for _, source := range sources {
		assert.NotEmpty(t, source.OptionsJSON)
		assert.NotEmpty(t, source.RetryPolicyJSON)
		assert.NotEqual(t, `{}`, string(source.OptionsJSON))
		assert.NotEqual(t, `{}`, string(source.RetryPolicyJSON))
	}
	assert.NotNil(t, runtime.RegistryService)
}

func TestBuildCollectorRuntime_RunServiceUsesResolvedRuntimeConfig(t *testing.T) {
	provider := memory.NewProvider()
	repos := &RuntimeRepos{
		CollectorSourceRepo:    provider.CollectorSourceRepo(),
		CollectorRunRepo:       provider.CollectorRunRepo(),
		CollectorEntryRepo:     provider.CollectorEntryRepo(),
		CollectorArticleRepo:   provider.CollectorArticleRepo(),
		CollectorAttemptRepo:   provider.CollectorAttemptRepo(),
		CollectorSchedulerRepo: provider.CollectorSchedulerRepo(),
		WorkspaceRepo:          provider.WorkspaceRepo(),
	}

	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		assert.Equal(t, "SUB=alt-cookie", r.Header.Get("Cookie"))
		assert.Equal(t, "from-plugin", r.Header.Get("X-Plugin-Header"))
		assert.Equal(t, "from-source", r.Header.Get("X-Source-Header"))
		if attempts == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		body, err := os.ReadFile(filepath.Join("..", "testdata", "collector", "fixtures", "weibo-hotlist.json"))
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)

	restoreConfig := overrideCollectorRuntimeConfig(t, func() config.Config {
		cfg := config.DefaultConfig()
		for sourceID, source := range cfg.Collector.Sources {
			source.Enabled = false
			cfg.Collector.Sources[sourceID] = source
		}
		cfg.Collector.HTTPClients["runtime_weibo_client"] = config.HTTPClientProfile{Headers: map[string]string{"X-Plugin-Header": "from-plugin"}}
		cfg.Collector.RetryPolicies["runtime_retry"] = config.RetryPolicyProfile{MaxAttempts: 2, BaseWaitMS: 1, MaxWaitMS: 1}
		weibo := cfg.Collector.Sources["weibo"]
		weibo.Enabled = true
		weibo.SourceURL = server.URL
		weibo.HTTPClient = "runtime_weibo_client"
		weibo.RetryPolicy = "runtime_retry"
		cfg.Collector.Sources["weibo"] = weibo
		return cfg
	})
	defer restoreConfig()
	restoreSecrets := overrideCollectorRuntimeSecrets(t, func() collectorruntime.SecretResolver {
		return collectorRuntimeSecretResolverStub{"env.WEIBO_COOKIE_ALT": "SUB=alt-cookie"}
	})
	defer restoreSecrets()

	runtime, err := BuildCollectorRuntime(t.Context(), repos, time.Minute)
	require.NoError(t, err)

	source, err := repos.CollectorSourceRepo.GetByID(t.Context(), "weibo")
	require.NoError(t, err)
	source.CookieSecretRef = "env.WEIBO_COOKIE_ALT"
	source.HeadersJSON = []byte(`{"X-Source-Header":"from-source"}`)
	require.NoError(t, repos.CollectorSourceRepo.Update(t.Context(), source))

	result, err := runtime.RunService.RunHotlist(t.Context(), "manual")

	require.NoError(t, err)
	detail, err := runtime.RunService.GetRun(t.Context(), result.RunID)
	require.NoError(t, err)
	assert.Equal(t, domain.CollectorRunSucceeded, result.Status)
	assert.Equal(t, 2, result.EntryCount)
	assert.Equal(t, 2, attempts)
	require.Len(t, detail.SourceRuns, 1)
	assert.Empty(t, detail.SourceRuns[0].ErrorMessage)
}

type collectorRuntimeSecretResolverStub map[string]string

func (s collectorRuntimeSecretResolverStub) Resolve(ref string) (string, error) {
	value, ok := s[ref]
	if !ok {
		return "", collectorruntime.ErrSecretNotFound(ref)
	}
	return value, nil
}

func overrideCollectorRuntimeConfig(t *testing.T, fn func() config.Config) func() {
	t.Helper()
	previous := buildCollectorRuntimeConfig
	buildCollectorRuntimeConfig = fn
	return func() { buildCollectorRuntimeConfig = previous }
}

func overrideCollectorRuntimeSecrets(t *testing.T, fn func() collectorruntime.SecretResolver) func() {
	t.Helper()
	previous := buildCollectorRuntimeSecretResolver
	buildCollectorRuntimeSecretResolver = fn
	return func() { buildCollectorRuntimeSecretResolver = previous }
}
