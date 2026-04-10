package handlers

import (
	"content-hub/domain"
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

type collectorSchedulerService interface {
	RunOnce(ctx context.Context) (*domain.CollectorRunSummary, error)
	StartDaemon(ctx context.Context) (*domain.CollectorSchedulerControlResult, error)
	Status(ctx context.Context) (*domain.CollectorSchedulerStatus, error)
	Health(ctx context.Context) (*domain.CollectorSchedulerHealthReport, error)
	Stop(ctx context.Context) (*domain.CollectorSchedulerControlResult, error)
}

type CollectorSchedulerHandler struct {
	svc collectorSchedulerService
}

func NewCollectorSchedulerHandler(svc collectorSchedulerService) *CollectorSchedulerHandler {
	return &CollectorSchedulerHandler{svc: svc}
}

func (h *CollectorSchedulerHandler) RunOnce(c *gin.Context) {
	result, err := h.svc.RunOnce(c.Request.Context())
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *CollectorSchedulerHandler) Daemon(c *gin.Context) {
	result, err := h.svc.StartDaemon(c.Request.Context())
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *CollectorSchedulerHandler) Status(c *gin.Context) {
	result, err := h.svc.Status(c.Request.Context())
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *CollectorSchedulerHandler) Health(c *gin.Context) {
	result, err := h.svc.Health(c.Request.Context())
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *CollectorSchedulerHandler) Stop(c *gin.Context) {
	result, err := h.svc.Stop(c.Request.Context())
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
