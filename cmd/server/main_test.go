package main

import (
	"content-hub/domain"
	"content-hub/infra/config"
	"content-hub/infra/memory"
	"content-hub/service"
	httpserver "content-hub/transport/http"
	"errors"
	"fmt"
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
	workingDir := t.TempDir()
	t.Chdir(workingDir)
	configDir := filepath.Join(workingDir, "config")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.json"), []byte(`{
	  "llm": {
	    "default_profile": "default_openai",
	    "profiles": {
	      "default_openai": {
	        "provider": "openai",
	        "model": "gpt-4.1",
	        "temperature": 0.2,
	        "max_tokens": 4096,
	        "timeout_sec": 60
	      }
	    }
	  }
	}`), 0o644))

	originalBuild := buildRuntimeReposFn
	originalStandaloneBuild := buildStandaloneRuntimeReposFn
	originalNewServer := newHTTPServer
	defer func() {
		buildRuntimeReposFn = originalBuild
		buildStandaloneRuntimeReposFn = originalStandaloneBuild
		newHTTPServer = originalNewServer
	}()
	buildRuntimeReposFn = func(root string) (*service.RuntimeRepos, func() error, error) {
		provider := memory.NewProvider()
		return &service.RuntimeRepos{
			ArticleRepo: provider.ArticleRepo(), TemplateRepo: provider.TemplateRepo(), DraftRepo: provider.DraftRepo(), AssetRepo: provider.AssetRepo(), ReviewRepo: provider.ReviewRepo(), PublishRepo: provider.PublishRepo(), JobRepo: provider.JobRepo(), JobEventRepo: provider.JobEventRepo(), IngestionRepo: provider.IngestionRepo(), WorkspaceRepo: provider.WorkspaceRepo(), CollectorSourceRepo: provider.CollectorSourceRepo(), CollectorRunRepo: provider.CollectorRunRepo(), CollectorEntryRepo: provider.CollectorEntryRepo(), CollectorSchedulerRepo: provider.CollectorSchedulerRepo(), RenderedDir: t.TempDir(),
		}, func() error { return nil }, nil
	}
	buildStandaloneRuntimeReposFn = func(cfg config.Config) (*service.RuntimeRepos, func() error, error) {
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

func TestRunUsesStandaloneExternalConfigFallbackWhenWorkspaceConfigFails(t *testing.T) {
	workspaceRoot := t.TempDir()
	t.Setenv("CONTENT_HUB_WORKSPACE_ROOT", workspaceRoot)
	workingDir := t.TempDir()
	t.Chdir(workingDir)
	require.NoError(t, os.WriteFile(filepath.Join(workspaceRoot, "workspace.yaml"), []byte("name: [\n"), 0o644))

	configDir := filepath.Join(workingDir, "config")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.json"), []byte(`{
	  "llm": {
	    "default_profile": "external_profile",
	    "profiles": {
	      "external_profile": {
	        "provider": "openai",
	        "api_key_ref": "env.OPENAI_API_KEY",
	        "base_url_ref": "env.OPENAI_BASE_URL",
	        "model": "gpt-4.1",
	        "temperature": 0.2,
	        "max_tokens": 4096,
	        "timeout_sec": 60
	      }
	    }
	  }
	}`), 0o644))

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
		return failingServerRunner{err: fmt.Errorf("bind failed after standalone config load on %s", cfg.LLM.DefaultProfile)}
	}

	err := run()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "bind failed after standalone config load on external_profile")
	assert.NotContains(t, err.Error(), "load standalone config")
}

func TestRunReturnsStandaloneConfigLoadFailureWhenWorkspaceConfigFails(t *testing.T) {
	workspaceRoot := t.TempDir()
	t.Setenv("CONTENT_HUB_WORKSPACE_ROOT", workspaceRoot)
	workingDir := t.TempDir()
	t.Chdir(workingDir)
	require.NoError(t, os.WriteFile(filepath.Join(workspaceRoot, "workspace.yaml"), []byte("name: [\n"), 0o644))

	originalBuild := buildRuntimeReposFn
	originalNewServer := newHTTPServer
	defer func() {
		buildRuntimeReposFn = originalBuild
		newHTTPServer = originalNewServer
	}()

	err := run()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "load standalone config")
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestRunStandaloneFallbackRemainsAuthoritativeThroughRuntimeBootstrap(t *testing.T) {
	workspaceRoot := t.TempDir()
	t.Setenv("CONTENT_HUB_WORKSPACE_ROOT", workspaceRoot)
	workingDir := t.TempDir()
	t.Chdir(workingDir)
	t.Setenv("OPENAI_API_KEY", "standalone-key")
	t.Setenv("OPENAI_BASE_URL", "https://standalone.example.test")
	require.NoError(t, os.WriteFile(filepath.Join(workspaceRoot, "workspace.yaml"), []byte("name: [\n"), 0o644))

	configDir := filepath.Join(workingDir, "config")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.json"), []byte(`{
	  "database": {
	    "path": "./standalone/authoritative.db"
	  },
	  "template": {
	    "prompt_dir": "./standalone-prompts",
	    "default_prompt": "standalone-default"
	  },
	  "llm": {
	    "default_profile": "external_profile",
	    "profiles": {
	      "external_profile": {
	        "provider": "openai",
	        "api_key_ref": "env.OPENAI_API_KEY",
	        "base_url_ref": "env.OPENAI_BASE_URL",
	        "model": "gpt-4.1-mini",
	        "temperature": 0.25,
	        "max_tokens": 2048,
	        "timeout_sec": 45
	      }
	    }
	  }
	}`), 0o644))

	originalBuild := buildRuntimeReposFn
	originalStandaloneBuild := buildStandaloneRuntimeReposFn
	originalNewServer := newHTTPServer
	defer func() {
		buildRuntimeReposFn = originalBuild
		buildStandaloneRuntimeReposFn = originalStandaloneBuild
		newHTTPServer = originalNewServer
	}()
	buildRuntimeReposFn = func(root string) (*service.RuntimeRepos, func() error, error) {
		return nil, nil, fmt.Errorf("workspace runtime builder called for %s", root)
	}
	buildStandaloneRuntimeReposFn = func(cfg config.Config) (*service.RuntimeRepos, func() error, error) {
		return nil, nil, fmt.Errorf("standalone runtime bootstrap db=%s llm=%s prompt=%s", cfg.Database.Path, cfg.LLM.Model, cfg.Template.DefaultPrompt)
	}
	newHTTPServer = func(cfg config.Config, provider *httpserver.Provider) serverRunner {
		return failingServerRunner{err: fmt.Errorf("server received db=%s llm=%s prompt=%s", cfg.Database.Path, cfg.LLM.Model, cfg.Template.DefaultPrompt)}
	}

	err := run()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "standalone runtime bootstrap db=./standalone/authoritative.db llm=gpt-4.1-mini prompt=standalone-default")
	assert.NotContains(t, err.Error(), "workspace runtime builder called")
	assert.NotContains(t, err.Error(), "workspace_data")
	assert.NotContains(t, err.Error(), "daily-intelligence")
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
	automationSvc := service.NewAutomationService(workspaceConfigSvc, ingestionSvc, nil, nil)
	engine := service.BuildDefaultWorkflowEngine(t.TempDir(), automationSvc)

	assert.Contains(t, engine.RegisteredNames(), "automation_dispatch")
	assert.Contains(t, engine.RegisteredNames(), "automation_snapshot")
}

func TestRunBuildsWebControlProviderDependencies(t *testing.T) {
	workingDir := t.TempDir()
	t.Chdir(workingDir)
	configDir := filepath.Join(workingDir, "config")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.json"), []byte(`{
	  "llm": {
	    "default_profile": "default_openai",
	    "profiles": {
	      "default_openai": {
	        "provider": "openai",
	        "model": "gpt-4.1",
	        "temperature": 0.2,
	        "max_tokens": 4096,
	        "timeout_sec": 60
	      }
	    }
	  }
	}`), 0o644))

	originalBuild := buildRuntimeReposFn
	originalStandaloneBuild := buildStandaloneRuntimeReposFn
	originalNewServer := newHTTPServer
	defer func() {
		buildRuntimeReposFn = originalBuild
		buildStandaloneRuntimeReposFn = originalStandaloneBuild
		newHTTPServer = originalNewServer
	}()

	buildRuntimeReposFn = func(root string) (*service.RuntimeRepos, func() error, error) {
		provider := memory.NewProvider()
		return &service.RuntimeRepos{
			ArticleRepo:                provider.ArticleRepo(),
			TemplateRepo:               provider.TemplateRepo(),
			DraftRepo:                  provider.DraftRepo(),
			AssetRepo:                  provider.AssetRepo(),
			ReviewRepo:                 provider.ReviewRepo(),
			PublishRepo:                provider.PublishRepo(),
			JobRepo:                    provider.JobRepo(),
			JobEventRepo:               provider.JobEventRepo(),
			IngestionRepo:              provider.IngestionRepo(),
			WorkspaceRepo:              provider.WorkspaceRepo(),
			BundleImportTxStarter:      provider,
			CollectorSourceRepo:        provider.CollectorSourceRepo(),
			CollectorRunRepo:           provider.CollectorRunRepo(),
			CollectorEntryRepo:         provider.CollectorEntryRepo(),
			CollectorSchedulerRepo:     provider.CollectorSchedulerRepo(),
			RewritePipelineRunRepo:     provider.RewritePipelineRunRepo(),
			RewriteStageRunRepo:        provider.RewriteStageRunRepo(),
			SourceDocumentRepo:         &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}},
			BusinessConfigRepo:         &stubBusinessConfigRepo{},
			SystemControlStateRepo:     &stubSystemControlStateRepo{},
			AuditLogRepo:               &stubAuditLogRepo{},
			RenderedDir:                t.TempDir(),
		}, func() error { return nil }, nil
	}
	buildStandaloneRuntimeReposFn = func(cfg config.Config) (*service.RuntimeRepos, func() error, error) {
		provider := memory.NewProvider()
		return &service.RuntimeRepos{
			ArticleRepo:                provider.ArticleRepo(),
			TemplateRepo:               provider.TemplateRepo(),
			DraftRepo:                  provider.DraftRepo(),
			AssetRepo:                  provider.AssetRepo(),
			ReviewRepo:                 provider.ReviewRepo(),
			PublishRepo:                provider.PublishRepo(),
			JobRepo:                    provider.JobRepo(),
			JobEventRepo:               provider.JobEventRepo(),
			IngestionRepo:              provider.IngestionRepo(),
			WorkspaceRepo:              provider.WorkspaceRepo(),
			BundleImportTxStarter:      provider,
			CollectorSourceRepo:        provider.CollectorSourceRepo(),
			CollectorRunRepo:           provider.CollectorRunRepo(),
			CollectorEntryRepo:         provider.CollectorEntryRepo(),
			CollectorSchedulerRepo:     provider.CollectorSchedulerRepo(),
			RewritePipelineRunRepo:     provider.RewritePipelineRunRepo(),
			RewriteStageRunRepo:        provider.RewriteStageRunRepo(),
			SourceDocumentRepo:         &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}},
			BusinessConfigRepo:         &stubBusinessConfigRepo{},
			SystemControlStateRepo:     &stubSystemControlStateRepo{},
			AuditLogRepo:               &stubAuditLogRepo{},
			RenderedDir:                t.TempDir(),
		}, func() error { return nil }, nil
	}

	newHTTPServer = func(cfg config.Config, provider *httpserver.Provider) serverRunner {
		require.NotNil(t, provider.WebControlRuntime)
		require.NotNil(t, provider.SourceDocumentRepo)
		require.NotNil(t, provider.RewriteRunRepo)
		require.NotNil(t, provider.RewriteStageRepo)
		require.NotNil(t, provider.AuditLogRepo)
		return failingServerRunner{err: errors.New("bind failed")}
	}

	err := run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bind failed")
}

func TestRunDefaultsWebControlPlaneToPrimaryPort8123(t *testing.T) {
	workingDir := t.TempDir()
	t.Chdir(workingDir)
	configDir := filepath.Join(workingDir, "config")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.json"), []byte(`{
	  "llm": {
	    "default_profile": "default_openai",
	    "profiles": {
	      "default_openai": {
	        "provider": "openai",
	        "model": "gpt-4.1",
	        "temperature": 0.2,
	        "max_tokens": 4096,
	        "timeout_sec": 60
	      }
	    }
	  }
	}`), 0o644))

	originalBuild := buildRuntimeReposFn
	originalStandaloneBuild := buildStandaloneRuntimeReposFn
	originalNewServer := newHTTPServer
	defer func() {
		buildRuntimeReposFn = originalBuild
		buildStandaloneRuntimeReposFn = originalStandaloneBuild
		newHTTPServer = originalNewServer
	}()

	buildRuntimeReposFn = func(root string) (*service.RuntimeRepos, func() error, error) {
		provider := memory.NewProvider()
		return &service.RuntimeRepos{
			ArticleRepo:                provider.ArticleRepo(),
			TemplateRepo:               provider.TemplateRepo(),
			DraftRepo:                  provider.DraftRepo(),
			AssetRepo:                  provider.AssetRepo(),
			ReviewRepo:                 provider.ReviewRepo(),
			PublishRepo:                provider.PublishRepo(),
			JobRepo:                    provider.JobRepo(),
			JobEventRepo:               provider.JobEventRepo(),
			IngestionRepo:              provider.IngestionRepo(),
			WorkspaceRepo:              provider.WorkspaceRepo(),
			BundleImportTxStarter:      provider,
			CollectorSourceRepo:        provider.CollectorSourceRepo(),
			CollectorRunRepo:           provider.CollectorRunRepo(),
			CollectorEntryRepo:         provider.CollectorEntryRepo(),
			CollectorSchedulerRepo:     provider.CollectorSchedulerRepo(),
			RewritePipelineRunRepo:     provider.RewritePipelineRunRepo(),
			RewriteStageRunRepo:        provider.RewriteStageRunRepo(),
			SourceDocumentRepo:         &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}},
			BusinessConfigRepo:         &stubBusinessConfigRepo{},
			SystemControlStateRepo:     &stubSystemControlStateRepo{},
			AuditLogRepo:               &stubAuditLogRepo{},
			RenderedDir:                t.TempDir(),
		}, func() error { return nil }, nil
	}
	buildStandaloneRuntimeReposFn = func(cfg config.Config) (*service.RuntimeRepos, func() error, error) {
		provider := memory.NewProvider()
		return &service.RuntimeRepos{
			ArticleRepo:                provider.ArticleRepo(),
			TemplateRepo:               provider.TemplateRepo(),
			DraftRepo:                  provider.DraftRepo(),
			AssetRepo:                  provider.AssetRepo(),
			ReviewRepo:                 provider.ReviewRepo(),
			PublishRepo:                provider.PublishRepo(),
			JobRepo:                    provider.JobRepo(),
			JobEventRepo:               provider.JobEventRepo(),
			IngestionRepo:              provider.IngestionRepo(),
			WorkspaceRepo:              provider.WorkspaceRepo(),
			BundleImportTxStarter:      provider,
			CollectorSourceRepo:        provider.CollectorSourceRepo(),
			CollectorRunRepo:           provider.CollectorRunRepo(),
			CollectorEntryRepo:         provider.CollectorEntryRepo(),
			CollectorSchedulerRepo:     provider.CollectorSchedulerRepo(),
			RewritePipelineRunRepo:     provider.RewritePipelineRunRepo(),
			RewriteStageRunRepo:        provider.RewriteStageRunRepo(),
			SourceDocumentRepo:         &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}},
			BusinessConfigRepo:         &stubBusinessConfigRepo{},
			SystemControlStateRepo:     &stubSystemControlStateRepo{},
			AuditLogRepo:               &stubAuditLogRepo{},
			RenderedDir:                t.TempDir(),
		}, func() error { return nil }, nil
	}

	newHTTPServer = func(cfg config.Config, provider *httpserver.Provider) serverRunner {
		assert.Equal(t, 8123, cfg.HTTP.Port)
		return failingServerRunner{err: errors.New("bind failed")}
	}

	err := run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bind failed")
}

type failingServerRunner struct{ err error }

func (f failingServerRunner) Run() error { return f.err }
