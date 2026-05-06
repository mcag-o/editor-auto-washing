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

type templateDefinitionRepoStub struct {
	stored    *domain.TemplateDefinition
	list      []domain.TemplateDefinition
	upsertErr error
	getErr    error
	listErr   error
	deleteErr error
}

func (r *templateDefinitionRepoStub) Upsert(_ context.Context, template *domain.TemplateDefinition) error {
	if r.upsertErr != nil {
		return r.upsertErr
	}
	r.stored = template
	if template != nil {
		replaced := false
		for i := range r.list {
			if r.list[i].ID == template.ID {
				r.list[i] = *template
				replaced = true
				break
			}
		}
		if !replaced {
			r.list = append(r.list, *template)
		}
	}
	return nil
}

func (r *templateDefinitionRepoStub) GetByID(_ context.Context, id string) (*domain.TemplateDefinition, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	if r.stored == nil || r.stored.ID != id {
		return nil, domain.NewNotFoundErr("template_definition", id)
	}
	return r.stored, nil
}

func (r *templateDefinitionRepoStub) List(_ context.Context, limit int) ([]domain.TemplateDefinition, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	items := r.list
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *templateDefinitionRepoStub) Delete(_ context.Context, id string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	if r.stored == nil || r.stored.ID != id {
		return domain.NewNotFoundErr("template_definition", id)
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

func TestAPITemplatesCreateAndList(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	svc := service.NewTemplateDefinitionService(&templateDefinitionRepoStub{})
	handler := NewAPITemplatesHandler(svc)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/templates", handler.Create)
	router.GET("/api/templates", handler.List)

	body := `{"id":"tpl-1","name":"Draft prompt","type":"prompt","version":"v1","enabled":true,"content":"Write a draft for {{title}}","variables_json":{"title":"string"},"updated_by":"tester"}`
	req := httptest.NewRequest(http.MethodPost, "/api/templates", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var created domain.TemplateDefinition
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.Equal(t, "tpl-1", created.ID)
	require.Equal(t, "Draft prompt", created.Name)
	require.JSONEq(t, `{"title":"string"}`, string(created.VariablesJSON))

	req = httptest.NewRequest(http.MethodGet, "/api/templates", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var listResp struct {
		Data []domain.TemplateDefinition `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listResp))
	require.Len(t, listResp.Data, 1)
	require.Equal(t, "tpl-1", listResp.Data[0].ID)
	require.JSONEq(t, `{"title":"string"}`, string(listResp.Data[0].VariablesJSON))
}

func TestAPITemplatesCreateWithoutIDGeneratesNonEmptyID(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	repo := &templateDefinitionRepoStub{}
	svc := service.NewTemplateDefinitionService(repo)
	handler := NewAPITemplatesHandler(svc)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/templates", handler.Create)

	body := `{"name":"Draft prompt","type":"prompt","version":"v1","enabled":true,"content":"Write a draft for {{title}}","variables_json":{"title":"string"},"updated_by":"tester"}`
	req := httptest.NewRequest(http.MethodPost, "/api/templates", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var created domain.TemplateDefinition
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.NotEmpty(t, created.ID)
	require.NotEqual(t, "", repo.stored.ID)
}

func TestAPITemplatesGetAndUpdate(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	repo := &templateDefinitionRepoStub{}
	require.NoError(t, repo.Upsert(t.Context(), &domain.TemplateDefinition{
		ID:            "tpl-1",
		Name:          "Draft prompt",
		Type:          "prompt",
		Version:       "v1",
		Enabled:       true,
		Content:       "Write a draft for {{title}}",
		VariablesJSON: []byte(`{"title":"string"}`),
	}))
	svc := service.NewTemplateDefinitionService(repo)
	handler := NewAPITemplatesHandler(svc)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.GET("/api/templates/:id", handler.Get)
	router.PUT("/api/templates/:id", handler.Update)

	req := httptest.NewRequest(http.MethodGet, "/api/templates/tpl-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var stored domain.TemplateDefinition
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &stored))
	require.Equal(t, "tpl-1", stored.ID)
	require.Equal(t, "Draft prompt", stored.Name)

	body := `{"name":"Repair prompt","type":"stage","version":"v2","enabled":false,"content":"Repair the draft for {{title}}","variables_json":{"title":"string","tone":"string"},"updated_by":"editor"}`
	req = httptest.NewRequest(http.MethodPut, "/api/templates/tpl-1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var updated domain.TemplateDefinition
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &updated))
	require.Equal(t, "tpl-1", updated.ID)
	require.Equal(t, "Repair prompt", updated.Name)
	require.Equal(t, "stage", updated.Type)
	require.Equal(t, "v2", updated.Version)
	require.False(t, updated.Enabled)
	require.JSONEq(t, `{"title":"string","tone":"string"}`, string(updated.VariablesJSON))
}

func TestAPITemplatesCreateDeleteAndGetReturnsNotFound(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	repo := &templateDefinitionRepoStub{}
	svc := service.NewTemplateDefinitionService(repo)
	handler := NewAPITemplatesHandler(svc)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/templates", handler.Create)
	router.DELETE("/api/templates/:id", handler.Delete)
	router.GET("/api/templates/:id", handler.Get)
	router.GET("/api/templates", handler.List)

	body := `{"id":"tpl-1","name":"Draft prompt","type":"prompt","version":"v1","enabled":true,"content":"Write a draft for {{title}}","variables_json":{"title":"string"},"updated_by":"tester"}`
	req := httptest.NewRequest(http.MethodPost, "/api/templates", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	req = httptest.NewRequest(http.MethodDelete, "/api/templates/tpl-1", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusNoContent, w.Code)

	req = httptest.NewRequest(http.MethodGet, "/api/templates/tpl-1", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), domain.ErrNotFound)

	req = httptest.NewRequest(http.MethodGet, "/api/templates", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var listResp struct {
		Data []domain.TemplateDefinition `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listResp))
	require.Empty(t, listResp.Data)
}

func TestAPITemplatesUpdateMissingReturnsNotFound(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	repo := &templateDefinitionRepoStub{}
	svc := service.NewTemplateDefinitionService(repo)
	handler := NewAPITemplatesHandler(svc)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.PUT("/api/templates/:id", handler.Update)

	body := `{"name":"Repair prompt","type":"stage","version":"v2","enabled":false,"content":"Repair the draft for {{title}}","variables_json":{"title":"string","tone":"string"},"updated_by":"editor"}`
	req := httptest.NewRequest(http.MethodPut, "/api/templates/tpl-missing", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), domain.ErrNotFound)
	require.Nil(t, repo.stored)
	require.Empty(t, repo.list)
}

func TestAPITemplatesDeleteThenUpdateReturnsNotFound(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	repo := &templateDefinitionRepoStub{}
	require.NoError(t, repo.Upsert(t.Context(), &domain.TemplateDefinition{
		ID:            "tpl-1",
		Name:          "Draft prompt",
		Type:          "prompt",
		Version:       "v1",
		Enabled:       true,
		Content:       "Write a draft for {{title}}",
		VariablesJSON: []byte(`{"title":"string"}`),
	}))
	svc := service.NewTemplateDefinitionService(repo)
	handler := NewAPITemplatesHandler(svc)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.DELETE("/api/templates/:id", handler.Delete)
	router.PUT("/api/templates/:id", handler.Update)

	req := httptest.NewRequest(http.MethodDelete, "/api/templates/tpl-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusNoContent, w.Code)

	body := `{"name":"Repair prompt","type":"stage","version":"v2","enabled":false,"content":"Repair the draft for {{title}}","variables_json":{"title":"string","tone":"string"},"updated_by":"editor"}`
	req = httptest.NewRequest(http.MethodPut, "/api/templates/tpl-1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), domain.ErrNotFound)
	require.Nil(t, repo.stored)
	require.Empty(t, repo.list)
}
