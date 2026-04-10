package domain

import (
	"content-hub/pkg/id"
	"time"
)

const (
	ReviewStatusPending  = "review_pending"
	ReviewStatusApproved = "approved"
	ReviewStatusRejected = "review_rejected"
)

type ReviewTask struct {
	ID             string    `json:"id"`
	ArticleID      string    `json:"article_id"`
	AssetIDs       []string  `json:"asset_ids"`
	Status         string    `json:"status"`
	PublishProfile string    `json:"publish_profile"`
	Reviewer       string    `json:"reviewer"`
	Notes          string    `json:"notes"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func NewReviewTask(articleID string, assetIDs []string, publishProfile string) *ReviewTask {
	now := time.Now().UTC()
	return &ReviewTask{
		ID:             id.New(),
		ArticleID:      articleID,
		AssetIDs:       assetIDs,
		Status:         ReviewStatusPending,
		PublishProfile: publishProfile,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
