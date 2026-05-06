package handlers

import (
	"bytes"
	"content-hub/domain"
	"content-hub/service"
	"content-hub/transport/http/middleware"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAPIWorkflowsCreateAndList(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	svc := service.NewWorkflowTemplateService(&stubWorkflowDefinitionRepo{})
	handler := NewAPIWorkflowsHandler(svc)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/workflows", handler.Create)
	router.GET("/api/workflows", handler.List)

	body := `{"id":"wf-1","name":"Default workflow","description":"Mainline graph","version":"v1","enabled":true,"entry_node_id":"start-1","nodes":[{"id":"start-1","type":"start","name":"Start","config_json":"{}"}],"edges":[],"updated_by":"tester"}`
	req := httptest.NewRequest(http.MethodPost, "/api/workflows", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var created domain.WorkflowDefinition
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.Equal(t, "wf-1", created.ID)
	require.Equal(t, "Default workflow", created.Name)
	require.Len(t, created.Nodes, 1)

	req = httptest.NewRequest(http.MethodGet, "/api/workflows", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var listResp struct {
		Data []domain.WorkflowDefinition `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listResp))
	require.Len(t, listResp.Data, 1)
	require.Equal(t, "wf-1", listResp.Data[0].ID)
}

func TestAPIWorkflowsGetAndUpdate(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	repo := &stubWorkflowDefinitionRepo{}
	require.NoError(t, repo.Upsert(t.Context(), &domain.WorkflowDefinition{
		ID:          "wf-1",
		Name:        "Default workflow",
		Description: "Mainline graph",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "start-1",
		Nodes: []domain.WorkflowNode{{
			ID:         "start-1",
			Type:       "start",
			Name:       "Start",
			ConfigJSON: `{}`,
		}},
	}))
	svc := service.NewWorkflowTemplateService(repo)
	handler := NewAPIWorkflowsHandler(svc)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.GET("/api/workflows/:id", handler.Get)
	router.PUT("/api/workflows/:id", handler.Update)

	req := httptest.NewRequest(http.MethodGet, "/api/workflows/wf-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var stored domain.WorkflowDefinition
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &stored))
	require.Equal(t, "wf-1", stored.ID)
	require.Equal(t, "Default workflow", stored.Name)

	body := `{"name":"Updated workflow","description":"Updated graph","version":"v2","enabled":false,"entry_node_id":"start-1","nodes":[{"id":"start-1","type":"start","name":"Start","config_json":"{}"}],"edges":[{"from_node_id":"start-1","to_node_id":"start-1","condition":"retry","priority":1}],"updated_by":"editor"}`
	req = httptest.NewRequest(http.MethodPut, "/api/workflows/wf-1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var updated domain.WorkflowDefinition
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &updated))
	require.Equal(t, "wf-1", updated.ID)
	require.Equal(t, "Updated workflow", updated.Name)
	require.Equal(t, "v2", updated.Version)
	require.False(t, updated.Enabled)
	require.Len(t, updated.Edges, 1)
}
