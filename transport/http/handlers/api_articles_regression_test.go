package handlers

import (
	"bytes"
	"content-hub/domain"
	"content-hub/service"
	"content-hub/transport/http/middleware"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type browserArticlePayloadFixture struct {
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

func (p browserArticlePayloadFixture) ValidateBasic() error {
	if p.ID == "" || p.WorkspaceArticleID == "" || p.Title == "" || p.Status == "" {
		return domain.NewValidationErr("browser article payload is incomplete", nil)
	}
	return nil
}

func TestAPIArticlesListReturnsBrowserArticleProjection(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	workspaceRepo := &stubArticleWorkspaceRepo{storedByID: map[string]*domain.ArticleWorkspaceRecord{}}
	runRepo := &stubRewritePipelineRunRepo{runs: map[string]*domain.RewritePipelineRun{}}
	workflowRepo := &stubWorkflowRunRepo{runs: map[string]*domain.WorkflowRun{}}

	workspace := domain.NewArticleWorkspaceRecord("workspace-1", "List Title", "List Summary", domain.ArticleWorkspaceSource{SourceType: "upload", URL: "browser://upload/list-article.md"}, map[string]any{
		"source_body": "List Body",
		"source_profile": "web-upload",
	})
	workspace.Status = domain.ArticleWorkspaceStatusRendered
	require.NoError(t, workspaceRepo.Create(t.Context(), workspace))

	articles := service.NewBrowserArticleQueryService(workspaceRepo, runRepo, workflowRepo)
	handler := NewAPIArticlesHandler(articles, runRepo, &stubRewriteStageRunRepo{}, &stubWorkflowDefinitionRepo{}, &stubAuditLogRepo{}, &stubSystemControlStateRepo{})
	router := gin.New()
	router.Use(middleware.TraceID())
	router.GET("/api/articles", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/api/articles", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), "archived_path")
	require.NotContains(t, w.Body.String(), "claimed_by")
	require.NotContains(t, w.Body.String(), "claimed_at")
	require.NotContains(t, w.Body.String(), "\"hash\"")

	var payload struct {
		Data []browserArticlePayloadFixture `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Len(t, payload.Data,1)
	require.NoError(t, payload.Data[0].ValidateBasic())
	require.Equal(t, workspace.ID, payload.Data[0].WorkspaceArticleID)
	require.Equal(t, "upload", payload.Data[0].SourceType)
	require.Equal(t, "list-article.md", payload.Data[0].OriginalFilename)
	require.Equal(t, "md", payload.Data[0].FileType)
	require.Equal(t, "browser://upload/list-article.md", payload.Data[0].OriginalPath)
}

func TestAPIArticlesGetReturnsBrowserArticleProjection(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	workspaceRepo := &stubArticleWorkspaceRepo{storedByID: map[string]*domain.ArticleWorkspaceRecord{}}
	runRepo := &stubRewritePipelineRunRepo{runs: map[string]*domain.RewritePipelineRun{}}
	workflowRepo := &stubWorkflowRunRepo{runs: map[string]*domain.WorkflowRun{}}

	workspace := domain.NewArticleWorkspaceRecord("workspace-1", "Get Title", "Get Summary", domain.ArticleWorkspaceSource{SourceType: "paste", URL: "browser://paste/get-article"}, map[string]any{
		"source_body": "Get Body",
		"source_profile": "web-paste",
	})
	workspace.Status = domain.ArticleWorkspaceStatusImported
	require.NoError(t, workspaceRepo.Create(t.Context(), workspace))

	articles := service.NewBrowserArticleQueryService(workspaceRepo, runRepo, workflowRepo)
	handler := NewAPIArticlesHandler(articles, runRepo, &stubRewriteStageRunRepo{}, &stubWorkflowDefinitionRepo{}, &stubAuditLogRepo{}, &stubSystemControlStateRepo{})
	router := gin.New()
	router.Use(middleware.TraceID())
	router.GET("/api/articles/:id", handler.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/articles/"+workspace.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), "archived_path")
	require.NotContains(t, w.Body.String(), "claimed_by")
	require.NotContains(t, w.Body.String(), "claimed_at")
	require.NotContains(t, w.Body.String(), "\"hash\"")

	var payload browserArticlePayloadFixture
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.NoError(t, payload.ValidateBasic())
	require.Equal(t, workspace.ID, payload.WorkspaceArticleID)
	require.Equal(t, "paste", payload.SourceType)
	require.Equal(t, "browser-article.txt", payload.OriginalFilename)
	require.Equal(t, "txt", payload.FileType)
	require.Equal(t, "browser://paste/get-article", payload.OriginalPath)
}

func TestAPIArticlesStagesResponseUsesBrowserArticleProjection(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	workspaceRepo := &stubArticleWorkspaceRepo{storedByID: map[string]*domain.ArticleWorkspaceRecord{}}
	runRepo := &stubRewritePipelineRunRepo{runs: map[string]*domain.RewritePipelineRun{}}
	stageRepo := &stubRewriteStageRunRepo{}
	workflowRepo := &stubWorkflowRunRepo{runs: map[string]*domain.WorkflowRun{}}

	workspace := domain.NewArticleWorkspaceRecord("workspace-1", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "upload", URL: "browser://upload/article.md"}, map[string]any{
		"source_body": "Body",
		"source_profile": "web-upload",
	})
	workspace.Status = domain.ArticleWorkspaceStatusRewriting
	require.NoError(t, workspaceRepo.Create(t.Context(), workspace))

	run := domain.NewRewritePipelineRun("profile-1", "v1", workspace.ID, "wechat-longform", "web-upload")
	run.ID = "rewrite-run-1"
	run.Status = domain.RewriteRunRunning
	require.NoError(t, runRepo.Create(t.Context(), run))
	require.NoError(t, stageRepo.Create(t.Context(), &domain.RewriteStageRun{ID: "stage-1", PipelineRunID: run.ID, StageName: "rewrite", StageType: "llm", Status: domain.RewriteStageRunning, Attempt:1, StartedAt: time.Now().UTC()}))

	articles := service.NewBrowserArticleQueryService(workspaceRepo, runRepo, workflowRepo)
	handler := NewAPIArticlesHandler(articles, runRepo, stageRepo, &stubWorkflowDefinitionRepo{}, &stubAuditLogRepo{}, &stubSystemControlStateRepo{})
	router := gin.New()
	router.Use(middleware.TraceID())
	router.GET("/api/articles/:id/stages", handler.Stages)

	req := httptest.NewRequest(http.MethodGet, "/api/articles/"+workspace.ID+"/stages", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), "archived_path")
	require.NotContains(t, w.Body.String(), "claimed_by")
	require.NotContains(t, w.Body.String(), "claimed_at")
	require.NotContains(t, w.Body.String(), "\"hash\"")

	var payload struct {
		Article browserArticlePayloadFixture `json:"article"`
		Run *domain.RewritePipelineRun `json:"run"`
		Stages []domain.RewriteStageRun `json:"stages"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.NoError(t, payload.Article.ValidateBasic())
	require.Equal(t, workspace.ID, payload.Article.WorkspaceArticleID)
	require.Equal(t, "article.md", payload.Article.OriginalFilename)
	require.Equal(t, "md", payload.Article.FileType)
	require.Equal(t, "browser://upload/article.md", payload.Article.OriginalPath)
	require.NotNil(t, payload.Run)
	require.Len(t, payload.Stages,1)
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
	require.NotContains(t, w.Body.String(), "archived_path")
	require.NotContains(t, w.Body.String(), "claimed_by")
	require.NotContains(t, w.Body.String(), "claimed_at")
	require.NotContains(t, w.Body.String(), "\"hash\"")
	var payload struct {
		Article browserArticlePayloadFixture `json:"article"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.NoError(t, payload.Article.ValidateBasic())
	require.Equal(t, articleID, payload.Article.WorkspaceArticleID)
	require.Equal(t, "paste", payload.Article.SourceType)
	require.Equal(t, "browser-article.txt", payload.Article.OriginalFilename)
}

func TestAPIArticlesAssignWorkflowTemplateReturnsBrowserArticleProjection(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	workspaceRepo := &stubArticleWorkspaceRepo{storedByID: map[string]*domain.ArticleWorkspaceRecord{}}
	runRepo := &stubRewritePipelineRunRepo{runs: map[string]*domain.RewritePipelineRun{}}
	stageRepo := &stubRewriteStageRunRepo{}
	workflowRepo := &stubWorkflowDefinitionRepo{stored: &domain.WorkflowDefinition{ID: "workflow-1", Name: "Template A", Version: "v1", Enabled: true}}
	workflowRunRepo := &stubWorkflowRunRepo{runs: map[string]*domain.WorkflowRun{}}
	auditRepo := &stubAuditLogRepo{}

	workspace := domain.NewArticleWorkspaceRecord("workspace-1", "Title", "Summary", domain.ArticleWorkspaceSource{SourceType: "paste", URL: "browser://paste/article"}, map[string]any{"source_body": "Body", "source_profile": "web-paste"})
	require.NoError(t, workspaceRepo.Create(t.Context(), workspace))

	articles := service.NewBrowserArticleQueryService(workspaceRepo, runRepo, workflowRunRepo)
	handler := NewAPIArticlesHandler(articles, runRepo, stageRepo, workflowRepo, auditRepo, &stubSystemControlStateRepo{})
	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/articles/:id/workflow-template", handler.AssignWorkflowTemplate)

	req := httptest.NewRequest(http.MethodPost, "/api/articles/"+workspace.ID+"/workflow-template", strings.NewReader(`{"workflow_template_id":"workflow-1"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), "archived_path")
	require.NotContains(t, w.Body.String(), "claimed_by")
	require.NotContains(t, w.Body.String(), "claimed_at")
	require.NotContains(t, w.Body.String(), "\"hash\"")
	var payload browserArticlePayloadFixture
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.NoError(t, payload.ValidateBasic())
	require.Equal(t, "workflow-1", payload.Metadata["workflow_template_id"])
	require.Equal(t, "Template A", payload.Metadata["workflow_template_name"])
	require.Equal(t, "v1", payload.Metadata["workflow_template_version"])
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
