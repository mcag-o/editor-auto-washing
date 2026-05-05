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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAPIArticlesListReturnsSourceDocuments(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	sourceRepo := &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}}
	doc := domain.NewSourceDocument("article.md", "article.md", "md", "Title", "Body", "hash-1")
	doc.Status = domain.SourceDocumentStatusPending
	require.NoError(t, sourceRepo.Create(t.Context(), doc))

	handler := NewAPIArticlesHandler(
		service.NewArticleQueryService(sourceRepo),
		&stubRewritePipelineRunRepo{},
		&stubRewriteStageRunRepo{},
		sourceRepo,
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.GET("/api/articles", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/api/articles", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data []domain.SourceDocument `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 1)
	require.Equal(t, doc.ID, resp.Data[0].ID)
}

func TestAPIArticlesGetDetailReturnsSourceDocument(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	sourceRepo := &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}}
	doc := domain.NewSourceDocument("article.md", "article.md", "md", "Title", "Body", "hash-1")
	doc.Status = domain.SourceDocumentStatusCompleted
	require.NoError(t, sourceRepo.Create(t.Context(), doc))

	handler := NewAPIArticlesHandler(
		service.NewArticleQueryService(sourceRepo),
		&stubRewritePipelineRunRepo{},
		&stubRewriteStageRunRepo{},
		sourceRepo,
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.GET("/api/articles/:id", handler.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/articles/"+doc.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp domain.SourceDocument
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, doc.ID, resp.ID)
	require.Equal(t, domain.SourceDocumentStatusCompleted, resp.Status)
}

func TestAPIArticlesStagesReturnsSourceAndRewriteStages(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	sourceRepo := &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}}
	runRepo := &stubRewritePipelineRunRepo{}
	stageRepo := &stubRewriteStageRunRepo{}
	doc := domain.NewSourceDocument("article.md", "article.md", "md", "Title", "Body", "hash-1")
	doc.Status = domain.SourceDocumentStatusProcessing
	doc.RewriteRunID = "run-1"
	require.NoError(t, sourceRepo.Create(t.Context(), doc))

	run := domain.NewRewritePipelineRun("profile-1", "v1", "workspace-1", "collector-1", "wechat-longform", "sspai")
	run.ID = "run-1"
	run.Status = domain.RewriteRunRunning
	run.CurrentStage = "draft"
	require.NoError(t, runRepo.Create(t.Context(), run))

	stage := &domain.RewriteStageRun{
		ID:            "stage-1",
		PipelineRunID: run.ID,
		StageName:     "draft",
		StageType:     "generate",
		Status:        domain.RewriteStageRunning,
		Attempt:       1,
		StartedAt:     time.Now().UTC(),
	}
	require.NoError(t, stageRepo.Create(t.Context(), stage))

	handler := NewAPIArticlesHandler(
		service.NewArticleQueryService(sourceRepo),
		runRepo,
		stageRepo,
		sourceRepo,
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.GET("/api/articles/:id/stages", handler.Stages)

	req := httptest.NewRequest(http.MethodGet, "/api/articles/"+doc.ID+"/stages", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Article domain.SourceDocument    `json:"article"`
		Run     *domain.RewritePipelineRun `json:"run"`
		Stages  []domain.RewriteStageRun `json:"stages"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, doc.ID, resp.Article.ID)
	require.NotNil(t, resp.Run)
	require.Equal(t, run.ID, resp.Run.ID)
	require.Len(t, resp.Stages, 1)
	require.Equal(t, stage.ID, resp.Stages[0].ID)
}

func TestAPIArticlesRetryResetsFailedDocumentToPending(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	sourceRepo := &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}}
	doc := domain.NewSourceDocument("article.md", "article.md", "md", "Title", "Body", "hash-1")
	doc.Status = domain.SourceDocumentStatusFailed
	doc.ErrorSummary = "broken"
	require.NoError(t, sourceRepo.Create(t.Context(), doc))

	handler := NewAPIArticlesHandler(
		service.NewArticleQueryService(sourceRepo),
		&stubRewritePipelineRunRepo{},
		&stubRewriteStageRunRepo{},
		sourceRepo,
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/articles/:id/retry", handler.Retry)

	req := httptest.NewRequest(http.MethodPost, "/api/articles/"+doc.ID+"/retry", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	stored, err := sourceRepo.GetByID(t.Context(), doc.ID)
	require.NoError(t, err)
	require.Equal(t, domain.SourceDocumentStatusPending, stored.Status)
	require.Empty(t, stored.ErrorSummary)
	require.Empty(t, stored.ClaimedBy)
	require.Empty(t, stored.RewriteRunID)
}
