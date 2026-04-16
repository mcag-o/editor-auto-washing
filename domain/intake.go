package domain

import (
	"strings"
	"time"
)

type IntakeArticle struct {
	ExternalID            string         `json:"external_id"`
	SourceType            string         `json:"source_type"`
	SubscriptionID        string         `json:"subscription_id"`
	Title                 string         `json:"title"`
	Body                  string         `json:"body"`
	Summary               string         `json:"summary"`
	Author                string         `json:"author"`
	OriginalURL           string         `json:"original_url"`
	PublishedAt           *time.Time     `json:"published_at"`
	Tags                  []string       `json:"tags"`
	TargetType            string         `json:"target_type"`
	SourceProfile         string         `json:"source_profile"`
	RewriteProfileVersion string         `json:"rewrite_profile_version"`
	Metadata              map[string]any `json:"metadata"`
}

func (a IntakeArticle) Validate() error {
	if strings.TrimSpace(a.SourceType) == "" {
		return NewValidationErr("source type is required", nil)
	}
	if strings.TrimSpace(a.Title) == "" {
		return NewValidationErr("title is required", nil)
	}
	if strings.TrimSpace(a.Body) == "" {
		return NewValidationErr("body is required", nil)
	}
	if strings.TrimSpace(a.OriginalURL) == "" {
		return NewValidationErr("original url is required", nil)
	}
	if strings.TrimSpace(a.TargetType) == "" {
		return NewValidationErr("target type is required", nil)
	}
	if strings.TrimSpace(a.SourceProfile) == "" {
		return NewValidationErr("source profile is required", nil)
	}
	return nil
}
