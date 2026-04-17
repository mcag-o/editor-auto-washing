package handlers

import (
	"content-hub/domain"
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

type rssItemsService interface {
	List(ctx context.Context, limit int) ([]domain.RSSItemRecord, error)
	GetByID(ctx context.Context, id string) (*domain.RSSItemRecord, error)
}

type RSSItemsHandler struct {
	svc rssItemsService
}

func NewRSSItemsHandler(svc rssItemsService) *RSSItemsHandler {
	return &RSSItemsHandler{svc: svc}
}

func (h *RSSItemsHandler) List(c *gin.Context) {
	if h.svc == nil {
		HandleError(c, domain.NewInternalErr("rss item service is not configured", nil))
		return
	}

	limit, err := parseRSSLimit(c.DefaultQuery("limit", "20"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	items, err := h.svc.List(c.Request.Context(), limit)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *RSSItemsHandler) Get(c *gin.Context) {
	if h.svc == nil {
		HandleError(c, domain.NewInternalErr("rss item service is not configured", nil))
		return
	}

	item, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}
