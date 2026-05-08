package handlers

import (
	"bytes"
	"content-hub/domain"
	"content-hub/service"
	"content-hub/transport/http/middleware"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type workflowDefinitionRepoStub struct {
	stored    *domain.WorkflowDefinition
	list      []domain.WorkflowDefinition
	created   *domain.WorkflowDefinition
	updated   *domain.WorkflowDefinition
	createErr error
	updateErr error
	upsertErr error
	getErr    error
	listErr   error
	deleteErr error
}

func (r *workflowDefinitionRepoStub) Create(_ context.Context, workflow *domain.WorkflowDefinition) error {
	r.created = workflow
	if r.createErr != nil {
		return r.createErr
	}
	r.stored = workflow
	if workflow != nil {
		replaced := false
		for i := range r.list {
			if r.list[i].ID == workflow.ID {
				r.list[i] = *workflow
				replaced = true
				break
			}
		}
		if !replaced {
			r.list = append(r.list, *workflow)
		}
	}
	return nil
}

func (r *workflowDefinitionRepoStub) Upsert(_ context.Context, workflow *domain.WorkflowDefinition) error {
	if r.upsertErr != nil {
		return r.upsertErr
	}
	r.stored = workflow
	if workflow != nil {
		replaced := false
		for i := range r.list {
			if r.list[i].ID == workflow.ID {
				r.list[i] = *workflow
				replaced = true
				break
			}
		}
		if !replaced {
			r.list = append(r.list, *workflow)
		}
	}
	return nil
}

func (r *workflowDefinitionRepoStub) Update(_ context.Context, workflow *domain.WorkflowDefinition) error {
	r.updated = workflow
	if r.updateErr != nil {
		return r.updateErr
	}
	if r.stored == nil || r.stored.ID != workflow.ID {
		return domain.NewNotFoundErr("workflow_definition", workflow.ID)
	}
	r.stored = workflow
	if workflow != nil {
		replaced := false
		for i := range r.list {
			if r.list[i].ID == workflow.ID {
				r.list[i] = *workflow
				replaced = true
				break
			}
		}
		if !replaced {
			r.list = append(r.list, *workflow)
		}
	}
	return nil
}

func (r *workflowDefinitionRepoStub) GetByID(_ context.Context, id string) (*domain.WorkflowDefinition, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	if r.stored == nil || r.stored.ID != id {
		return nil, domain.NewNotFoundErr("workflow_definition", id)
	}
	return r.stored, nil
}

func (r *workflowDefinitionRepoStub) List(_ context.Context, limit int) ([]domain.WorkflowDefinition, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	items := r.list
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *workflowDefinitionRepoStub) Delete(_ context.Context, id string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	if r.stored == nil || r.stored.ID != id {
		return domain.NewNotFoundErr("workflow_definition", id)
	}
	r.stored = nil
	filtered := r.list[:0]
	for _, item := range r.list {
		if item.ID != id {
			filtered = append(filtered, item)
		}
	}
	r.list = filtered
	return nil
}

func TestAPIWorkflowsCreateAndList(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	svc := service.NewWorkflowTemplateService(&workflowDefinitionRepoStub{})
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

func TestAPIWorkflowsCreateWithoutIDGeneratesNonEmptyID(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	repo := &workflowDefinitionRepoStub{}
	svc := service.NewWorkflowTemplateService(repo)
	handler := NewAPIWorkflowsHandler(svc)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/workflows", handler.Create)

	body := `{"name":"Default workflow","description":"Mainline graph","version":"v1","enabled":true,"entry_node_id":"start-1","nodes":[{"id":"start-1","type":"start","name":"Start","config_json":"{}"}],"edges":[],"updated_by":"tester"}`
	req := httptest.NewRequest(http.MethodPost, "/api/workflows", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var created domain.WorkflowDefinition
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.NotEmpty(t, created.ID)
	require.NotEqual(t, "", repo.stored.ID)
}

func TestAPIWorkflowsCreateWithExistingExplicitIDReturnsConflict(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	repo := &workflowDefinitionRepoStub{createErr: domain.NewConflictErr("workflow definition already exists")}
	svc := service.NewWorkflowTemplateService(repo)
	handler := NewAPIWorkflowsHandler(svc)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/workflows", handler.Create)

	body := `{"id":"wf-1","name":"Replacement workflow","description":"Updated graph","version":"v2","enabled":false,"entry_node_id":"start-1","nodes":[{"id":"start-1","type":"start","name":"Start","config_json":"{}"}],"edges":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/workflows", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code)
	require.Contains(t, w.Body.String(), domain.ErrConflict)
	require.NotNil(t, repo.created)
	require.Equal(t, "wf-1", repo.created.ID)
	require.Nil(t, repo.stored)
	require.Empty(t, repo.list)
}

func TestAPIWorkflowsCreateWithExplicitNewIDStillSucceeds(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	repo := &workflowDefinitionRepoStub{}
	svc := service.NewWorkflowTemplateService(repo)
	handler := NewAPIWorkflowsHandler(svc)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/workflows", handler.Create)

	body := `{"id":"wf-new","name":"Default workflow","description":"Mainline graph","version":"v1","enabled":true,"entry_node_id":"start-1","nodes":[{"id":"start-1","type":"start","name":"Start","config_json":"{}"}],"edges":[],"updated_by":"tester"}`
	req := httptest.NewRequest(http.MethodPost, "/api/workflows", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var created domain.WorkflowDefinition
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.Equal(t, "wf-new", created.ID)
	require.Equal(t, "wf-new", repo.stored.ID)
}

func TestAPIWorkflowsGetAndUpdate(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	repo := &workflowDefinitionRepoStub{}
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

func TestAPIWorkflowsCreatePreservesFallbackEdgeConditionAndPriority(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	repo := &workflowDefinitionRepoStub{}
	svc := service.NewWorkflowTemplateService(repo)
	handler := NewAPIWorkflowsHandler(svc)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/workflows", handler.Create)
	router.GET("/api/workflows/:id", handler.Get)

	body := `{"id":"wf-fallback","name":"Fallback workflow","description":"Graph with implicit fallback edge","version":"v1","enabled":true,"entry_node_id":"start-1","nodes":[{"id":"start-1","type":"start","name":"Start","config_json":"{}"},{"id":"next-1","type":"action","name":"Next","config_json":"{}"}],"edges":[{"from_node_id":"start-1","to_node_id":"next-1","condition":"","priority":0}],"updated_by":"tester"}`
	req := httptest.NewRequest(http.MethodPost, "/api/workflows", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var created domain.WorkflowDefinition
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.Len(t, created.Edges, 1)
	require.Equal(t, "", created.Edges[0].Condition)
	require.Equal(t, 0, created.Edges[0].Priority)

	req = httptest.NewRequest(http.MethodGet, "/api/workflows/wf-fallback", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var stored domain.WorkflowDefinition
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &stored))
	require.Len(t, stored.Edges, 1)
	require.Equal(t, "", stored.Edges[0].Condition)
	require.Equal(t, 0, stored.Edges[0].Priority)
}

func TestAPIWorkflowsCreateDeleteAndGetReturnsNotFound(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	repo := &workflowDefinitionRepoStub{}
	svc := service.NewWorkflowTemplateService(repo)
	handler := NewAPIWorkflowsHandler(svc)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/workflows", handler.Create)
	router.DELETE("/api/workflows/:id", handler.Delete)
	router.GET("/api/workflows/:id", handler.Get)
	router.GET("/api/workflows", handler.List)

	body := `{"id":"wf-1","name":"Default workflow","description":"Mainline graph","version":"v1","enabled":true,"entry_node_id":"start-1","nodes":[{"id":"start-1","type":"start","name":"Start","config_json":"{}"}],"edges":[],"updated_by":"tester"}`
	req := httptest.NewRequest(http.MethodPost, "/api/workflows", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	req = httptest.NewRequest(http.MethodDelete, "/api/workflows/wf-1", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusNoContent, w.Code)

	req = httptest.NewRequest(http.MethodGet, "/api/workflows/wf-1", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), domain.ErrNotFound)

	req = httptest.NewRequest(http.MethodGet, "/api/workflows", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var listResp struct {
		Data []domain.WorkflowDefinition `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listResp))
	require.Empty(t, listResp.Data)
}

func TestAPIWorkflowsUpdateMissingReturnsNotFound(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	repo := &workflowDefinitionRepoStub{}
	svc := service.NewWorkflowTemplateService(repo)
	handler := NewAPIWorkflowsHandler(svc)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.PUT("/api/workflows/:id", handler.Update)

	body := `{"name":"Updated workflow","description":"Updated graph","version":"v2","enabled":false,"entry_node_id":"start-1","nodes":[{"id":"start-1","type":"start","name":"Start","config_json":"{}"}],"edges":[]}`
	req := httptest.NewRequest(http.MethodPut, "/api/workflows/wf-missing", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), domain.ErrNotFound)
	require.Nil(t, repo.stored)
	require.Empty(t, repo.list)
}

func TestAPIWorkflowsDeleteThenUpdateReturnsNotFound(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	repo := &workflowDefinitionRepoStub{}
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
	router.DELETE("/api/workflows/:id", handler.Delete)
	router.PUT("/api/workflows/:id", handler.Update)

	req := httptest.NewRequest(http.MethodDelete, "/api/workflows/wf-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusNoContent, w.Code)

	body := `{"name":"Updated workflow","description":"Updated graph","version":"v2","enabled":false,"entry_node_id":"start-1","nodes":[{"id":"start-1","type":"start","name":"Start","config_json":"{}"}],"edges":[]}`
	req = httptest.NewRequest(http.MethodPut, "/api/workflows/wf-1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), domain.ErrNotFound)
	require.Nil(t, repo.stored)
	require.Empty(t, repo.list)
}
