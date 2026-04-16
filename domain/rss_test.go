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
	if err := sub.Validate(); err != nil {
		t.Fatalf("expected new subscription to validate: %v", err)
	}
}

func TestRSSSubscriptionValidateRequiresCoreFields(t *testing.T) {
	sub := &RSSSubscription{}
	if err := sub.Validate(); err == nil {
		t.Fatal("expected validate to reject empty subscription fields")
	}
}

func TestIntakeArticleRequiresRequiredFields(t *testing.T) {
	article := IntakeArticle{SourceType: "", Title: "", Body: "", OriginalURL: "", TargetType: "", SourceProfile: ""}
	if err := article.Validate(); err == nil {
		t.Fatal("expected validate to reject missing required fields")
	}
}

func TestIntakeArticleValidateAcceptsCompleteArticle(t *testing.T) {
	article := IntakeArticle{
		SourceType:    "rss",
		Title:         "Article title",
		Body:          "Article body",
		OriginalURL:   "https://example.com/articles/1",
		TargetType:    "wechat-longform",
		SourceProfile: "sspai",
	}
	if err := article.Validate(); err != nil {
		t.Fatalf("expected intake article to validate: %v", err)
	}
}
