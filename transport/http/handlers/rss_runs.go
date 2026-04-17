package handlers

import (
	"content-hub/domain"
	"content-hub/service"
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type rssRunsScheduler interface {
	RunByID(ctx context.Context, subscriptionID string) (*service.RSSPullResult, error)
	RunAll(ctx context.Context) ([]service.RSSScheduledRunResult, error)
}

type rssRunsService interface {
	rssRunsScheduler
	List(ctx context.Context, limit int) ([]domain.RSSPullRun, error)
	GetByID(ctx context.Context, id string) (*domain.RSSPullRun, error)
}

type RSSRunsHandler struct {
	svc rssRunsService
}

func NewRSSRunsHandler(svc rssRunsService) *RSSRunsHandler {
	return &RSSRunsHandler{svc: svc}
}

func (h *RSSRunsHandler) RunSubscription(c *gin.Context) {
	if h.svc == nil {
		HandleError(c, domain.NewInternalErr("rss scheduler is not configured", nil))
		return
	}

	result, err := h.svc.RunByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, result)
}

func (h *RSSRunsHandler) RunAll(c *gin.Context) {
	if h.svc == nil {
		HandleError(c, domain.NewInternalErr("rss scheduler is not configured", nil))
		return
	}

	results, err := h.svc.RunAll(c.Request.Context())
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, results)
}

func (h *RSSRunsHandler) List(c *gin.Context) {
	if h.svc == nil {
		HandleError(c, domain.NewInternalErr("rss pull run service is not configured", nil))
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

func (h *RSSRunsHandler) Get(c *gin.Context) {
	if h.svc == nil {
		HandleError(c, domain.NewInternalErr("rss pull run service is not configured", nil))
		return
	}

	item, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func parseRSSLimit(value string) (int, error) {
	value = strings.TrimSpace(value)
	limit, err := strconv.Atoi(value)
	if err != nil || limit <= 0 {
		return 0, domain.NewValidationErr("invalid limit: must be a positive integer", nil)
	}
	return limit, nil
}
