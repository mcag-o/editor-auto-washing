package handlers

import (
	"content-hub/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AutomationHandler struct {
	svc  *service.AutomationService
	root string
}

func NewAutomationHandler(svc *service.AutomationService, root string) *AutomationHandler {
	return &AutomationHandler{svc: svc, root: root}
}

func (h *AutomationHandler) RunOnce(c *gin.Context) {
	result, err := h.svc.RunOnce(c.Request.Context(), h.root)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AutomationHandler) RetryFailed(c *gin.Context) {
	result, err := h.svc.RetryFailed(c.Request.Context(), h.root)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AutomationHandler) Daemon(c *gin.Context) {
	result, err := h.svc.StartDaemon(c.Request.Context(), h.root, 0)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AutomationHandler) Status(c *gin.Context) {
	result, err := h.svc.Status(c.Request.Context(), h.root)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AutomationHandler) Health(c *gin.Context) {
	result, err := h.svc.Health(c.Request.Context(), h.root)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AutomationHandler) Stop(c *gin.Context) {
	result, err := h.svc.Stop(c.Request.Context(), h.root)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
