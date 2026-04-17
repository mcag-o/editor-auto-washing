package handlers_test

import (
	"bytes"
	"content-hub/domain"
	"content-hub/transport/http/handlers"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRSSSubscriptionsHandler_CreateAndList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	svc := &rssSubscriptionsServiceStub{}
	handler := handlers.NewRSSSubscriptionsHandler(svc)
	router.POST("/rss/subscriptions", handler.Create)
	router.GET("/rss/subscriptions", handler.List)

	req := httptest.NewRequest(http.MethodPost, "/rss/subscriptions", bytes.NewBufferString(`{"name":"Tech Feed","feed_url":"https://example.com/feed.xml","target_type":"wechat-longform","source_profile":"sspai","rewrite_profile_version":"v1","enabled":true,"poll_interval_sec":900}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, 1, svc.createCalls)
	require.NotNil(t, svc.lastCreated)
	assert.Equal(t, "Tech Feed", svc.lastCreated.Name)
	assert.Equal(t, "https://example.com/feed.xml", svc.lastCreated.FeedURL)
	assert.Equal(t, "wechat-longform", svc.lastCreated.TargetType)
	assert.Equal(t, "sspai", svc.lastCreated.SourceProfile)
	assert.Equal(t, "v1", svc.lastCreated.RewriteProfileVersion)
	assert.Equal(t, 900, svc.lastCreated.PollIntervalSec)
	assert.True(t, svc.lastCreated.Enabled)

	req = httptest.NewRequest(http.MethodGet, "/rss/subscriptions", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, svc.listCalls)
	assert.Contains(t, w.Body.String(), "Tech Feed")
	assert.Contains(t, w.Body.String(), "feed_url")
}

func TestRSSSubscriptionsHandler_CreateAcceptsExplicitZeroPollInterval(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	svc := &rssSubscriptionsServiceStub{}
	handler := handlers.NewRSSSubscriptionsHandler(svc)
	router.POST("/rss/subscriptions", handler.Create)

	req := httptest.NewRequest(http.MethodPost, "/rss/subscriptions", bytes.NewBufferString(`{"name":"Tech Feed","feed_url":"https://example.com/feed.xml","target_type":"wechat-longform","source_profile":"sspai","poll_interval_sec":0}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	require.NotNil(t, svc.lastCreated)
	assert.Equal(t, 0, svc.lastCreated.PollIntervalSec)
}

func TestRSSSubscriptionsHandler_GetUpdateDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	svc := &rssSubscriptionsServiceStub{}
	handler := handlers.NewRSSSubscriptionsHandler(svc)
	router.GET("/rss/subscriptions/:id", handler.Get)
	router.PUT("/rss/subscriptions/:id", handler.Update)
	router.DELETE("/rss/subscriptions/:id", handler.Delete)

	req := httptest.NewRequest(http.MethodGet, "/rss/subscriptions/sub-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "sub-1", svc.lastGetID)
	assert.Contains(t, w.Body.String(), "sub-1")

	req = httptest.NewRequest(http.MethodPut, "/rss/subscriptions/sub-1", bytes.NewBufferString(`{"name":"Updated Feed","feed_url":"https://example.com/updated.xml","target_type":"wechat-longform","source_profile":"sspai","rewrite_profile_version":"v2","enabled":false,"poll_interval_sec":1800}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, svc.updateCalls)
	require.NotNil(t, svc.lastUpdated)
	assert.Equal(t, "sub-1", svc.lastUpdated.ID)
	assert.Equal(t, "Updated Feed", svc.lastUpdated.Name)
	assert.False(t, svc.lastUpdated.Enabled)
	assert.Equal(t, 1800, svc.lastUpdated.PollIntervalSec)

	req = httptest.NewRequest(http.MethodDelete, "/rss/subscriptions/sub-1", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "sub-1", svc.lastDeleteID)
	assert.Contains(t, w.Body.String(), "deleted")
}

func TestRSSSubscriptionsHandler_UpdatePreservesPersistedFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	createdAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	lastPulledAt := time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC)
	svc := &rssSubscriptionsServiceStub{
		getResult: &domain.RSSSubscription{
			ID:                    "sub-1",
			Name:                  "Existing Feed",
			FeedURL:               "https://example.com/feed.xml",
			TargetType:            "wechat-longform",
			SourceProfile:         "sspai",
			RewriteProfileVersion: "v1",
			Enabled:               true,
			PollIntervalSec:       900,
			LastPulledAt:          &lastPulledAt,
			Metadata:              map[string]any{"existing": "value"},
			CreatedAt:             createdAt,
			UpdatedAt:             createdAt,
		},
	}
	handler := handlers.NewRSSSubscriptionsHandler(svc)
	router.PUT("/rss/subscriptions/:id", handler.Update)

	req := httptest.NewRequest(http.MethodPut, "/rss/subscriptions/sub-1", bytes.NewBufferString(`{"name":"Updated Feed","feed_url":"https://example.com/updated.xml","target_type":"wechat-longform","source_profile":"sspai","rewrite_profile_version":"v2","enabled":false}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "sub-1", svc.lastGetID)
	require.NotNil(t, svc.lastUpdated)
	assert.Equal(t, createdAt, svc.lastUpdated.CreatedAt)
	require.NotNil(t, svc.lastUpdated.LastPulledAt)
	assert.Equal(t, lastPulledAt, *svc.lastUpdated.LastPulledAt)
	assert.Equal(t, map[string]any{"existing": "value"}, svc.lastUpdated.Metadata)
	assert.Equal(t, "sub-1", svc.lastUpdated.ID)
	assert.Equal(t, "Updated Feed", svc.lastUpdated.Name)
	assert.Equal(t, "v2", svc.lastUpdated.RewriteProfileVersion)
	assert.Equal(t, 900, svc.lastUpdated.PollIntervalSec)
}

func TestRSSSubscriptionsHandler_UpdateAcceptsExplicitZeroPollInterval(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	svc := &rssSubscriptionsServiceStub{
		getResult: &domain.RSSSubscription{
			ID:              "sub-1",
			Name:            "Existing Feed",
			FeedURL:         "https://example.com/feed.xml",
			TargetType:      "wechat-longform",
			SourceProfile:   "sspai",
			Enabled:         true,
			PollIntervalSec: 900,
			Metadata:        map[string]any{"existing": "value"},
			CreatedAt:       time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
			UpdatedAt:       time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
		},
	}
	handler := handlers.NewRSSSubscriptionsHandler(svc)
	router.PUT("/rss/subscriptions/:id", handler.Update)

	req := httptest.NewRequest(http.MethodPut, "/rss/subscriptions/sub-1", bytes.NewBufferString(`{"name":"Updated Feed","feed_url":"https://example.com/updated.xml","target_type":"wechat-longform","source_profile":"sspai","poll_interval_sec":0}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, svc.lastUpdated)
	assert.Equal(t, 0, svc.lastUpdated.PollIntervalSec)
}

func TestRSSSubscriptionsHandler_MapsValidationErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	svc := &rssSubscriptionsServiceStub{createErr: domain.NewValidationErr("invalid subscription", nil)}
	handler := handlers.NewRSSSubscriptionsHandler(svc)
	router.POST("/rss/subscriptions", handler.Create)

	req := httptest.NewRequest(http.MethodPost, "/rss/subscriptions", bytes.NewBufferString(`{"name":"Tech Feed","feed_url":"https://example.com/feed.xml","target_type":"wechat-longform","source_profile":"sspai"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid subscription")
	assert.Contains(t, w.Body.String(), string(domain.ErrValidation))
}

func TestRSSSubscriptionsHandler_ReturnsBadRequestForMalformedJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	svc := &rssSubscriptionsServiceStub{}
	handler := handlers.NewRSSSubscriptionsHandler(svc)
	router.POST("/rss/subscriptions", handler.Create)

	req := httptest.NewRequest(http.MethodPost, "/rss/subscriptions", bytes.NewBufferString(`{"name":"Tech Feed",`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "error")
	assert.Zero(t, svc.createCalls)
}

type rssSubscriptionsServiceStub struct {
	createCalls  int
	listCalls    int
	updateCalls  int
	lastGetID    string
	lastDeleteID string
	lastCreated  *domain.RSSSubscription
	lastUpdated  *domain.RSSSubscription
	createErr    error
	getResult    *domain.RSSSubscription
}

func (s *rssSubscriptionsServiceStub) Create(_ context.Context, sub *domain.RSSSubscription) (*domain.RSSSubscription, error) {
	s.createCalls++
	if s.createErr != nil {
		return nil, s.createErr
	}
	clone := cloneRSSSubscription(sub)
	s.lastCreated = clone
	return clone, nil
}

func (s *rssSubscriptionsServiceStub) Get(_ context.Context, id string) (*domain.RSSSubscription, error) {
	s.lastGetID = id
	if s.getResult != nil {
		return cloneRSSSubscription(s.getResult), nil
	}
	now := time.Now().UTC()
	return &domain.RSSSubscription{ID: id, Name: "Tech Feed", FeedURL: "https://example.com/feed.xml", TargetType: "wechat-longform", SourceProfile: "sspai", Enabled: true, PollIntervalSec: 900, Metadata: map[string]any{}, CreatedAt: now, UpdatedAt: now}, nil
}

func (s *rssSubscriptionsServiceStub) List(_ context.Context) ([]domain.RSSSubscription, error) {
	s.listCalls++
	now := time.Now().UTC()
	return []domain.RSSSubscription{{ID: "sub-1", Name: "Tech Feed", FeedURL: "https://example.com/feed.xml", TargetType: "wechat-longform", SourceProfile: "sspai", Enabled: true, PollIntervalSec: 900, Metadata: map[string]any{}, CreatedAt: now, UpdatedAt: now}}, nil
}

func (s *rssSubscriptionsServiceStub) Update(_ context.Context, sub *domain.RSSSubscription) (*domain.RSSSubscription, error) {
	s.updateCalls++
	clone := cloneRSSSubscription(sub)
	s.lastUpdated = clone
	return clone, nil
}

func (s *rssSubscriptionsServiceStub) Delete(_ context.Context, id string) error {
	s.lastDeleteID = id
	return nil
}

func cloneRSSSubscription(sub *domain.RSSSubscription) *domain.RSSSubscription {
	if sub == nil {
		return nil
	}
	clone := *sub
	if sub.Metadata != nil {
		data, _ := json.Marshal(sub.Metadata)
		var copied map[string]any
		_ = json.Unmarshal(data, &copied)
		clone.Metadata = copied
	}
	return &clone
}
