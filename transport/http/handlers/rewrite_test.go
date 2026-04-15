package handlers

import (
	"bytes"
	"content-hub/domain"
	"content-hub/service"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRewriteHandlerRunReturnsCreated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	runner := &stubRewriteRunner{
		run: &domain.RewritePipelineRun{
			ID:                 "run-1",
			ProfileID:          "profile-1",
			ProfileVersion:     "v1",
			WorkspaceArticleID: "article-1",
			CollectorArticleID: "collector-1",
			TargetType:         "wechat-longform",
			SourceProfile:      "sspai",
			Status:             domain.RewriteRunRunning,
			StartedAt:          time.Unix(1710000000, 0).UTC(),
			Metadata:           map[string]any{"title": "Source"},
		},
	}
	h := NewRewriteHandler(runner)
	router := gin.New()
	router.POST("/rewrite/runs", h.Run)

	req := httptest.NewRequest(http.MethodPost, "/rewrite/runs", bytes.NewBufferString(`{"workspace_article_id":"article-1","collector_article_id":"collector-1","title":"Source","target_type":"wechat-longform","source_profile":"sspai","version":"v1"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusCreated, resp.Code)
	require.Equal(t, service.RewriteRunRequest{
		WorkspaceArticleID: "article-1",
		CollectorArticleID: "collector-1",
		Title:              "Source",
		TargetType:         "wechat-longform",
		SourceProfile:      "sspai",
		Version:            "v1",
	}, runner.lastReq)

	var run domain.RewritePipelineRun
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &run))
	assert.Equal(t, "run-1", run.ID)
	assert.Equal(t, domain.RewriteRunRunning, run.Status)
	assert.Equal(t, "Source", run.Metadata["title"])
}

func TestRewriteHandlerRunMapsServiceErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewRewriteHandler(&stubRewriteRunner{err: domain.NewValidationErr("invalid rewrite request", nil)})
	router := gin.New()
	router.POST("/rewrite/runs", h.Run)

	req := httptest.NewRequest(http.MethodPost, "/rewrite/runs", bytes.NewBufferString(`{"workspace_article_id":"article-1","collector_article_id":"collector-1","title":"Source","target_type":"wechat-longform","source_profile":"sspai","version":"v1"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Contains(t, resp.Body.String(), "invalid rewrite request")
	assert.Contains(t, resp.Body.String(), string(domain.ErrValidation))
}

func TestRewriteHandlerRunReturnsBadRequestForMalformedJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	runner := &stubRewriteRunner{}
	h := NewRewriteHandler(runner)
	router := gin.New()
	router.POST("/rewrite/runs", h.Run)

	req := httptest.NewRequest(http.MethodPost, "/rewrite/runs", bytes.NewBufferString(`{"workspace_article_id":"article-1",`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Contains(t, resp.Body.String(), "error")
	require.Equal(t, service.RewriteRunRequest{}, runner.lastReq)
}

func TestRewriteRunRequestHasHTTPBindingTags(t *testing.T) {
	type fieldExpectation struct {
		name    string
		jsonTag string
	}

	expected := []fieldExpectation{
		{name: "WorkspaceArticleID", jsonTag: "workspace_article_id"},
		{name: "CollectorArticleID", jsonTag: "collector_article_id"},
		{name: "Title", jsonTag: "title"},
		{name: "TargetType", jsonTag: "target_type"},
		{name: "SourceProfile", jsonTag: "source_profile"},
		{name: "Version", jsonTag: "version"},
	}

	typ := reflect.TypeOf(service.RewriteRunRequest{})
	for _, field := range expected {
		sf, ok := typ.FieldByName(field.name)
		require.True(t, ok)
		assert.Equal(t, field.jsonTag, sf.Tag.Get("json"))
		assert.Equal(t, "required", sf.Tag.Get("binding"))
	}
}

type stubRewriteRunner struct {
	err     error
	lastReq service.RewriteRunRequest
	run     *domain.RewritePipelineRun
}

func (s *stubRewriteRunner) Run(_ context.Context, req service.RewriteRunRequest) (*domain.RewritePipelineRun, error) {
	s.lastReq = req
	if s.err != nil {
		return nil, s.err
	}
	return s.run, nil
}
