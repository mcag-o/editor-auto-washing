package domain

import "testing"

func TestNewRSSSubscriptionDefaultsEnabled(t *testing.T) {
	sub := NewRSSSubscription("Tech Feed", "https://example.com/feed.xml", "wechat-longform", "sspai")
	if !sub.Enabled {
		t.Fatal("expected new subscription to be enabled")
	}
	if sub.FeedURL != "https://example.com/feed.xml" {
		t.Fatalf("unexpected feed url: %s", sub.FeedURL)
	}
}

func TestIntakeArticleRequiresTitleAndBody(t *testing.T) {
	article := IntakeArticle{SourceType: "rss", Title: "", Body: ""}
	if err := article.Validate(); err == nil {
		t.Fatal("expected validate to reject empty title/body")
	}
}
