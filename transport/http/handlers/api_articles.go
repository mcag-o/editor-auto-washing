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

	"github.com/gin-gonic/gin"
)

type APIArticlesHandler struct {
	articles  *service.ArticleQueryService
	runs      repo.RewritePipelineRunRepo
	stages    repo.RewriteStageRunRepo
	source    repo.SourceDocumentRepo
	workflows interface {
		GetByID(context.Context, string) (*domain.WorkflowDefinition, error)
	}
	audit   repo.AuditLogRepo
	control interface {
		Get(context.Context) (*domain.SystemControlState, error)
	}
}

type sourceDocumentDeleteRepo interface {
	Delete(context.Context, string) error
}

type rewriteStageRunUpdater interface {
	Update(context.Context, *domain.RewriteStageRun) error
}

const articleOperationsActor = "local-admin"

func NewAPIArticlesHandler(articles *service.ArticleQueryService, runs repo.RewritePipelineRunRepo, stages repo.RewriteStageRunRepo, source repo.SourceDocumentRepo, workflows interface {
	GetByID(context.Context, string) (*domain.WorkflowDefinition, error)
}, audit repo.AuditLogRepo, control interface {
	Get(context.Context) (*domain.SystemControlState, error)
}) *APIArticlesHandler {
	return &APIArticlesHandler{articles: articles, runs: runs, stages: stages, source: source, workflows: workflows, audit: audit, control: control}
}

func (h *APIArticlesHandler) List(c *gin.Context) {
	items, err := h.articles.ListSourceDocuments(c.Request.Context(), service.ArticleQueryFilter{Status: c.Query("status"), Limit: 100})
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *APIArticlesHandler) Get(c *gin.Context) {
	item, err := h.articles.GetSourceDocument(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *APIArticlesHandler) Stages(c *gin.Context) {
	item, err := h.articles.GetSourceDocument(c.Request.Context(), c.Param("id"))
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

	c.JSON(http.StatusOK, gin.H{
		"article": item,
		"run":     run,
		"stages":  stageRuns,
	})
}

func (h *APIArticlesHandler) Retry(c *gin.Context) {
	item, err := h.source.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	if item.Status != domain.SourceDocumentStatusFailed {
		err := domain.NewValidationErr("retry is only allowed from failed state", nil)
		h.recordArticleAudit(c.Request.Context(), item, "retry", "failure", err.Error(), nil)
		HandleError(c, err)
		return
	}
	previousRunID := item.RewriteRunID
	workflowStateReset := false

	item.Status = domain.SourceDocumentStatusPending
	item.ErrorSummary = ""
	item.ClaimedBy = ""
	item.ClaimedAt = nil
	item.ProcessingStartedAt = nil
	item.CompletedAt = nil
	item.RewriteRunID = previousRunID

	workflowChanged, err := h.workflowExecutionChanged(c.Request.Context(), item)
	if err != nil {
		HandleError(c, err)
		return
	}
	if workflowChanged {
		workflowStateReset = true
	}
	if workflowStateReset {
		item.RewriteRunID = ""
	}

	if err := h.source.UpdateIfStatus(c.Request.Context(), item, domain.SourceDocumentStatusFailed); err != nil {
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

	c.JSON(http.StatusOK, gin.H{
		"status":         "requeued",
		"message":        message,
		"worker_running": workerRunning,
		"system_state":   systemState,
		"article":        item,
	})
}

func (h *APIArticlesHandler) Stop(c *gin.Context) {
	item, err := h.source.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	if item.Status != domain.SourceDocumentStatusProcessing {
		err := domain.NewValidationErr("stop is only allowed from processing state", nil)
		h.recordArticleAudit(c.Request.Context(), item, "stop", "failure", err.Error(), nil)
		HandleError(c, err)
		return
	}

	item.Status = domain.SourceDocumentStatusPaused
	item.ClaimedBy = ""
	item.ClaimedAt = nil
	if err := h.source.UpdateIfStatus(c.Request.Context(), item, domain.SourceDocumentStatusProcessing); err != nil {
		HandleError(c, err)
		return
	}
	message := "pause requested; the current worker step is not synchronously interrupted"
	h.recordArticleAuditBestEffort(c.Request.Context(), item, "stop", "success", message, map[string]any{
		"workflow_position_preserved": strings.TrimSpace(item.RewriteRunID) != "",
		"pause_requested":            true,
		"workflow_run_id":            strings.TrimSpace(item.RewriteRunID),
		"pause_source":               "manual",
	})

	c.JSON(http.StatusAccepted, gin.H{
		"status":          domain.SourceDocumentStatusPaused,
		"requested_pause": true,
		"message":         message,
		"article":         item,
	})
}

func (h *APIArticlesHandler) Resume(c *gin.Context) {
	item, err := h.source.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	if item.Status != domain.SourceDocumentStatusPaused {
		err := domain.NewValidationErr("resume is only allowed from paused state", nil)
		h.recordArticleAudit(c.Request.Context(), item, "resume", "failure", err.Error(), nil)
		HandleError(c, err)
		return
	}
	if strings.TrimSpace(item.ClaimedBy) != "" || item.ClaimedAt != nil {
		err := domain.NewValidationErr("resume is not allowed while prior processing markers are still active", nil)
		h.recordArticleAudit(c.Request.Context(), item, "resume", "failure", err.Error(), nil)
		HandleError(c, err)
		return
	}

	item.Status = domain.SourceDocumentStatusPending
	item.ClaimedBy = ""
	item.ClaimedAt = nil
	item.ProcessingStartedAt = nil
	if err := h.source.UpdateIfStatus(c.Request.Context(), item, domain.SourceDocumentStatusPaused); err != nil {
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

	c.JSON(http.StatusAccepted, gin.H{
		"status":         "requeued",
		"message":        message,
		"worker_running": workerRunning,
		"system_state":   systemState,
		"article":        item,
	})
}

func (h *APIArticlesHandler) Delete(c *gin.Context) {
	item, err := h.source.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	if item.Status == domain.SourceDocumentStatusProcessing {
		err := domain.NewValidationErr("delete is not allowed while article is processing", nil)
		h.recordArticleAudit(c.Request.Context(), item, "delete", "failure", err.Error(), nil)
		HandleError(c, err)
		return
	}
	if item.Status != domain.SourceDocumentStatusPending && item.Status != domain.SourceDocumentStatusPaused && item.Status != domain.SourceDocumentStatusCompleted {
		err := domain.NewValidationErr("delete is only allowed from pending, paused, or completed state", nil)
		h.recordArticleAudit(c.Request.Context(), item, "delete", "failure", err.Error(), nil)
		HandleError(c, err)
		return
	}

	workflowRecordsDeleted := false
	runID := strings.TrimSpace(item.RewriteRunID)
	if err := h.source.DeleteIfStatus(c.Request.Context(), item.ID, domain.SourceDocumentStatusPending, domain.SourceDocumentStatusPaused, domain.SourceDocumentStatusCompleted); err != nil {
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
	item, err := h.source.GetByID(c.Request.Context(), c.Param("id"))
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
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}
	item.Metadata["workflow_template_id"] = workflow.ID
	item.Metadata["workflow_template_name"] = workflow.Name
	item.Metadata["workflow_template_version"] = workflow.Version
	if err := h.source.Update(c.Request.Context(), item); err != nil {
		HandleError(c, err)
		return
	}
	if _, err := service.NewAuditLogService(h.audit).Create(c.Request.Context(), service.AuditLogCreateInput{
		Actor:      articleOperationsActor,
		Action:     "web_control.article.workflow_template_assigned",
		Resource:   "source_document",
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
	c.JSON(http.StatusOK, item)
}

func (h *APIArticlesHandler) recordArticleAudit(ctx context.Context, item *domain.SourceDocument, action, result, message string, metadata map[string]any) error {
	if h == nil || h.audit == nil || item == nil {
		return nil
	}
	_, err := service.NewAuditLogService(h.audit).Create(ctx, service.AuditLogCreateInput{
		Actor:      articleOperationsActor,
		Action:     "web_control.article." + strings.TrimSpace(action),
		Resource:   "source_document",
		ResourceID: item.ID,
		Result:     strings.TrimSpace(result),
		Message:    strings.TrimSpace(message),
		Metadata:   metadata,
	})
	return err
}

func (h *APIArticlesHandler) recordArticleAuditBestEffort(ctx context.Context, item *domain.SourceDocument, action, result, message string, metadata map[string]any) {
	if err := h.recordArticleAudit(ctx, item, action, result, message, metadata); err != nil {
		log.Printf("warning: write article lifecycle audit action=%s resource_id=%s: %v", strings.TrimSpace(action), item.ID, err)
	}
}

func (h *APIArticlesHandler) workflowExecutionChanged(ctx context.Context, item *domain.SourceDocument) (bool, error) {
	if item == nil || strings.TrimSpace(item.RewriteRunID) == "" {
		return false, nil
	}
	run, err := h.runs.GetByID(ctx, item.RewriteRunID)
	if err != nil {
		return false, err
	}
	return workflowMetadataValue(item.Metadata, "workflow_template_id") != workflowMetadataValue(run.Metadata, "workflow_template_id") ||
		workflowMetadataValue(item.Metadata, "workflow_template_version") != workflowMetadataValue(run.Metadata, "workflow_template_version") ||
		workflowMetadataValue(item.Metadata, "workflow_template_name") != workflowMetadataValue(run.Metadata, "workflow_template_name"), nil
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
	if err := h.stages.DeleteByPipelineRunID(ctx, runID); err != nil {
		return err
	}
	if err := h.runs.Delete(ctx, runID); err != nil {
		return err
	}
	return nil
}

func (h *APIArticlesHandler) resetWorkflowExecution(ctx context.Context, item *domain.SourceDocument) error {
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
