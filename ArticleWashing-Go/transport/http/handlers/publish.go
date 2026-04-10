package handlers

import (
	"content-hub/domain"
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

type publishGateService interface {
	PublishReview(ctx context.Context, reviewID string) (*domain.PublishOutcome, error)
	History(ctx context.Context, articleID string) ([]domain.PublishRecord, error)
}

type PublishHandler struct {
	svc publishGateService
}

func NewPublishHandler(svc publishGateService) *PublishHandler {
	return &PublishHandler{svc: svc}
}

func (h *PublishHandler) Publish(c *gin.Context) {
	var req struct {
		ReviewID string `json:"review_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	outcome, err := h.svc.PublishReview(c.Request.Context(), req.ReviewID)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": outcome})
}

func (h *PublishHandler) History(c *gin.Context) {
	articleID := c.Query("article_id")
	records, err := h.svc.History(c.Request.Context(), articleID)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": records})
}
