package handlers

import (
	"content-hub/service"
	"content-hub/transport/http/middleware"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAPIAuditListReturnsLogs(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	repo := &stubAuditLogRepo{}
	svc := service.NewAuditLogService(repo)
	first, err := svc.Create(t.Context(), service.AuditLogCreateInput{Actor: "local-admin", Action: "upload_article", Result: "success"})
	require.NoError(t, err)
	_, err = svc.Create(t.Context(), service.AuditLogCreateInput{Actor: "local-admin", Action: "pause_system", Result: "success"})
	require.NoError(t, err)

	handler := NewAPIAuditHandler(svc, repo)
	router := gin.New()
	router.Use(middleware.TraceID())
	router.GET("/api/audit", handler.List)
	router.GET("/api/audit/:id", handler.Get)

	listReq := httptest.NewRequest(http.MethodGet, "/api/audit", nil)
	listW := httptest.NewRecorder()
	router.ServeHTTP(listW, listReq)

	require.Equal(t, http.StatusOK, listW.Code)
	var listResp struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listW.Body.Bytes(), &listResp))
	require.Len(t, listResp.Data, 2)

	getReq := httptest.NewRequest(http.MethodGet, "/api/audit/"+first.ID, nil)
	getW := httptest.NewRecorder()
	router.ServeHTTP(getW, getReq)
	require.Equal(t, http.StatusOK, getW.Code)
	require.Contains(t, getW.Body.String(), first.ID)
}

func TestAPIAuditListFiltersByResourceAndActionPrefixWithoutDroppingMatches(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	repos, cleanup, err := service.BuildRuntimeRepos(filepath.Join(t.TempDir(), "runtime"))
	if cleanup != nil {
		defer func() {
			require.NoError(t, cleanup())
		}()
	}
	require.NoError(t, err)
	repo := repos.AuditLogRepo
	svc := service.NewAuditLogService(repo)
	_, err = svc.Create(t.Context(), service.AuditLogCreateInput{
		Actor:      "local-admin",
		Action:     "web_control.workflow_run.resume",
		Resource:   "workflow_run",
		ResourceID: "run-1",
		Result:     "success",
		Metadata:   map[string]any{"workflow_run_id": "run-1"},
	})
	require.NoError(t, err)
	for i := 0; i < 120; i++ {
		_, err := svc.Create(t.Context(), service.AuditLogCreateInput{Actor: "local-admin", Action: "upload_article", Resource: "article", ResourceID: "article-1", Result: "success"})
		require.NoError(t, err)
	}

	handler := NewAPIAuditHandler(svc, repo)
	router := gin.New()
	router.Use(middleware.TraceID())
	router.GET("/api/audit", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/api/audit?resource=workflow_run&action_prefix=web_control.workflow_run", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 1)
	require.Equal(t, "workflow_run", resp.Data[0]["resource"])
	require.Equal(t, "web_control.workflow_run.resume", resp.Data[0]["action"])
}

func TestAPIAuditListFiltersByWorkflowRunAndResourceIDWithoutDroppingMatches(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	repos, cleanup, err := service.BuildRuntimeRepos(filepath.Join(t.TempDir(), "runtime"))
	if cleanup != nil {
		defer func() {
			require.NoError(t, cleanup())
		}()
	}
	require.NoError(t, err)
	repo := repos.AuditLogRepo
	svc := service.NewAuditLogService(repo)

	_, err = svc.Create(t.Context(), service.AuditLogCreateInput{
		Actor:      "local-admin",
		Action:     "web_control.workflow_run.resume",
		Resource:   "workflow_run",
		ResourceID: "run-1",
		Result:     "success",
		Metadata:   map[string]any{"workflow_run_id": "run-1"},
	})
	require.NoError(t, err)
	_, err = svc.Create(t.Context(), service.AuditLogCreateInput{
		Actor:      "local-admin",
		Action:     "web_control.workflow_run.resume",
		Resource:   "workflow_run",
		ResourceID: "run-2",
		Result:     "success",
		Metadata:   map[string]any{"workflow_run_id": "run-2"},
	})
	require.NoError(t, err)
	for i := 0; i < 120; i++ {
		_, err := svc.Create(t.Context(), service.AuditLogCreateInput{
			Actor:      "local-admin",
			Action:     "upload_article",
			Resource:   "article",
			ResourceID: "article-1",
			Result:     "success",
		})
		require.NoError(t, err)
	}

	handler := NewAPIAuditHandler(svc, repo)
	router := gin.New()
	router.Use(middleware.TraceID())
	router.GET("/api/audit", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/api/audit?workflow_run_id=run-1&resource_id=run-1&action_prefix=web_control.workflow_run", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 1)
	require.Equal(t, "workflow_run", resp.Data[0]["resource"])
	require.Equal(t, "run-1", resp.Data[0]["resource_id"])
	require.Equal(t, "web_control.workflow_run.resume", resp.Data[0]["action"])
}
