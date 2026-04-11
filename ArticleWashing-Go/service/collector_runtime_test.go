package service

import (
	"content-hub/infra/memory"
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
