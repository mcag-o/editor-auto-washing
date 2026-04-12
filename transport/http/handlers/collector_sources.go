package handlers

import (
	"content-hub/domain"
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

type collectorSourcesService interface {
	ListSources(ctx context.Context) ([]domain.CollectorSource, error)
	Health(ctx context.Context) ([]domain.CollectorSourceHealthStatus, error)
}

type CollectorSourcesHandler struct {
	svc collectorSourcesService
}

func NewCollectorSourcesHandler(svc collectorSourcesService) *CollectorSourcesHandler {
	return &CollectorSourcesHandler{svc: svc}
}

func (h *CollectorSourcesHandler) List(c *gin.Context) {
	items, err := h.svc.ListSources(c.Request.Context())
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *CollectorSourcesHandler) Health(c *gin.Context) {
	items, err := h.svc.Health(c.Request.Context())
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}
