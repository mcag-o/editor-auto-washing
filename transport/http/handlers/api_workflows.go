package handlers

import (
	"content-hub/domain"
	"content-hub/pkg/id"
	"content-hub/service"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type APIWorkflowsHandler struct {
	svc *service.WorkflowTemplateService
}

func NewAPIWorkflowsHandler(svc *service.WorkflowTemplateService) *APIWorkflowsHandler {
	return &APIWorkflowsHandler{svc: svc}
}

func (h *APIWorkflowsHandler) Create(c *gin.Context) {
	var workflow domain.WorkflowDefinition
	if err := c.ShouldBindJSON(&workflow); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(workflow.ID) == "" {
		workflow.ID = id.New()
	}
	if err := h.svc.Create(c.Request.Context(), &workflow); err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, workflow)
}

func (h *APIWorkflowsHandler) Get(c *gin.Context) {
	workflow, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, workflow)
}

func (h *APIWorkflowsHandler) List(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context(), 100)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *APIWorkflowsHandler) Update(c *gin.Context) {
	var workflow domain.WorkflowDefinition
	if err := c.ShouldBindJSON(&workflow); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	targetID := c.Param("id")
	if _, err := h.svc.GetByID(c.Request.Context(), targetID); err != nil {
		HandleError(c, err)
		return
	}
	workflow.ID = targetID
	if err := h.svc.Upsert(c.Request.Context(), &workflow); err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, workflow)
}

func (h *APIWorkflowsHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		HandleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
