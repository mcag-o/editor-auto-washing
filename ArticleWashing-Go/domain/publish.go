package domain

import (
	"content-hub/pkg/id"
	"time"
)

type PublishRecord struct {
	ID           string         `json:"id"`
	ArticleTitle string         `json:"article_title"`
	Platform     string         `json:"platform"`
	Success      bool           `json:"success"`
	Message      string         `json:"message"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
}

type PublishResult struct {
	Success  bool           `json:"success"`
	Platform string         `json:"platform"`
	Message  string         `json:"message"`
	Metadata map[string]any `json:"metadata"`
}

type PublishRequest struct {
	Platform    string         `json:"platform"`
	Account     string         `json:"account"`
	Title       string         `json:"title"`
	HTMLContent string         `json:"html_content"`
	Author      string         `json:"author"`
	MediaIDs    []string       `json:"media_ids"`
	Metadata    map[string]any `json:"metadata"`
}

func NewPublishRecord(articleTitle, platform string, success bool, message string, metadata map[string]any) *PublishRecord {
	if metadata == nil {
		metadata = make(map[string]any)
	}
	return &PublishRecord{
		ID:           id.New(),
		ArticleTitle: articleTitle,
		Platform:     platform,
		Success:      success,
		Message:      message,
		Metadata:     metadata,
		CreatedAt:    time.Now().UTC(),
	}
}
