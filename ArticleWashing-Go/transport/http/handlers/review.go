package handlers

import (
	"content-hub/domain"
	"content-hub/service"
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ReviewHandler struct {
	svc *service.ReviewService
}

func NewReviewHandler(svc *service.ReviewService) *ReviewHandler {
	return &ReviewHandler{svc: svc}
}

func (h *ReviewHandler) Create(c *gin.Context) {
	var req struct {
		ArticleID      string   `json:"article_id" binding:"required"`
		AssetIDs       []string `json:"asset_ids" binding:"required"`
		PublishProfile string   `json:"publish_profile"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	review, err := h.svc.CreateReview(c.Request.Context(), req.ArticleID, req.AssetIDs, req.PublishProfile)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, review)
}

func (h *ReviewHandler) List(c *gin.Context) {
	articleID := c.Query("article_id")
	reviews, err := h.svc.ListReviews(c.Request.Context(), articleID)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": reviews})
}

func (h *ReviewHandler) Approve(c *gin.Context) {
	h.updateStatus(c, h.svc.ApproveReview)
}

func (h *ReviewHandler) Reject(c *gin.Context) {
	h.updateStatus(c, h.svc.RejectReview)
}

func (h *ReviewHandler) updateStatus(c *gin.Context, fn func(ctx context.Context, id, reviewer, notes string) (*domain.ReviewTask, error)) {
	var req struct {
		Reviewer string `json:"reviewer"`
		Notes    string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	review, err := fn(c.Request.Context(), c.Param("id"), req.Reviewer, req.Notes)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, review)
}
