package handlers

import (
	"content-hub/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type JobHandler struct {
	svc *service.JobService
}

func NewJobHandler(svc *service.JobService) *JobHandler {
	return &JobHandler{svc: svc}
}

func (h *JobHandler) Submit(c *gin.Context) {
	var req struct {
		Topic string `json:"topic" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	job, err := h.svc.Submit(c.Request.Context(), req.Topic)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, job)
}

func (h *JobHandler) List(c *gin.Context) {
	status := c.Query("status")
	var statusPtr *string
	if status != "" {
		statusPtr = &status
	}

	jobs, err := h.svc.ListJobs(c.Request.Context(), statusPtr)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": jobs})
}

func (h *JobHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	job, err := h.svc.GetJob(c.Request.Context(), id)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, job)
}

func (h *JobHandler) Cancel(c *gin.Context) {
	id := c.Param("id")

	_, err := h.svc.GetJob(c.Request.Context(), id)
	if err != nil {
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"cancelled": true, "job_id": id})
}

func (h *JobHandler) GetEvents(c *gin.Context) {
	id := c.Param("id")

	events, err := h.svc.GetEvents(c.Request.Context(), id)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": events})
}
