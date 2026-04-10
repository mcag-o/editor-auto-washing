package main

import (
	"content-hub/domain"
	workspaceinfra "content-hub/infra/workspace"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRuntimeReposUsesSQLiteWorkspaceDatabase(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceConfigFileName), []byte("name: runtime\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))

	repos, cleanup, err := buildRuntimeRepos(root)
	require.NoError(t, err)
	defer cleanup()

	assert.NotNil(t, repos.IngestionRepo)
	assert.NotNil(t, repos.WorkspaceRepo)
	assert.NotNil(t, repos.DraftRepo)
	assert.NotNil(t, repos.AssetRepo)
	assert.NotNil(t, repos.PublishRepo)
	assert.NotNil(t, repos.JobRepo)
	assert.NotNil(t, repos.JobEventRepo)
	assert.NotNil(t, repos.Formatter)
	assert.Equal(t, filepath.Join(root, "rendered"), repos.RenderedDir)
	assert.FileExists(t, filepath.Join(root, "workspace_data", "content-hub.db"))
	job := domain.NewJobRun("runtime-job")
	require.NoError(t, repos.JobRepo.Create(t.Context(), job))
	evt := domain.NewJobEvent(job.ID, "started", "runtime event")
	require.NoError(t, repos.JobEventRepo.Add(t.Context(), evt))
	storedJob, err := repos.JobRepo.GetByID(t.Context(), job.ID)
	require.NoError(t, err)
	assert.Equal(t, job.ID, storedJob.ID)
	events, err := repos.JobEventRepo.ListByJob(t.Context(), job.ID)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, evt.Message, events[0].Message)
}
