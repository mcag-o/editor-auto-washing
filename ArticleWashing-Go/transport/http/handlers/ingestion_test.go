package handlers

import (
	"bytes"
	"content-hub/domain"
	"content-hub/infra/sqlite"
	workspaceinfra "content-hub/infra/workspace"
	"content-hub/service"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIngestionImportHandler_ImportsIncomingBundles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	loader := workspaceinfra.NewLoader()
	require.NoError(t, loader.Save(root, domain.DefaultWorkspaceSettings()))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))

	resolved, err := loader.Resolve(root)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(resolved.Paths.AutomationIncomingDir, 0o755))
	bundleData, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "bundles", "mainline-success.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(resolved.Paths.AutomationIncomingDir, "bundle.json"), bundleData, 0o644))

	txProvider, err := sqlite.NewProvider(filepath.Join(root, "handler-import.db"))
	require.NoError(t, err)
	defer txProvider.Close()
	svc := service.NewIngestionPipelineService(txProvider.IngestionRepo(), txProvider.WorkspaceRepo(), txProvider, loader)
	handler := NewIngestionHandler(svc)

	router := gin.New()
	router.POST("/ingestion/import", handler.Import)

	body := bytes.NewBufferString(`{"workspace_root":"` + root + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/ingestion/import", body)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusCreated, resp.Code)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
	assert.EqualValues(t, 1, payload["imported_files"])
}

func TestIngestionRetryAndStatusHandlers_ReturnRecordDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	loader := workspaceinfra.NewLoader()
	require.NoError(t, loader.Save(root, domain.DefaultWorkspaceSettings()))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))

	resolved, err := loader.Resolve(root)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(resolved.Paths.AutomationFailedDir, 0o755))
	bundleData, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "bundles", "mainline-success.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(resolved.Paths.AutomationFailedDir, "bundle.json"), bundleData, 0o644))

	txProvider, err := sqlite.NewProvider(filepath.Join(root, "handler-retry.db"))
	require.NoError(t, err)
	defer txProvider.Close()
	svc := service.NewIngestionPipelineService(txProvider.IngestionRepo(), txProvider.WorkspaceRepo(), txProvider, loader)
	handler := NewIngestionHandler(svc)

	router := gin.New()
	router.POST("/ingestion/retry-failed", handler.RetryFailed)
	router.GET("/ingestion", handler.List)
	router.GET("/ingestion/:id", handler.Status)

	retryReq := httptest.NewRequest(http.MethodPost, "/ingestion/retry-failed", bytes.NewBufferString(`{"workspace_root":"`+root+`"}`))
	retryReq.Header.Set("Content-Type", "application/json")
	retryResp := httptest.NewRecorder()
	router.ServeHTTP(retryResp, retryReq)
	require.Equal(t, http.StatusOK, retryResp.Code)

	listReq := httptest.NewRequest(http.MethodGet, "/ingestion", nil)
	listResp := httptest.NewRecorder()
	router.ServeHTTP(listResp, listReq)
	require.Equal(t, http.StatusOK, listResp.Code)

	var listPayload struct {
		Data []domain.IngestionRecord `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listResp.Body.Bytes(), &listPayload))
	require.Len(t, listPayload.Data, 1)

	statusReq := httptest.NewRequest(http.MethodGet, "/ingestion/"+listPayload.Data[0].ID, nil)
	statusResp := httptest.NewRecorder()
	router.ServeHTTP(statusResp, statusReq)
	require.Equal(t, http.StatusOK, statusResp.Code)
	assert.Contains(t, statusResp.Body.String(), listPayload.Data[0].ID)
	assert.Contains(t, statusResp.Body.String(), "articles")
}

func TestWorkspaceListHandler_ReturnsImportedArticles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	loader := workspaceinfra.NewLoader()
	root := t.TempDir()
	require.NoError(t, loader.Save(root, domain.DefaultWorkspaceSettings()))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))
	resolved, err := loader.Resolve(root)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(resolved.Paths.AutomationIncomingDir, 0o755))
	bundleData, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "bundles", "mainline-success.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(resolved.Paths.AutomationIncomingDir, "bundle.json"), bundleData, 0o644))

	txProvider, err := sqlite.NewProvider(filepath.Join(root, "handler-workspace.db"))
	require.NoError(t, err)
	defer txProvider.Close()
	ingestionSvc := service.NewIngestionPipelineService(txProvider.IngestionRepo(), txProvider.WorkspaceRepo(), txProvider, loader)
	_, err = ingestionSvc.ImportIncoming(t.Context(), root)
	require.NoError(t, err)
	workspaceSvc := service.NewWorkspaceArticleService(txProvider.WorkspaceRepo())

	handler := NewWorkspaceHandler(workspaceSvc)
	router := gin.New()
	router.GET("/workspace/articles", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/workspace/articles?status=imported", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "OpenAI launches new editor workflow")
	assert.Contains(t, resp.Body.String(), "imported")
}
