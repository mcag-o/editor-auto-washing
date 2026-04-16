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

func TestRSSSubscriptionValidateRejectsWhitespaceOnlyFields(t *testing.T) {
	sub := &RSSSubscription{
		Name:          "   ",
		FeedURL:       "https://example.com/feed.xml",
		TargetType:    "wechat-longform",
		SourceProfile: "sspai",
	}
	if err := sub.Validate(); err == nil {
		t.Fatal("expected validate to reject whitespace-only required fields")
	}
}

func TestRSSSubscriptionValidateRejectsInvalidFeedURL(t *testing.T) {
	sub := &RSSSubscription{
		Name:          "Tech Feed",
		FeedURL:       "not-a-url",
		TargetType:    "wechat-longform",
		SourceProfile: "sspai",
	}
	if err := sub.Validate(); err == nil {
		t.Fatal("expected validate to reject invalid feed url")
	}
}

func TestRSSSubscriptionValidateRejectsNegativePollInterval(t *testing.T) {
	sub := &RSSSubscription{
		Name:            "Tech Feed",
		FeedURL:         "https://example.com/feed.xml",
		TargetType:      "wechat-longform",
		SourceProfile:   "sspai",
		PollIntervalSec: -1,
	}
	if err := sub.Validate(); err == nil {
		t.Fatal("expected validate to reject negative poll interval")
	}
}

func TestRSSSubscriptionValidateAcceptsZeroPollInterval(t *testing.T) {
	sub := &RSSSubscription{
		Name:            "Tech Feed",
		FeedURL:         "https://example.com/feed.xml",
		TargetType:      "wechat-longform",
		SourceProfile:   "sspai",
		PollIntervalSec: 0,
	}
	if err := sub.Validate(); err != nil {
		t.Fatalf("expected zero poll interval to validate: %v", err)
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

func TestRSSDuplicateKeyValidateRequiresSubscriptionID(t *testing.T) {
	key := RSSDuplicateKey{GUID: "guid-1"}
	if err := key.Validate(); err == nil {
		t.Fatal("expected validate to reject missing subscription id")
	}
}

func TestRSSDuplicateKeyValidateRequiresIdentityValue(t *testing.T) {
	key := RSSDuplicateKey{SubscriptionID: "sub-1"}
	if err := key.Validate(); err == nil {
		t.Fatal("expected validate to reject empty duplicate identity")
	}
}

func TestRSSDuplicateKeyValidateAcceptsValidKey(t *testing.T) {
	key := RSSDuplicateKey{SubscriptionID: "sub-1", GUID: "guid-1"}
	if err := key.Validate(); err != nil {
		t.Fatalf("expected duplicate key to validate: %v", err)
	}
}

func TestRSSPullRunValidateRejectsMissingFields(t *testing.T) {
	run := RSSPullRun{}
	if err := run.Validate(); err == nil {
		t.Fatal("expected pull run validation to reject missing fields")
	}
}

func TestRSSPullRunValidateAcceptsValidRun(t *testing.T) {
	run := RSSPullRun{SubscriptionID: "sub-1", Status: RSSPullRunStatusPending}
	if err := run.Validate(); err != nil {
		t.Fatalf("expected pull run to validate: %v", err)
	}
}

func TestRSSPullRunValidateRejectsUnsupportedStatus(t *testing.T) {
	run := RSSPullRun{SubscriptionID: "sub-1", Status: "unknown"}
	if err := run.Validate(); err == nil {
		t.Fatal("expected pull run validation to reject unsupported status")
	}
}

func TestRSSItemRecordValidateRejectsMissingFields(t *testing.T) {
	item := RSSItemRecord{}
	if err := item.Validate(); err == nil {
		t.Fatal("expected item record validation to reject missing fields")
	}
}

func TestRSSItemRecordValidateRejectsMissingPullRunID(t *testing.T) {
	item := RSSItemRecord{
		SubscriptionID: "sub-1",
		Title:          "Item title",
		Status:         RSSItemStatusPending,
		GUID:           "guid-1",
	}
	if err := item.Validate(); err == nil {
		t.Fatal("expected item record validation to reject missing pull run id")
	}
}

func TestRSSItemRecordValidateRejectsUnsupportedStatus(t *testing.T) {
	item := RSSItemRecord{
		SubscriptionID: "sub-1",
		PullRunID:      "run-1",
		Title:          "Item title",
		Status:         "unknown",
		GUID:           "guid-1",
	}
	if err := item.Validate(); err == nil {
		t.Fatal("expected item record validation to reject unsupported status")
	}
}

func TestRSSItemRecordValidateAcceptsValidItem(t *testing.T) {
	item := RSSItemRecord{
		SubscriptionID: "sub-1",
		PullRunID:      "run-1",
		Title:          "Item title",
		Status:         RSSItemStatusPending,
		GUID:           "guid-1",
	}
	if err := item.Validate(); err != nil {
		t.Fatalf("expected item record to validate: %v", err)
	}
}
