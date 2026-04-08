package handlers

import (
	"content-hub/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type DraftHandler struct {
	svc *service.DraftService
}

func NewDraftHandler(svc *service.DraftService) *DraftHandler {
	return &DraftHandler{svc: svc}
}

func (h *DraftHandler) Create(c *gin.Context) {
	var req struct {
		Template string `json:"template" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	draft, err := h.svc.CreateDraft(c.Request.Context(), req.Template)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, draft)
}

func (h *DraftHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	draft, err := h.svc.GetDraft(c.Request.Context(), id)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, draft)
}
