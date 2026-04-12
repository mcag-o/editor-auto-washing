package handlers_test

import (
	"content-hub/domain"
	"content-hub/transport/http/handlers"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectorSchedulerHandler_ExposesStatusHealthAndStop(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := handlers.NewCollectorSchedulerHandler(&collectorSchedulerServiceStub{})
	router.GET("/collector/scheduler/status", handler.Status)
	router.GET("/collector/scheduler/health", handler.Health)
	router.POST("/collector/scheduler/stop", handler.Stop)
	router.POST("/collector/scheduler/run-once", handler.RunOnce)
	router.POST("/collector/scheduler/daemon", handler.Daemon)

	req := httptest.NewRequest(http.MethodGet, "/collector/scheduler/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), domain.CollectorSchedulerIdle)

	req = httptest.NewRequest(http.MethodGet, "/collector/scheduler/health", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "healthy")

	req = httptest.NewRequest(http.MethodPost, "/collector/scheduler/stop", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "operator request")

	req = httptest.NewRequest(http.MethodPost, "/collector/scheduler/run-once", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "run-1")

	req = httptest.NewRequest(http.MethodPost, "/collector/scheduler/daemon", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), domain.CollectorSchedulerRunning)
}

type collectorSchedulerServiceStub struct{}

func (s *collectorSchedulerServiceStub) RunOnce(_ context.Context) (*domain.CollectorRunSummary, error) {
	return &domain.CollectorRunSummary{RunID: "run-1", Trigger: "manual", Status: domain.CollectorRunSucceeded, SourceCount: 1, SuccessfulSources: 1, EntryCount: 2}, nil
}

func (s *collectorSchedulerServiceStub) StartDaemon(_ context.Context) (*domain.CollectorSchedulerControlResult, error) {
	return &domain.CollectorSchedulerControlResult{Started: true, State: domain.CollectorSchedulerRunning, UpdatedAt: time.Now().UTC()}, nil
}

func (s *collectorSchedulerServiceStub) Status(_ context.Context) (*domain.CollectorSchedulerStatus, error) {
	return &domain.CollectorSchedulerStatus{Name: domain.DefaultCollectorSchedulerName, State: domain.CollectorSchedulerIdle, UpdatedAt: time.Now().UTC()}, nil
}

func (s *collectorSchedulerServiceStub) Health(_ context.Context) (*domain.CollectorSchedulerHealthReport, error) {
	return &domain.CollectorSchedulerHealthReport{Status: "healthy", Checks: map[string]string{"state": domain.CollectorSchedulerIdle}, UpdatedAt: time.Now().UTC()}, nil
}

func (s *collectorSchedulerServiceStub) Stop(_ context.Context) (*domain.CollectorSchedulerControlResult, error) {
	return &domain.CollectorSchedulerControlResult{Stopped: true, State: domain.CollectorSchedulerStopped, Reason: "operator request", UpdatedAt: time.Now().UTC()}, nil
}
