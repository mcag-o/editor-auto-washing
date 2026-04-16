package service

import (
	"content-hub/domain"
	"fmt"
	"strings"
)

type RSSFeedItem struct {
	GUID        string
	Title       string
	Link        string
	Description string
	Content     string
	Author      string
	Tags        []string
}

func NormalizeRSSItem(subscriptionID, targetType, sourceProfile, rewriteProfileVersion string, item RSSFeedItem) (domain.IntakeArticle, error) {
	body := strings.TrimSpace(item.Content)
	if body == "" {
		body = strings.TrimSpace(item.Description)
	}

	article := domain.IntakeArticle{
		ExternalID:            strings.TrimSpace(item.GUID),
		SourceType:            "rss",
		SubscriptionID:        strings.TrimSpace(subscriptionID),
		Title:                 strings.TrimSpace(item.Title),
		Body:                  body,
		Summary:               strings.TrimSpace(item.Description),
		Author:                strings.TrimSpace(item.Author),
		OriginalURL:           strings.TrimSpace(item.Link),
		Tags:                  append([]string(nil), item.Tags...),
		TargetType:            strings.TrimSpace(targetType),
		SourceProfile:         strings.TrimSpace(sourceProfile),
		RewriteProfileVersion: strings.TrimSpace(rewriteProfileVersion),
		Metadata:              map[string]any{},
	}

	if err := article.Validate(); err != nil {
		return domain.IntakeArticle{}, fmt.Errorf("normalize rss item: %w", err)
	}
	return article, nil
}
