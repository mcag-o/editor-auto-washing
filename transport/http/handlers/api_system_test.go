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

func TestAPISystemStartPauseResumeAndStatus(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	repo := &stubSystemControlStateRepo{}
	handler := NewAPISystemHandler(service.NewControlStateService(repo))

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/system/start", handler.Start)
	router.POST("/api/system/pause", handler.Pause)
	router.POST("/api/system/resume", handler.Resume)
	router.GET("/api/system/status", handler.Status)

	startReq := httptest.NewRequest(http.MethodPost, "/api/system/start", bytes.NewBufferString(`{"concurrency_limit":3}`))
	startReq.Header.Set("Content-Type", "application/json")
	startW := httptest.NewRecorder()
	router.ServeHTTP(startW, startReq)
	require.Equal(t, http.StatusOK, startW.Code)

	statusReq := httptest.NewRequest(http.MethodGet, "/api/system/status", nil)
	statusW := httptest.NewRecorder()
	router.ServeHTTP(statusW, statusReq)
	require.Equal(t, http.StatusOK, statusW.Code)
	var statusResp domain.SystemControlState
	require.NoError(t, json.Unmarshal(statusW.Body.Bytes(), &statusResp))
	require.Equal(t, domain.SystemStateRunning, statusResp.State)
	require.Equal(t, float64(3), statusResp.Metadata["concurrency_limit"])

	pauseReq := httptest.NewRequest(http.MethodPost, "/api/system/pause", bytes.NewBufferString(`{}`))
	pauseReq.Header.Set("Content-Type", "application/json")
	pauseW := httptest.NewRecorder()
	router.ServeHTTP(pauseW, pauseReq)
	require.Equal(t, http.StatusOK, pauseW.Code)

	resumeReq := httptest.NewRequest(http.MethodPost, "/api/system/resume", bytes.NewBufferString(`{}`))
	resumeReq.Header.Set("Content-Type", "application/json")
	resumeW := httptest.NewRecorder()
	router.ServeHTTP(resumeW, resumeReq)
	require.Equal(t, http.StatusOK, resumeW.Code)
	var resumeResp domain.SystemControlState
	require.NoError(t, json.Unmarshal(resumeW.Body.Bytes(), &resumeResp))
	require.Equal(t, domain.SystemStateRunning, resumeResp.State)
	require.Equal(t, "resumed", resumeResp.Reason)
}

func TestAPISystemStartRejectsInvalidConcurrency(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	handler := NewAPISystemHandler(service.NewControlStateService(&stubSystemControlStateRepo{}))
	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/system/start", handler.Start)

	req := httptest.NewRequest(http.MethodPost, "/api/system/start", bytes.NewBufferString(`{"concurrency_limit":0}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "concurrency limit must be greater than zero")
}
