package handlers

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"content-hub/service"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type APIArticlesHandler struct {
	articles *service.ArticleQueryService
	runs     repo.RewritePipelineRunRepo
	stages   repo.RewriteStageRunRepo
	source   repo.SourceDocumentRepo
}

func NewAPIArticlesHandler(articles *service.ArticleQueryService, runs repo.RewritePipelineRunRepo, stages repo.RewriteStageRunRepo, source repo.SourceDocumentRepo) *APIArticlesHandler {
	return &APIArticlesHandler{articles: articles, runs: runs, stages: stages, source: source}
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

	c.JSON(http.StatusOK, item)
}
