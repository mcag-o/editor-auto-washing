package handlers

import (
	"content-hub/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type FormattingHandler struct {
	svc *service.FormattingPipelineService
}

func NewFormattingHandler(svc *service.FormattingPipelineService) *FormattingHandler {
	return &FormattingHandler{svc: svc}
}

type formattingRequest struct {
	Platform string `json:"platform" binding:"required"`
	Template string `json:"template"`
}

func (h *FormattingHandler) Render(c *gin.Context) {
	var req formattingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	asset, err := h.svc.Render(c.Request.Context(), c.Param("id"), req.Platform, req.Template)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, asset)
}

func (h *FormattingHandler) Validate(c *gin.Context) {
	var req formattingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.svc.Validate(c.Request.Context(), c.Param("id"), req.Platform, req.Template)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *FormattingHandler) GetAsset(c *gin.Context) {
	asset, err := h.svc.GetAsset(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, asset)
}
