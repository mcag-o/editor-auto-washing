package handlers

import (
	"bytes"
	"content-hub/service"
	"content-hub/transport/http/middleware"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAPIConfigGetReturnsStoredBusinessConfigCategory(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	svc := service.NewBusinessConfigService(&stubBusinessConfigRepo{})
	require.NoError(t, svc.SetJSON(t.Context(), "web_control", "settings", []byte(`{"default_target_type":"wechat-longform"}`), "seed"))

	handler := NewAPIConfigHandler(svc)
	router := gin.New()
	router.Use(middleware.TraceID())
	router.GET("/api/config", handler.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "wechat-longform", resp["default_target_type"])
}

func TestAPIConfigPutUpsertsBusinessConfigCategory(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	svc := service.NewBusinessConfigService(&stubBusinessConfigRepo{})
	handler := NewAPIConfigHandler(svc)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.PUT("/api/config", handler.Update)

	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewBufferString(`{"default_target_type":"wechat-longform","auto_resume":true}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	stored, err := svc.Get(t.Context(), "web_control", "settings")
	require.NoError(t, err)
	require.JSONEq(t, `{"default_target_type":"wechat-longform","auto_resume":true}`, string(stored.ValueJSON))
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, true, resp["auto_resume"])
}
