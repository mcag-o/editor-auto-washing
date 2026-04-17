package handlers_test

import (
	"content-hub/domain"
	"content-hub/service"
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

func TestRSSRunsHandler_RunSubscriptionAndRunAll(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	svc := &rssRunsServiceStub{}
	handler := handlers.NewRSSRunsHandler(svc)
	router.POST("/rss/subscriptions/:id/run", handler.RunSubscription)
	router.POST("/rss/run-all", handler.RunAll)

	req := httptest.NewRequest(http.MethodPost, "/rss/subscriptions/sub-1/run", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "sub-1", svc.lastRunByID)
	assert.Contains(t, w.Body.String(), "run-sub-1")

	req = httptest.NewRequest(http.MethodPost, "/rss/run-all", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, 1, svc.runAllCalls)
	assert.Contains(t, w.Body.String(), "sub-1")
	assert.Contains(t, w.Body.String(), "run-sub-1")
}

func TestRSSRunsHandler_ListAndGet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	svc := &rssRunsServiceStub{}
	handler := handlers.NewRSSRunsHandler(svc)
	router.GET("/rss/runs", handler.List)
	router.GET("/rss/runs/:id", handler.Get)

	req := httptest.NewRequest(http.MethodGet, "/rss/runs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, svc.listCalls)
	assert.Contains(t, w.Body.String(), "run-1")
	assert.Contains(t, w.Body.String(), domain.RSSPullRunStatusSucceeded)

	req = httptest.NewRequest(http.MethodGet, "/rss/runs/run-1", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "run-1", svc.lastGetID)
	assert.Contains(t, w.Body.String(), "run-1")
}

func TestRSSRunsHandler_ListRejectsInvalidLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	svc := &rssRunsServiceStub{}
	handler := handlers.NewRSSRunsHandler(svc)
	router.GET("/rss/runs", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/rss/runs?limit=abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid limit")
	assert.Zero(t, svc.listCalls)
}

type rssRunsServiceStub struct {
	lastRunByID string
	runAllCalls int
	listCalls   int
	lastGetID   string
	listErr     error
}

func (s *rssRunsServiceStub) RunByID(_ context.Context, subscriptionID string) (*service.RSSPullResult, error) {
	s.lastRunByID = subscriptionID
	return &service.RSSPullResult{Run: &domain.RSSPullRun{ID: "run-" + subscriptionID, SubscriptionID: subscriptionID, Status: domain.RSSPullRunStatusSucceeded, StartedAt: time.Now().UTC(), Metadata: map[string]any{}}}, nil
}

func (s *rssRunsServiceStub) RunAll(_ context.Context) ([]service.RSSScheduledRunResult, error) {
	s.runAllCalls++
	result, _ := s.RunByID(context.Background(), "sub-1")
	return []service.RSSScheduledRunResult{{SubscriptionID: "sub-1", Result: result, Err: nil}}, nil
}

func (s *rssRunsServiceStub) List(_ context.Context, limit int) ([]domain.RSSPullRun, error) {
	s.listCalls++
	if s.listErr != nil {
		return nil, s.listErr
	}
	if limit <= 0 {
		return nil, errors.New("unexpected non-positive limit")
	}
	now := time.Now().UTC()
	return []domain.RSSPullRun{{ID: "run-1", SubscriptionID: "sub-1", Status: domain.RSSPullRunStatusSucceeded, StartedAt: now, CompletedAt: &now, Metadata: map[string]any{}}}, nil
}

func (s *rssRunsServiceStub) GetByID(_ context.Context, id string) (*domain.RSSPullRun, error) {
	s.lastGetID = id
	now := time.Now().UTC()
	return &domain.RSSPullRun{ID: id, SubscriptionID: "sub-1", Status: domain.RSSPullRunStatusSucceeded, StartedAt: now, CompletedAt: &now, Metadata: map[string]any{}}, nil
}
