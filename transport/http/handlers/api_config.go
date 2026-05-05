package handlers

import (
	"content-hub/domain"
	"content-hub/service"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	apiConfigCategory = "web_control"
	apiConfigKey      = "settings"
)

type APIConfigHandler struct {
	svc *service.BusinessConfigService
}

func NewAPIConfigHandler(svc *service.BusinessConfigService) *APIConfigHandler {
	return &APIConfigHandler{svc: svc}
}

func (h *APIConfigHandler) Get(c *gin.Context) {
	value, err := h.svc.Get(c.Request.Context(), apiConfigCategory, apiConfigKey)
	if err != nil {
		if appErr, ok := err.(*domain.AppError); ok && appErr.Code == domain.ErrNotFound {
			c.JSON(http.StatusOK, gin.H{})
			return
		}
		HandleError(c, err)
		return
	}

	var payload map[string]any
	if err := json.Unmarshal(value.ValueJSON, &payload); err != nil {
		HandleError(c, domain.NewInternalErr("decode web control config", err))
		return
	}
	if payload == nil {
		payload = map[string]any{}
	}
	c.JSON(http.StatusOK, payload)
}

func (h *APIConfigHandler) Update(c *gin.Context) {
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	valueJSON, err := json.Marshal(payload)
	if err != nil {
		HandleError(c, domain.NewValidationErr("config payload must be valid json", err))
		return
	}
	if err := h.svc.SetJSON(c.Request.Context(), apiConfigCategory, apiConfigKey, valueJSON, "local-admin"); err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, payload)
}
