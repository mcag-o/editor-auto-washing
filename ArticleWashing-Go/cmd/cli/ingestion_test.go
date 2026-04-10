package main

import (
	"bytes"
	"content-hub/domain"
	"content-hub/infra/sqlite"
	workspaceinfra "content-hub/infra/workspace"
	"content-hub/service"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIngestionImportCommandImportsBundles(t *testing.T) {
	root := t.TempDir()
	loader := workspaceinfra.NewLoader()
	require.NoError(t, loader.Save(root, domain.DefaultWorkspaceSettings()))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))
	resolved, err := loader.Resolve(root)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(resolved.Paths.AutomationIncomingDir, 0o755))
	bundleData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "bundles", "mainline-success.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(resolved.Paths.AutomationIncomingDir, "bundle.json"), bundleData, 0o644))

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run([]string{"ingestion", "import", "--root", root}, stdout, stderr)

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout.String(), "importedfiles: 1")
	assert.Empty(t, stderr.String())
}

func TestIngestionRetryFailedCommandRetriesBundles(t *testing.T) {
	root := t.TempDir()
	loader := workspaceinfra.NewLoader()
	require.NoError(t, loader.Save(root, domain.DefaultWorkspaceSettings()))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))
	resolved, err := loader.Resolve(root)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(resolved.Paths.AutomationFailedDir, 0o755))
	bundleData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "bundles", "mainline-success.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(resolved.Paths.AutomationFailedDir, "bundle.json"), bundleData, 0o644))

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run([]string{"ingestion", "retry-failed", "--root", root}, stdout, stderr)

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout.String(), "importedfiles: 1")
	assert.Empty(t, stderr.String())
}

func TestIngestionImportCommandPersistsToWorkspaceSQLiteDatabase(t *testing.T) {
	root := t.TempDir()
	loader := workspaceinfra.NewLoader()
	require.NoError(t, loader.Save(root, domain.DefaultWorkspaceSettings()))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))
	resolved, err := loader.Resolve(root)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(resolved.Paths.AutomationIncomingDir, 0o755))
	bundleData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "bundles", "mainline-success.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(resolved.Paths.AutomationIncomingDir, "bundle.json"), bundleData, 0o644))

	exitCode := run([]string{"ingestion", "import", "--root", root}, &bytes.Buffer{}, &bytes.Buffer{})
	require.Equal(t, 0, exitCode)

	provider, err := sqlite.NewProvider(filepath.Join(root, "workspace_data", "content-hub.db"))
	require.NoError(t, err)
	defer provider.Close()

	records, err := provider.IngestionRepo().List(t.Context(), domain.IngestionStatusImported)
	require.NoError(t, err)
	require.Len(t, records, 1)
	articles, err := provider.WorkspaceRepo().List(t.Context(), nil)
	require.NoError(t, err)
	assert.Len(t, articles, 2)
}

func TestIngestionImportCommandUsesRuntimeTransactionalImportPath(t *testing.T) {
	root := t.TempDir()
	loader := workspaceinfra.NewLoader()
	require.NoError(t, loader.Save(root, domain.DefaultWorkspaceSettings()))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))
	resolved, err := loader.Resolve(root)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(resolved.Paths.AutomationIncomingDir, 0o755))
	bundleData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "bundles", "mainline-success.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(resolved.Paths.AutomationIncomingDir, "bundle.json"), bundleData, 0o644))

	originalFactory := runtimeIngestionServiceFactory
	runtimeIngestionServiceFactory = func(root string) (*service.IngestionPipelineService, func() error, error) {
		repos, cleanup, err := service.BuildRuntimeRepos(root)
		if err != nil {
			return nil, nil, err
		}
		return service.NewIngestionPipelineService(repos.IngestionRepo, repos.WorkspaceRepo, repos.BundleImportTxStarter, workspaceinfra.NewLoader()), cleanup, nil
	}
	defer func() { runtimeIngestionServiceFactory = originalFactory }()

	exitCode := run([]string{"ingestion", "import", "--root", root}, &bytes.Buffer{}, &bytes.Buffer{})
	require.Equal(t, 0, exitCode)

	provider, err := sqlite.NewProvider(filepath.Join(root, "workspace_data", "content-hub.db"))
	require.NoError(t, err)
	defer provider.Close()
	articles, err := provider.WorkspaceRepo().List(t.Context(), nil)
	require.NoError(t, err)
	require.Len(t, articles, 2)
	records, err := provider.IngestionRepo().List(t.Context(), domain.IngestionStatusImported)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, 2, records[0].CreatedArticles)
}
