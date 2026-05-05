package handlers

import (
	"content-hub/pkg/repo"
	"content-hub/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type APIAuditHandler struct {
	svc  *service.AuditLogService
	repo repo.AuditLogRepo
}

func NewAPIAuditHandler(svc *service.AuditLogService, repo repo.AuditLogRepo) *APIAuditHandler {
	return &APIAuditHandler{svc: svc, repo: repo}
}

func (h *APIAuditHandler) List(c *gin.Context) {
	logs, err := h.svc.List(c.Request.Context(), 100)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": logs})
}

func (h *APIAuditHandler) Get(c *gin.Context) {
	log, err := h.repo.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, log)
}
