package handlers

import (
	"bytes"
	"content-hub/domain"
	"content-hub/infra/memory"
	"content-hub/service"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormattingHandlerRenderAndGetAsset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	provider := memory.NewProvider()
	draft := buildHandlerDraft()
	require.NoError(t, provider.DraftRepo().Create(t.Context(), draft))
	workspace := domain.NewArticleWorkspaceRecord(draft.ID, "市场快讯", "摘要", domain.ArticleWorkspaceSource{}, nil)
	workspace.Status = domain.ArticleWorkspaceStatusDraft
	workspace.StatusHistory = []string{domain.ArticleWorkspaceStatusDraft}
	require.NoError(t, provider.WorkspaceRepo().Create(t.Context(), workspace))

	handler := NewFormattingHandler(service.NewFormattingPipelineService(provider.DraftRepo(), provider.AssetRepo(), provider.WorkspaceRepo(), &handlerFormatter{}))
	router := gin.New()
	router.POST("/drafts/:id/render", handler.Render)
	router.GET("/assets/:id", handler.GetAsset)

	body := bytes.NewBufferString(`{"platform":"wechat","template":"daily-intelligence"}`)
	req := httptest.NewRequest(http.MethodPost, "/drafts/"+draft.ID+"/render", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var rendered map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &rendered))
	assetID := rendered["asset_id"].(string)

	req = httptest.NewRequest(http.MethodGet, "/assets/"+assetID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "市场快讯")
}

func TestFormattingHandlerValidateReturnsPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	provider := memory.NewProvider()
	draft := buildHandlerDraft()
	require.NoError(t, provider.DraftRepo().Create(t.Context(), draft))
	handler := NewFormattingHandler(service.NewFormattingPipelineService(provider.DraftRepo(), provider.AssetRepo(), provider.WorkspaceRepo(), &handlerFormatter{
		validation: domain.DraftValidationResult{Warnings: []string{"meta.thumb_media_id is missing"}},
	}))
	router := gin.New()
	router.POST("/drafts/:id/validate", handler.Validate)

	body := bytes.NewBufferString(`{"platform":"wechat","template":"daily-intelligence"}`)
	req := httptest.NewRequest(http.MethodPost, "/drafts/"+draft.ID+"/validate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "meta.thumb_media_id is missing")
}

type handlerFormatter struct {
	validation domain.DraftValidationResult
}

func (h *handlerFormatter) Render(draft *domain.ArticleDraft, templateName string) (string, error) {
	return `<html><body><h1>市场快讯</h1></body></html>`, nil
}

func (h *handlerFormatter) ValidateDraft(draft *domain.ArticleDraft, templateName string) domain.DraftValidationResult {
	return h.validation
}

func (h *handlerFormatter) ValidateRenderedOutput(html string) []string {
	return nil
}

func buildHandlerDraft() *domain.ArticleDraft {
	draft := domain.NewArticleDraft("daily-intelligence")
	draft.Meta["title"] = "市场快讯"
	draft.Meta["digest"] = "摘要"
	draft.Meta["author"] = "编辑部"
	draft.Headline["title"] = "头条"
	draft.Headline["body"] = []string{"正文"}
	draft.Sections = []any{map[string]any{"cn": "版块", "blocks": []map[string]any{{"type": "card", "title": "观察", "body": []string{"内容"}}}}}
	return draft
}
