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
	created               []*domain.RSSItemRecord
	updated               []*domain.RSSItemRecord
	duplicates            map[string]*domain.RSSItemRecord
	duplicateChecks       int
	lastDuplicateKey      domain.RSSDuplicateKey
	findDuplicateErr      error
	findDuplicateErrCalls map[int]error
	createErr             error
	updateErrCalls        map[int]error
	updateCalls           int
	err                   error
}

func (r *stubRSSItemRepo) Create(_ context.Context, item *domain.RSSItemRecord) error {
	if r.createErr != nil {
		return r.createErr
	}
	if r.err != nil {
		return r.err
	}
	copyValue := *item
	r.created = append(r.created, &copyValue)
	return nil
}

func (r *stubRSSItemRepo) Update(_ context.Context, item *domain.RSSItemRecord) error {
	r.updateCalls++
	if err, ok := r.updateErrCalls[r.updateCalls]; ok {
		return err
	}
	if r.err != nil {
		return r.err
	}
	copyValue := *item
	r.updated = append(r.updated, &copyValue)
	return nil
}

func (r *stubRSSItemRepo) FindDuplicate(_ context.Context, key domain.RSSDuplicateKey) (*domain.RSSItemRecord, error) {
	r.duplicateChecks++
	r.lastDuplicateKey = key
	if err, ok := r.findDuplicateErrCalls[r.duplicateChecks]; ok {
		return nil, err
	}
	if r.findDuplicateErr != nil {
		return nil, r.findDuplicateErr
	}
	if r.err != nil {
		return nil, r.err
	}
	if r.duplicates == nil {
		return nil, nil
	}
	if item, ok := r.duplicates[key.GUID]; ok {
		copyValue := *item
		return &copyValue, nil
	}
	if item, ok := r.duplicates[key.Link]; ok {
		copyValue := *item
		return &copyValue, nil
	}
	if item, ok := r.duplicates[key.ContentHash]; ok {
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
	called      bool
	callCount   int
	articles    []domain.IntakeArticle
	workspaceID string
	reusedIDs   []string
	returnOnErr bool
	errAtCall   int
	err         error
}

func (i *stubRSSArticleIntake) Intake(_ context.Context, article domain.IntakeArticle) (*domain.ArticleWorkspaceRecord, error) {
	i.called = true
	i.callCount++
	i.articles = append(i.articles, article)
	if i.err != nil && (i.errAtCall == 0 || i.errAtCall == i.callCount) {
		if i.returnOnErr {
			return &domain.ArticleWorkspaceRecord{ID: i.workspaceID}, i.err
		}
		return nil, i.err
	}
	return &domain.ArticleWorkspaceRecord{ID: i.workspaceID}, nil
}

func (i *stubRSSArticleIntake) IntakeIntoWorkspace(_ context.Context, workspaceArticleID string, article domain.IntakeArticle) (*domain.ArticleWorkspaceRecord, error) {
	i.called = true
	i.callCount++
	i.articles = append(i.articles, article)
	i.reusedIDs = append(i.reusedIDs, workspaceArticleID)
	if i.err != nil && (i.errAtCall == 0 || i.errAtCall == i.callCount) {
		if i.returnOnErr {
			return &domain.ArticleWorkspaceRecord{ID: workspaceArticleID}, i.err
		}
		return nil, i.err
	}
	return &domain.ArticleWorkspaceRecord{ID: workspaceArticleID}, nil
}

func TestRSSPullServiceImportsNewItemsAndSkipsDuplicates(t *testing.T) {
	feeds := &stubRSSFeedFetcher{body: []byte(`<?xml version="1.0"?><rss><channel><item><guid>guid-1</guid><title>Title</title><link>https://example.com/a</link><description>Body</description></item></channel></rss>`)}
	itemRepo := &stubRSSItemRepo{}
	runs := &stubRSSPullRunRepo{}
	intake := &stubRSSArticleIntake{workspaceID: "workspace-1"}
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
	require.Equal(t, "workspace-1", itemRepo.updated[0].WorkspaceArticleID)
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
	require.Equal(t, 4, itemRepo.duplicateChecks)
	require.Len(t, itemRepo.created, 2)
	require.Equal(t, domain.RSSItemStatusSkippedDuplicate, itemRepo.created[1].Status)
	require.Equal(t, domain.RSSPullRunStatusSucceeded, result.Run.Status)
}

func TestRSSPullServiceSkipsItemWhenDuplicateMatchesByLinkEvenWithNewGUID(t *testing.T) {
	feeds := &stubRSSFeedFetcher{body: []byte(`<?xml version="1.0"?><rss><channel><item><guid>guid-2</guid><title>Title</title><link>https://example.com/a</link><description>Body</description></item></channel></rss>`)}
	itemRepo := &stubRSSItemRepo{duplicates: map[string]*domain.RSSItemRecord{"https://example.com/a": {ID: "existing-1"}}}
	runs := &stubRSSPullRunRepo{}
	intake := &stubRSSArticleIntake{workspaceID: "workspace-1"}
	svc := NewRSSPullService(feeds, runs, itemRepo, intake)
	sub := domain.NewRSSSubscription("Tech", "https://example.com/feed.xml", "wechat-longform", "sspai")

	result, err := svc.RunOnce(t.Context(), *sub)

	require.NoError(t, err)
	require.Equal(t, 1, result.FetchedItems)
	require.Equal(t, 0, result.ImportedItems)
	require.Equal(t, 1, result.SkippedItems)
	require.False(t, intake.called)
	require.Equal(t, "guid-2", itemRepo.lastDuplicateKey.GUID)
	require.Equal(t, "https://example.com/a", itemRepo.lastDuplicateKey.Link)
	require.Len(t, itemRepo.created, 1)
	require.Equal(t, domain.RSSItemStatusSkippedDuplicate, itemRepo.created[0].Status)
	require.Equal(t, "existing-1", itemRepo.created[0].Metadata["duplicate_item_id"])
}

func TestRSSPullServiceFailsRunAndMarksItemFailedWhenIntakeFails(t *testing.T) {
	feeds := &stubRSSFeedFetcher{body: []byte(`<?xml version="1.0"?><rss><channel><item><guid>guid-1</guid><title>Title</title><link>https://example.com/a</link><description>Body</description></item></channel></rss>`)}
	itemRepo := &stubRSSItemRepo{}
	runs := &stubRSSPullRunRepo{}
	intake := &stubRSSArticleIntake{workspaceID: "workspace-1", returnOnErr: true, err: errors.New("rewrite failed")}
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
	require.Equal(t, "workspace-1", itemRepo.updated[0].WorkspaceArticleID)
	require.Len(t, runs.updated, 1)
	require.Equal(t, float64(1), result.Run.Metadata["fetched_items"])
	require.Equal(t, float64(0), result.Run.Metadata["imported_items"])
	require.Equal(t, float64(0), result.Run.Metadata["skipped_items"])
}

func TestRSSPullServiceRetriesPreviouslyFailedMatchingItem(t *testing.T) {
	feeds := &stubRSSFeedFetcher{body: []byte(`<?xml version="1.0"?><rss><channel><item><guid>guid-1</guid><title>Title</title><link>https://example.com/a</link><description>Body</description></item></channel></rss>`)}
	itemRepo := &stubRSSItemRepo{duplicates: map[string]*domain.RSSItemRecord{
		"guid-1": {ID: "existing-failed", Status: domain.RSSItemStatusFailed, WorkspaceArticleID: "workspace-old"},
	}}
	runs := &stubRSSPullRunRepo{}
	intake := &stubRSSArticleIntake{workspaceID: "workspace-2"}
	svc := NewRSSPullService(feeds, runs, itemRepo, intake)
	sub := domain.NewRSSSubscription("Tech", "https://example.com/feed.xml", "wechat-longform", "sspai")

	result, err := svc.RunOnce(t.Context(), *sub)

	require.NoError(t, err)
	require.Equal(t, 1, result.FetchedItems)
	require.Equal(t, 1, result.ImportedItems)
	require.Equal(t, 0, result.SkippedItems)
	require.True(t, intake.called)
	require.Len(t, itemRepo.created, 0)
	require.Len(t, itemRepo.updated, 2)
	require.Equal(t, "existing-failed", itemRepo.updated[0].ID)
	require.Equal(t, domain.RSSItemStatusPending, itemRepo.updated[0].Status)
	require.Equal(t, "existing-failed", itemRepo.updated[1].ID)
	require.Equal(t, domain.RSSItemStatusImported, itemRepo.updated[1].Status)
	require.Equal(t, "workspace-old", itemRepo.updated[1].WorkspaceArticleID)
	require.Equal(t, []string{"workspace-old"}, intake.reusedIDs)
	require.Nil(t, itemRepo.updated[0].Metadata["duplicate_item_id"])
}

func TestRSSPullServiceIgnoresInvalidRSSDate(t *testing.T) {
	feeds := &stubRSSFeedFetcher{body: []byte(`<?xml version="1.0"?><rss><channel><item><guid>guid-1</guid><title>Title</title><link>https://example.com/a</link><description>Body</description><pubDate>not-a-date</pubDate></item></channel></rss>`)}
	itemRepo := &stubRSSItemRepo{}
	runs := &stubRSSPullRunRepo{}
	intake := &stubRSSArticleIntake{workspaceID: "workspace-1"}
	svc := NewRSSPullService(feeds, runs, itemRepo, intake)
	sub := domain.NewRSSSubscription("Tech", "https://example.com/feed.xml", "wechat-longform", "sspai")

	result, err := svc.RunOnce(t.Context(), *sub)

	require.NoError(t, err)
	require.Equal(t, 1, result.ImportedItems)
	require.Len(t, intake.articles, 1)
	require.Nil(t, intake.articles[0].PublishedAt)
	require.Len(t, itemRepo.created, 1)
	require.Nil(t, itemRepo.created[0].PublishedAt)
	require.Len(t, itemRepo.updated, 1)
	require.Nil(t, itemRepo.updated[0].PublishedAt)
}

func TestRSSPullServiceContinuesAfterItemFailureAndSucceedsWhenLaterItemImports(t *testing.T) {
	feeds := &stubRSSFeedFetcher{body: []byte(`<?xml version="1.0"?><rss><channel><item><guid>guid-1</guid><title></title><link>https://example.com/a</link><description>Body</description></item><item><guid>guid-2</guid><title>Second</title><link>https://example.com/b</link><description>Body</description></item></channel></rss>`)}
	itemRepo := &stubRSSItemRepo{}
	runs := &stubRSSPullRunRepo{}
	intake := &stubRSSArticleIntake{workspaceID: "workspace-2"}
	svc := NewRSSPullService(feeds, runs, itemRepo, intake)
	sub := domain.NewRSSSubscription("Tech", "https://example.com/feed.xml", "wechat-longform", "sspai")

	result, err := svc.RunOnce(t.Context(), *sub)

	require.NoError(t, err)
	require.Equal(t, 2, result.FetchedItems)
	require.Equal(t, 1, result.ImportedItems)
	require.Equal(t, 0, result.SkippedItems)
	require.Equal(t, 1, result.FailedItems)
	require.Equal(t, domain.RSSPullRunStatusSucceeded, result.Run.Status)
	require.Len(t, intake.articles, 1)
	require.Equal(t, "guid-2", intake.articles[0].ExternalID)
	require.Len(t, itemRepo.created, 2)
	require.Equal(t, domain.RSSItemStatusFailed, itemRepo.created[0].Status)
	require.Equal(t, domain.RSSItemStatusPending, itemRepo.created[1].Status)
	require.Len(t, itemRepo.updated, 1)
	require.Equal(t, domain.RSSItemStatusImported, itemRepo.updated[0].Status)
	require.Equal(t, float64(1), result.Run.Metadata["failed_items"])
	require.Contains(t, result.Run.ErrorSummary, "normalize rss item")
}

func TestRSSPullServiceFailsRunWhenAllItemsFail(t *testing.T) {
	feeds := &stubRSSFeedFetcher{body: []byte(`<?xml version="1.0"?><rss><channel><item><guid>guid-1</guid><title>First</title><link>https://example.com/a</link><description>Body</description></item><item><guid>guid-2</guid><title>Second</title><link>https://example.com/b</link><description>Body</description></item></channel></rss>`)}
	itemRepo := &stubRSSItemRepo{}
	runs := &stubRSSPullRunRepo{}
	intake := &stubRSSArticleIntake{workspaceID: "workspace-1", returnOnErr: true, err: errors.New("rewrite failed")}
	svc := NewRSSPullService(feeds, runs, itemRepo, intake)
	sub := domain.NewRSSSubscription("Tech", "https://example.com/feed.xml", "wechat-longform", "sspai")

	result, err := svc.RunOnce(t.Context(), *sub)

	require.Error(t, err)
	require.ErrorContains(t, err, "rewrite failed")
	require.NotNil(t, result)
	require.Equal(t, 2, result.FetchedItems)
	require.Equal(t, 0, result.ImportedItems)
	require.Equal(t, 0, result.SkippedItems)
	require.Equal(t, 2, result.FailedItems)
	require.Equal(t, domain.RSSPullRunStatusFailed, result.Run.Status)
	require.Len(t, itemRepo.created, 2)
	require.Len(t, itemRepo.updated, 2)
	require.Equal(t, float64(2), result.Run.Metadata["failed_items"])
	require.Contains(t, result.Run.ErrorSummary, "guid-1")
	require.Contains(t, result.Run.ErrorSummary, "guid-2")
}

func TestRSSPullServiceFailsRunWhenImportedStateUpdateFailsAfterIntake(t *testing.T) {
	feeds := &stubRSSFeedFetcher{body: []byte(`<?xml version="1.0"?><rss><channel><item><guid>guid-1</guid><title>Title</title><link>https://example.com/a</link><description>Body</description></item><item><guid>guid-2</guid><title>Second</title><link>https://example.com/b</link><description>Body</description></item></channel></rss>`)}
	itemRepo := &stubRSSItemRepo{updateErrCalls: map[int]error{1: errors.New("update failed")}}
	runs := &stubRSSPullRunRepo{}
	intake := &stubRSSArticleIntake{workspaceID: "workspace-1"}
	svc := NewRSSPullService(feeds, runs, itemRepo, intake)
	sub := domain.NewRSSSubscription("Tech", "https://example.com/feed.xml", "wechat-longform", "sspai")

	result, err := svc.RunOnce(t.Context(), *sub)

	require.Error(t, err)
	require.ErrorContains(t, err, "mark rss item imported")
	require.NotNil(t, result)
	require.Equal(t, domain.RSSPullRunStatusFailed, result.Run.Status)
	require.Equal(t, 2, result.FetchedItems)
	require.Equal(t, 0, result.ImportedItems)
	require.Equal(t, 0, result.SkippedItems)
	require.Equal(t, 0, result.FailedItems)
	require.Len(t, intake.articles, 1)
	require.Equal(t, float64(2), runs.updated[len(runs.updated)-1].Metadata["fetched_items"])
}

func TestRSSPullServiceMarksDivergenceAndRetriesItOnRerun(t *testing.T) {
	feeds := &stubRSSFeedFetcher{body: []byte(`<?xml version="1.0"?><rss><channel><item><guid>guid-1</guid><title>Title</title><link>https://example.com/a</link><description>Body</description></item></channel></rss>`)}
	itemRepo := &stubRSSItemRepo{updateErrCalls: map[int]error{1: errors.New("update failed")}}
	runs := &stubRSSPullRunRepo{}
	intake := &stubRSSArticleIntake{workspaceID: "workspace-1"}
	svc := NewRSSPullService(feeds, runs, itemRepo, intake)
	sub := domain.NewRSSSubscription("Tech", "https://example.com/feed.xml", "wechat-longform", "sspai")

	result, err := svc.RunOnce(t.Context(), *sub)

	require.Error(t, err)
	require.ErrorContains(t, err, "mark rss item imported")
	require.Len(t, itemRepo.created, 1)
	require.Len(t, itemRepo.updated, 1)
	require.Equal(t, domain.RSSItemStatusImportDiverged, itemRepo.updated[0].Status)
	diverged := itemRepo.updated[0]
	require.Equal(t, "workspace-1", diverged.WorkspaceArticleID)
	require.Contains(t, diverged.Metadata["error"], "update failed")
	require.Equal(t, domain.RSSPullRunStatusFailed, result.Run.Status)

	itemRepo.duplicates = map[string]*domain.RSSItemRecord{"guid-1": diverged}
	itemRepo.updateErrCalls = nil
	itemRepo.updateCalls = 0
	intake.called = false

	result, err = svc.RunOnce(t.Context(), *sub)

	require.NoError(t, err)
	require.True(t, intake.called)
	require.Equal(t, []string{"workspace-1"}, intake.reusedIDs)
	require.Equal(t, 1, result.ImportedItems)
	require.Equal(t, 0, result.SkippedItems)
	require.Equal(t, domain.RSSPullRunStatusSucceeded, result.Run.Status)
	require.Len(t, itemRepo.updated, 3)
	require.Equal(t, domain.RSSItemStatusPending, itemRepo.updated[1].Status)
	require.Equal(t, domain.RSSItemStatusImported, itemRepo.updated[2].Status)
	require.Equal(t, diverged.ID, itemRepo.updated[1].ID)
	require.Equal(t, diverged.ID, itemRepo.updated[2].ID)
	require.Equal(t, "workspace-1", itemRepo.updated[1].WorkspaceArticleID)
	require.Equal(t, "workspace-1", itemRepo.updated[2].WorkspaceArticleID)
}

func TestRSSPullServicePreservesEarlyLookupErrorWhenLaterFailureOccurs(t *testing.T) {
	feeds := &stubRSSFeedFetcher{body: []byte(`<?xml version="1.0"?><rss><channel><item><guid>guid-1</guid><title>Title</title><link>https://example.com/a</link><description>Body</description></item></channel></rss>`)}
	itemRepo := &stubRSSItemRepo{findDuplicateErrCalls: map[int]error{1: errors.New("duplicate lookup failed")}}
	runs := &stubRSSPullRunRepo{}
	intake := &stubRSSArticleIntake{workspaceID: "workspace-1", returnOnErr: true, err: errors.New("rewrite failed")}
	svc := NewRSSPullService(feeds, runs, itemRepo, intake)
	sub := domain.NewRSSSubscription("Tech", "https://example.com/feed.xml", "wechat-longform", "sspai")

	result, err := svc.RunOnce(t.Context(), *sub)

	require.Error(t, err)
	require.ErrorContains(t, err, "duplicate lookup failed")
	require.ErrorContains(t, err, "rewrite failed")
	require.Contains(t, result.Run.ErrorSummary, "failed-row lookup")
	require.Contains(t, result.Run.ErrorSummary, "rewrite failed")
	require.Len(t, itemRepo.updated, 1)
	require.Contains(t, itemRepo.updated[0].Metadata["error"], "failed-row lookup")
}

func TestRSSPullServicePersistsEarlyLookupWarningOnSuccessfulRetryableImport(t *testing.T) {
	feeds := &stubRSSFeedFetcher{body: []byte(`<?xml version="1.0"?><rss><channel><item><guid>guid-1</guid><title>Title</title><link>https://example.com/a</link><description>Body</description></item></channel></rss>`)}
	existing := domain.NewRSSItemRecord("sub-1", "run-1", "guid-1", "https://example.com/a", "hash-1", "Existing")
	existing.Status = domain.RSSItemStatusFailed
	existing.WorkspaceArticleID = "workspace-old"
	itemRepo := &stubRSSItemRepo{duplicates: map[string]*domain.RSSItemRecord{"guid-1": existing}, findDuplicateErrCalls: map[int]error{1: errors.New("duplicate lookup warning")}}
	runs := &stubRSSPullRunRepo{}
	intake := &stubRSSArticleIntake{workspaceID: "workspace-old"}
	svc := NewRSSPullService(feeds, runs, itemRepo, intake)
	sub := domain.NewRSSSubscription("Tech", "https://example.com/feed.xml", "wechat-longform", "sspai")
	sub.ID = "sub-1"

	result, err := svc.RunOnce(t.Context(), *sub)

	require.NoError(t, err)
	require.Equal(t, 1, result.ImportedItems)
	require.Len(t, itemRepo.updated, 2)
	require.Equal(t, "workspace-old", itemRepo.updated[0].WorkspaceArticleID)
	require.Equal(t, "workspace-old", itemRepo.updated[1].WorkspaceArticleID)
	require.Contains(t, itemRepo.updated[1].Metadata["warnings"], "failed-row lookup: duplicate lookup warning")
}

func TestRSSPullServicePersistsEarlyLookupWarningOnDuplicateSkip(t *testing.T) {
	feeds := &stubRSSFeedFetcher{body: []byte(`<?xml version="1.0"?><rss><channel><item><guid>guid-2</guid><title>Title</title><link>https://example.com/a</link><description>Body</description></item></channel></rss>`)}
	itemRepo := &stubRSSItemRepo{duplicates: map[string]*domain.RSSItemRecord{"https://example.com/a": {ID: "existing-1", Status: domain.RSSItemStatusImported}}, findDuplicateErrCalls: map[int]error{1: errors.New("duplicate lookup warning")}}
	runs := &stubRSSPullRunRepo{}
	intake := &stubRSSArticleIntake{workspaceID: "workspace-1"}
	svc := NewRSSPullService(feeds, runs, itemRepo, intake)
	sub := domain.NewRSSSubscription("Tech", "https://example.com/feed.xml", "wechat-longform", "sspai")

	result, err := svc.RunOnce(t.Context(), *sub)

	require.NoError(t, err)
	require.Equal(t, 1, result.SkippedItems)
	require.Len(t, itemRepo.created, 1)
	require.Contains(t, itemRepo.created[0].Metadata["warnings"], "failed-row lookup: duplicate lookup warning")
}

func TestRSSPullServiceReusesFailedRowForRepeatedEarlyNormalizationFailure(t *testing.T) {
	feeds := &stubRSSFeedFetcher{body: []byte(`<?xml version="1.0"?><rss><channel><item><guid>guid-1</guid><title></title><link>https://example.com/a</link><description>Body</description></item></channel></rss>`)}
	itemRepo := &stubRSSItemRepo{}
	runs := &stubRSSPullRunRepo{}
	intake := &stubRSSArticleIntake{workspaceID: "workspace-1"}
	svc := NewRSSPullService(feeds, runs, itemRepo, intake)
	sub := domain.NewRSSSubscription("Tech", "https://example.com/feed.xml", "wechat-longform", "sspai")

	result, err := svc.RunOnce(t.Context(), *sub)

	require.Error(t, err)
	require.Equal(t, 1, result.FailedItems)
	require.Len(t, itemRepo.created, 1)
	require.Len(t, itemRepo.updated, 0)
	firstFailed := itemRepo.created[0]
	itemRepo.duplicates = map[string]*domain.RSSItemRecord{"guid-1": firstFailed}

	result, err = svc.RunOnce(t.Context(), *sub)

	require.Error(t, err)
	require.Equal(t, 1, result.FailedItems)
	require.Len(t, itemRepo.created, 1)
	require.Len(t, itemRepo.updated, 1)
	require.Equal(t, firstFailed.ID, itemRepo.updated[0].ID)
	require.Equal(t, domain.RSSItemStatusFailed, itemRepo.updated[0].Status)
}

func TestRSSPullServiceDoesNotOverwriteImportedRowForEarlyNormalizationFailure(t *testing.T) {
	feeds := &stubRSSFeedFetcher{body: []byte(`<?xml version="1.0"?><rss><channel><item><guid>guid-1</guid><title></title><link>https://example.com/a</link><description>Body</description></item></channel></rss>`)}
	existing := domain.NewRSSItemRecord("sub-1", "run-1", "guid-1", "https://example.com/a", "hash-1", "Existing")
	existing.Status = domain.RSSItemStatusImported
	itemRepo := &stubRSSItemRepo{duplicates: map[string]*domain.RSSItemRecord{"guid-1": existing}}
	runs := &stubRSSPullRunRepo{}
	intake := &stubRSSArticleIntake{workspaceID: "workspace-1"}
	svc := NewRSSPullService(feeds, runs, itemRepo, intake)
	sub := domain.NewRSSSubscription("Tech", "https://example.com/feed.xml", "wechat-longform", "sspai")
	sub.ID = "sub-1"

	result, err := svc.RunOnce(t.Context(), *sub)

	require.Error(t, err)
	require.Equal(t, 1, result.FailedItems)
	require.Len(t, itemRepo.created, 1)
	require.Len(t, itemRepo.updated, 0)
	require.NotEqual(t, existing.ID, itemRepo.created[0].ID)
	require.Equal(t, domain.RSSItemStatusFailed, itemRepo.created[0].Status)
}

func TestRSSPullServiceReportsEarlyFailedRowLookupErrorExplicitly(t *testing.T) {
	feeds := &stubRSSFeedFetcher{body: []byte(`<?xml version="1.0"?><rss><channel><item><guid>guid-1</guid><title></title><link>https://example.com/a</link><description>Body</description></item></channel></rss>`)}
	itemRepo := &stubRSSItemRepo{findDuplicateErr: errors.New("duplicate lookup failed")}
	runs := &stubRSSPullRunRepo{}
	intake := &stubRSSArticleIntake{workspaceID: "workspace-1"}
	svc := NewRSSPullService(feeds, runs, itemRepo, intake)
	sub := domain.NewRSSSubscription("Tech", "https://example.com/feed.xml", "wechat-longform", "sspai")

	result, err := svc.RunOnce(t.Context(), *sub)

	require.Error(t, err)
	require.ErrorContains(t, err, "failed-row lookup")
	require.ErrorContains(t, err, "duplicate lookup failed")
	require.Equal(t, 1, result.FailedItems)
	require.Len(t, itemRepo.created, 1)
	require.Len(t, itemRepo.updated, 0)
	require.Equal(t, domain.RSSItemStatusFailed, itemRepo.created[0].Status)
	require.Contains(t, result.Run.ErrorSummary, "failed-row lookup")
}
