package handlers

import (
	"content-hub/domain"
	"content-hub/pkg/id"
	"content-hub/service"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type APITemplatesHandler struct {
	svc *service.TemplateDefinitionService
}

func NewAPITemplatesHandler(svc *service.TemplateDefinitionService) *APITemplatesHandler {
	return &APITemplatesHandler{svc: svc}
}

type apiTemplatePayload struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Type          string          `json:"type"`
	Version       string          `json:"version"`
	Enabled       bool            `json:"enabled"`
	Content       string          `json:"content"`
	VariablesJSON json.RawMessage `json:"variables_json"`
	UpdatedBy     string          `json:"updated_by"`
}

func (h *APITemplatesHandler) Create(c *gin.Context) {
	template, err := decodeTemplateDefinitionPayload(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(template.ID) == "" {
		template.ID = id.New()
	} else {
		if _, err := h.svc.GetByID(c.Request.Context(), template.ID); err == nil {
			HandleError(c, domain.NewConflictErr("template definition already exists"))
			return
		} else {
			var appErr *domain.AppError
			if !errors.As(err, &appErr) || appErr.Code != domain.ErrNotFound {
				HandleError(c, err)
				return
			}
		}
	}
	if err := h.svc.Upsert(c.Request.Context(), template); err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, template)
}

func (h *APITemplatesHandler) Get(c *gin.Context) {
	template, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, template)
}

func (h *APITemplatesHandler) List(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context(), 100)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *APITemplatesHandler) Update(c *gin.Context) {
	template, err := decodeTemplateDefinitionPayload(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	targetID := c.Param("id")
	if _, err := h.svc.GetByID(c.Request.Context(), targetID); err != nil {
		HandleError(c, err)
		return
	}
	template.ID = targetID
	if err := h.svc.Upsert(c.Request.Context(), template); err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, template)
}

func (h *APITemplatesHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		HandleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func decodeTemplateDefinitionPayload(c *gin.Context) (*domain.TemplateDefinition, error) {
	var payload apiTemplatePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		return nil, err
	}
	variablesJSON := []byte(`{}`)
	if len(payload.VariablesJSON) > 0 {
		variablesJSON = append([]byte(nil), payload.VariablesJSON...)
	}
	return &domain.TemplateDefinition{
		ID:            payload.ID,
		Name:          payload.Name,
		Type:          payload.Type,
		Version:       payload.Version,
		Enabled:       payload.Enabled,
		Content:       payload.Content,
		VariablesJSON: variablesJSON,
		UpdatedBy:     payload.UpdatedBy,
	}, nil
}
