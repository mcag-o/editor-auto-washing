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
