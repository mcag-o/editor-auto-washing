package handlers

import (
	"content-hub/domain"
	"content-hub/service"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type APIIntakeHandler struct {
	svc    *service.WebIntakeService
	jobSvc *service.JobService
}

func NewAPIIntakeHandler(svc *service.WebIntakeService, jobSvc ...*service.JobService) *APIIntakeHandler {
	h := &APIIntakeHandler{svc: svc}
	if len(jobSvc) > 0 {
		h.jobSvc = jobSvc[0]
	}
	return h
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

func (h *APIIntakeHandler) Article(c *gin.Context) {
	if h.jobSvc == nil {
		HandleError(c, domain.NewInternalErr("job service is not configured", nil))
		return
	}
	var req domain.IntakeArticle
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	doc, err := h.svc.CreateFromExternalArticle(c.Request.Context(), service.CreateExternalArticleIntakeInput{
		Actor:   "external-api",
		Article: req,
	})
	if err != nil {
		HandleError(c, err)
		return
	}
	job, err := h.jobSvc.SubmitWithArtifact(c.Request.Context(), service.ExternalIntakeProcessTopic, doc.ID)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"workspace_article": service.BuildBrowserIntakeResponse(doc), "job": job})
}

func (h *APIIntakeHandler) File(c *gin.Context) {
	if h.jobSvc == nil {
		HandleError(c, domain.NewInternalErr("job service is not configured", nil))
		return
	}
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
	metadata, err := parseExternalFileMetadata(c.PostForm("metadata"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	doc, err := h.svc.CreateFromExternalFile(c.Request.Context(), service.CreateExternalFileIntakeInput{
		Actor:                 "external-api",
		Filename:              file.Filename,
		ContentType:           file.Header.Get("Content-Type"),
		Content:               opened,
		SourceType:            c.PostForm("source_type"),
		OriginalURL:           c.PostForm("original_url"),
		TargetType:            c.PostForm("target_type"),
		SourceProfile:         c.PostForm("source_profile"),
		RewriteProfileVersion: c.PostForm("rewrite_profile_version"),
		Metadata:              metadata,
	})
	if err != nil {
		HandleError(c, err)
		return
	}
	job, err := h.jobSvc.SubmitWithArtifact(c.Request.Context(), service.ExternalIntakeProcessTopic, doc.ID)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"workspace_article": service.BuildBrowserIntakeResponse(doc), "job": job})
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

func parseExternalFileMetadata(raw string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		return nil, err
	}
	return metadata, nil
}
