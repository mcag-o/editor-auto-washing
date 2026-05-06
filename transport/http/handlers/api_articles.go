package handlers

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"content-hub/service"
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type APIArticlesHandler struct {
	articles *service.ArticleQueryService
	runs     repo.RewritePipelineRunRepo
	stages   repo.RewriteStageRunRepo
	source   repo.SourceDocumentRepo
	control  interface {
		Get(context.Context) (*domain.SystemControlState, error)
	}
}

type sourceDocumentDeleteRepo interface {
	Delete(context.Context, string) error
}

type rewriteStageRunUpdater interface {
	Update(context.Context, *domain.RewriteStageRun) error
}

func NewAPIArticlesHandler(articles *service.ArticleQueryService, runs repo.RewritePipelineRunRepo, stages repo.RewriteStageRunRepo, source repo.SourceDocumentRepo, control interface {
	Get(context.Context) (*domain.SystemControlState, error)
}) *APIArticlesHandler {
	return &APIArticlesHandler{articles: articles, runs: runs, stages: stages, source: source, control: control}
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
		HandleError(c, domain.NewValidationErr("retry is only allowed from failed state", nil))
		return
	}
	previousRunID := item.RewriteRunID

	item.Status = domain.SourceDocumentStatusPending
	item.ErrorSummary = ""
	item.ClaimedBy = ""
	item.ClaimedAt = nil
	item.ProcessingStartedAt = nil
	item.CompletedAt = nil
	item.RewriteRunID = previousRunID

	if err := h.resetWorkflowExecution(c.Request.Context(), item); err != nil {
		HandleError(c, err)
		return
	}
	item.RewriteRunID = ""

	if err := h.source.Update(c.Request.Context(), item); err != nil {
		HandleError(c, err)
		return
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
		HandleError(c, domain.NewValidationErr("stop is only allowed from processing state", nil))
		return
	}

	item.Status = domain.SourceDocumentStatusPaused
	if err := h.source.Update(c.Request.Context(), item); err != nil {
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"status":  domain.SourceDocumentStatusPaused,
		"message": "article paused",
		"article": item,
	})
}

func (h *APIArticlesHandler) Resume(c *gin.Context) {
	item, err := h.source.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	if item.Status != domain.SourceDocumentStatusPaused {
		HandleError(c, domain.NewValidationErr("resume is only allowed from paused state", nil))
		return
	}

	item.Status = domain.SourceDocumentStatusPending
	if err := h.source.Update(c.Request.Context(), item); err != nil {
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
		HandleError(c, domain.NewValidationErr("delete is not allowed while article is processing", nil))
		return
	}
	if item.Status != domain.SourceDocumentStatusPending && item.Status != domain.SourceDocumentStatusPaused && item.Status != domain.SourceDocumentStatusCompleted {
		HandleError(c, domain.NewValidationErr("delete is only allowed from pending, paused, or completed state", nil))
		return
	}

	deleteRepo, ok := h.source.(sourceDocumentDeleteRepo)
	if !ok {
		HandleError(c, domain.NewInternalErr("source document delete is not configured", nil))
		return
	}
	if err := deleteRepo.Delete(c.Request.Context(), item.ID); err != nil {
		HandleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
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
