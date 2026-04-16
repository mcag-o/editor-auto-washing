package sqlite

import (
	"content-hub/domain"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProviderExposesRSSRepos(t *testing.T) {
	provider := newRuntimeProvider(t)

	require.NotNil(t, provider.RSSSubscriptionRepo())
	require.NotNil(t, provider.RSSPullRunRepo())
	require.NotNil(t, provider.RSSItemRepo())
}

func TestRSSSubscriptionRepoCreateAndList(t *testing.T) {
	provider := newRuntimeProvider(t)
	sub := domain.NewRSSSubscription("Tech", "https://example.com/feed.xml", "wechat-longform", "sspai")
	sub.RewriteProfileVersion = "v2"
	sub.PollIntervalSec = 900
	sub.Metadata = map[string]any{"category": "tech"}

	require.NoError(t, provider.RSSSubscriptionRepo().Create(t.Context(), sub))

	list, err := provider.RSSSubscriptionRepo().List(t.Context())
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, sub.ID, list[0].ID)
	require.Equal(t, sub.FeedURL, list[0].FeedURL)
	require.Equal(t, sub.RewriteProfileVersion, list[0].RewriteProfileVersion)
	require.Equal(t, sub.PollIntervalSec, list[0].PollIntervalSec)
	require.Equal(t, "tech", list[0].Metadata["category"])
}

func TestRSSItemRepoFindDuplicateByStructuredKey(t *testing.T) {
	provider := newRuntimeProvider(t)
	sub := domain.NewRSSSubscription("Tech", "https://example.com/feed.xml", "wechat-longform", "sspai")
	require.NoError(t, provider.RSSSubscriptionRepo().Create(t.Context(), sub))
	run := domain.NewRSSPullRun(sub.ID)
	require.NoError(t, provider.RSSPullRunRepo().Create(t.Context(), run))
	item := domain.NewRSSItemRecord(sub.ID, run.ID, "guid-1", "https://example.com/a", "hash-1", "A")
	item.Metadata = map[string]any{"source": "feed"}

	require.NoError(t, provider.RSSItemRepo().Create(t.Context(), item))

	dup, err := provider.RSSItemRepo().FindDuplicate(t.Context(), domain.RSSDuplicateKey{SubscriptionID: sub.ID, GUID: "guid-1"})
	require.NoError(t, err)
	require.Equal(t, item.ID, dup.ID)
	require.Equal(t, item.Link, dup.Link)
	require.Equal(t, "feed", dup.Metadata["source"])
}

func TestRSSItemRepoFindDuplicateMatchesAnyProvidedIdentifier(t *testing.T) {
	provider := newRuntimeProvider(t)
	sub := domain.NewRSSSubscription("Tech", "https://example.com/feed.xml", "wechat-longform", "sspai")
	require.NoError(t, provider.RSSSubscriptionRepo().Create(t.Context(), sub))
	run := domain.NewRSSPullRun(sub.ID)
	require.NoError(t, provider.RSSPullRunRepo().Create(t.Context(), run))

	item := domain.NewRSSItemRecord(sub.ID, run.ID, "guid-1", "https://example.com/original", "hash-1", "A")
	require.NoError(t, provider.RSSItemRepo().Create(t.Context(), item))

	dup, err := provider.RSSItemRepo().FindDuplicate(t.Context(), domain.RSSDuplicateKey{
		SubscriptionID: sub.ID,
		GUID:           "guid-2",
		Link:           "https://example.com/original",
		ContentHash:    "different-hash",
	})
	require.NoError(t, err)
	require.NotNil(t, dup)
	require.Equal(t, item.ID, dup.ID)
}

func TestRSSItemRepoFindDuplicateMatchesByContentHashWhenGUIDAndLinkDiffer(t *testing.T) {
	provider := newRuntimeProvider(t)
	sub := domain.NewRSSSubscription("Tech", "https://example.com/feed.xml", "wechat-longform", "sspai")
	require.NoError(t, provider.RSSSubscriptionRepo().Create(t.Context(), sub))
	run := domain.NewRSSPullRun(sub.ID)
	require.NoError(t, provider.RSSPullRunRepo().Create(t.Context(), run))

	item := domain.NewRSSItemRecord(sub.ID, run.ID, "guid-1", "https://example.com/by-link", "hash-1", "A")
	require.NoError(t, provider.RSSItemRepo().Create(t.Context(), item))

	dup, err := provider.RSSItemRepo().FindDuplicate(t.Context(), domain.RSSDuplicateKey{
		SubscriptionID: sub.ID,
		GUID:           "different-guid",
		Link:           "https://example.com/different",
		ContentHash:    "hash-1",
	})
	require.NoError(t, err)
	require.NotNil(t, dup)
	require.Equal(t, item.ID, dup.ID)
}

func TestRSSItemRepoRoundTripsWorkspaceArticleIDAndRawPayload(t *testing.T) {
	provider := newRuntimeProvider(t)
	sub := domain.NewRSSSubscription("Tech", "https://example.com/feed.xml", "wechat-longform", "sspai")
	require.NoError(t, provider.RSSSubscriptionRepo().Create(t.Context(), sub))
	run := domain.NewRSSPullRun(sub.ID)
	require.NoError(t, provider.RSSPullRunRepo().Create(t.Context(), run))

	publishedAt := time.Now().UTC().Truncate(time.Second)
	importedAt := publishedAt.Add(time.Minute)
	item := domain.NewRSSItemRecord(sub.ID, run.ID, "guid-1", "https://example.com/a", "hash-1", "A")
	item.Status = domain.RSSItemStatusImported
	item.PublishedAt = &publishedAt
	item.ImportedAt = &importedAt
	item.WorkspaceArticleID = "workspace-123"
	item.RawPayloadJSON = []byte(`{"entry":{"id":"guid-1"},"title":"A"}`)
	item.Metadata = map[string]any{"source": "feed", "priority": float64(1)}

	require.NoError(t, provider.RSSItemRepo().Create(t.Context(), item))

	stored, err := provider.RSSItemRepo().GetByID(t.Context(), item.ID)
	require.NoError(t, err)
	require.Equal(t, item.WorkspaceArticleID, stored.WorkspaceArticleID)
	require.JSONEq(t, string(item.RawPayloadJSON), string(stored.RawPayloadJSON))
	require.Equal(t, "feed", stored.Metadata["source"])

	list, err := provider.RSSItemRepo().List(t.Context(), 10)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, item.WorkspaceArticleID, list[0].WorkspaceArticleID)
	require.JSONEq(t, string(item.RawPayloadJSON), string(list[0].RawPayloadJSON))
}

func TestRSSPullRunRepoCreateAndList(t *testing.T) {
	provider := newRuntimeProvider(t)
	sub := domain.NewRSSSubscription("Tech", "https://example.com/feed.xml", "wechat-longform", "sspai")
	require.NoError(t, provider.RSSSubscriptionRepo().Create(t.Context(), sub))

	older := domain.NewRSSPullRun(sub.ID)
	older.StartedAt = time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	newer := domain.NewRSSPullRun(sub.ID)
	newer.StartedAt = time.Now().UTC().Truncate(time.Second)
	newer.Status = domain.RSSPullRunStatusSucceeded
	completedAt := newer.StartedAt.Add(2 * time.Minute)
	newer.CompletedAt = &completedAt
	newer.Metadata = map[string]any{"count": float64(3)}

	require.NoError(t, provider.RSSPullRunRepo().Create(t.Context(), older))
	require.NoError(t, provider.RSSPullRunRepo().Create(t.Context(), newer))

	runs, err := provider.RSSPullRunRepo().List(t.Context(), 10)
	require.NoError(t, err)
	require.Len(t, runs, 2)
	require.Equal(t, newer.ID, runs[0].ID)
	require.Equal(t, older.ID, runs[1].ID)
	require.Equal(t, float64(3), runs[0].Metadata["count"])
	require.NotNil(t, runs[0].CompletedAt)
}
