package handlers_test

import (
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

func TestCollectorSourcesHandler_ListsSourcesAndHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := handlers.NewCollectorSourcesHandler(&collectorSourcesServiceStub{})
	router.GET("/collector/sources", handler.List)
	router.GET("/collector/sources/health", handler.Health)

	req := httptest.NewRequest(http.MethodGet, "/collector/sources", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "baidu")
	assert.Contains(t, w.Body.String(), "GitHub Trending")

	req = httptest.NewRequest(http.MethodGet, "/collector/sources/health", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var health []domain.CollectorSourceHealthStatus
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &health))
	require.Len(t, health, 1)
	assert.Equal(t, "baidu", health[0].SourceID)
	assert.Equal(t, "Baidu Hotlist", health[0].DisplayName)
	assert.True(t, health[0].Enabled)
	assert.True(t, health[0].Capabilities.SupportsHotlist)
	assert.False(t, health[0].Capabilities.SupportsArticle)
	assert.Equal(t, []string{domain.CollectorAuthModeNone}, health[0].Capabilities.AuthModes)
	assert.True(t, health[0].Health.OK)
	assert.Equal(t, "healthy", health[0].Health.Code)
	assert.Equal(t, "api reachable", health[0].Health.Message)
	assert.False(t, health[0].Health.CheckedAt.IsZero())
}

type collectorSourcesServiceStub struct{}

func (s *collectorSourcesServiceStub) ListSources(_ context.Context) ([]domain.CollectorSource, error) {
	return []domain.CollectorSource{
		{ID: "baidu", DisplayName: "Baidu Hotlist", Enabled: true},
		{ID: "github", DisplayName: "GitHub Trending", Enabled: true},
	}, nil
}

func (s *collectorSourcesServiceStub) Health(_ context.Context) ([]domain.CollectorSourceHealthStatus, error) {
	return []domain.CollectorSourceHealthStatus{{
		SourceID:     "baidu",
		DisplayName:  "Baidu Hotlist",
		Enabled:      true,
		Capabilities: domain.CollectorSourceCapabilities{SupportsHotlist: true, SupportsArticle: false, AuthModes: []string{domain.CollectorAuthModeNone}},
		Health:       domain.CollectorHealthInfo{OK: true, Code: "healthy", Message: "api reachable", CheckedAt: time.Now().UTC()},
	}}, nil
}
