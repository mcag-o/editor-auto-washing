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

type browserArticlePayload struct {
	ID                 string         `json:"id"`
	WorkspaceArticleID string         `json:"workspace_article_id"`
	Title              string         `json:"title"`
	Summary            string         `json:"summary"`
	Body               string         `json:"body"`
	Status             string         `json:"status"`
	SourceType         string         `json:"source_type"`
	OriginalPath       string         `json:"original_path"`
	OriginalFilename   string         `json:"original_filename"`
	FileType           string         `json:"file_type"`
	RewriteRunID       string         `json:"rewrite_run_id"`
	WorkflowRunID      string         `json:"workflow_run_id"`
	ErrorSummary       string         `json:"error_summary"`
	ImportedAt         *time.Time     `json:"imported_at"`
	ProcessingStartedAt *time.Time    `json:"processing_started_at"`
	CompletedAt        *time.Time     `json:"completed_at"`
	Metadata           map[string]any `json:"metadata"`
}

func (p browserArticlePayload) ValidateBasic() error {
	if p.ID == "" || p.WorkspaceArticleID == "" || p.Title == "" || p.Status == "" {
		return domain.NewValidationErr("browser article payload is incomplete", nil)
	}
	return nil
}

func TestAPIArticlesRetryWithWorkflowChangeDeletesWorkflowRunAndCheckpoints(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	workspaceRepo := &stubArticleWorkspaceRepo{storedByID: map[string]*domain.ArticleWorkspaceRecord{}}
	runRepo := &stubRewritePipelineRunRepo{runs: map[string]*domain.RewritePipelineRun{}}
	stageRepo := &stubRewriteStageRunRepo{}
	workflowRepo := &stubWorkflowRunRepo{runs: map[string]*domain.WorkflowRun{}}
	checkpointRepo := &stubWorkflowCheckpointRepo{checkpoints: map[string][]*domain.WorkflowCheckpoint{}}
	auditRepo := &stubAuditLogRepo{}

	workspace := domain.NewArticleWorkspaceRecord("workspace-1", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "upload", URL: "browser://upload/article.md"}, map[string]any{
		"source_body":               "Body",
		"source_profile":            "web-upload",
		"workflow_template_id":      "workflow-new",
		"workflow_template_name":    "Workflow New",
		"workflow_template_version": "v2",
	})
	workspace.Status = domain.ArticleWorkspaceStatusRewriteFailed
	require.NoError(t, workspaceRepo.Create(t.Context(), workspace))

	rewriteRun := domain.NewRewritePipelineRun("profile-1", "v1", workspace.ID, "wechat-longform", "web-upload")
	rewriteRun.ID = "rewrite-run-1"
	rewriteRun.Status = domain.RewriteRunFailed
	rewriteRun.ErrorSummary = "broken"
	rewriteRun.Metadata["workflow_template_id"] = "workflow-old"
	rewriteRun.Metadata["workflow_template_name"] = "Workflow Old"
	rewriteRun.Metadata["workflow_template_version"] = "v1"
	require.NoError(t, runRepo.Create(t.Context(), rewriteRun))

	workflowRun, err := domain.NewWorkflowRun(domain.WorkflowRunSpec{WorkflowID: "workflow-old", WorkflowVersion: "v1", EntryNodeID: "review", WorkspaceArticleID: workspace.ID})
	require.NoError(t, err)
	workflowRun.ID = "workflow-run-1"
	workflowRun.Status = domain.WorkflowRunPaused
	require.NoError(t, workflowRepo.Create(t.Context(), workflowRun))
	require.NoError(t, checkpointRepo.Create(t.Context(), &domain.WorkflowCheckpoint{
		ID:            "checkpoint-1",
		WorkflowRunID: workflowRun.ID,
		NodeID:        "review",
		State:         domain.WorkflowCheckpointStateActive,
		Resumable:     true,
		ResumeToken:   "token-1",
		CreatedAt:     time.Now().UTC(),
	}))

	articles := service.NewBrowserArticleQueryService(workspaceRepo, runRepo, workflowRepo)
	handler := NewAPIArticlesHandler(articles, runRepo, stageRepo, &stubWorkflowDefinitionRepo{}, auditRepo, &stubSystemControlStateRepo{}, checkpointRepo)
	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/articles/:id/retry", handler.Retry)

	req := httptest.NewRequest(http.MethodPost, "/api/articles/"+workspace.ID+"/retry", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	_, err = workflowRepo.GetByID(t.Context(), workflowRun.ID)
	require.Error(t, err)
	checkpoints, err := checkpointRepo.ListByWorkflowRunID(t.Context(), workflowRun.ID)
	require.NoError(t, err)
	require.Len(t, checkpoints, 0)
	_, err = runRepo.GetByID(t.Context(), rewriteRun.ID)
	require.Error(t, err)
}

func TestAPIArticlesDeleteRemovesWorkflowRunAndCheckpoints(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	workspaceRepo := &stubArticleWorkspaceRepo{storedByID: map[string]*domain.ArticleWorkspaceRecord{}}
	runRepo := &stubRewritePipelineRunRepo{runs: map[string]*domain.RewritePipelineRun{}}
	stageRepo := &stubRewriteStageRunRepo{}
	workflowRepo := &stubWorkflowRunRepo{runs: map[string]*domain.WorkflowRun{}}
	checkpointRepo := &stubWorkflowCheckpointRepo{checkpoints: map[string][]*domain.WorkflowCheckpoint{}}
	auditRepo := &stubAuditLogRepo{}

	workspace := domain.NewArticleWorkspaceRecord("workspace-1", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "paste", URL: "browser://paste/article"}, map[string]any{"source_body": "Body", "source_profile": "web-paste"})
	workspace.Status = domain.ArticleWorkspaceStatusRendered
	require.NoError(t, workspaceRepo.Create(t.Context(), workspace))

	rewriteRun := domain.NewRewritePipelineRun("profile-1", "v1", workspace.ID, "wechat-longform", "web-paste")
	rewriteRun.ID = "rewrite-run-1"
	require.NoError(t, runRepo.Create(t.Context(), rewriteRun))

	workflowRun, err := domain.NewWorkflowRun(domain.WorkflowRunSpec{WorkflowID: "workflow-old", WorkflowVersion: "v1", EntryNodeID: "review", WorkspaceArticleID: workspace.ID})
	require.NoError(t, err)
	workflowRun.ID = "workflow-run-1"
	workflowRun.Status = domain.WorkflowRunPaused
	require.NoError(t, workflowRepo.Create(t.Context(), workflowRun))
	require.NoError(t, checkpointRepo.Create(t.Context(), &domain.WorkflowCheckpoint{
		ID:            "checkpoint-1",
		WorkflowRunID: workflowRun.ID,
		NodeID:        "review",
		State:         domain.WorkflowCheckpointStateActive,
		Resumable:     true,
		ResumeToken:   "token-1",
		CreatedAt:     time.Now().UTC(),
	}))

	articles := service.NewBrowserArticleQueryService(workspaceRepo, runRepo, workflowRepo)
	handler := NewAPIArticlesHandler(articles, runRepo, stageRepo, &stubWorkflowDefinitionRepo{}, auditRepo, &stubSystemControlStateRepo{}, checkpointRepo)
	router := gin.New()
	router.Use(middleware.TraceID())
	router.DELETE("/api/articles/:id", handler.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/articles/"+workspace.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)
	_, err = workflowRepo.GetByID(t.Context(), workflowRun.ID)
	require.Error(t, err)
	checkpoints, err := checkpointRepo.ListByWorkflowRunID(t.Context(), workflowRun.ID)
	require.NoError(t, err)
	require.Len(t, checkpoints, 0)
}

func TestAPIArticlesRetryResponseReturnsCoherentBrowserArticleProjection(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	workspaceRepo := &stubArticleWorkspaceRepo{storedByID: map[string]*domain.ArticleWorkspaceRecord{}}
	runRepo := &stubRewritePipelineRunRepo{runs: map[string]*domain.RewritePipelineRun{}}
	workflowRepo := &stubWorkflowRunRepo{runs: map[string]*domain.WorkflowRun{}}
	articleID := "workspace-1"
	workspace := domain.NewArticleWorkspaceRecord(articleID, "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "paste", URL: "browser://paste/article"}, map[string]any{"source_body": "Body", "source_profile": "web-paste"})
	workspace.Status = domain.ArticleWorkspaceStatusRewriteFailed
	require.NoError(t, workspaceRepo.Create(t.Context(), workspace))

	articles := service.NewBrowserArticleQueryService(workspaceRepo, runRepo, workflowRepo)
	handler := NewAPIArticlesHandler(articles, runRepo, &stubRewriteStageRunRepo{}, &stubWorkflowDefinitionRepo{}, &stubAuditLogRepo{}, &stubSystemControlStateRepo{})
	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/articles/:id/retry", handler.Retry)

	req := httptest.NewRequest(http.MethodPost, "/api/articles/"+articleID+"/retry", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var payload struct {
		Article browserArticlePayload `json:"article"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.NoError(t, payload.Article.ValidateBasic())
}

func TestAPIArticlesStopRollsBackWorkflowPauseWhenWorkspacePausePersistenceFails(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	workspaceRepo := &stubArticleWorkspaceRepo{storedByID: map[string]*domain.ArticleWorkspaceRecord{}, transitionErr: domain.NewConflictErr("workspace state changed")}
	runRepo := &stubRewritePipelineRunRepo{runs: map[string]*domain.RewritePipelineRun{}}
	stageRepo := &stubRewriteStageRunRepo{}
	workflowRepo := &stubWorkflowRunRepo{runs: map[string]*domain.WorkflowRun{}}

	workspace := domain.NewArticleWorkspaceRecord("workspace-1", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "paste", URL: "browser://paste/article"}, map[string]any{"source_body": "Body", "source_profile": "web-paste"})
	workspace.Status = domain.ArticleWorkspaceStatusRewriting
	require.NoError(t, workspaceRepo.Create(t.Context(), workspace))

	rewriteRun := domain.NewRewritePipelineRun("profile-1", "v1", workspace.ID, "wechat-longform", "web-paste")
	rewriteRun.ID = "rewrite-run-1"
	rewriteRun.Status = domain.RewriteRunRunning
	require.NoError(t, runRepo.Create(t.Context(), rewriteRun))

	workflowRun, err := domain.NewWorkflowRun(domain.WorkflowRunSpec{WorkflowID: "workflow-1", WorkflowVersion: "v1", EntryNodeID: "review", WorkspaceArticleID: workspace.ID})
	require.NoError(t, err)
	workflowRun.ID = "workflow-run-1"
	workflowRun.Status = domain.WorkflowRunRunning
	require.NoError(t, workflowRepo.Create(t.Context(), workflowRun))

	articles := service.NewBrowserArticleQueryService(workspaceRepo, runRepo, workflowRepo)
	handler := NewAPIArticlesHandler(articles, runRepo, stageRepo, &stubWorkflowDefinitionRepo{}, &stubAuditLogRepo{}, &stubSystemControlStateRepo{})
	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/articles/:id/stop", handler.Stop)

	req := httptest.NewRequest(http.MethodPost, "/api/articles/"+workspace.ID+"/stop", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code)
	storedWorkflowRun, err := workflowRepo.GetByID(t.Context(), workflowRun.ID)
	require.NoError(t, err)
	require.Equal(t, domain.WorkflowRunRunning, storedWorkflowRun.Status)
	storedWorkspace, err := workspaceRepo.GetByID(t.Context(), workspace.ID)
	require.NoError(t, err)
	require.Equal(t, domain.ArticleWorkspaceStatusRewriting, storedWorkspace.Status)
}

func TestAPIArticlesDeleteAllowsPausedWorkspaceArticles(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	workspaceRepo := &stubArticleWorkspaceRepo{storedByID: map[string]*domain.ArticleWorkspaceRecord{}}
	runRepo := &stubRewritePipelineRunRepo{runs: map[string]*domain.RewritePipelineRun{}}
	stageRepo := &stubRewriteStageRunRepo{}
	workflowRepo := &stubWorkflowRunRepo{runs: map[string]*domain.WorkflowRun{}}
	checkpointRepo := &stubWorkflowCheckpointRepo{checkpoints: map[string][]*domain.WorkflowCheckpoint{}}

	workspace := domain.NewArticleWorkspaceRecord("workspace-1", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "paste", URL: "browser://paste/article"}, map[string]any{"source_body": "Body", "source_profile": "web-paste"})
	workspace.Status = domain.ArticleWorkspaceStatusPaused
	require.NoError(t, workspaceRepo.Create(t.Context(), workspace))

	rewriteRun := domain.NewRewritePipelineRun("profile-1", "v1", workspace.ID, "wechat-longform", "web-paste")
	rewriteRun.ID = "rewrite-run-1"
	require.NoError(t, runRepo.Create(t.Context(), rewriteRun))

	workflowRun, err := domain.NewWorkflowRun(domain.WorkflowRunSpec{WorkflowID: "workflow-old", WorkflowVersion: "v1", EntryNodeID: "review", WorkspaceArticleID: workspace.ID})
	require.NoError(t, err)
	workflowRun.ID = "workflow-run-1"
	workflowRun.Status = domain.WorkflowRunPaused
	require.NoError(t, workflowRepo.Create(t.Context(), workflowRun))
	require.NoError(t, checkpointRepo.Create(t.Context(), &domain.WorkflowCheckpoint{ID: "checkpoint-1", WorkflowRunID: workflowRun.ID, NodeID: "review", State: domain.WorkflowCheckpointStateActive, Resumable: true, ResumeToken: "token-1", CreatedAt: time.Now().UTC()}))

	articles := service.NewBrowserArticleQueryService(workspaceRepo, runRepo, workflowRepo)
	handler := NewAPIArticlesHandler(articles, runRepo, stageRepo, &stubWorkflowDefinitionRepo{}, &stubAuditLogRepo{}, &stubSystemControlStateRepo{}, checkpointRepo)
	router := gin.New()
	router.Use(middleware.TraceID())
	router.DELETE("/api/articles/:id", handler.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/api/articles/"+workspace.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestAPIArticlesRetryPreservesWorkspaceLifecycleHistory(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	workspaceRepo := &stubArticleWorkspaceRepo{storedByID: map[string]*domain.ArticleWorkspaceRecord{}}
	runRepo := &stubRewritePipelineRunRepo{runs: map[string]*domain.RewritePipelineRun{}}
	workflowRepo := &stubWorkflowRunRepo{runs: map[string]*domain.WorkflowRun{}}
	now := time.Now().UTC()
	workspace := domain.NewArticleWorkspaceRecord("workspace-1", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "paste", URL: "browser://paste/article"}, map[string]any{"source_body": "Body", "source_profile": "web-paste"})
	workspace.Status = domain.ArticleWorkspaceStatusRewriteFailed
	workspace.StatusHistory = []string{domain.ArticleWorkspaceStatusImported, domain.ArticleWorkspaceStatusRewriting, domain.ArticleWorkspaceStatusRewriteFailed}
	workspace.LifecycleHistory = []domain.ArticleWorkspaceLifecycleEntry{{Status: domain.ArticleWorkspaceStatusImported, Notes: "imported", CreatedAt: now.Add(-3 * time.Minute)}, {Status: domain.ArticleWorkspaceStatusRewriting, Notes: "rewriting", CreatedAt: now.Add(-2 * time.Minute)}, {Status: domain.ArticleWorkspaceStatusRewriteFailed, Notes: "failed", CreatedAt: now.Add(-time.Minute)}}
	require.NoError(t, workspaceRepo.Create(t.Context(), workspace))
	rewriteRun := domain.NewRewritePipelineRun("profile-1", "v1", workspace.ID, "wechat-longform", "web-paste")
	rewriteRun.ID = "rewrite-run-1"
	rewriteRun.Status = domain.RewriteRunFailed
	require.NoError(t, runRepo.Create(t.Context(), rewriteRun))

	articles := service.NewBrowserArticleQueryService(workspaceRepo, runRepo, workflowRepo)
	handler := NewAPIArticlesHandler(articles, runRepo, &stubRewriteStageRunRepo{}, &stubWorkflowDefinitionRepo{}, &stubAuditLogRepo{}, &stubSystemControlStateRepo{})
	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/articles/:id/retry", handler.Retry)

	req := httptest.NewRequest(http.MethodPost, "/api/articles/"+workspace.ID+"/retry", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	storedWorkspace, err := workspaceRepo.GetByID(t.Context(), workspace.ID)
	require.NoError(t, err)
	require.Equal(t, []string{domain.ArticleWorkspaceStatusImported, domain.ArticleWorkspaceStatusRewriting, domain.ArticleWorkspaceStatusRewriteFailed, domain.ArticleWorkspaceStatusImported}, storedWorkspace.StatusHistory)
	require.Len(t, storedWorkspace.LifecycleHistory, 4)
	require.Equal(t, domain.ArticleWorkspaceStatusImported, storedWorkspace.LifecycleHistory[3].Status)
}

func TestAPIArticlesResumePreservesWorkspaceLifecycleHistory(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	workspaceRepo := &stubArticleWorkspaceRepo{storedByID: map[string]*domain.ArticleWorkspaceRecord{}}
	runRepo := &stubRewritePipelineRunRepo{runs: map[string]*domain.RewritePipelineRun{}}
	workflowRepo := &stubWorkflowRunRepo{runs: map[string]*domain.WorkflowRun{}}
	now := time.Now().UTC()
	workspace := domain.NewArticleWorkspaceRecord("workspace-1", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "paste", URL: "browser://paste/article"}, map[string]any{"source_body": "Body", "source_profile": "web-paste"})
	workspace.Status = domain.ArticleWorkspaceStatusPaused
	workspace.StatusHistory = []string{domain.ArticleWorkspaceStatusImported, domain.ArticleWorkspaceStatusRewriting, domain.ArticleWorkspaceStatusPaused}
	workspace.LifecycleHistory = []domain.ArticleWorkspaceLifecycleEntry{{Status: domain.ArticleWorkspaceStatusImported, Notes: "imported", CreatedAt: now.Add(-3 * time.Minute)}, {Status: domain.ArticleWorkspaceStatusRewriting, Notes: "rewriting", CreatedAt: now.Add(-2 * time.Minute)}, {Status: domain.ArticleWorkspaceStatusPaused, Notes: "paused", CreatedAt: now.Add(-time.Minute)}}
	require.NoError(t, workspaceRepo.Create(t.Context(), workspace))
	rewriteRun := domain.NewRewritePipelineRun("profile-1", "v1", workspace.ID, "wechat-longform", "web-paste")
	rewriteRun.ID = "rewrite-run-1"
	rewriteRun.Status = domain.RewriteRunRunning
	require.NoError(t, runRepo.Create(t.Context(), rewriteRun))
	workflowRun, err := domain.NewWorkflowRun(domain.WorkflowRunSpec{WorkflowID: "workflow-1", WorkflowVersion: "v1", EntryNodeID: "review", WorkspaceArticleID: workspace.ID})
	require.NoError(t, err)
	workflowRun.ID = "workflow-run-1"
	workflowRun.Status = domain.WorkflowRunPaused
	require.NoError(t, workflowRepo.Create(t.Context(), workflowRun))

	articles := service.NewBrowserArticleQueryService(workspaceRepo, runRepo, workflowRepo)
	handler := NewAPIArticlesHandler(articles, runRepo, &stubRewriteStageRunRepo{}, &stubWorkflowDefinitionRepo{}, &stubAuditLogRepo{}, &stubSystemControlStateRepo{})
	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/articles/:id/resume", handler.Resume)

	req := httptest.NewRequest(http.MethodPost, "/api/articles/"+workspace.ID+"/resume", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	storedWorkspace, err := workspaceRepo.GetByID(t.Context(), workspace.ID)
	require.NoError(t, err)
	require.Equal(t, []string{domain.ArticleWorkspaceStatusImported, domain.ArticleWorkspaceStatusRewriting, domain.ArticleWorkspaceStatusPaused, domain.ArticleWorkspaceStatusImported}, storedWorkspace.StatusHistory)
	require.Len(t, storedWorkspace.LifecycleHistory, 4)
	require.Equal(t, domain.ArticleWorkspaceStatusImported, storedWorkspace.LifecycleHistory[3].Status)
}
