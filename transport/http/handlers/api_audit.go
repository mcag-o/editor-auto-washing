package handlers

import (
	"content-hub/pkg/repo"
	"content-hub/service"
	"net/http"
	"strings"

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
	resource := strings.TrimSpace(c.Query("resource"))
	workflowRunID := strings.TrimSpace(c.Query("workflow_run_id"))
	actionPrefix := strings.TrimSpace(c.Query("action_prefix"))
	resourceID := strings.TrimSpace(c.Query("resource_id"))
	if resource != "" || workflowRunID != "" || actionPrefix != "" || resourceID != "" {
		logs, err := h.svc.ListByQuery(c.Request.Context(), service.AuditLogQuery{
			Resource:     resource,
			WorkflowRunID: workflowRunID,
			ActionPrefix: actionPrefix,
			ResourceID:   resourceID,
		})
		if err != nil {
			HandleError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": logs})
		return
	}
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
