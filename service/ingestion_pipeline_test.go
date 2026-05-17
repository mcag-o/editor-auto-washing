package service

import (
	"content-hub/domain"
	"content-hub/infra/memory"
	"content-hub/infra/sqlite"
	workspaceinfra "content-hub/infra/workspace"
	"content-hub/pkg/repo"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIngestionPipeline_ImportBundleRoutesAndPersistsWorkspaceItems(t *testing.T) {
	root := t.TempDir()
	loader := workspaceinfra.NewLoader()
	settings := domain.DefaultWorkspaceSettings()
	require.NoError(t, loader.Save(root, settings))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))

	resolved, err := loader.Resolve(root)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(resolved.Paths.AutomationIncomingDir, 0o755))

	bundlePath := filepath.Join(resolved.Paths.AutomationIncomingDir, "bundle.json")
	bundleData, err := os.ReadFile(filepath.Join("..", "testdata", "bundles", "mainline-success.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(bundlePath, bundleData, 0o644))

	txProvider, err := sqlite.NewProvider(filepath.Join(root, "ingestion-mainline.db"))
	require.NoError(t, err)
	defer txProvider.Close()
	svc := NewIngestionPipelineService(txProvider.IngestionRepo(), txProvider.WorkspaceRepo(), txProvider, loader)

	result, err := svc.ImportIncoming(context.Background(), root)
	require.NoError(t, err)
	assert.Equal(t, 1, result.ImportedFiles)
	assert.Equal(t, 0, result.FailedFiles)
	assert.Equal(t, 2, result.TotalImportedItems)
	assert.Equal(t, 2, result.TotalCreatedArticles)
	assert.FileExists(t, filepath.Join(resolved.Paths.AutomationProcessedDir, "bundle.json"))
	assert.NoFileExists(t, bundlePath)

	records, err := svc.ListRecords(context.Background(), "")
	require.NoError(t, err)
	assert.Len(t, records, 1)
	assert.Equal(t, domain.IngestionStatusImported, records[0].Status)
	assert.Equal(t, 2, records[0].ImportedItems)

	articles, err := svc.ListWorkspaceItems(context.Background(), "")
	require.NoError(t, err)
	assert.Len(t, articles, 2)
	assert.Equal(t, domain.ArticleWorkspaceStatusImported, articles[0].Status)
	assert.Contains(t, articles[0].LifecycleHistory[0].Status, domain.ArticleWorkspaceStatusImported)
	assert.NotEmpty(t, articles[0].Source.BundleFile)
	assert.NotEmpty(t, articles[0].Source.IngestionID)
	assert.NotEmpty(t, articles[0].Source.URL)

	status, err := svc.GetStatus(context.Background(), records[0].ID)
	require.NoError(t, err)
	assert.Equal(t, records[0].ID, status.Record.ID)
	assert.Len(t, status.Articles, 2)
	assert.Equal(t, filepath.Join(resolved.Paths.AutomationProcessedDir, "bundle.json"), status.Record.RoutedPath)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(records[0].Payload, &payload))
	assert.Equal(t, "1.0", payload["bundleVersion"])
}

func TestIngestionPipeline_RetryFailedReimportsBundle(t *testing.T) {
	root := t.TempDir()
	loader := workspaceinfra.NewLoader()
	settings := domain.DefaultWorkspaceSettings()
	require.NoError(t, loader.Save(root, settings))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))

	resolved, err := loader.Resolve(root)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(resolved.Paths.AutomationFailedDir, 0o755))

	bundlePath := filepath.Join(resolved.Paths.AutomationFailedDir, "bundle.json")
	bundleData, err := os.ReadFile(filepath.Join("..", "testdata", "bundles", "mainline-success.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(bundlePath, bundleData, 0o644))

	txProvider, err := sqlite.NewProvider(filepath.Join(root, "ingestion-retry.db"))
	require.NoError(t, err)
	defer txProvider.Close()
	svc := NewIngestionPipelineService(txProvider.IngestionRepo(), txProvider.WorkspaceRepo(), txProvider, loader)

	result, err := svc.RetryFailed(context.Background(), root)
	require.NoError(t, err)
	assert.Equal(t, 1, result.ImportedFiles)
	assert.Equal(t, 0, result.FailedFiles)
	assert.FileExists(t, filepath.Join(resolved.Paths.AutomationProcessedDir, "bundle.json"))
	assert.NoFileExists(t, bundlePath)

	records, err := svc.ListRecords(context.Background(), domain.IngestionStatusImported)
	require.NoError(t, err)
	assert.Len(t, records, 1)
	assert.Equal(t, "failed", records[0].OriginalLocation)
	assert.True(t, records[0].Retried)
}

func TestIngestionPipeline_InvalidBundlePersistsFailedRecordAndRoutesFile(t *testing.T) {
	root := t.TempDir()
	provider := memory.NewProvider()
	loader := workspaceinfra.NewLoader()
	settings := domain.DefaultWorkspaceSettings()
	require.NoError(t, loader.Save(root, settings))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))

	resolved, err := loader.Resolve(root)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(resolved.Paths.AutomationIncomingDir, 0o755))

	bundlePath := filepath.Join(resolved.Paths.AutomationIncomingDir, "invalid.json")
	bundleData, err := os.ReadFile(filepath.Join("..", "testdata", "bundles", "mainline-invalid.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(bundlePath, bundleData, 0o644))

	svc := NewIngestionPipelineService(provider.IngestionRepo(), provider.WorkspaceRepo(), nil, loader)
	svc.beginImportTx = func(context.Context) (repo.BundleImportTx, error) {
		return &failingBundleImportTx{}, nil
	}

	result, err := svc.ImportIncoming(context.Background(), root)
	require.NoError(t, err)
	assert.Equal(t, 0, result.ImportedFiles)
	assert.Equal(t, 1, result.FailedFiles)
	assert.FileExists(t, filepath.Join(resolved.Paths.AutomationFailedDir, "invalid.json"))
	assert.NoFileExists(t, bundlePath)

	records, err := svc.ListRecords(context.Background(), domain.IngestionStatusFailed)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, domain.IngestionStatusFailed, records[0].Status)
	assert.Contains(t, records[0].ErrorMessage, "bundle must include a list field: items")
	assert.Equal(t, filepath.Join(resolved.Paths.AutomationFailedDir, "invalid.json"), records[0].RoutedPath)

	articles, err := svc.ListWorkspaceItems(context.Background(), "")
	require.NoError(t, err)
	assert.Empty(t, articles)
}

func TestIngestionPipeline_FailsImportWhenWorkspaceCreationFails(t *testing.T) {
	root := t.TempDir()
	provider := memory.NewProvider()
	loader := workspaceinfra.NewLoader()
	settings := domain.DefaultWorkspaceSettings()
	require.NoError(t, loader.Save(root, settings))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))

	resolved, err := loader.Resolve(root)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(resolved.Paths.AutomationIncomingDir, 0o755))

	bundlePath := filepath.Join(resolved.Paths.AutomationIncomingDir, "bundle.json")
	bundleData, err := os.ReadFile(filepath.Join("..", "testdata", "bundles", "mainline-success.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(bundlePath, bundleData, 0o644))

	svc := NewIngestionPipelineService(provider.IngestionRepo(), failingWorkspaceRepo{createErr: domain.NewInternalErr("workspace create failed", nil)}, nil, loader)
	svc.beginImportTx = func(context.Context) (repo.BundleImportTx, error) {
		return &failingWorkspaceBundleImportTx{err: domain.NewInternalErr("workspace create failed", nil)}, nil
	}

	result, err := svc.ImportIncoming(context.Background(), root)
	require.NoError(t, err)
	assert.Equal(t, 0, result.ImportedFiles)
	assert.Equal(t, 1, result.FailedFiles)
	assert.FileExists(t, filepath.Join(resolved.Paths.AutomationFailedDir, "bundle.json"))

	records, err := svc.ListRecords(context.Background(), domain.IngestionStatusFailed)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, 0, records[0].CreatedArticles)
	assert.Contains(t, records[0].ErrorMessage, "workspace create failed")
}

func TestIngestionPipeline_RollsBackWhenSecondArticleCreateFailsInSQLiteTx(t *testing.T) {
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

	svc := NewIngestionPipelineService(provider.IngestionRepo(), provider.WorkspaceRepo(), nil, loader)
	svc.beginImportTx = func(context.Context) (repo.BundleImportTx, error) {
		return &failingBundleImportTx{failOnArticleIndex: 1}, nil
	}
	result, err := svc.ImportIncoming(context.Background(), root)
	require.NoError(t, err)
	assert.Equal(t, 0, result.ImportedFiles)
	assert.Equal(t, 1, result.FailedFiles)

	articles, err := provider.WorkspaceRepo().List(context.Background(), nil)
	require.NoError(t, err)
	assert.Len(t, articles, 0)
	records, err := provider.IngestionRepo().List(context.Background(), domain.IngestionStatusFailed)
	require.NoError(t, err)
	assert.Len(t, records, 1)
	assert.FileExists(t, filepath.Join(resolved.Paths.AutomationFailedDir, "bundle.json"))
}

func TestIngestionPipeline_RecordsRoutingFailureBeforeImportedStateIsCommitted(t *testing.T) {
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
	bundlePath := filepath.Join(resolved.Paths.AutomationIncomingDir, "bundle.json")
	bundleData, err := os.ReadFile(filepath.Join("..", "testdata", "bundles", "mainline-success.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(bundlePath, bundleData, 0o644))
	svc := NewIngestionPipelineService(provider.IngestionRepo(), provider.WorkspaceRepo(), nil, loader)
	tx := &successBundleImportTx{}
	svc.beginImportTx = func(context.Context) (repo.BundleImportTx, error) {
		return tx, nil
	}
	svc.router = failingProcessedRouter{failedDir: resolved.Paths.AutomationFailedDir}
	result, err := svc.ImportIncoming(context.Background(), root)
	require.NoError(t, err)
	assert.Equal(t, 0, result.ImportedFiles)
	assert.Equal(t, 0, result.FailedFiles)
	assert.Equal(t, domain.IngestionStatusRoutingFailed, result.FileResults[0].Status)

	articles, err := provider.WorkspaceRepo().List(context.Background(), nil)
	require.NoError(t, err)
	assert.Len(t, articles, 0)
	assert.False(t, tx.recorded)
	assert.Len(t, tx.created, 2)
	assert.FileExists(t, filepath.Join(resolved.Paths.AutomationProcessedDir, "routing_failed", "bundle.json"))
	records, err := provider.IngestionRepo().List(context.Background(), domain.IngestionStatusRoutingFailed)
	require.NoError(t, err)
	assert.Len(t, records, 1)
	assert.Contains(t, result.FileResults[0].ErrorMessage, "processed routing failed")
}

func TestIngestionPipeline_RecordsPersistenceFailureAndRoutesBundleToFailed(t *testing.T) {
	root := t.TempDir()
	provider := memory.NewProvider()
	loader := workspaceinfra.NewLoader()
	require.NoError(t, loader.Save(root, domain.DefaultWorkspaceSettings()))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))

	resolved, err := loader.Resolve(root)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(resolved.Paths.AutomationIncomingDir, 0o755))
	bundlePath := filepath.Join(resolved.Paths.AutomationIncomingDir, "bundle.json")
	bundleData, err := os.ReadFile(filepath.Join("..", "testdata", "bundles", "mainline-success.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(bundlePath, bundleData, 0o644))

	svc := NewIngestionPipelineService(provider.IngestionRepo(), failingWorkspaceRepo{createErr: domain.NewInternalErr("workspace create failed", nil)}, nil, loader)
	svc.beginImportTx = func(context.Context) (repo.BundleImportTx, error) {
		return &failingWorkspaceBundleImportTx{err: domain.NewInternalErr("workspace create failed", nil)}, nil
	}

	result, err := svc.ImportIncoming(context.Background(), root)

	require.NoError(t, err)
	assert.Equal(t, 0, result.ImportedFiles)
	assert.Equal(t, 1, result.FailedFiles)
	assert.Equal(t, domain.IngestionStatusFailed, result.FileResults[0].Status)
	assert.FileExists(t, filepath.Join(resolved.Paths.AutomationFailedDir, "bundle.json"))
	assert.NoFileExists(t, filepath.Join(resolved.Paths.AutomationProcessedDir, "bundle.json"))
	records, listErr := svc.ListRecords(context.Background(), domain.IngestionStatusFailed)
	require.NoError(t, listErr)
	require.Len(t, records, 1)
	assert.Contains(t, records[0].ErrorMessage, "workspace create failed")
	assert.Equal(t, filepath.Join(resolved.Paths.AutomationFailedDir, "bundle.json"), records[0].RoutedPath)
}

func TestIngestionPipeline_PersistsRoutingFailureWithoutCommittingWorkspaceArticles(t *testing.T) {
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
	bundlePath := filepath.Join(resolved.Paths.AutomationIncomingDir, "bundle.json")
	bundleData, err := os.ReadFile(filepath.Join("..", "testdata", "bundles", "mainline-success.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(bundlePath, bundleData, 0o644))

	svc := NewIngestionPipelineService(provider.IngestionRepo(), provider.WorkspaceRepo(), provider, loader)
	svc.router = postCommitFailingProcessedRouter{failedDir: resolved.Paths.AutomationFailedDir}

	result, err := svc.ImportIncoming(context.Background(), root)

	require.NoError(t, err)
	assert.Equal(t, 0, result.ImportedFiles)
	assert.Equal(t, 0, result.FailedFiles)
	assert.Equal(t, 1, result.RoutingFailedFiles)
	assert.Equal(t, domain.IngestionStatusRoutingFailed, result.FileResults[0].Status)
	assert.FileExists(t, filepath.Join(resolved.Paths.AutomationProcessedDir, "routing_failed", "bundle.json"))
	assert.NoFileExists(t, filepath.Join(resolved.Paths.AutomationProcessedDir, "bundle.json"))
	records, listErr := provider.IngestionRepo().List(context.Background(), domain.IngestionStatusRoutingFailed)
	require.NoError(t, listErr)
	require.Len(t, records, 1)
	assert.Contains(t, records[0].ErrorMessage, "processed routing failed after commit")
	assert.Equal(t, filepath.Join(resolved.Paths.AutomationProcessedDir, "routing_failed", "bundle.json"), records[0].RoutedPath)
	articles, articleErr := provider.WorkspaceRepo().List(context.Background(), nil)
	require.NoError(t, articleErr)
	assert.Len(t, articles, 0)
}

func TestIngestionPipeline_LeavesRoutingFailedImportsOutOfRetryFailedScan(t *testing.T) {
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

	svc := NewIngestionPipelineService(provider.IngestionRepo(), provider.WorkspaceRepo(), provider, loader)
	svc.router = postCommitFailingProcessedRouter{failedDir: resolved.Paths.AutomationFailedDir}

	result, err := svc.ImportIncoming(context.Background(), root)

	require.NoError(t, err)
	assert.Equal(t, 0, result.ImportedFiles)
	assert.Equal(t, 0, result.FailedFiles)
	assert.Equal(t, 1, result.RoutingFailedFiles)
	assert.Equal(t, domain.IngestionStatusRoutingFailed, result.FileResults[0].Status)
	records, listErr := provider.IngestionRepo().List(context.Background(), domain.IngestionStatusRoutingFailed)
	require.NoError(t, listErr)
	require.Len(t, records, 1)
	require.NoError(t, os.MkdirAll(resolved.Paths.AutomationFailedDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(resolved.Paths.AutomationFailedDir, "bundle-again.json"), bundleData, 0o644))
	retryResult, retryErr := svc.RetryFailed(context.Background(), root)
	require.NoError(t, retryErr)
	assert.Equal(t, 0, retryResult.ImportedFiles)
	assert.Equal(t, 1, retryResult.ScannedFiles)
	assert.Equal(t, 0, retryResult.FailedFiles)
	assert.Equal(t, 1, retryResult.RoutingFailedFiles)
	assert.Equal(t, domain.IngestionStatusRoutingFailed, retryResult.FileResults[0].Status)
	routingFailedRecords, routingErr := provider.IngestionRepo().List(context.Background(), domain.IngestionStatusRoutingFailed)
	require.NoError(t, routingErr)
	assert.Len(t, routingFailedRecords, 2)
}

func TestIngestionPipeline_AvoidsProcessedSuccessMetadataDriftByRecordingImportedStateInTx(t *testing.T) {
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

	svc := NewIngestionPipelineService(provider.IngestionRepo(), provider.WorkspaceRepo(), provider, loader)

	result, runErr := svc.ImportIncoming(context.Background(), root)

	require.NoError(t, runErr)
	assert.Equal(t, 1, result.ImportedFiles)
	assert.Equal(t, 0, result.RoutingFailedFiles)
	assert.Equal(t, domain.IngestionStatusImported, result.FileResults[0].Status)
	records, listErr := provider.IngestionRepo().List(context.Background(), domain.IngestionStatusImported)
	require.NoError(t, listErr)
	require.Len(t, records, 1)
	assert.Equal(t, filepath.Join(resolved.Paths.AutomationProcessedDir, "bundle.json"), records[0].RoutedPath)
}

func TestIngestionPipeline_RequiresTransactionalImportSupport(t *testing.T) {
	root := t.TempDir()
	provider := memory.NewProvider()
	loader := workspaceinfra.NewLoader()
	require.NoError(t, loader.Save(root, domain.DefaultWorkspaceSettings()))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))

	resolved, err := loader.Resolve(root)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(resolved.Paths.AutomationIncomingDir, 0o755))
	bundleData, err := os.ReadFile(filepath.Join("..", "testdata", "bundles", "mainline-success.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(resolved.Paths.AutomationIncomingDir, "bundle.json"), bundleData, 0o644))

	svc := NewIngestionPipelineService(provider.IngestionRepo(), provider.WorkspaceRepo(), nil, loader)

	result, runErr := svc.ImportIncoming(context.Background(), root)

	require.NoError(t, runErr)
	assert.Equal(t, 0, result.ImportedFiles)
	assert.Equal(t, 1, result.FailedFiles)
	records, listErr := provider.IngestionRepo().List(context.Background(), domain.IngestionStatusFailed)
	require.NoError(t, listErr)
	require.Len(t, records, 1)
	assert.Contains(t, records[0].ErrorMessage, "transactional bundle import support is required")
}

type failingWorkspaceRepo struct {
	createErr error
}

var _ repo.WorkspaceRepo = failingWorkspaceRepo{}

func (f failingWorkspaceRepo) Create(context.Context, *domain.ArticleWorkspaceRecord) error {
	return f.createErr
}

func (f failingWorkspaceRepo) Update(context.Context, *domain.ArticleWorkspaceRecord) error {
	return f.createErr
}

func (f failingWorkspaceRepo) GetByID(context.Context, string) (*domain.ArticleWorkspaceRecord, error) {
	return nil, domain.NewNotFoundErr("workspace", "missing")
}

func (f failingWorkspaceRepo) List(context.Context, *string) ([]domain.ArticleWorkspaceRecord, error) {
	return nil, nil
}

func (f failingWorkspaceRepo) ListByIngestionID(context.Context, string) ([]domain.ArticleWorkspaceRecord, error) {
	return nil, nil
}

func (f failingWorkspaceRepo) TransitionStatus(context.Context, string, string, string) error {
	return nil
}

func (f failingWorkspaceRepo) Delete(context.Context, string) error {
	return nil
}

type failingProcessedRouter struct {
	failedDir string
}

func (f failingProcessedRouter) RouteToProcessed(string, string) (string, error) {
	return "", fmt.Errorf("processed routing failed")
}

func (f failingProcessedRouter) RouteToFailed(sourcePath, failedDir string) (string, error) {
	return osRenameRoute(sourcePath, failedDir)
}

type postCommitFailingProcessedRouter struct {
	failedDir string
}

func (f postCommitFailingProcessedRouter) RouteToProcessed(string, string) (string, error) {
	return "", fmt.Errorf("processed routing failed after commit")
}

func (f postCommitFailingProcessedRouter) RouteToFailed(sourcePath, failedDir string) (string, error) {
	return osRenameRoute(sourcePath, failedDir)
}

type failingBundleImportTx struct {
	created            int
	failOnArticleIndex int
}

type successBundleImportTx struct {
	created  []string
	recorded bool
}

type failingWorkspaceBundleImportTx struct {
	err error
}

func (f *failingBundleImportTx) CreateWorkspaceArticle(context.Context, *domain.ArticleWorkspaceRecord) error {
	if f.created == f.failOnArticleIndex {
		return domain.NewConflictErr("duplicate second insert")
	}
	f.created++
	return nil
}

func (f *failingBundleImportTx) RecordIngestion(context.Context, *domain.IngestionRecord) error {
	return nil
}

func (f *failingBundleImportTx) Commit() error {
	return nil
}

func (f *failingBundleImportTx) Rollback() error {
	return nil
}

func (s *successBundleImportTx) CreateWorkspaceArticle(_ context.Context, article *domain.ArticleWorkspaceRecord) error {
	s.created = append(s.created, article.ID)
	return nil
}

func (s *successBundleImportTx) RecordIngestion(context.Context, *domain.IngestionRecord) error {
	s.recorded = true
	return nil
}

func (s *successBundleImportTx) Commit() error {
	return nil
}

func (s *successBundleImportTx) Rollback() error {
	return nil
}

func (f *failingWorkspaceBundleImportTx) CreateWorkspaceArticle(context.Context, *domain.ArticleWorkspaceRecord) error {
	return f.err
}

func (f *failingWorkspaceBundleImportTx) RecordIngestion(context.Context, *domain.IngestionRecord) error {
	return nil
}

func (f *failingWorkspaceBundleImportTx) Commit() error {
	return nil
}

func (f *failingWorkspaceBundleImportTx) Rollback() error {
	return nil
}

func osRenameRoute(sourcePath, failedDir string) (string, error) {
	if err := os.MkdirAll(failedDir, 0o755); err != nil {
		return "", err
	}
	targetPath := filepath.Join(failedDir, filepath.Base(sourcePath))
	if err := os.Rename(sourcePath, targetPath); err != nil {
		return "", err
	}
	return targetPath, nil
}
