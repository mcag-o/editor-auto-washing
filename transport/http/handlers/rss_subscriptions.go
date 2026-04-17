package handlers

import (
	"content-hub/domain"
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

type rssSubscriptionsService interface {
	Create(ctx context.Context, sub *domain.RSSSubscription) (*domain.RSSSubscription, error)
	Get(ctx context.Context, id string) (*domain.RSSSubscription, error)
	List(ctx context.Context) ([]domain.RSSSubscription, error)
	Update(ctx context.Context, sub *domain.RSSSubscription) (*domain.RSSSubscription, error)
	Delete(ctx context.Context, id string) error
}

type RSSSubscriptionsHandler struct {
	svc rssSubscriptionsService
}

func NewRSSSubscriptionsHandler(svc rssSubscriptionsService) *RSSSubscriptionsHandler {
	return &RSSSubscriptionsHandler{svc: svc}
}

type rssSubscriptionRequest struct {
	Name                  string         `json:"name" binding:"required"`
	FeedURL               string         `json:"feed_url" binding:"required"`
	TargetType            string         `json:"target_type" binding:"required"`
	SourceProfile         string         `json:"source_profile" binding:"required"`
	RewriteProfileVersion *string        `json:"rewrite_profile_version"`
	Enabled               *bool          `json:"enabled"`
	PollIntervalSec       *int           `json:"poll_interval_sec"`
	Metadata              map[string]any `json:"metadata"`
}

func (h *RSSSubscriptionsHandler) Create(c *gin.Context) {
	if h.svc == nil {
		HandleError(c, domain.NewInternalErr("rss subscription service is not configured", nil))
		return
	}

	var req rssSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sub := domain.NewRSSSubscription(req.Name, req.FeedURL, req.TargetType, req.SourceProfile)
	if req.RewriteProfileVersion != nil {
		sub.RewriteProfileVersion = *req.RewriteProfileVersion
	}
	if req.Enabled != nil {
		sub.Enabled = *req.Enabled
	}
	if req.PollIntervalSec != nil {
		sub.PollIntervalSec = *req.PollIntervalSec
	}
	if req.Metadata != nil {
		sub.Metadata = req.Metadata
	}

	created, err := h.svc.Create(c.Request.Context(), sub)
	if err != nil {
		HandleError(c, err)
		return
	}
	if created == nil {
		HandleError(c, domain.NewInternalErr("rss subscription service returned nil subscription", nil))
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *RSSSubscriptionsHandler) List(c *gin.Context) {
	if h.svc == nil {
		HandleError(c, domain.NewInternalErr("rss subscription service is not configured", nil))
		return
	}

	items, err := h.svc.List(c.Request.Context())
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *RSSSubscriptionsHandler) Get(c *gin.Context) {
	if h.svc == nil {
		HandleError(c, domain.NewInternalErr("rss subscription service is not configured", nil))
		return
	}

	item, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *RSSSubscriptionsHandler) Update(c *gin.Context) {
	if h.svc == nil {
		HandleError(c, domain.NewInternalErr("rss subscription service is not configured", nil))
		return
	}

	var req rssSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sub, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	if sub == nil {
		HandleError(c, domain.NewInternalErr("rss subscription service returned nil subscription", nil))
		return
	}

	sub.Name = req.Name
	sub.FeedURL = req.FeedURL
	sub.TargetType = req.TargetType
	sub.SourceProfile = req.SourceProfile
	if req.RewriteProfileVersion != nil {
		sub.RewriteProfileVersion = *req.RewriteProfileVersion
	}
	if req.Enabled != nil {
		sub.Enabled = *req.Enabled
	}
	if req.PollIntervalSec != nil {
		sub.PollIntervalSec = *req.PollIntervalSec
	}
	if req.Metadata != nil {
		sub.Metadata = req.Metadata
	}

	updated, err := h.svc.Update(c.Request.Context(), sub)
	if err != nil {
		HandleError(c, err)
		return
	}
	if updated == nil {
		HandleError(c, domain.NewInternalErr("rss subscription service returned nil subscription", nil))
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *RSSSubscriptionsHandler) Delete(c *gin.Context) {
	if h.svc == nil {
		HandleError(c, domain.NewInternalErr("rss subscription service is not configured", nil))
		return
	}

	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}
