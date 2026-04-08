package domain

import "time"

type IngestionRecord struct {
	ID         string         `json:"id"`
	SourceType string         `json:"source_type"`
	Payload    map[string]any `json:"payload"`
	Status     string         `json:"status"`
	CreatedAt  time.Time      `json:"created_at"`
}

type IngestionBundle struct {
	BundleVersion string `json:"bundle_version"`
	GeneratedAt   string `json:"generated_at"`
	Sources       []any  `json:"sources"`
	Items         []any  `json:"items"`
	Failures      []any  `json:"failures"`
}

type CollectResult struct {
	Bundle   *IngestionBundle `json:"bundle"`
	Failures []CollectError   `json:"failures"`
	Duration string           `json:"duration"`
}

type CollectError struct {
	Platform string `json:"platform"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

type PlatformInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Enabled     bool   `json:"enabled"`
	SourceType  string `json:"source_type"`
	SourceURL   string `json:"source_url"`
}
