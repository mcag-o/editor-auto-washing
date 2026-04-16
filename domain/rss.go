package domain

import (
	"content-hub/pkg/id"
	"strings"
	"time"
)

const RSSPullRunStatusPending = "pending"

type RSSSubscription struct {
	ID                    string         `json:"id"`
	Name                  string         `json:"name"`
	FeedURL               string         `json:"feed_url"`
	TargetType            string         `json:"target_type"`
	SourceProfile         string         `json:"source_profile"`
	RewriteProfileVersion string         `json:"rewrite_profile_version"`
	Enabled               bool           `json:"enabled"`
	PollIntervalSec       int            `json:"poll_interval_sec"`
	LastPulledAt          *time.Time     `json:"last_pulled_at"`
	Metadata              map[string]any `json:"metadata"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
}

type RSSPullRun struct {
	ID             string         `json:"id"`
	SubscriptionID string         `json:"subscription_id"`
	Status         string         `json:"status"`
	StartedAt      time.Time      `json:"started_at"`
	CompletedAt    *time.Time     `json:"completed_at"`
	ErrorSummary   string         `json:"error_summary"`
	Metadata       map[string]any `json:"metadata"`
}

type RSSItemRecord struct {
	ID             string         `json:"id"`
	SubscriptionID string         `json:"subscription_id"`
	PullRunID      string         `json:"pull_run_id"`
	GUID           string         `json:"guid"`
	Link           string         `json:"link"`
	ContentHash    string         `json:"content_hash"`
	Title          string         `json:"title"`
	PublishedAt    *time.Time     `json:"published_at"`
	ImportedAt     *time.Time     `json:"imported_at"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type RSSDuplicateKey struct {
	SubscriptionID string `json:"subscription_id"`
	GUID           string `json:"guid"`
	Link           string `json:"link"`
	ContentHash    string `json:"content_hash"`
}

func NewRSSSubscription(name, feedURL, targetType, sourceProfile string) *RSSSubscription {
	now := time.Now().UTC()
	return &RSSSubscription{
		ID:                    id.New(),
		Name:                  strings.TrimSpace(name),
		FeedURL:               strings.TrimSpace(feedURL),
		TargetType:            strings.TrimSpace(targetType),
		SourceProfile:         strings.TrimSpace(sourceProfile),
		RewriteProfileVersion: "",
		Enabled:               true,
		PollIntervalSec:       3600,
		Metadata:              map[string]any{},
		CreatedAt:             now,
		UpdatedAt:             now,
	}
}

func (s RSSSubscription) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return NewValidationErr("name is required", nil)
	}
	if strings.TrimSpace(s.FeedURL) == "" {
		return NewValidationErr("feed url is required", nil)
	}
	if strings.TrimSpace(s.TargetType) == "" {
		return NewValidationErr("target type is required", nil)
	}
	if strings.TrimSpace(s.SourceProfile) == "" {
		return NewValidationErr("source profile is required", nil)
	}
	return nil
}

func NewRSSPullRun(subscriptionID string) *RSSPullRun {
	now := time.Now().UTC()
	return &RSSPullRun{
		ID:             id.New(),
		SubscriptionID: subscriptionID,
		Status:         RSSPullRunStatusPending,
		StartedAt:      now,
		Metadata:       map[string]any{},
	}
}

func NewRSSItemRecord(subscriptionID, pullRunID, guid, link, contentHash, title string) *RSSItemRecord {
	now := time.Now().UTC()
	return &RSSItemRecord{
		ID:             id.New(),
		SubscriptionID: subscriptionID,
		PullRunID:      pullRunID,
		GUID:           guid,
		Link:           link,
		ContentHash:    contentHash,
		Title:          title,
		Metadata:       map[string]any{},
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
