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

func TestRSSItemsHandler_ListAndGet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	svc := &rssItemsServiceStub{}
	handler := handlers.NewRSSItemsHandler(svc)
	router.GET("/rss/items", handler.List)
	router.GET("/rss/items/:id", handler.Get)

	req := httptest.NewRequest(http.MethodGet, "/rss/items", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, svc.listCalls)
	assert.Contains(t, w.Body.String(), "item-1")
	assert.Contains(t, w.Body.String(), domain.RSSItemStatusImported)

	req = httptest.NewRequest(http.MethodGet, "/rss/items/item-1", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "item-1", svc.lastGetID)
	assert.Contains(t, w.Body.String(), "workspace-1")
	assert.Contains(t, w.Body.String(), "item-1")
}

func TestRSSItemsHandler_ListRejectsInvalidLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	svc := &rssItemsServiceStub{}
	handler := handlers.NewRSSItemsHandler(svc)
	router.GET("/rss/items", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/rss/items?limit=0", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid limit")
	assert.Zero(t, svc.listCalls)
}

type rssItemsServiceStub struct {
	listCalls int
	lastGetID string
	listErr   error
}

func (s *rssItemsServiceStub) List(_ context.Context, limit int) ([]domain.RSSItemRecord, error) {
	s.listCalls++
	if s.listErr != nil {
		return nil, s.listErr
	}
	if limit <= 0 {
		return nil, errors.New("unexpected non-positive limit")
	}
	now := time.Now().UTC()
	return []domain.RSSItemRecord{{ID: "item-1", SubscriptionID: "sub-1", PullRunID: "run-1", GUID: "guid-1", Link: "https://example.com/post-1", ContentHash: "hash-1", Title: "Item 1", Status: domain.RSSItemStatusImported, WorkspaceArticleID: "workspace-1", RawPayloadJSON: []byte(`{}`), Metadata: map[string]any{}, CreatedAt: now, UpdatedAt: now}}, nil
}

func (s *rssItemsServiceStub) GetByID(_ context.Context, id string) (*domain.RSSItemRecord, error) {
	s.lastGetID = id
	now := time.Now().UTC()
	return &domain.RSSItemRecord{ID: id, SubscriptionID: "sub-1", PullRunID: "run-1", GUID: "guid-1", Link: "https://example.com/post-1", ContentHash: "hash-1", Title: "Item 1", Status: domain.RSSItemStatusImported, WorkspaceArticleID: "workspace-1", RawPayloadJSON: []byte(`{}`), Metadata: map[string]any{}, CreatedAt: now, UpdatedAt: now}, nil
}
