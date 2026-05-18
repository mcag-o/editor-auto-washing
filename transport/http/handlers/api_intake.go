package handlers

import (
	"content-hub/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type APIIntakeHandler struct {
	svc *service.WebIntakeService
}

func NewAPIIntakeHandler(svc *service.WebIntakeService) *APIIntakeHandler {
	return &APIIntakeHandler{svc: svc}
}

func (h *APIIntakeHandler) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	opened, err := file.Open()
	if err != nil {
		HandleError(c, err)
		return
	}
	defer opened.Close()

	doc, err := h.svc.CreateFromUpload(c.Request.Context(), service.CreateUploadIntakeInput{
		Actor:       "local-admin",
		Filename:    file.Filename,
		ContentType: file.Header.Get("Content-Type"),
		Content:     opened,
	})
	if err != nil {
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, service.BuildBrowserIntakeResponse(doc))
}

func (h *APIIntakeHandler) Paste(c *gin.Context) {
	var req struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	doc, err := h.svc.CreateFromPaste(c.Request.Context(), service.CreatePasteIntakeInput{
		Actor: "local-admin",
		Title: req.Title,
		Body:  req.Body,
	})
	if err != nil {
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, service.BuildBrowserIntakeResponse(doc))
}
