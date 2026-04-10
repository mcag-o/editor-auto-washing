package integration

import (
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

func TestBundleImportMainline(t *testing.T) {
	root := t.TempDir()
	loader := workspaceinfra.NewLoader()
	require.NoError(t, loader.Save(root, domain.DefaultWorkspaceSettings()))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))

	provider, err := sqlite.NewProvider(filepath.Join(root, "content-hub.db"))
	require.NoError(t, err)
	defer provider.Close()

	resolved, err := loader.Resolve(root)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(resolved.Paths.AutomationIncomingDir, 0o755))
	bundleData, err := os.ReadFile(filepath.Join("..", "testdata", "bundles", "mainline-success.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(resolved.Paths.AutomationIncomingDir, "bundle.json"), bundleData, 0o644))

	svc := service.NewIngestionPipelineService(provider.IngestionRepo(), provider.WorkspaceRepo(), provider, loader)

	result, err := svc.ImportIncoming(t.Context(), root)
	require.NoError(t, err)
	assert.Equal(t, 1, result.ImportedFiles)
	assert.Equal(t, 2, result.TotalCreatedArticles)

	records, err := svc.ListRecords(t.Context(), domain.IngestionStatusImported)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, domain.IngestionStatusImported, records[0].Status)

	articles, err := svc.ListWorkspaceItems(t.Context(), domain.ArticleWorkspaceStatusImported)
	require.NoError(t, err)
	assert.Len(t, articles, 2)
	assert.Equal(t, records[0].ID, articles[0].Source.IngestionID)
	assert.FileExists(t, filepath.Join(resolved.Paths.AutomationProcessedDir, "bundle.json"))
	assert.NoFileExists(t, filepath.Join(resolved.Paths.AutomationIncomingDir, "bundle.json"))
}
