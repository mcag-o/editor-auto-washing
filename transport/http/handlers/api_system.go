package handlers

import (
	"content-hub/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type APISystemHandler struct {
	svc *service.WebControlPlaneService
}

func NewAPISystemHandler(svc *service.WebControlPlaneService) *APISystemHandler {
	return &APISystemHandler{svc: svc}
}

func (h *APISystemHandler) Start(c *gin.Context) {
	var req struct {
		ConcurrencyLimit int `json:"concurrency_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	state, err := h.svc.Start(c.Request.Context(), "local-admin", req.ConcurrencyLimit)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, state)
}

func (h *APISystemHandler) Pause(c *gin.Context) {
	state, err := h.svc.Pause(c.Request.Context(), "local-admin")
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, state)
}

func (h *APISystemHandler) Resume(c *gin.Context) {
	state, err := h.svc.Resume(c.Request.Context(), "local-admin")
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, state)
}

func (h *APISystemHandler) Status(c *gin.Context) {
	state, err := h.svc.Get(c.Request.Context())
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, state)
}
