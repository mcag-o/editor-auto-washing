package handlers

import (
	"content-hub/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type IngestionHandler struct {
	svc *service.IngestionPipelineService
}

func NewIngestionHandler(svc *service.IngestionPipelineService) *IngestionHandler {
	return &IngestionHandler{svc: svc}
}

func (h *IngestionHandler) Import(c *gin.Context) {
	var req struct {
		WorkspaceRoot string `json:"workspace_root" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.svc.ImportIncoming(c.Request.Context(), req.WorkspaceRoot)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, result)
}

func (h *IngestionHandler) RetryFailed(c *gin.Context) {
	var req struct {
		WorkspaceRoot string `json:"workspace_root" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.svc.RetryFailed(c.Request.Context(), req.WorkspaceRoot)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *IngestionHandler) List(c *gin.Context) {
	data, err := h.svc.ListRecords(c.Request.Context(), c.Query("status"))
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func (h *IngestionHandler) Status(c *gin.Context) {
	status, err := h.svc.GetStatus(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, status)
}
