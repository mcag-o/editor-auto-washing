package handlers

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"content-hub/service"
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type APIArticlesHandler struct {
	articles     *service.ArticleQueryService
	runs         repo.RewritePipelineRunRepo
	stages       repo.RewriteStageRunRepo
	workspaces   articleWorkspaceRepo
	workflowRuns repo.WorkflowRunRepo
	checkpoints  repo.WorkflowCheckpointRepo
	workflows    interface {
		GetByID(context.Context, string) (*domain.WorkflowDefinition, error)
	}
	audit   repo.AuditLogRepo
	control interface {
		Get(context.Context) (*domain.SystemControlState, error)
	}
}

type articleWorkspaceRepo interface {
	GetByID(context.Context, string) (*domain.ArticleWorkspaceRecord, error)
	List(context.Context, *string) ([]domain.ArticleWorkspaceRecord, error)
	Create(context.Context, *domain.ArticleWorkspaceRecord) error
	Update(context.Context, *domain.ArticleWorkspaceRecord) error
	TransitionStatus(context.Context, string, string, string) error
	Delete(context.Context, string) error
}

type rewriteStageRunUpdater interface {
	Update(context.Context, *domain.RewriteStageRun) error
}

type browserArticleListResponse struct {
	Data []service.BrowserArticle `json:"data"`
}

type browserArticleStagesResponse struct {
	Article *service.BrowserArticle `json:"article"`
	Run     *domain.RewritePipelineRun `json:"run"`
	Stages  []domain.RewriteStageRun `json:"stages"`
}

type browserArticleActionResponse struct {
	Status         string                  `json:"status"`
	Message        string                  `json:"message"`
	WorkerRunning  bool                    `json:"worker_running,omitempty"`
	SystemState    string                  `json:"system_state,omitempty"`
	RequestedPause bool                    `json:"requested_pause,omitempty"`
	Article        *service.BrowserArticle `json:"article"`
}

const articleOperationsActor = "local-admin"

func NewAPIArticlesHandler(articles *service.ArticleQueryService, runs repo.RewritePipelineRunRepo, stages repo.RewriteStageRunRepo, workflows interface {
	GetByID(context.Context, string) (*domain.WorkflowDefinition, error)
}, audit repo.AuditLogRepo, control interface {
	Get(context.Context) (*domain.SystemControlState, error)
}, checkpoints ...repo.WorkflowCheckpointRepo) *APIArticlesHandler {
	var workspaces articleWorkspaceRepo
	var workflowRuns repo.WorkflowRunRepo
	var workflowCheckpoints repo.WorkflowCheckpointRepo
	if articles != nil {
		workspaces = articles.WorkspaceRepo()
		workflowRuns = articles.WorkflowRunRepo()
	}
	if len(checkpoints) > 0 {
		workflowCheckpoints = checkpoints[0]
	}
	return &APIArticlesHandler{articles: articles, runs: runs, stages: stages, workspaces: workspaces, workflowRuns: workflowRuns, checkpoints: workflowCheckpoints, workflows: workflows, audit: audit, control: control}
}

func (h *APIArticlesHandler) List(c *gin.Context) {
	items, err := h.articles.ListArticles(c.Request.Context(), service.ArticleQueryFilter{Status: c.Query("status"), Limit: 100})
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, browserArticleListPayload(items))
}

func (h *APIArticlesHandler) Get(c *gin.Context) {
	item, err := h.articles.GetArticle(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, browserArticlePayload(item))
}

func (h *APIArticlesHandler) Stages(c *gin.Context) {
	item, err := h.articles.GetArticle(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}

	var run *domain.RewritePipelineRun
	stageRuns := []domain.RewriteStageRun{}
	if strings.TrimSpace(item.RewriteRunID) != "" {
		run, err = h.runs.GetByID(c.Request.Context(), item.RewriteRunID)
		if err != nil {
			HandleError(c, err)
			return
		}
		stageRuns, err = h.stages.ListByPipelineRunID(c.Request.Context(), item.RewriteRunID)
		if err != nil {
			HandleError(c, err)
			return
		}
	}

	c.JSON(http.StatusOK, browserArticleStagesResponse{
		Article: browserArticlePayload(item),
		Run:     run,
		Stages:  stageRuns,
	})
}

func (h *APIArticlesHandler) Retry(c *gin.Context) {
	workspace, err := h.workspaces.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	item, err := h.articles.GetArticle(c.Request.Context(), workspace.ID)
	if err != nil {
		HandleError(c, err)
		return
	}
	if workspace.Status != domain.ArticleWorkspaceStatusRewriteFailed && workspace.Status != domain.ArticleWorkspaceStatusFailed {
		err := domain.NewValidationErr("retry is only allowed from failed state", nil)
		h.recordArticleAudit(c.Request.Context(), item, "retry", "failure", err.Error(), nil)
		HandleError(c, err)
		return
	}
	previousRunID := strings.TrimSpace(item.RewriteRunID)
	workflowStateReset := false
	workflowChanged, err := h.workflowExecutionChanged(c.Request.Context(), workspace, previousRunID)
	if err != nil {
		HandleError(c, err)
		return
	}
	if workflowChanged {
		workflowStateReset = true
	}
	now := time.Now().UTC()
	workspace.Status = domain.ArticleWorkspaceStatusImported
	workspace.StatusHistory = append(workspace.StatusHistory, domain.ArticleWorkspaceStatusImported)
	workspace.LifecycleHistory = append(workspace.LifecycleHistory, domain.ArticleWorkspaceLifecycleEntry{
		Status:    domain.ArticleWorkspaceStatusImported,
		Notes:     "retry queued from browser article operations",
		CreatedAt: now,
	})
	workspace.Notes = "retry queued from browser article operations"
	workspace.UpdatedAt = now
	if !workflowStateReset && previousRunID != "" {
		if workspace.Metadata == nil {
			workspace.Metadata = map[string]any{}
		}
		workspace.Metadata["resume_rewrite_run_id"] = previousRunID
	} else if workspace.Metadata != nil {
		delete(workspace.Metadata, "resume_rewrite_run_id")
	}
	if err := h.workspaces.Update(c.Request.Context(), workspace); err != nil {
		HandleError(c, err)
		return
	}
	if workflowStateReset {
		if err := h.deleteWorkflowExecution(c.Request.Context(), previousRunID); err != nil {
			HandleError(c, err)
			return
		}
	}

	workerRunning := false
	systemState := domain.SystemStateStopped
	message := "article re-queued for retry"
	if h.control != nil {
		state, stateErr := h.control.Get(c.Request.Context())
		if stateErr == nil {
			systemState = state.State
			workerRunning = state.State == domain.SystemStateRunning
		}
	}
	if !workerRunning {
		message = "article re-queued, but the worker is not actively running"
	}
	h.recordArticleAuditBestEffort(c.Request.Context(), item, "retry", "success", message, map[string]any{
		"workflow_state_reset": workflowStateReset,
		"worker_running":       workerRunning,
		"system_state":         systemState,
	})
	updatedItem, loadErr := h.articles.GetArticle(c.Request.Context(), workspace.ID)
	if loadErr == nil {
		item = updatedItem
	}

	c.JSON(http.StatusOK, browserArticleActionResponse{
		Status:        "requeued",
		Message:       message,
		WorkerRunning: workerRunning,
		SystemState:   systemState,
		Article:       browserArticlePayload(item),
	})
}

func (h *APIArticlesHandler) Stop(c *gin.Context) {
	workspace, err := h.workspaces.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	item, err := h.articles.GetArticle(c.Request.Context(), workspace.ID)
	if err != nil {
		HandleError(c, err)
		return
	}
	if workspace.Status != domain.ArticleWorkspaceStatusRewriting {
		err := domain.NewValidationErr("stop is only allowed from processing state", nil)
		h.recordArticleAudit(c.Request.Context(), item, "stop", "failure", err.Error(), nil)
		HandleError(c, err)
		return
	}
	workflowRun := h.latestWorkflowRunForWorkspace(c.Request.Context(), workspace.ID)
	if workflowRun != nil && workflowRun.Status != domain.WorkflowRunPaused {
		originalStatus := workflowRun.Status
		originalResumable := workflowRun.Resumable
		originalMetadata := map[string]any{}
		for key, value := range workflowRun.Metadata {
			originalMetadata[key] = value
		}
		workflowRun.Status = domain.WorkflowRunPaused
		workflowRun.Resumable = true
		if workflowRun.Metadata == nil {
			workflowRun.Metadata = map[string]any{}
		}
		workflowRun.Metadata["pause_source"] = "manual"
		workflowRun.Metadata["pause_reason"] = "pause requested from article operations"
		workflowRun.Metadata["pause_allowed_resume_modes"] = []string{"continue_active_tokens", "replay_from_checkpoint"}
		if err := h.workflowRuns.Update(c.Request.Context(), workflowRun); err != nil {
			HandleError(c, err)
			return
		}
		if err := h.workspaces.TransitionStatus(c.Request.Context(), workspace.ID, domain.ArticleWorkspaceStatusPaused, "pause requested; awaiting workflow resume"); err != nil {
			workflowRun.Status = originalStatus
			workflowRun.Resumable = originalResumable
			workflowRun.Metadata = originalMetadata
			_ = h.workflowRuns.Update(c.Request.Context(), workflowRun)
			HandleError(c, err)
			return
		}
	} else {
		if err := h.workspaces.TransitionStatus(c.Request.Context(), workspace.ID, domain.ArticleWorkspaceStatusPaused, "pause requested; awaiting workflow resume"); err != nil {
			HandleError(c, err)
			return
		}
	}
	message := "pause requested; the current worker step is not synchronously interrupted"
	h.recordArticleAuditBestEffort(c.Request.Context(), item, "stop", "success", message, map[string]any{
		"workflow_position_preserved": strings.TrimSpace(item.RewriteRunID) != "",
		"pause_requested":             true,
		"workflow_run_id":             strings.TrimSpace(item.RewriteRunID),
		"pause_source":                "manual",
	})

	c.JSON(http.StatusAccepted, browserArticleActionResponse{
		Status:         domain.WorkflowRunPaused,
		RequestedPause: true,
		Message:        message,
		Article:        browserArticlePayload(mustLoadBrowserArticle(c.Request.Context(), h.articles, workspace.ID, item)),
	})
}

func (h *APIArticlesHandler) Resume(c *gin.Context) {
	workspace, err := h.workspaces.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	item, err := h.articles.GetArticle(c.Request.Context(), workspace.ID)
	if err != nil {
		HandleError(c, err)
		return
	}
	workflowRun := h.latestWorkflowRunForWorkspace(c.Request.Context(), workspace.ID)
	if workspace.Status != domain.ArticleWorkspaceStatusPaused || workflowRun == nil || workflowRun.Status != domain.WorkflowRunPaused {
		err := domain.NewValidationErr("resume is only allowed from paused state", nil)
		h.recordArticleAudit(c.Request.Context(), item, "resume", "failure", err.Error(), nil)
		HandleError(c, err)
		return
	}
	now := time.Now().UTC()
	workspace.Status = domain.ArticleWorkspaceStatusImported
	workspace.StatusHistory = append(workspace.StatusHistory, domain.ArticleWorkspaceStatusImported)
	workspace.LifecycleHistory = append(workspace.LifecycleHistory, domain.ArticleWorkspaceLifecycleEntry{
		Status:    domain.ArticleWorkspaceStatusImported,
		Notes:     "resume queued from paused workflow position",
		CreatedAt: now,
	})
	workspace.Notes = "resume queued from paused workflow position"
	workspace.UpdatedAt = now
	if workspace.Metadata == nil {
		workspace.Metadata = map[string]any{}
	}
	workspace.Metadata["resume_rewrite_run_id"] = strings.TrimSpace(item.RewriteRunID)
	if err := h.workspaces.Update(c.Request.Context(), workspace); err != nil {
		HandleError(c, err)
		return
	}

	workerRunning := false
	systemState := domain.SystemStateStopped
	message := "article re-queued to resume from saved workflow position"
	if h.control != nil {
		state, stateErr := h.control.Get(c.Request.Context())
		if stateErr == nil {
			systemState = state.State
			workerRunning = state.State == domain.SystemStateRunning
		}
	}
	if !workerRunning {
		message = "article queued to resume, but the worker is not actively running"
	}
	h.recordArticleAuditBestEffort(c.Request.Context(), item, "resume", "success", message, map[string]any{
		"workflow_position_preserved": strings.TrimSpace(item.RewriteRunID) != "",
		"worker_running":              workerRunning,
		"system_state":                systemState,
		"resume_requested":            true,
		"workflow_run_id":             strings.TrimSpace(item.RewriteRunID),
		"resume_mode":                 "continue_saved_position",
	})

	c.JSON(http.StatusAccepted, browserArticleActionResponse{
		Status:        "requeued",
		Message:       message,
		WorkerRunning: workerRunning,
		SystemState:   systemState,
		Article:       browserArticlePayload(mustLoadBrowserArticle(c.Request.Context(), h.articles, workspace.ID, item)),
	})
}

func (h *APIArticlesHandler) Delete(c *gin.Context) {
	workspace, err := h.workspaces.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	item, err := h.articles.GetArticle(c.Request.Context(), workspace.ID)
	if err != nil {
		HandleError(c, err)
		return
	}
	if workspace.Status == domain.ArticleWorkspaceStatusRewriting {
		err := domain.NewValidationErr("delete is not allowed while article is processing", nil)
		h.recordArticleAudit(c.Request.Context(), item, "delete", "failure", err.Error(), nil)
		HandleError(c, err)
		return
	}
	if workspace.Status != domain.ArticleWorkspaceStatusImported && workspace.Status != domain.ArticleWorkspaceStatusDraft && workspace.Status != domain.ArticleWorkspaceStatusRendered && workspace.Status != domain.ArticleWorkspaceStatusRewriteFailed && workspace.Status != domain.ArticleWorkspaceStatusFailed && workspace.Status != domain.ArticleWorkspaceStatusPaused {
		err := domain.NewValidationErr("delete is only allowed from pending, paused, or completed state", nil)
		h.recordArticleAudit(c.Request.Context(), item, "delete", "failure", err.Error(), nil)
		HandleError(c, err)
		return
	}

	workflowRecordsDeleted := false
	runID := strings.TrimSpace(item.RewriteRunID)
	if err := h.workspaces.Delete(c.Request.Context(), workspace.ID); err != nil {
		HandleError(c, err)
		return
	}
	if runID != "" {
		if err := h.deleteWorkflowExecution(c.Request.Context(), runID); err != nil {
			h.recordArticleAuditBestEffort(c.Request.Context(), item, "delete", "failure", "source deletion succeeded but workflow cleanup failed", map[string]any{
				"workflow_records_deleted": false,
				"source_deleted":           true,
				"workflow_cleanup_failed":  true,
				"workflow_run_id":          runID,
			})
			HandleError(c, err)
			return
		}
		workflowRecordsDeleted = true
	}
	h.recordArticleAuditBestEffort(c.Request.Context(), item, "delete", "success", "deleted article", map[string]any{
		"workflow_records_deleted": workflowRecordsDeleted,
	})
	c.Status(http.StatusNoContent)
}

func (h *APIArticlesHandler) AssignWorkflowTemplate(c *gin.Context) {
	var req struct {
		WorkflowTemplateID string `json:"workflow_template_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if h.workflows == nil || h.audit == nil {
		HandleError(c, domain.NewInternalErr("workflow template assignment is not configured", nil))
		return
	}
	workspace, err := h.workspaces.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	workflow, err := h.workflows.GetByID(c.Request.Context(), strings.TrimSpace(req.WorkflowTemplateID))
	if err != nil {
		HandleError(c, err)
		return
	}
	if !workflow.Enabled {
		HandleError(c, domain.NewValidationErr(fmt.Sprintf("workflow template %s is disabled", workflow.ID), nil))
		return
	}
	if workspace.Metadata == nil {
		workspace.Metadata = map[string]any{}
	}
	workspace.Metadata["workflow_template_id"] = workflow.ID
	workspace.Metadata["workflow_template_name"] = workflow.Name
	workspace.Metadata["workflow_template_version"] = workflow.Version
	workspace.UpdatedAt = time.Now().UTC()
	if err := h.workspaces.Update(c.Request.Context(), workspace); err != nil {
		HandleError(c, err)
		return
	}
	item, err := h.articles.GetArticle(c.Request.Context(), workspace.ID)
	if err != nil {
		HandleError(c, err)
		return
	}
	if _, err := service.NewAuditLogService(h.audit).Create(c.Request.Context(), service.AuditLogCreateInput{
		Actor:      articleOperationsActor,
		Action:     "web_control.article.workflow_template_assigned",
		Resource:   "workspace_article",
		ResourceID: item.ID,
		Result:     "success",
		Message:    "assigned workflow template to article",
		Metadata: map[string]any{
			"workflow_template_id": workflow.ID,
		},
	}); err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, browserArticlePayload(item))
}

func browserArticleListPayload(items []service.BrowserArticle) browserArticleListResponse {
	return browserArticleListResponse{Data: items}
}

func browserArticlePayload(item *service.BrowserArticle) *service.BrowserArticle {
	return item
}

func (h *APIArticlesHandler) recordArticleAudit(ctx context.Context, item *service.BrowserArticle, action, result, message string, metadata map[string]any) error {
	if h == nil || h.audit == nil || item == nil {
		return nil
	}
	_, err := service.NewAuditLogService(h.audit).Create(ctx, service.AuditLogCreateInput{
		Actor:      articleOperationsActor,
		Action:     "web_control.article." + strings.TrimSpace(action),
		Resource:   "workspace_article",
		ResourceID: item.ID,
		Result:     strings.TrimSpace(result),
		Message:    strings.TrimSpace(message),
		Metadata:   metadata,
	})
	return err
}

func (h *APIArticlesHandler) recordArticleAuditBestEffort(ctx context.Context, item *service.BrowserArticle, action, result, message string, metadata map[string]any) {
	if err := h.recordArticleAudit(ctx, item, action, result, message, metadata); err != nil {
		log.Printf("warning: write article lifecycle audit action=%s resource_id=%s: %v", strings.TrimSpace(action), item.ID, err)
	}
}

func (h *APIArticlesHandler) workflowExecutionChanged(ctx context.Context, workspace *domain.ArticleWorkspaceRecord, rewriteRunID string) (bool, error) {
	if workspace == nil || strings.TrimSpace(rewriteRunID) == "" {
		return false, nil
	}
	run, err := h.runs.GetByID(ctx, rewriteRunID)
	if err != nil {
		return false, err
	}
	return workflowMetadataValue(workspace.Metadata, "workflow_template_id") != workflowMetadataValue(run.Metadata, "workflow_template_id") ||
		workflowMetadataValue(workspace.Metadata, "workflow_template_version") != workflowMetadataValue(run.Metadata, "workflow_template_version") ||
		workflowMetadataValue(workspace.Metadata, "workflow_template_name") != workflowMetadataValue(run.Metadata, "workflow_template_name"), nil
}

func workflowMetadataValue(metadata map[string]any, key string) string {
	if len(metadata) == 0 {
		return ""
	}
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

func (h *APIArticlesHandler) deleteWorkflowExecution(ctx context.Context, runID string) error {
	if strings.TrimSpace(runID) == "" {
		return nil
	}
	workspaceArticleID := ""
	if h.runs != nil {
		run, err := h.runs.GetByID(ctx, runID)
		if err != nil {
			return err
		}
		workspaceArticleID = strings.TrimSpace(run.WorkspaceArticleID)
	}
	if h.workflowRuns != nil {
		workflowRuns, err := h.workflowRuns.List(ctx, 0)
		if err != nil {
			return err
		}
		for i := range workflowRuns {
			workflowRun := workflowRuns[i]
			if workspaceArticleID == "" || strings.TrimSpace(workflowRun.WorkspaceArticleID) != workspaceArticleID {
				continue
			}
			if h.checkpoints != nil {
				if err := h.checkpoints.DeleteByWorkflowRunID(ctx, workflowRun.ID); err != nil {
					return err
				}
			}
			if err := h.workflowRuns.Delete(ctx, workflowRun.ID); err != nil {
				return err
			}
		}
	}
	if err := h.stages.DeleteByPipelineRunID(ctx, runID); err != nil {
		return err
	}
	if err := h.runs.Delete(ctx, runID); err != nil {
		return err
	}
	return nil
}

func (h *APIArticlesHandler) resetWorkflowExecution(ctx context.Context, item *service.BrowserArticle) error {
	if item == nil || strings.TrimSpace(item.RewriteRunID) == "" {
		return nil
	}
	run, err := h.runs.GetByID(ctx, item.RewriteRunID)
	if err != nil {
		return err
	}
	run.Status = domain.RewriteRunPending
	run.CurrentStage = ""
	run.CompletedAt = nil
	run.FinalDraftID = ""
	run.ErrorSummary = ""
	if err := h.runs.Update(ctx, run); err != nil {
		return err
	}

	stageRuns, err := h.stages.ListByPipelineRunID(ctx, item.RewriteRunID)
	if err != nil {
		return err
	}
	updater, ok := h.stages.(rewriteStageRunUpdater)
	if !ok {
		return domain.NewInternalErr("rewrite stage run reset is not configured", nil)
	}
	for i := range stageRuns {
		stage := stageRuns[i]
		stage.Status = domain.RewriteStagePending
		stage.Attempt = 0
		stage.OutputJSON = ""
		stage.ErrorSummary = ""
		stage.CompletedAt = nil
		if err := updater.Update(ctx, &stage); err != nil {
			return err
		}
	}
	return nil
}

func (h *APIArticlesHandler) latestWorkflowRunForWorkspace(ctx context.Context, workspaceID string) *domain.WorkflowRun {
	if h == nil || h.workflowRuns == nil || strings.TrimSpace(workspaceID) == "" {
		return nil
	}
	runs, err := h.workflowRuns.List(ctx, 0)
	if err != nil {
		return nil
	}
	var selected *domain.WorkflowRun
	for i := range runs {
		run := runs[i]
		if strings.TrimSpace(run.WorkspaceArticleID) != strings.TrimSpace(workspaceID) {
			continue
		}
		if selected == nil || run.StartedAt.After(selected.StartedAt) || (run.StartedAt.Equal(selected.StartedAt) && strings.TrimSpace(run.ID) > strings.TrimSpace(selected.ID)) {
			copyRun := run
			selected = &copyRun
		}
	}
	return selected
}

func mustLoadBrowserArticle(ctx context.Context, query *service.ArticleQueryService, id string, fallback *service.BrowserArticle) *service.BrowserArticle {
	if query == nil {
		return fallback
	}
	item, err := query.GetArticle(ctx, id)
	if err != nil {
		return fallback
	}
	return item
}
