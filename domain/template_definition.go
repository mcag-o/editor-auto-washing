package domain

import (
	"strings"
	"time"
)

type TemplateDefinition struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Type          string    `json:"type"`
	Version       string    `json:"version"`
	Enabled       bool      `json:"enabled"`
	Content       string    `json:"content"`
	VariablesJSON []byte    `json:"variables_json"`
	UpdatedBy     string    `json:"updated_by"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (t TemplateDefinition) Validate() error {
	if strings.TrimSpace(t.Name) == "" {
		return NewValidationErr("template name is required", nil)
	}
	if strings.TrimSpace(t.Type) == "" {
		return NewValidationErr("template type is required", nil)
	}
	if strings.TrimSpace(t.Content) == "" {
		return NewValidationErr("template content is required", nil)
	}
	return nil
}
