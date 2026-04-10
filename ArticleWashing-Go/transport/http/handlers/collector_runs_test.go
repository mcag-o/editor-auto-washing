package handlers_test

import (
	"content-hub/domain"
	"content-hub/transport/http/handlers"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectorRunsHandler_ListsAndGetsRun(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := handlers.NewCollectorRunsHandler(&collectorRunsServiceStub{})
	router.GET("/collector/runs", handler.List)
	router.GET("/collector/runs/:id", handler.Get)

	req := httptest.NewRequest(http.MethodGet, "/collector/runs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "run-1")
	assert.Contains(t, w.Body.String(), domain.CollectorRunSucceeded)

	req = httptest.NewRequest(http.MethodGet, "/collector/runs/run-1", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "source_runs")
	assert.Contains(t, w.Body.String(), "baidu")
}

func TestCollectorRunsHandler_ListRejectsInvalidLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	service := &collectorRunsServiceStub{}
	handler := handlers.NewCollectorRunsHandler(service)
	router.GET("/collector/runs", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/collector/runs?limit=abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid limit")
	assert.Zero(t, service.listCalls)
}

func TestCollectorRunsHandler_ListRejectsNegativeLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	service := &collectorRunsServiceStub{}
	handler := handlers.NewCollectorRunsHandler(service)
	router.GET("/collector/runs", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/collector/runs?limit=-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid limit")
	assert.Zero(t, service.listCalls)
}

type collectorRunsServiceStub struct {
	listCalls int
	listErr   error
}

func (s *collectorRunsServiceStub) ListRuns(_ context.Context, limit int) ([]domain.CollectorRun, error) {
	s.listCalls++
	if s.listErr != nil {
		return nil, s.listErr
	}
	if limit <= 0 {
		return nil, errors.New("unexpected non-positive limit")
	}
	now := time.Now().UTC()
	return []domain.CollectorRun{{ID: "run-1", Trigger: "manual", Status: domain.CollectorRunSucceeded, CreatedAt: now, UpdatedAt: now}}, nil
}

func (s *collectorRunsServiceStub) GetRun(_ context.Context, runID string) (*domain.CollectorRunDetail, error) {
	now := time.Now().UTC()
	return &domain.CollectorRunDetail{
		Run:        domain.CollectorRun{ID: runID, Trigger: "manual", Status: domain.CollectorRunSucceeded, CreatedAt: now, UpdatedAt: now},
		SourceRuns: []domain.CollectorSourceRun{{ID: "sr-1", RunID: runID, SourceID: "baidu", Stage: domain.CollectorStageHotlist, Status: domain.CollectorSourceRunSucceeded, CreatedAt: now, UpdatedAt: now}},
	}, nil
}
