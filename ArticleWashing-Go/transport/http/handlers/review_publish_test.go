package handlers

import (
	"bytes"
	"content-hub/domain"
	"content-hub/infra/memory"
	"content-hub/service"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewHandlerCreateApproveRejectAndList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("draft-1", "Review Draft", "", domain.ArticleWorkspaceSource{}, nil)
	workspace.Status = domain.ArticleWorkspaceStatusRendered
	workspace.StatusHistory = []string{domain.ArticleWorkspaceStatusImported, domain.ArticleWorkspaceStatusDraft, domain.ArticleWorkspaceStatusRendered}
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))
	reviewSvc := service.NewReviewService(provider.ReviewRepo(), provider.WorkspaceRepo())
	handler := NewReviewHandler(reviewSvc)
	router := gin.New()
	router.POST("/reviews", handler.Create)
	router.GET("/reviews", handler.List)
	router.POST("/reviews/:id/approve", handler.Approve)
	router.POST("/reviews/:id/reject", handler.Reject)

	body := bytes.NewBufferString(`{"article_id":"draft-1","asset_ids":["asset-1"],"publish_profile":"wechat-main"}`)
	req := httptest.NewRequest(http.MethodPost, "/reviews", body)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusCreated, resp.Code)

	var created domain.ReviewTask
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &created))
	assert.Equal(t, domain.ReviewStatusPending, created.Status)

	req = httptest.NewRequest(http.MethodPost, "/reviews/"+created.ID+"/approve", bytes.NewBufferString(`{"reviewer":"alice","notes":"ok"}`))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), domain.ReviewStatusApproved)

	workspaceReject := domain.NewArticleWorkspaceRecord("draft-2", "Review Draft 2", "", domain.ArticleWorkspaceSource{}, nil)
	workspaceReject.Status = domain.ArticleWorkspaceStatusRendered
	workspaceReject.StatusHistory = []string{domain.ArticleWorkspaceStatusImported, domain.ArticleWorkspaceStatusDraft, domain.ArticleWorkspaceStatusRendered}
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspaceReject))
	body = bytes.NewBufferString(`{"article_id":"draft-2","asset_ids":["asset-2"],"publish_profile":"wechat-main"}`)
	req = httptest.NewRequest(http.MethodPost, "/reviews", body)
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusCreated, resp.Code)
	var rejectReview domain.ReviewTask
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &rejectReview))

	req = httptest.NewRequest(http.MethodPost, "/reviews/"+rejectReview.ID+"/reject", bytes.NewBufferString(`{"reviewer":"bob","notes":"retry"}`))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), domain.ReviewStatusRejected)

	req = httptest.NewRequest(http.MethodGet, "/reviews?article_id=draft-1", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), created.ID)
	assert.Contains(t, resp.Body.String(), domain.ReviewStatusApproved)

	req = httptest.NewRequest(http.MethodGet, "/reviews?article_id=draft-2", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), rejectReview.ID)
	assert.Contains(t, resp.Body.String(), domain.ReviewStatusRejected)
}

func TestPublishHandlerPublishesReviewAndListsHistory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	provider := memory.NewProvider()
	workspace := domain.NewArticleWorkspaceRecord("draft-1", "HTTP Draft", "", domain.ArticleWorkspaceSource{}, nil)
	workspace.Status = domain.ArticleWorkspaceStatusRendered
	workspace.StatusHistory = []string{domain.ArticleWorkspaceStatusImported, domain.ArticleWorkspaceStatusDraft, domain.ArticleWorkspaceStatusRendered}
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))
	reviewSvc := service.NewReviewService(provider.ReviewRepo(), provider.WorkspaceRepo())
	publisher := &httpPublishProviderStub{}
	gate := service.NewPublishGateService(provider.ReviewRepo(), provider.AssetRepo(), provider.DraftRepo(), provider.PublishRepo(), provider.WorkspaceRepo(), map[string]service.PublisherProvider{"wechat": publisher})
	handler := NewPublishHandler(gate)
	router := gin.New()
	router.POST("/publish", handler.Publish)
	router.GET("/publish/history", handler.History)

	draft := domain.NewArticleDraft("daily")
	draft.Meta["title"] = "HTTP Draft"
	require.NoError(t, provider.DraftRepo().Create(t.Context(), draft))
	workspace = domain.NewArticleWorkspaceRecord(draft.ID, "HTTP Draft", "", domain.ArticleWorkspaceSource{}, nil)
	workspace.Status = domain.ArticleWorkspaceStatusRendered
	workspace.StatusHistory = []string{domain.ArticleWorkspaceStatusImported, domain.ArticleWorkspaceStatusDraft, domain.ArticleWorkspaceStatusRendered}
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))
	asset := domain.NewRenderedAssetRecord(draft.ID, "wechat", "html", "daily", "<html></html>", "")
	asset.Status = domain.AssetStatusReady
	require.NoError(t, provider.AssetRepo().Create(t.Context(), asset))
	review, err := reviewSvc.CreateReview(t.Context(), draft.ID, []string{asset.AssetID}, "wechat-main")
	require.NoError(t, err)
	_, err = reviewSvc.ApproveReview(t.Context(), review.ID, "alice", "ok")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/publish", bytes.NewBufferString(`{"review_id":"`+review.ID+`"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusCreated, resp.Code)
	assert.Contains(t, resp.Body.String(), review.ID)
	assert.Contains(t, resp.Body.String(), draft.ID)

	req = httptest.NewRequest(http.MethodGet, "/publish/history?article_id="+draft.ID, nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), review.ID)
	assert.Contains(t, resp.Body.String(), "remote-1")
	assert.Len(t, publisher.requests, 1)
}

type httpPublishProviderStub struct {
	requests []domain.PublishRequest
}

func (s *httpPublishProviderStub) Publish(_ context.Context, req domain.PublishRequest) (*domain.PublishResult, error) {
	s.requests = append(s.requests, req)
	return &domain.PublishResult{Success: true, Platform: req.Platform, Message: "published", Metadata: map[string]any{"remote_id": "remote-1"}}, nil
}

func (s *httpPublishProviderStub) Platforms() []string {
	return []string{"wechat"}
}
