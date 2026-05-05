package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildWebControlRuntimeReturnsReadyServices(t *testing.T) {
	repos, cleanup, err := BuildRuntimeRepos(t.TempDir())
	if cleanup != nil {
		defer func() {
			require.NoError(t, cleanup())
		}()
	}
	require.NoError(t, err)

	runtime, err := BuildWebControlRuntime(repos)

	require.NoError(t, err)
	require.NotNil(t, runtime)
	require.NotNil(t, runtime.Config)
	require.NotNil(t, runtime.Control)
	require.NotNil(t, runtime.Audit)
	require.NotNil(t, runtime.Intake)
	require.NotNil(t, runtime.Articles)
}

func TestBuildWebControlRuntimeRequiresRepos(t *testing.T) {
	runtime, err := BuildWebControlRuntime(nil)

	require.Nil(t, runtime)
	require.Error(t, err)
	require.ErrorContains(t, err, "web control runtime repos are required")
}
