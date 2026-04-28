package service

import (
	"content-hub/domain"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildFolderIntakeRuntimeFailsWhenConfigMissing(t *testing.T) {
	repos, cleanup, err := BuildRuntimeRepos(t.TempDir())
	if cleanup != nil {
		defer func() {
			if closeErr := cleanup(); closeErr != nil {
				t.Fatalf("cleanup returned error: %v", closeErr)
			}
		}()
	}
	if err != nil {
		t.Fatalf("BuildRuntimeRepos error: %v", err)
	}

	runtime, err := BuildFolderIntakeRuntime(repos, FolderIntakeConfig{})
	if err == nil {
		t.Fatal("expected BuildFolderIntakeRuntime to reject missing folder intake config")
	}
	if runtime != nil {
		t.Fatal("expected nil runtime when folder intake config is missing")
	}
}

func TestBuildFolderIntakeRuntimeReturnsReadyServicesWithExplicitConfig(t *testing.T) {
	repos, cleanup, err := BuildRuntimeRepos(t.TempDir())
	if cleanup != nil {
		defer func() {
			if closeErr := cleanup(); closeErr != nil {
				t.Fatalf("cleanup returned error: %v", closeErr)
			}
		}()
	}
	if err != nil {
		t.Fatalf("BuildRuntimeRepos error: %v", err)
	}

	cfg := FolderIntakeConfig{
		WatchDir:    t.TempDir(),
		ArchiveDir:  t.TempDir(),
		Concurrency: 3,
	}

	runtime, err := BuildFolderIntakeRuntime(repos, cfg)
	if err != nil {
		t.Fatalf("BuildFolderIntakeRuntime error: %v", err)
	}
	if runtime == nil {
		t.Fatal("expected folder intake runtime to be configured")
	}
	if runtime.ImportService == nil || runtime.Scanner == nil {
		t.Fatal("expected folder intake runtime import services to be configured")
	}
	if runtime.Scheduler == nil || runtime.Worker == nil {
		t.Fatal("expected folder intake runtime processing services to be configured")
	}
	if runtime.SourceDocumentRepo == nil || runtime.ImportRunRepo == nil {
		t.Fatal("expected folder intake runtime repos to be exposed")
	}
	if runtime.Scheduler.repo != repos.SourceDocumentRepo {
		t.Fatal("expected scheduler to use runtime source document repo")
	}
	if runtime.Scheduler.worker != runtime.Worker {
		t.Fatal("expected scheduler to use runtime worker")
	}
	if runtime.Scheduler.concurrencyLimit != cfg.Concurrency {
		t.Fatal("expected scheduler to use explicit folder intake concurrency")
	}
	if runtime.Scanner.importer != runtime.ImportService {
		t.Fatal("expected scanner to use runtime import service")
	}
	if runtime.Worker.repo != repos.SourceDocumentRepo {
		t.Fatal("expected worker to use runtime source document repo")
	}
	if runtime.ImportService.archiveDir != cfg.ArchiveDir {
		t.Fatal("expected import service to use explicit folder intake archive directory")
	}
	if runtime.WatchDir != cfg.WatchDir || runtime.ArchiveDir != cfg.ArchiveDir {
		t.Fatal("expected runtime to expose explicit folder intake directories")
	}
	if runtime.Worker.rewrite == nil || runtime.Worker.render == nil {
		t.Fatal("expected worker rewrite and render dependencies to be configured")
	}
}

func TestBuildFolderIntakeConfigFromWorkspaceProvidesDefaultProcessingMetadata(t *testing.T) {
	resolved := domain.ResolvedWorkspaceSettings{
		Paths: domain.ResolvedWorkspacePaths{
			IncomingDir:  "/tmp/incoming",
			ProcessedDir: "/tmp/incoming/processed",
		},
		Workspace: domain.WorkspaceSettings{
			DefaultArticleProfile: "wechat-daily",
			ArticleProfiles: map[string]domain.ArticleProfile{
				"wechat-daily": {Template: "daily-intelligence"},
			},
			Collector: domain.CollectorPolicy{GlobalConcurrency: 4},
		},
	}

	cfg, err := BuildFolderIntakeConfigFromWorkspace(resolved)

	require.NoError(t, err)
	require.Equal(t, "/tmp/incoming", cfg.WatchDir)
	require.Equal(t, "/tmp/incoming/processed", cfg.ArchiveDir)
	require.Equal(t, 4, cfg.Concurrency)
	require.Equal(t, "wechat-longform", cfg.TargetType)
	require.Equal(t, "folder-default", cfg.SourceProfile)
	require.NotEmpty(t, cfg.RenderPlatform)
	require.Equal(t, "wechat", cfg.RenderPlatform)
	require.Equal(t, "v1", cfg.RewriteProfileVersion)
}
