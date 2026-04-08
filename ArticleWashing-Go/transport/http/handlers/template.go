package handlers

import (
	"content-hub/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type TemplateHandler struct {
	svc *service.TemplateService
}

func NewTemplateHandler(svc *service.TemplateService) *TemplateHandler {
	return &TemplateHandler{svc: svc}
}

func (h *TemplateHandler) Create(c *gin.Context) {
	var req struct {
		Category string `json:"category" binding:"required"`
		Name     string `json:"name" binding:"required"`
		Content  string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tpl, err := h.svc.CreateTemplate(c.Request.Context(), req.Category, req.Name, req.Content)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, tpl)
}

func (h *TemplateHandler) List(c *gin.Context) {
	category := c.Query("category")

	templates, err := h.svc.ListTemplates(c.Request.Context(), category)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": templates})
}

func (h *TemplateHandler) GetCategories(c *gin.Context) {
	categories, err := h.svc.ListCategories(c.Request.Context())
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": categories})
}
