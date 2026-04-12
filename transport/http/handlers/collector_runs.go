package handlers

import (
	"content-hub/domain"
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type collectorRunsService interface {
	ListRuns(ctx context.Context, limit int) ([]domain.CollectorRun, error)
	GetRun(ctx context.Context, runID string) (*domain.CollectorRunDetail, error)
}

type CollectorRunsHandler struct {
	svc collectorRunsService
}

func NewCollectorRunsHandler(svc collectorRunsService) *CollectorRunsHandler {
	return &CollectorRunsHandler{svc: svc}
}

func (h *CollectorRunsHandler) List(c *gin.Context) {
	limit, err := parseCollectorRunLimit(c.DefaultQuery("limit", "20"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	items, err := h.svc.ListRuns(c.Request.Context(), limit)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *CollectorRunsHandler) Get(c *gin.Context) {
	item, err := h.svc.GetRun(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func parseCollectorRunLimit(value string) (int, error) {
	value = strings.TrimSpace(value)
	limit, err := strconv.Atoi(value)
	if err != nil || limit <= 0 {
		return 0, domain.NewValidationErr("invalid limit: must be a positive integer", nil)
	}
	return limit, nil
}
