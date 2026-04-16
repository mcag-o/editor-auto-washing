package service

import (
	"content-hub/domain"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubRSSFeedFetcher struct {
	body []byte
	err  error
	url  string
}

func (f *stubRSSFeedFetcher) Fetch(ctx context.Context, feedURL string) ([]byte, error) {
	f.url = feedURL
	if f.err != nil {
		return nil, f.err
	}
	return append([]byte(nil), f.body...), nil
}

type stubRSSPullRunRepo struct {
	created []*domain.RSSPullRun
	updated []*domain.RSSPullRun
	err     error
}

func (r *stubRSSPullRunRepo) Create(_ context.Context, run *domain.RSSPullRun) error {
	if r.err != nil {
		return r.err
	}
	copyValue := *run
	r.created = append(r.created, &copyValue)
	return nil
}

func (r *stubRSSPullRunRepo) Update(_ context.Context, run *domain.RSSPullRun) error {
	if r.err != nil {
		return r.err
	}
	copyValue := *run
	r.updated = append(r.updated, &copyValue)
	return nil
}

func (r *stubRSSPullRunRepo) GetByID(context.Context, string) (*domain.RSSPullRun, error) {
	return nil, domain.NewNotFoundErr("rss pull run", "missing")
}

func (r *stubRSSPullRunRepo) List(context.Context, int) ([]domain.RSSPullRun, error) {
	return nil, nil
}

type stubRSSItemRepo struct {
	created          []*domain.RSSItemRecord
	updated          []*domain.RSSItemRecord
	duplicates       map[string]*domain.RSSItemRecord
	duplicateChecks  int
	lastDuplicateKey domain.RSSDuplicateKey
	err              error
}

func (r *stubRSSItemRepo) Create(_ context.Context, item *domain.RSSItemRecord) error {
	if r.err != nil {
		return r.err
	}
	copyValue := *item
	r.created = append(r.created, &copyValue)
	return nil
}

func (r *stubRSSItemRepo) Update(_ context.Context, item *domain.RSSItemRecord) error {
	if r.err != nil {
		return r.err
	}
	copyValue := *item
	r.updated = append(r.updated, &copyValue)
	return nil
}

func (r *stubRSSItemRepo) FindDuplicate(_ context.Context, key domain.RSSDuplicateKey) (*domain.RSSItemRecord, error) {
	if r.err != nil {
		return nil, r.err
	}
	r.duplicateChecks++
	r.lastDuplicateKey = key
	if r.duplicates == nil {
		return nil, nil
	}
	if item, ok := r.duplicates[key.GUID]; ok {
		copyValue := *item
		return &copyValue, nil
	}
	return nil, nil
}

func (r *stubRSSItemRepo) GetByID(context.Context, string) (*domain.RSSItemRecord, error) {
	return nil, domain.NewNotFoundErr("rss item", "missing")
}

func (r *stubRSSItemRepo) List(context.Context, int) ([]domain.RSSItemRecord, error) {
	return nil, nil
}

type stubRSSArticleIntake struct {
	called   bool
	articles []domain.IntakeArticle
	err      error
}

func (i *stubRSSArticleIntake) Intake(_ context.Context, article domain.IntakeArticle) error {
	i.called = true
	i.articles = append(i.articles, article)
	return i.err
}

func TestRSSPullServiceImportsNewItemsAndSkipsDuplicates(t *testing.T) {
	feeds := &stubRSSFeedFetcher{body: []byte(`<?xml version="1.0"?><rss><channel><item><guid>guid-1</guid><title>Title</title><link>https://example.com/a</link><description>Body</description></item></channel></rss>`)}
	itemRepo := &stubRSSItemRepo{}
	runs := &stubRSSPullRunRepo{}
	intake := &stubRSSArticleIntake{}
	svc := NewRSSPullService(feeds, runs, itemRepo, intake)
	sub := domain.NewRSSSubscription("Tech", "https://example.com/feed.xml", "wechat-longform", "sspai")

	result, err := svc.RunOnce(t.Context(), *sub)

	require.NoError(t, err)
	require.Equal(t, 1, result.FetchedItems)
	require.Equal(t, 1, result.ImportedItems)
	require.Equal(t, 0, result.SkippedItems)
	require.Equal(t, domain.RSSPullRunStatusSucceeded, result.Run.Status)
	require.True(t, intake.called)
	require.Len(t, itemRepo.created, 1)
	require.Equal(t, domain.RSSItemStatusPending, itemRepo.created[0].Status)
	require.Len(t, itemRepo.updated, 1)
	require.Equal(t, domain.RSSItemStatusImported, itemRepo.updated[0].Status)
	require.Equal(t, "guid-1", itemRepo.lastDuplicateKey.GUID)
	require.Equal(t, float64(1), result.Run.Metadata["fetched_items"])
	require.Equal(t, float64(1), result.Run.Metadata["imported_items"])
	require.Equal(t, float64(0), result.Run.Metadata["skipped_items"])
	require.Len(t, runs.created, 1)
	require.Len(t, runs.updated, 1)

	itemRepo.duplicates = map[string]*domain.RSSItemRecord{"guid-1": itemRepo.created[0]}
	intake.called = false

	result, err = svc.RunOnce(t.Context(), *sub)

	require.NoError(t, err)
	require.Equal(t, 1, result.FetchedItems)
	require.Equal(t, 0, result.ImportedItems)
	require.Equal(t, 1, result.SkippedItems)
	require.False(t, intake.called)
	require.Equal(t, 2, itemRepo.duplicateChecks)
	require.Len(t, itemRepo.created, 2)
	require.Equal(t, domain.RSSItemStatusSkippedDuplicate, itemRepo.created[1].Status)
	require.Equal(t, domain.RSSPullRunStatusSucceeded, result.Run.Status)
}

func TestRSSPullServiceFailsRunAndMarksItemFailedWhenIntakeFails(t *testing.T) {
	feeds := &stubRSSFeedFetcher{body: []byte(`<?xml version="1.0"?><rss><channel><item><guid>guid-1</guid><title>Title</title><link>https://example.com/a</link><description>Body</description></item></channel></rss>`)}
	itemRepo := &stubRSSItemRepo{}
	runs := &stubRSSPullRunRepo{}
	intake := &stubRSSArticleIntake{err: errors.New("rewrite failed")}
	svc := NewRSSPullService(feeds, runs, itemRepo, intake)
	sub := domain.NewRSSSubscription("Tech", "https://example.com/feed.xml", "wechat-longform", "sspai")

	result, err := svc.RunOnce(t.Context(), *sub)

	require.Error(t, err)
	require.ErrorContains(t, err, "intake rss item")
	require.NotNil(t, result)
	require.Equal(t, 1, result.FetchedItems)
	require.Equal(t, 0, result.ImportedItems)
	require.Equal(t, 0, result.SkippedItems)
	require.Equal(t, domain.RSSPullRunStatusFailed, result.Run.Status)
	require.NotEmpty(t, result.Run.ErrorSummary)
	require.Len(t, itemRepo.created, 1)
	require.Len(t, itemRepo.updated, 1)
	require.Equal(t, domain.RSSItemStatusFailed, itemRepo.updated[0].Status)
	require.Len(t, runs.updated, 1)
	require.Equal(t, float64(1), result.Run.Metadata["fetched_items"])
	require.Equal(t, float64(0), result.Run.Metadata["imported_items"])
	require.Equal(t, float64(0), result.Run.Metadata["skipped_items"])
}
