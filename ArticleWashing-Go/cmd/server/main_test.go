package main

import (
	"content-hub/infra/config"
	"content-hub/infra/memory"
	"content-hub/service"
	httpserver "content-hub/transport/http"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunUsesWorkspaceEnvRootWhenPresent(t *testing.T) {
	t.Setenv("CONTENT_HUB_WORKSPACE_ROOT", "/tmp/workspace-root")

	assert.Equal(t, "/tmp/workspace-root", workspaceRootFromEnv())
}

func TestRunUsesCurrentDirectoryWorkspaceRootByDefault(t *testing.T) {
	os.Unsetenv("CONTENT_HUB_WORKSPACE_ROOT")

	assert.Equal(t, ".", workspaceRootFromEnv())
}

func TestRunPropagatesServerStartupFailure(t *testing.T) {
	originalBuild := buildRuntimeReposFn
	originalNewServer := newHTTPServer
	defer func() {
		buildRuntimeReposFn = originalBuild
		newHTTPServer = originalNewServer
	}()
	buildRuntimeReposFn = func(root string) (*service.RuntimeRepos, func() error, error) {
		provider := memory.NewProvider()
		return &service.RuntimeRepos{
			ArticleRepo: provider.ArticleRepo(), TemplateRepo: provider.TemplateRepo(), DraftRepo: provider.DraftRepo(), AssetRepo: provider.AssetRepo(), ReviewRepo: provider.ReviewRepo(), PublishRepo: provider.PublishRepo(), JobRepo: provider.JobRepo(), JobEventRepo: provider.JobEventRepo(), IngestionRepo: provider.IngestionRepo(), WorkspaceRepo: provider.WorkspaceRepo(), CollectorSourceRepo: provider.CollectorSourceRepo(), CollectorRunRepo: provider.CollectorRunRepo(), CollectorEntryRepo: provider.CollectorEntryRepo(), CollectorSchedulerRepo: provider.CollectorSchedulerRepo(), RenderedDir: t.TempDir(),
		}, func() error { return nil }, nil
	}
	newHTTPServer = func(cfg config.Config, provider *httpserver.Provider) serverRunner {
		return failingServerRunner{err: errors.New("bind failed")}
	}

	err := run()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "bind failed")
}

func TestBuildRuntimeReposExposesCollectorRepos(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "workspace.yaml"), []byte("name: runtime\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "secrets.yaml"), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))

	repos, cleanup, err := buildRuntimeRepos(root)
	require.NoError(t, err)
	defer cleanup()

	assert.NotNil(t, repos.CollectorSourceRepo)
	assert.NotNil(t, repos.CollectorRunRepo)
	assert.NotNil(t, repos.CollectorEntryRepo)
	assert.NotNil(t, repos.CollectorSchedulerRepo)
}

func TestRuntimeWorkflowEngineRegistersDefaultAutomationNodes(t *testing.T) {
	provider := memory.NewProvider()
	workspaceConfigSvc := service.NewWorkspaceConfigService(nil, nil)
	ingestionSvc := service.NewIngestionPipelineService(provider.IngestionRepo(), provider.WorkspaceRepo(), provider, nil)
	automationSvc := service.NewAutomationService(workspaceConfigSvc, ingestionSvc, nil)
	engine := service.BuildDefaultWorkflowEngine(t.TempDir(), automationSvc)

	assert.Contains(t, engine.RegisteredNames(), "automation_dispatch")
	assert.Contains(t, engine.RegisteredNames(), "automation_snapshot")
}

type failingServerRunner struct{ err error }

func (f failingServerRunner) Run() error { return f.err }
