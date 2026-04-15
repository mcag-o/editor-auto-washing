package handlers

import (
	"content-hub/domain"
	"content-hub/service"
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

type rewriteRunner interface {
	Run(ctx context.Context, req service.RewriteRunRequest) (*domain.RewritePipelineRun, error)
}

type RewriteHandler struct {
	svc rewriteRunner
}

func NewRewriteHandler(svc rewriteRunner) *RewriteHandler {
	return &RewriteHandler{svc: svc}
}

func (h *RewriteHandler) Run(c *gin.Context) {
	if h.svc == nil {
		HandleError(c, domain.NewInternalErr("rewrite orchestrator is not configured", nil))
		return
	}

	var req service.RewriteRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	run, err := h.svc.Run(c.Request.Context(), req)
	if err != nil {
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, run)
}
