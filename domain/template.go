package domain

import (
	"content-hub/pkg/id"
	"time"
)

type TemplateAsset struct {
	ID        string         `json:"id"`
	Category  string         `json:"category"`
	Name      string         `json:"name"`
	Content   string         `json:"content"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

func NewTemplateAsset(category, name, content string) (*TemplateAsset, error) {
	if category == "" || name == "" {
		return nil, NewValidationErr("category and name are required", nil)
	}
	now := time.Now().UTC()
	return &TemplateAsset{
		ID:        id.New(),
		Category:  category,
		Name:      name,
		Content:   content,
		Metadata:  make(map[string]any),
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}
