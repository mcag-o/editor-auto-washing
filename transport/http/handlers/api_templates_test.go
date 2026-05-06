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

func TestAPITemplatesCreateAndList(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	svc := service.NewTemplateDefinitionService(&stubTemplateDefinitionRepo{})
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

func TestAPITemplatesGetAndUpdate(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	repo := &stubTemplateDefinitionRepo{}
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
