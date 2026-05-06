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
		&stubWorkflowDefinitionRepo{},
		&stubAuditLogRepo{},
		&stubSystemControlStateRepo{state: startedControlState("runner", domain.SystemStateRunning, "started", 2)},
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
		&stubWorkflowDefinitionRepo{},
		&stubAuditLogRepo{},
		&stubSystemControlStateRepo{},
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
		&stubWorkflowDefinitionRepo{},
		&stubAuditLogRepo{},
		&stubSystemControlStateRepo{},
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.GET("/api/articles/:id/stages", handler.Stages)

	req := httptest.NewRequest(http.MethodGet, "/api/articles/"+doc.ID+"/stages", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Article domain.SourceDocument      `json:"article"`
		Run     *domain.RewritePipelineRun `json:"run"`
		Stages  []domain.RewriteStageRun   `json:"stages"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, doc.ID, resp.Article.ID)
	require.NotNil(t, resp.Run)
	require.Equal(t, run.ID, resp.Run.ID)
	require.Len(t, resp.Stages, 1)
	require.Equal(t, stage.ID, resp.Stages[0].ID)
}

func TestAPIArticlesRetryRequeuesFailedDocumentWhenSystemRunning(t *testing.T) {
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
		&stubWorkflowDefinitionRepo{},
		&stubAuditLogRepo{},
		&stubSystemControlStateRepo{state: startedControlState("runner", domain.SystemStateRunning, "started", 2)},
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/articles/:id/retry", handler.Retry)

	req := httptest.NewRequest(http.MethodPost, "/api/articles/"+doc.ID+"/retry", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "requeued", resp["status"])
	require.Equal(t, true, resp["worker_running"])
	require.Contains(t, resp["message"], "re-queued")
	stored, err := sourceRepo.GetByID(t.Context(), doc.ID)
	require.NoError(t, err)
	require.Equal(t, domain.SourceDocumentStatusPending, stored.Status)
	require.Empty(t, stored.ErrorSummary)
	require.Empty(t, stored.ClaimedBy)
	require.Empty(t, stored.RewriteRunID)
}

func TestAPIArticlesRetryFromNonRetryableStateFails(t *testing.T) {
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
		&stubWorkflowDefinitionRepo{},
		&stubAuditLogRepo{},
		&stubSystemControlStateRepo{},
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/articles/:id/retry", handler.Retry)

	req := httptest.NewRequest(http.MethodPost, "/api/articles/"+doc.ID+"/retry", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "retry is only allowed from failed state")
	stored, err := sourceRepo.GetByID(t.Context(), doc.ID)
	require.NoError(t, err)
	require.Equal(t, domain.SourceDocumentStatusPending, stored.Status)
}

func TestAPIArticlesRetryWhenSystemPausedSignalsQueuedNotRunning(t *testing.T) {
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
		&stubWorkflowDefinitionRepo{},
		&stubAuditLogRepo{},
		&stubSystemControlStateRepo{state: startedControlState("operator", domain.SystemStatePaused, "paused", 2)},
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/articles/:id/retry", handler.Retry)

	req := httptest.NewRequest(http.MethodPost, "/api/articles/"+doc.ID+"/retry", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "requeued", resp["status"])
	require.Equal(t, false, resp["worker_running"])
	require.Equal(t, domain.SystemStatePaused, resp["system_state"])
	require.Contains(t, resp["message"], "worker is not actively running")
}

func TestAPIArticlesRetryResetsPreviousWorkflowExecutionForFreshRun(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	sourceRepo := &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}}
	runRepo := &stubRewritePipelineRunRepo{}
	stageRepo := &stubRewriteStageRunRepo{}
	doc := domain.NewSourceDocument("article.md", "article.md", "md", "Title", "Body", "hash-1")
	doc.Status = domain.SourceDocumentStatusFailed
	doc.ErrorSummary = "broken"
	doc.RewriteRunID = "run-1"
	doc.Metadata["workflow_template_id"] = "wf-1"
	doc.Metadata["workflow_template_version"] = "v2"
	now := time.Now().UTC()
	doc.ClaimedBy = "worker-1"
	doc.ClaimedAt = &now
	doc.ProcessingStartedAt = &now
	doc.CompletedAt = &now
	require.NoError(t, sourceRepo.Create(t.Context(), doc))

	run := domain.NewRewritePipelineRun("profile-1", "v2", "workspace-1", doc.ID, "wechat-longform", "sspai")
	run.ID = "run-1"
	run.Status = domain.RewriteRunFailed
	run.CurrentStage = "repair_draft"
	run.ErrorSummary = "stage failed"
	run.Metadata["workflow_template_id"] = "wf-1"
	run.Metadata["workflow_template_version"] = "v1"
	require.NoError(t, runRepo.Create(t.Context(), run))

	stage := &domain.RewriteStageRun{
		ID:            "stage-1",
		PipelineRunID: run.ID,
		StageName:     "repair_draft",
		StageType:     "repair",
		Status:        domain.RewriteStageFailed,
		Attempt:       2,
		StartedAt:     now,
		CompletedAt:   &now,
		ErrorSummary:  "bad output",
	}
	require.NoError(t, stageRepo.Create(t.Context(), stage))

	handler := NewAPIArticlesHandler(
		service.NewArticleQueryService(sourceRepo),
		runRepo,
		stageRepo,
		sourceRepo,
		&stubWorkflowDefinitionRepo{},
		&stubAuditLogRepo{},
		&stubSystemControlStateRepo{state: startedControlState("runner", domain.SystemStateRunning, "started", 2)},
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
	require.Nil(t, stored.ClaimedAt)
	require.Nil(t, stored.ProcessingStartedAt)
	require.Nil(t, stored.CompletedAt)
	require.Empty(t, stored.RewriteRunID)
	_, err = runRepo.GetByID(t.Context(), run.ID)
	require.Error(t, err)
	storedStages, err := stageRepo.ListByPipelineRunID(t.Context(), run.ID)
	require.NoError(t, err)
	require.Len(t, storedStages, 0)
}

func TestAPIArticlesRetryPreservesSavedWorkflowRunWhenWorkflowIsUnchanged(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	sourceRepo := &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}}
	runRepo := &stubRewritePipelineRunRepo{}
	stageRepo := &stubRewriteStageRunRepo{}
	auditRepo := &stubAuditLogRepo{}
	doc := domain.NewSourceDocument("article.md", "article.md", "md", "Title", "Body", "hash-1")
	doc.Status = domain.SourceDocumentStatusFailed
	doc.ErrorSummary = "broken"
	doc.RewriteRunID = "run-1"
	doc.Metadata["workflow_template_id"] = "wf-1"
	doc.Metadata["workflow_template_version"] = "v1"
	require.NoError(t, sourceRepo.Create(t.Context(), doc))

	run := domain.NewRewritePipelineRun("profile-1", "v2", "workspace-1", doc.ID, "wechat-longform", "sspai")
	run.ID = "run-1"
	run.Status = domain.RewriteRunFailed
	run.CurrentStage = "repair_draft"
	run.ErrorSummary = "stage failed"
	run.Metadata["workflow_template_id"] = "wf-1"
	run.Metadata["workflow_template_version"] = "v1"
	require.NoError(t, runRepo.Create(t.Context(), run))

	stage := &domain.RewriteStageRun{
		ID:            "stage-1",
		PipelineRunID: run.ID,
		StageName:     "repair_draft",
		StageType:     "repair",
		Status:        domain.RewriteStageFailed,
		Attempt:       2,
		StartedAt:     time.Now().UTC(),
		ErrorSummary:  "bad output",
	}
	require.NoError(t, stageRepo.Create(t.Context(), stage))

	handler := NewAPIArticlesHandler(
		service.NewArticleQueryService(sourceRepo),
		runRepo,
		stageRepo,
		sourceRepo,
		&stubWorkflowDefinitionRepo{},
		auditRepo,
		&stubSystemControlStateRepo{state: startedControlState("runner", domain.SystemStateRunning, "started", 2)},
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/articles/:id/retry", handler.Retry)

	req := httptest.NewRequest(http.MethodPost, "/api/articles/"+doc.ID+"/retry", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	storedRun, err := runRepo.GetByID(t.Context(), run.ID)
	require.NoError(t, err)
	require.Equal(t, domain.RewriteRunFailed, storedRun.Status)
	require.Equal(t, "repair_draft", storedRun.CurrentStage)
	require.Equal(t, "stage failed", storedRun.ErrorSummary)
	storedStages, err := stageRepo.ListByPipelineRunID(t.Context(), run.ID)
	require.NoError(t, err)
	require.Len(t, storedStages, 1)
	require.Equal(t, domain.RewriteStageFailed, storedStages[0].Status)
	require.Equal(t, 2, storedStages[0].Attempt)
	require.Equal(t, "bad output", storedStages[0].ErrorSummary)
	require.Len(t, auditRepo.logs, 1)
	require.Equal(t, "web_control.article.retry", auditRepo.logs[0].Action)
	require.Equal(t, "success", auditRepo.logs[0].Result)
	require.Equal(t, doc.ID, auditRepo.logs[0].ResourceID)
	require.Equal(t, false, auditRepo.logs[0].Metadata["workflow_state_reset"])
}

func TestAPIArticlesRetryWithWorkflowChangeResetsSavedWorkflowRun(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	sourceRepo := &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}}
	runRepo := &stubRewritePipelineRunRepo{}
	stageRepo := &stubRewriteStageRunRepo{}
	auditRepo := &stubAuditLogRepo{}
	doc := domain.NewSourceDocument("article.md", "article.md", "md", "Title", "Body", "hash-1")
	doc.Status = domain.SourceDocumentStatusFailed
	doc.ErrorSummary = "broken"
	doc.RewriteRunID = "run-1"
	doc.Metadata["workflow_template_id"] = "wf-2"
	doc.Metadata["workflow_template_version"] = "v2"
	require.NoError(t, sourceRepo.Create(t.Context(), doc))

	now := time.Now().UTC()
	run := domain.NewRewritePipelineRun("profile-1", "v1", "workspace-1", doc.ID, "wechat-longform", "sspai")
	run.ID = "run-1"
	run.Status = domain.RewriteRunFailed
	run.CurrentStage = "repair_draft"
	run.CompletedAt = &now
	run.ErrorSummary = "stage failed"
	run.Metadata["workflow_template_id"] = "wf-1"
	run.Metadata["workflow_template_version"] = "v1"
	require.NoError(t, runRepo.Create(t.Context(), run))

	stage := &domain.RewriteStageRun{
		ID:            "stage-1",
		PipelineRunID: run.ID,
		StageName:     "repair_draft",
		StageType:     "repair",
		Status:        domain.RewriteStageFailed,
		Attempt:       2,
		StartedAt:     now,
		CompletedAt:   &now,
		ErrorSummary:  "bad output",
	}
	require.NoError(t, stageRepo.Create(t.Context(), stage))

	handler := NewAPIArticlesHandler(
		service.NewArticleQueryService(sourceRepo),
		runRepo,
		stageRepo,
		sourceRepo,
		&stubWorkflowDefinitionRepo{},
		auditRepo,
		&stubSystemControlStateRepo{state: startedControlState("runner", domain.SystemStateRunning, "started", 2)},
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/articles/:id/retry", handler.Retry)

	req := httptest.NewRequest(http.MethodPost, "/api/articles/"+doc.ID+"/retry", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	_, err := runRepo.GetByID(t.Context(), run.ID)
	require.Error(t, err)
	storedStages, err := stageRepo.ListByPipelineRunID(t.Context(), run.ID)
	require.NoError(t, err)
	require.Len(t, storedStages, 0)
	storedDoc, err := sourceRepo.GetByID(t.Context(), doc.ID)
	require.NoError(t, err)
	require.Empty(t, storedDoc.RewriteRunID)
	require.Len(t, auditRepo.logs, 1)
	require.Equal(t, "web_control.article.retry", auditRepo.logs[0].Action)
	require.Equal(t, true, auditRepo.logs[0].Metadata["workflow_state_reset"])
}

func TestAPIArticlesRetrySuccessIgnoresAuditFailure(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	sourceRepo := &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}}
	auditRepo := &stubAuditLogRepo{createErr: domain.NewInternalErr("audit write failed", nil)}
	doc := domain.NewSourceDocument("article.md", "article.md", "md", "Title", "Body", "hash-1")
	doc.Status = domain.SourceDocumentStatusFailed
	doc.ErrorSummary = "broken"
	require.NoError(t, sourceRepo.Create(t.Context(), doc))

	handler := NewAPIArticlesHandler(
		service.NewArticleQueryService(sourceRepo),
		&stubRewritePipelineRunRepo{},
		&stubRewriteStageRunRepo{},
		sourceRepo,
		&stubWorkflowDefinitionRepo{},
		auditRepo,
		&stubSystemControlStateRepo{state: startedControlState("runner", domain.SystemStateRunning, "started", 2)},
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
}

func TestAPIArticlesResumeRejectsStalePersistedState(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	sourceRepo := &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}}
	doc := domain.NewSourceDocument("article.md", "article.md", "md", "Title", "Body", "hash-1")
	now := time.Now().UTC()
	doc.Status = domain.SourceDocumentStatusPaused
	doc.RewriteRunID = "run-1"
	doc.ProcessingStartedAt = &now
	require.NoError(t, sourceRepo.Create(t.Context(), doc))
	sourceRepo.afterGet = func(id string, stored *domain.SourceDocument) {
		if id == doc.ID {
			stored.Status = domain.SourceDocumentStatusCompleted
			completedAt := time.Now().UTC()
			stored.CompletedAt = &completedAt
		}
	}

	handler := NewAPIArticlesHandler(
		service.NewArticleQueryService(sourceRepo),
		&stubRewritePipelineRunRepo{},
		&stubRewriteStageRunRepo{},
		sourceRepo,
		&stubWorkflowDefinitionRepo{},
		&stubAuditLogRepo{},
		&stubSystemControlStateRepo{state: startedControlState("runner", domain.SystemStateRunning, "started", 2)},
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/articles/:id/resume", handler.Resume)

	req := httptest.NewRequest(http.MethodPost, "/api/articles/"+doc.ID+"/resume", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code)
	require.Contains(t, w.Body.String(), "state changed")
}

func TestAPIArticlesStopReturnsAccepted(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	sourceRepo := &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}}
	doc := domain.NewSourceDocument("article.md", "article.md", "md", "Title", "Body", "hash-1")
	now := time.Now().UTC()
	doc.Status = domain.SourceDocumentStatusProcessing
	doc.RewriteRunID = "run-1"
	doc.ClaimedBy = "worker-1"
	doc.ClaimedAt = &now
	doc.ProcessingStartedAt = &now
	require.NoError(t, sourceRepo.Create(t.Context(), doc))

	handler := NewAPIArticlesHandler(
		service.NewArticleQueryService(sourceRepo),
		&stubRewritePipelineRunRepo{},
		&stubRewriteStageRunRepo{},
		sourceRepo,
		&stubWorkflowDefinitionRepo{},
		&stubAuditLogRepo{},
		&stubSystemControlStateRepo{},
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/articles/:id/stop", handler.Stop)

	req := httptest.NewRequest(http.MethodPost, "/api/articles/"+doc.ID+"/stop", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "paused", resp["status"])
	require.Equal(t, true, resp["requested_pause"])
	require.Contains(t, resp["message"], "pause requested")
	stored, err := sourceRepo.GetByID(t.Context(), doc.ID)
	require.NoError(t, err)
	require.Equal(t, "paused", stored.Status)
	require.Equal(t, doc.RewriteRunID, stored.RewriteRunID)
	require.Empty(t, stored.ClaimedBy)
	require.Nil(t, stored.ClaimedAt)
	require.NotNil(t, stored.ProcessingStartedAt)
}

func TestAPIArticlesStopWritesAuditEntry(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	sourceRepo := &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}}
	auditRepo := &stubAuditLogRepo{}
	doc := domain.NewSourceDocument("article.md", "article.md", "md", "Title", "Body", "hash-1")
	now := time.Now().UTC()
	doc.Status = domain.SourceDocumentStatusProcessing
	doc.RewriteRunID = "run-1"
	doc.ClaimedBy = "worker-1"
	doc.ClaimedAt = &now
	doc.ProcessingStartedAt = &now
	require.NoError(t, sourceRepo.Create(t.Context(), doc))

	handler := NewAPIArticlesHandler(
		service.NewArticleQueryService(sourceRepo),
		&stubRewritePipelineRunRepo{},
		&stubRewriteStageRunRepo{},
		sourceRepo,
		&stubWorkflowDefinitionRepo{},
		auditRepo,
		&stubSystemControlStateRepo{},
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/articles/:id/stop", handler.Stop)

	req := httptest.NewRequest(http.MethodPost, "/api/articles/"+doc.ID+"/stop", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	require.Len(t, auditRepo.logs, 1)
	require.Equal(t, "web_control.article.stop", auditRepo.logs[0].Action)
	require.Equal(t, "source_document", auditRepo.logs[0].Resource)
	require.Equal(t, doc.ID, auditRepo.logs[0].ResourceID)
	require.Equal(t, "success", auditRepo.logs[0].Result)
}

func TestAPIArticlesStopSuccessIgnoresAuditFailure(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	sourceRepo := &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}}
	auditRepo := &stubAuditLogRepo{createErr: domain.NewInternalErr("audit write failed", nil)}
	doc := domain.NewSourceDocument("article.md", "article.md", "md", "Title", "Body", "hash-1")
	now := time.Now().UTC()
	doc.Status = domain.SourceDocumentStatusProcessing
	doc.RewriteRunID = "run-1"
	doc.ClaimedBy = "worker-1"
	doc.ClaimedAt = &now
	doc.ProcessingStartedAt = &now
	require.NoError(t, sourceRepo.Create(t.Context(), doc))

	handler := NewAPIArticlesHandler(
		service.NewArticleQueryService(sourceRepo),
		&stubRewritePipelineRunRepo{},
		&stubRewriteStageRunRepo{},
		sourceRepo,
		&stubWorkflowDefinitionRepo{},
		auditRepo,
		&stubSystemControlStateRepo{},
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/articles/:id/stop", handler.Stop)

	req := httptest.NewRequest(http.MethodPost, "/api/articles/"+doc.ID+"/stop", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	stored, err := sourceRepo.GetByID(t.Context(), doc.ID)
	require.NoError(t, err)
	require.Equal(t, domain.SourceDocumentStatusPaused, stored.Status)
}

func TestAPIArticlesResumeReturnsAccepted(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	sourceRepo := &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}}
	doc := domain.NewSourceDocument("article.md", "article.md", "md", "Title", "Body", "hash-1")
	doc.Status = "paused"
	doc.RewriteRunID = "run-1"
	require.NoError(t, sourceRepo.Create(t.Context(), doc))

	handler := NewAPIArticlesHandler(
		service.NewArticleQueryService(sourceRepo),
		&stubRewritePipelineRunRepo{},
		&stubRewriteStageRunRepo{},
		sourceRepo,
		&stubWorkflowDefinitionRepo{},
		&stubAuditLogRepo{},
		&stubSystemControlStateRepo{state: startedControlState("runner", domain.SystemStateRunning, "started", 2)},
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/articles/:id/resume", handler.Resume)

	req := httptest.NewRequest(http.MethodPost, "/api/articles/"+doc.ID+"/resume", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "requeued", resp["status"])
	require.Equal(t, true, resp["worker_running"])
	stored, err := sourceRepo.GetByID(t.Context(), doc.ID)
	require.NoError(t, err)
	require.Equal(t, domain.SourceDocumentStatusPending, stored.Status)
	require.Equal(t, doc.RewriteRunID, stored.RewriteRunID)
	require.Empty(t, stored.ClaimedBy)
	require.Nil(t, stored.ClaimedAt)
	require.Nil(t, stored.ProcessingStartedAt)
}

func TestAPIArticlesResumeRejectsPausedArticleWithActiveProcessingMarkers(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	sourceRepo := &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}}
	doc := domain.NewSourceDocument("article.md", "article.md", "md", "Title", "Body", "hash-1")
	now := time.Now().UTC()
	doc.Status = "paused"
	doc.RewriteRunID = "run-1"
	doc.ClaimedBy = "worker-1"
	doc.ClaimedAt = &now
	doc.ProcessingStartedAt = &now
	require.NoError(t, sourceRepo.Create(t.Context(), doc))

	handler := NewAPIArticlesHandler(
		service.NewArticleQueryService(sourceRepo),
		&stubRewritePipelineRunRepo{},
		&stubRewriteStageRunRepo{},
		sourceRepo,
		&stubWorkflowDefinitionRepo{},
		&stubAuditLogRepo{},
		&stubSystemControlStateRepo{state: startedControlState("runner", domain.SystemStateRunning, "started", 2)},
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/articles/:id/resume", handler.Resume)

	req := httptest.NewRequest(http.MethodPost, "/api/articles/"+doc.ID+"/resume", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "resume is not allowed while prior processing markers are still active")
	stored, err := sourceRepo.GetByID(t.Context(), doc.ID)
	require.NoError(t, err)
	require.Equal(t, domain.SourceDocumentStatusPaused, stored.Status)
	require.Equal(t, "run-1", stored.RewriteRunID)
	require.Equal(t, "worker-1", stored.ClaimedBy)
	require.NotNil(t, stored.ClaimedAt)
	require.NotNil(t, stored.ProcessingStartedAt)
}

func TestAPIArticlesResumeWritesAuditEntry(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	sourceRepo := &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}}
	auditRepo := &stubAuditLogRepo{}
	doc := domain.NewSourceDocument("article.md", "article.md", "md", "Title", "Body", "hash-1")
	doc.Status = domain.SourceDocumentStatusPaused
	doc.RewriteRunID = "run-1"
	require.NoError(t, sourceRepo.Create(t.Context(), doc))

	handler := NewAPIArticlesHandler(
		service.NewArticleQueryService(sourceRepo),
		&stubRewritePipelineRunRepo{},
		&stubRewriteStageRunRepo{},
		sourceRepo,
		&stubWorkflowDefinitionRepo{},
		auditRepo,
		&stubSystemControlStateRepo{state: startedControlState("runner", domain.SystemStateRunning, "started", 2)},
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/articles/:id/resume", handler.Resume)

	req := httptest.NewRequest(http.MethodPost, "/api/articles/"+doc.ID+"/resume", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	require.Len(t, auditRepo.logs, 1)
	require.Equal(t, "web_control.article.resume", auditRepo.logs[0].Action)
	require.Equal(t, "success", auditRepo.logs[0].Result)
	require.Equal(t, doc.ID, auditRepo.logs[0].ResourceID)
}

func TestAPIArticlesResumeSuccessIgnoresAuditFailure(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	sourceRepo := &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}}
	auditRepo := &stubAuditLogRepo{createErr: domain.NewInternalErr("audit write failed", nil)}
	doc := domain.NewSourceDocument("article.md", "article.md", "md", "Title", "Body", "hash-1")
	doc.Status = domain.SourceDocumentStatusPaused
	doc.RewriteRunID = "run-1"
	require.NoError(t, sourceRepo.Create(t.Context(), doc))

	handler := NewAPIArticlesHandler(
		service.NewArticleQueryService(sourceRepo),
		&stubRewritePipelineRunRepo{},
		&stubRewriteStageRunRepo{},
		sourceRepo,
		&stubWorkflowDefinitionRepo{},
		auditRepo,
		&stubSystemControlStateRepo{state: startedControlState("runner", domain.SystemStateRunning, "started", 2)},
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/articles/:id/resume", handler.Resume)

	req := httptest.NewRequest(http.MethodPost, "/api/articles/"+doc.ID+"/resume", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	stored, err := sourceRepo.GetByID(t.Context(), doc.ID)
	require.NoError(t, err)
	require.Equal(t, domain.SourceDocumentStatusPending, stored.Status)
}

func TestAPIArticlesDeleteRejectsProcessingArticle(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	sourceRepo := &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}}
	doc := domain.NewSourceDocument("article.md", "article.md", "md", "Title", "Body", "hash-1")
	now := time.Now().UTC()
	doc.Status = domain.SourceDocumentStatusProcessing
	doc.ClaimedBy = "worker-1"
	doc.ClaimedAt = &now
	doc.ProcessingStartedAt = &now
	require.NoError(t, sourceRepo.Create(t.Context(), doc))

	handler := NewAPIArticlesHandler(
		service.NewArticleQueryService(sourceRepo),
		&stubRewritePipelineRunRepo{},
		&stubRewriteStageRunRepo{},
		sourceRepo,
		&stubWorkflowDefinitionRepo{},
		&stubAuditLogRepo{},
		&stubSystemControlStateRepo{},
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.DELETE("/api/articles/:id", handler.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/articles/"+doc.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "delete is not allowed while article is processing")
	stored, err := sourceRepo.GetByID(t.Context(), doc.ID)
	require.NoError(t, err)
	require.Equal(t, domain.SourceDocumentStatusProcessing, stored.Status)
}

func TestAPIArticlesDeleteRejectRecordsAuditFailure(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	sourceRepo := &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}}
	auditRepo := &stubAuditLogRepo{}
	doc := domain.NewSourceDocument("article.md", "article.md", "md", "Title", "Body", "hash-1")
	now := time.Now().UTC()
	doc.Status = domain.SourceDocumentStatusProcessing
	doc.ClaimedBy = "worker-1"
	doc.ClaimedAt = &now
	doc.ProcessingStartedAt = &now
	require.NoError(t, sourceRepo.Create(t.Context(), doc))

	handler := NewAPIArticlesHandler(
		service.NewArticleQueryService(sourceRepo),
		&stubRewritePipelineRunRepo{},
		&stubRewriteStageRunRepo{},
		sourceRepo,
		&stubWorkflowDefinitionRepo{},
		auditRepo,
		&stubSystemControlStateRepo{},
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.DELETE("/api/articles/:id", handler.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/articles/"+doc.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Len(t, auditRepo.logs, 1)
	require.Equal(t, "web_control.article.delete", auditRepo.logs[0].Action)
	require.Equal(t, "failure", auditRepo.logs[0].Result)
	require.Equal(t, doc.ID, auditRepo.logs[0].ResourceID)
}

func TestAPIArticlesDeleteRemovesWorkflowRecordsForPausedArticle(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	sourceRepo := &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}}
	runRepo := &stubRewritePipelineRunRepo{}
	stageRepo := &stubRewriteStageRunRepo{}
	auditRepo := &stubAuditLogRepo{}
	doc := domain.NewSourceDocument("article.md", "article.md", "md", "Title", "Body", "hash-1")
	now := time.Now().UTC()
	doc.Status = "paused"
	doc.RewriteRunID = "run-1"
	doc.ClaimedBy = "worker-1"
	doc.ClaimedAt = &now
	doc.ProcessingStartedAt = &now
	require.NoError(t, sourceRepo.Create(t.Context(), doc))
	run := domain.NewRewritePipelineRun("profile-1", "v1", "workspace-1", doc.ID, "wechat-longform", "sspai")
	run.ID = doc.RewriteRunID
	require.NoError(t, runRepo.Create(t.Context(), run))
	require.NoError(t, stageRepo.Create(t.Context(), &domain.RewriteStageRun{
		ID:            "stage-1",
		PipelineRunID: run.ID,
		StageName:     "repair_draft",
		StageType:     "repair",
		Status:        domain.RewriteStageRunning,
		Attempt:       1,
		StartedAt:     now,
	}))

	handler := NewAPIArticlesHandler(
		service.NewArticleQueryService(sourceRepo),
		runRepo,
		stageRepo,
		sourceRepo,
		&stubWorkflowDefinitionRepo{},
		auditRepo,
		&stubSystemControlStateRepo{},
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.DELETE("/api/articles/:id", handler.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/articles/"+doc.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)
	_, err := sourceRepo.GetByID(t.Context(), doc.ID)
	require.Error(t, err)
	appErr, ok := err.(*domain.AppError)
	require.True(t, ok)
	require.Equal(t, domain.ErrNotFound, appErr.Code)
	_, err = runRepo.GetByID(t.Context(), run.ID)
	require.Error(t, err)
	stages, err := stageRepo.ListByPipelineRunID(t.Context(), run.ID)
	require.NoError(t, err)
	require.Len(t, stages, 0)
	require.Len(t, auditRepo.logs, 1)
	require.Equal(t, "web_control.article.delete", auditRepo.logs[0].Action)
	require.Equal(t, "success", auditRepo.logs[0].Result)
	require.Equal(t, true, auditRepo.logs[0].Metadata["workflow_records_deleted"])
}

func TestAPIArticlesDeleteSuccessIgnoresAuditFailure(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	sourceRepo := &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}}
	runRepo := &stubRewritePipelineRunRepo{}
	stageRepo := &stubRewriteStageRunRepo{}
	auditRepo := &stubAuditLogRepo{createErr: domain.NewInternalErr("audit write failed", nil)}
	doc := domain.NewSourceDocument("article.md", "article.md", "md", "Title", "Body", "hash-1")
	now := time.Now().UTC()
	doc.Status = domain.SourceDocumentStatusPaused
	doc.RewriteRunID = "run-1"
	doc.ClaimedBy = "worker-1"
	doc.ClaimedAt = &now
	doc.ProcessingStartedAt = &now
	require.NoError(t, sourceRepo.Create(t.Context(), doc))
	run := domain.NewRewritePipelineRun("profile-1", "v1", "workspace-1", doc.ID, "wechat-longform", "sspai")
	run.ID = doc.RewriteRunID
	require.NoError(t, runRepo.Create(t.Context(), run))
	require.NoError(t, stageRepo.Create(t.Context(), &domain.RewriteStageRun{
		ID:            "stage-1",
		PipelineRunID: run.ID,
		StageName:     "repair_draft",
		StageType:     "repair",
		Status:        domain.RewriteStageRunning,
		Attempt:       1,
		StartedAt:     now,
	}))

	handler := NewAPIArticlesHandler(
		service.NewArticleQueryService(sourceRepo),
		runRepo,
		stageRepo,
		sourceRepo,
		&stubWorkflowDefinitionRepo{},
		auditRepo,
		&stubSystemControlStateRepo{},
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.DELETE("/api/articles/:id", handler.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/articles/"+doc.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)
	_, err := sourceRepo.GetByID(t.Context(), doc.ID)
	require.Error(t, err)
	_, err = runRepo.GetByID(t.Context(), run.ID)
	require.Error(t, err)
}

func TestAPIArticlesRetryWithWorkflowChangeKeepsWorkflowRecordsWhenSourceUpdateFails(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	sourceRepo := &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}, updateErr: domain.NewConflictErr("source document state changed before retry")}
	runRepo := &stubRewritePipelineRunRepo{}
	stageRepo := &stubRewriteStageRunRepo{}
	doc := domain.NewSourceDocument("article.md", "article.md", "md", "Title", "Body", "hash-1")
	doc.Status = domain.SourceDocumentStatusFailed
	doc.ErrorSummary = "broken"
	doc.RewriteRunID = "run-1"
	doc.Metadata["workflow_template_id"] = "wf-2"
	doc.Metadata["workflow_template_version"] = "v2"
	require.NoError(t, sourceRepo.Create(t.Context(), doc))

	now := time.Now().UTC()
	run := domain.NewRewritePipelineRun("profile-1", "v1", "workspace-1", doc.ID, "wechat-longform", "sspai")
	run.ID = "run-1"
	run.Status = domain.RewriteRunFailed
	run.CurrentStage = "repair_draft"
	run.CompletedAt = &now
	run.ErrorSummary = "stage failed"
	run.Metadata["workflow_template_id"] = "wf-1"
	run.Metadata["workflow_template_version"] = "v1"
	require.NoError(t, runRepo.Create(t.Context(), run))
	require.NoError(t, stageRepo.Create(t.Context(), &domain.RewriteStageRun{
		ID:            "stage-1",
		PipelineRunID: run.ID,
		StageName:     "repair_draft",
		StageType:     "repair",
		Status:        domain.RewriteStageFailed,
		Attempt:       2,
		StartedAt:     now,
		CompletedAt:   &now,
		ErrorSummary:  "bad output",
	}))

	handler := NewAPIArticlesHandler(
		service.NewArticleQueryService(sourceRepo),
		runRepo,
		stageRepo,
		sourceRepo,
		&stubWorkflowDefinitionRepo{},
		&stubAuditLogRepo{},
		&stubSystemControlStateRepo{state: startedControlState("runner", domain.SystemStateRunning, "started", 2)},
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/articles/:id/retry", handler.Retry)

	req := httptest.NewRequest(http.MethodPost, "/api/articles/"+doc.ID+"/retry", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code)
	storedRun, err := runRepo.GetByID(t.Context(), run.ID)
	require.NoError(t, err)
	require.Equal(t, domain.RewriteRunFailed, storedRun.Status)
	storedStages, err := stageRepo.ListByPipelineRunID(t.Context(), run.ID)
	require.NoError(t, err)
	require.Len(t, storedStages, 1)
}

func TestAPIArticlesDeleteKeepsWorkflowRecordsWhenSourceDeleteFails(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	sourceRepo := &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}, deleteErr: domain.NewConflictErr("source document state changed before delete")}
	runRepo := &stubRewritePipelineRunRepo{}
	stageRepo := &stubRewriteStageRunRepo{}
	doc := domain.NewSourceDocument("article.md", "article.md", "md", "Title", "Body", "hash-1")
	now := time.Now().UTC()
	doc.Status = domain.SourceDocumentStatusPaused
	doc.RewriteRunID = "run-1"
	doc.ClaimedBy = "worker-1"
	doc.ClaimedAt = &now
	doc.ProcessingStartedAt = &now
	require.NoError(t, sourceRepo.Create(t.Context(), doc))
	run := domain.NewRewritePipelineRun("profile-1", "v1", "workspace-1", doc.ID, "wechat-longform", "sspai")
	run.ID = doc.RewriteRunID
	require.NoError(t, runRepo.Create(t.Context(), run))
	require.NoError(t, stageRepo.Create(t.Context(), &domain.RewriteStageRun{
		ID:            "stage-1",
		PipelineRunID: run.ID,
		StageName:     "repair_draft",
		StageType:     "repair",
		Status:        domain.RewriteStageRunning,
		Attempt:       1,
		StartedAt:     now,
	}))

	handler := NewAPIArticlesHandler(
		service.NewArticleQueryService(sourceRepo),
		runRepo,
		stageRepo,
		sourceRepo,
		&stubWorkflowDefinitionRepo{},
		&stubAuditLogRepo{},
		&stubSystemControlStateRepo{},
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.DELETE("/api/articles/:id", handler.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/articles/"+doc.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code)
	storedRun, err := runRepo.GetByID(t.Context(), run.ID)
	require.NoError(t, err)
	require.Equal(t, run.ID, storedRun.ID)
	storedStages, err := stageRepo.ListByPipelineRunID(t.Context(), run.ID)
	require.NoError(t, err)
	require.Len(t, storedStages, 1)
}

func TestAPIArticlesDeleteWorkflowCleanupFailureRecordsAuditContext(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	sourceRepo := &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}}
	runRepo := &stubRewritePipelineRunRepo{}
	stageRepo := &stubRewriteStageRunRepo{deleteErr: domain.NewInternalErr("workflow cleanup failed", nil)}
	auditRepo := &stubAuditLogRepo{}
	doc := domain.NewSourceDocument("article.md", "article.md", "md", "Title", "Body", "hash-1")
	now := time.Now().UTC()
	doc.Status = domain.SourceDocumentStatusPaused
	doc.RewriteRunID = "run-1"
	doc.ClaimedBy = "worker-1"
	doc.ClaimedAt = &now
	doc.ProcessingStartedAt = &now
	require.NoError(t, sourceRepo.Create(t.Context(), doc))
	run := domain.NewRewritePipelineRun("profile-1", "v1", "workspace-1", doc.ID, "wechat-longform", "sspai")
	run.ID = doc.RewriteRunID
	require.NoError(t, runRepo.Create(t.Context(), run))

	handler := NewAPIArticlesHandler(
		service.NewArticleQueryService(sourceRepo),
		runRepo,
		stageRepo,
		sourceRepo,
		&stubWorkflowDefinitionRepo{},
		auditRepo,
		&stubSystemControlStateRepo{},
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.DELETE("/api/articles/:id", handler.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/articles/"+doc.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	_, err := sourceRepo.GetByID(t.Context(), doc.ID)
	require.Error(t, err)
	storedRun, err := runRepo.GetByID(t.Context(), run.ID)
	require.NoError(t, err)
	require.Equal(t, run.ID, storedRun.ID)
	require.Len(t, auditRepo.logs, 1)
	require.Equal(t, "web_control.article.delete", auditRepo.logs[0].Action)
	require.Equal(t, "failure", auditRepo.logs[0].Result)
	require.Equal(t, "source deletion succeeded but workflow cleanup failed", auditRepo.logs[0].Message)
	require.Equal(t, true, auditRepo.logs[0].Metadata["source_deleted"])
	require.Equal(t, true, auditRepo.logs[0].Metadata["workflow_cleanup_failed"])
	require.Equal(t, "run-1", auditRepo.logs[0].Metadata["workflow_run_id"])
}

func TestAPIArticlesAssignWorkflowTemplateRejectsDisabledWorkflow(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	sourceRepo := &stubSourceDocumentRepo{storedByID: map[string]*domain.SourceDocument{}}
	doc := domain.NewSourceDocument("article.md", "article.md", "md", "Title", "Body", "hash-1")
	doc.Status = domain.SourceDocumentStatusPending
	require.NoError(t, sourceRepo.Create(t.Context(), doc))
	workflowRepo := &stubWorkflowDefinitionRepo{stored: &domain.WorkflowDefinition{
		ID:          "wf-disabled",
		Name:        "Disabled workflow",
		Version:     "v1",
		Enabled:     false,
		EntryNodeID: "generate_draft",
		Nodes:       []domain.WorkflowNode{{ID: "generate_draft", Type: "rewrite_stage", Name: "generate_draft"}},
	}}

	handler := NewAPIArticlesHandler(
		service.NewArticleQueryService(sourceRepo),
		&stubRewritePipelineRunRepo{},
		&stubRewriteStageRunRepo{},
		sourceRepo,
		workflowRepo,
		&stubAuditLogRepo{},
		&stubSystemControlStateRepo{},
	)

	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/articles/:id/workflow-template", handler.AssignWorkflowTemplate)

	req := httptest.NewRequest(http.MethodPost, "/api/articles/"+doc.ID+"/workflow-template", bytes.NewBufferString(`{"workflow_template_id":"wf-disabled"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "disabled")
	stored, err := sourceRepo.GetByID(t.Context(), doc.ID)
	require.NoError(t, err)
	_, exists := stored.Metadata["workflow_template_id"]
	require.False(t, exists)
}

func startedControlState(updatedBy, state, reason string, limit int) *domain.SystemControlState {
	control := domain.NewSystemControlState(updatedBy)
	control.State = state
	control.Reason = reason
	control.Metadata["concurrency_limit"] = limit
	return control
}
