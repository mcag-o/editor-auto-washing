package domain

import (
	"content-hub/pkg/id"
	"time"
)

type PublishRecord struct {
	ID           string         `json:"id"`
	ArticleID    string         `json:"article_id"`
	ArticleTitle string         `json:"article_title"`
	ReviewID     string         `json:"review_id"`
	AssetID      string         `json:"asset_id"`
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

type PublishOutcome struct {
	Success         bool            `json:"success"`
	Partial         bool            `json:"partial"`
	WorkspaceSynced bool            `json:"workspace_synced"`
	FailedAssetID   string          `json:"failed_asset_id,omitempty"`
	Records         []PublishRecord `json:"records"`
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

func NewPublishRecord(articleID, articleTitle, reviewID, assetID, platform string, success bool, message string, metadata map[string]any) *PublishRecord {
	if metadata == nil {
		metadata = make(map[string]any)
	}
	return &PublishRecord{
		ID:           id.New(),
		ArticleID:    articleID,
		ArticleTitle: articleTitle,
		ReviewID:     reviewID,
		AssetID:      assetID,
		Platform:     platform,
		Success:      success,
		Message:      message,
		Metadata:     metadata,
		CreatedAt:    time.Now().UTC(),
	}
}
