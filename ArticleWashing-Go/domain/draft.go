package domain

import (
	"content-hub/pkg/id"
	"time"
)

type ArticleDraft struct {
	ID              string         `json:"id"`
	Template        string         `json:"template"`
	Meta            map[string]any `json:"meta"`
	Headline        map[string]any `json:"headline"`
	Sections        []any          `json:"sections"`
	Conclusion      string         `json:"conclusion"`
	CTA             string         `json:"cta"`
	SourceRefs      []any          `json:"source_refs"`
	TargetPlatforms []string       `json:"target_platforms"`
	ProviderProfile string         `json:"provider_profile"`
	ArticleProfile  string         `json:"article_profile"`
	PublishProfile  string         `json:"publish_profile"`
	Status          string         `json:"status"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

func NewArticleDraft(template string) *ArticleDraft {
	now := time.Now().UTC()
	return &ArticleDraft{
		ID:              id.New(),
		Template:        template,
		Meta:            make(map[string]any),
		Headline:        make(map[string]any),
		Sections:        []any{},
		SourceRefs:      []any{},
		TargetPlatforms: []string{},
		Status:          "draft",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}
