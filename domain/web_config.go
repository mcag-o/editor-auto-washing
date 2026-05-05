package domain

import (
	"content-hub/pkg/id"
	"strings"
	"time"
)

type BusinessConfig struct {
	ID        string         `json:"id"`
	Category  string         `json:"category"`
	Key       string         `json:"key"`
	Value     string         `json:"value"`
	Metadata  map[string]any `json:"metadata"`
	UpdatedBy string         `json:"updated_by"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

func NewBusinessConfig(category, key, value, updatedBy string) *BusinessConfig {
	now := time.Now().UTC()
	return &BusinessConfig{
		ID:        id.New(),
		Category:  strings.TrimSpace(category),
		Key:       strings.TrimSpace(key),
		Value:     value,
		Metadata:  map[string]any{},
		UpdatedBy: strings.TrimSpace(updatedBy),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (c BusinessConfig) Validate() error {
	if strings.TrimSpace(c.Category) == "" {
		return NewValidationErr("category is required", nil)
	}
	if strings.TrimSpace(c.Key) == "" {
		return NewValidationErr("key is required", nil)
	}
	if strings.TrimSpace(c.UpdatedBy) == "" {
		return NewValidationErr("updated by is required", nil)
	}
	return nil
}
