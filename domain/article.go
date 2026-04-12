package domain

import (
	"content-hub/pkg/id"
	"time"
)

type ContentDocument struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Body      string         `json:"body"`
	Format    string         `json:"format"`
	Summary   string         `json:"summary"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

func NewContentDocument(title, body, format string) (*ContentDocument, error) {
	if title == "" {
		return nil, NewValidationErr("title is required", nil)
	}
	if format == "" {
		format = "markdown"
	}
	now := time.Now().UTC()
	return &ContentDocument{
		ID:        id.New(),
		Title:     title,
		Body:      body,
		Format:    format,
		Metadata:  make(map[string]any),
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

type ListQuery struct {
	TitleQuery string
	Published  *bool
	Limit      int
	Offset     int
}
