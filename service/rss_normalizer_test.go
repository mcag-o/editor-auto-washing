package service

import (
	"content-hub/domain"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRSSNormalizerMapsFeedItemToIntakeArticle(t *testing.T) {
	item := RSSFeedItem{
		GUID:        "guid-1",
		Title:       "Title",
		Link:        "https://example.com/a",
		Description: "Summary",
		Content:     "Body",
	}

	normalized, err := NormalizeRSSItem("sub-1", "wechat-longform", "sspai", "latest", item)

	require.NoError(t, err)
	require.Equal(t, domain.IntakeArticle{
		ExternalID:            "guid-1",
		SourceType:            "rss",
		SubscriptionID:        "sub-1",
		Title:                 "Title",
		Body:                  "Body",
		Summary:               "Summary",
		OriginalURL:           "https://example.com/a",
		TargetType:            "wechat-longform",
		SourceProfile:         "sspai",
		RewriteProfileVersion: "latest",
		Metadata:              map[string]any{},
	}, normalized)
}

func TestRSSNormalizerFallsBackToDescriptionWhenContentMissing(t *testing.T) {
	item := RSSFeedItem{
		GUID:        "guid-2",
		Title:       "Fallback",
		Link:        "https://example.com/b",
		Description: "Summary only",
	}

	normalized, err := NormalizeRSSItem("sub-1", "wechat-longform", "sspai", "latest", item)

	require.NoError(t, err)
	require.Equal(t, "Summary only", normalized.Body)
	require.Equal(t, "Summary only", normalized.Summary)
}
