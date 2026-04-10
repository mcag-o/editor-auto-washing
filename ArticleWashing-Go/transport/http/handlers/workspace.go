package handlers

import (
	"content-hub/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type WorkspaceHandler struct {
	svc *service.WorkspaceArticleService
}

func NewWorkspaceHandler(svc *service.WorkspaceArticleService) *WorkspaceHandler {
	return &WorkspaceHandler{svc: svc}
}

func (h *WorkspaceHandler) List(c *gin.Context) {
	items, err := h.svc.ListArticles(c.Request.Context(), c.Query("status"))
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}
