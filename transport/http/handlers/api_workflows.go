package handlers

import (
	"content-hub/domain"
	"content-hub/service"
	"net/http"

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
	if err := h.svc.Upsert(c.Request.Context(), &workflow); err != nil {
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
	workflow.ID = c.Param("id")
	if err := h.svc.Upsert(c.Request.Context(), &workflow); err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, workflow)
}
