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

	item.Status = domain.SourceDocumentStatusPending
	item.ErrorSummary = ""
	item.ClaimedBy = ""
	item.ClaimedAt = nil
	item.ProcessingStartedAt = nil
	item.CompletedAt = nil
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
