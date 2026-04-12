package handlers

import (
	"content-hub/domain"
	"content-hub/service"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ContentHandler struct {
	svc *service.ContentService
}

func NewContentHandler(svc *service.ContentService) *ContentHandler {
	return &ContentHandler{svc: svc}
}

func (h *ContentHandler) Create(c *gin.Context) {
	var req struct {
		Title  string `json:"title" binding:"required"`
		Body   string `json:"body"`
		Format string `json:"format"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	doc, err := h.svc.CreateDocument(c.Request.Context(), req.Title, req.Body, req.Format)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, doc)
}

func (h *ContentHandler) List(c *gin.Context) {
	q := domain.ListQuery{
		TitleQuery: c.Query("q"),
		Limit:      50,
		Offset:     0,
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			q.Limit = limit
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			q.Offset = offset
		}
	}
	if pubStr := c.Query("published"); pubStr != "" {
		pub := pubStr == "true"
		q.Published = &pub
	}

	docs, err := h.svc.ListDocuments(c.Request.Context(), q)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": docs})
}

func (h *ContentHandler) GetDetail(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id query parameter is required"})
		return
	}

	doc, err := h.svc.GetDocument(c.Request.Context(), id)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, doc)
}

func (h *ContentHandler) GetRead(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id query parameter is required"})
		return
	}

	doc, err := h.svc.GetDocument(c.Request.Context(), id)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, doc)
}

func (h *ContentHandler) Update(c *gin.Context) {
	var req struct {
		ID   string `json:"id" binding:"required"`
		Body string `json:"body" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	doc, err := h.svc.UpdateDocument(c.Request.Context(), req.ID, req.Body)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, doc)
}

func (h *ContentHandler) Delete(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id query parameter is required"})
		return
	}

	err := h.svc.DeleteDocument(c.Request.Context(), id)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}
