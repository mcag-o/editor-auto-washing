package sqlite

import (
	"content-hub/domain"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCollectorProvider(t *testing.T) *Provider {
	t.Helper()
	dbPath := fmt.Sprintf("%s/test_collector_%d.db", t.TempDir(), os.Getpid())
	provider, err := NewProvider(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	return provider
}

func TestCollectorSourceRepo_CreateAndListEnabled(t *testing.T) {
	provider := newCollectorProvider(t)
	repo := provider.CollectorSourceRepo()

	enabled := domain.NewCollectorSource("baidu", "百度热搜")
	disabled := domain.NewCollectorSource("github", "GitHub Trending")
	disabled.Enabled = false

	require.NoError(t, repo.Create(t.Context(), enabled))
	require.NoError(t, repo.Create(t.Context(), disabled))

	items, err := repo.ListEnabled(t.Context())

	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "baidu", items[0].ID)
	assert.Equal(t, "百度热搜", items[0].DisplayName)
	assert.True(t, items[0].Enabled)
}

func TestCollectorSourceRepo_GetByIDReturnsNotFound(t *testing.T) {
	provider := newCollectorProvider(t)

	_, err := provider.CollectorSourceRepo().GetByID(t.Context(), "missing")

	require.Error(t, err)
	assert.ErrorContains(t, err, "collector_source")
}

func TestCollectorRunRepo_PersistsRunAndSourceRuns(t *testing.T) {
	provider := newCollectorProvider(t)
	sources := provider.CollectorSourceRepo()
	runs := provider.CollectorRunRepo()
	require.NoError(t, sources.Create(t.Context(), domain.NewCollectorSource("baidu", "百度热搜")))

	run := domain.NewCollectorRun("scheduled")
	require.NoError(t, runs.Create(t.Context(), run))

	hotlistRun := domain.NewCollectorSourceRun(run.ID, "baidu", domain.CollectorStageHotlist)
	detailRun := domain.NewCollectorSourceRun(run.ID, "baidu", domain.CollectorStageDetail)
	detailRun.Status = domain.CollectorSourceRunSucceeded

	require.NoError(t, runs.CreateSourceRun(t.Context(), hotlistRun))
	require.NoError(t, runs.CreateSourceRun(t.Context(), detailRun))

	got, err := runs.GetByID(t.Context(), run.ID)
	require.NoError(t, err)
	assert.Equal(t, run.ID, got.ID)
	assert.Equal(t, domain.CollectorRunPending, got.Status)

	sourceRuns, err := runs.ListSourceRuns(t.Context(), run.ID)
	require.NoError(t, err)
	require.Len(t, sourceRuns, 2)
	byStage := make(map[string]domain.CollectorSourceRun, len(sourceRuns))
	for _, item := range sourceRuns {
		byStage[item.Stage] = item
	}
	hotlistStored, ok := byStage[domain.CollectorStageHotlist]
	require.True(t, ok)
	assert.Equal(t, domain.CollectorSourceRunPending, hotlistStored.Status)
	detailStored, ok := byStage[domain.CollectorStageDetail]
	require.True(t, ok)
	assert.Equal(t, domain.CollectorSourceRunSucceeded, detailStored.Status)
}

func TestCollectorEntryAndArticleRepos_PersistNormalizedCollectorData(t *testing.T) {
	provider := newCollectorProvider(t)
	sources := provider.CollectorSourceRepo()
	runs := provider.CollectorRunRepo()
	entries := provider.CollectorEntryRepo()
	articles := provider.CollectorArticleRepo()
	require.NoError(t, sources.Create(t.Context(), domain.NewCollectorSource("baidu", "百度热搜")))

	run := domain.NewCollectorRun("manual")
	require.NoError(t, runs.Create(t.Context(), run))

	publishedAt := time.Date(2026, 4, 10, 8, 30, 0, 0, time.UTC)
	entry := domain.NewCollectorEntry(run.ID, "baidu", "entry-1", "示例热榜", "https://example.com/hot")
	entry.Status = domain.CollectorEntryFetchedDetail
	entry.PublishedAt = &publishedAt
	entry.RawJSON = []byte(`{"raw":true}`)
	entry.NormalizedJSON = []byte(`{"title":"示例热榜"}`)

	require.NoError(t, entries.Create(t.Context(), entry))

	article := domain.NewCollectorArticle(entry.ID, run.ID, "baidu", "article-1", "示例文章", "https://example.com/article")
	article.RawJSON = []byte(`{"html":"<p>body</p>"}`)
	article.NormalizedJSON = []byte(`{"title":"示例文章","body":"正文"}`)
	article.BridgeStatus = domain.CollectorArticleBridgePending

	require.NoError(t, articles.Create(t.Context(), article))

	storedEntries, err := entries.ListByRunID(t.Context(), run.ID)
	require.NoError(t, err)
	require.Len(t, storedEntries, 1)
	assert.Equal(t, entry.ID, storedEntries[0].ID)
	assert.Equal(t, domain.CollectorEntryFetchedDetail, storedEntries[0].Status)
	assert.JSONEq(t, string(entry.NormalizedJSON), string(storedEntries[0].NormalizedJSON))

	storedArticle, err := articles.GetByID(t.Context(), article.ID)
	require.NoError(t, err)
	assert.Equal(t, article.EntryID, storedArticle.EntryID)
	assert.Equal(t, domain.CollectorArticleBridgePending, storedArticle.BridgeStatus)
	assert.JSONEq(t, string(article.RawJSON), string(storedArticle.RawJSON))
	assert.JSONEq(t, string(article.NormalizedJSON), string(storedArticle.NormalizedJSON))
}

func TestCollectorEntryAndArticleRepos_NullableFieldsRoundTrip(t *testing.T) {
	provider := newCollectorProvider(t)
	sources := provider.CollectorSourceRepo()
	runs := provider.CollectorRunRepo()
	entries := provider.CollectorEntryRepo()
	articles := provider.CollectorArticleRepo()
	require.NoError(t, sources.Create(t.Context(), domain.NewCollectorSource("baidu", "百度热搜")))

	run := domain.NewCollectorRun("manual")
	require.NoError(t, runs.Create(t.Context(), run))

	entry := domain.NewCollectorEntry(run.ID, "baidu", "entry-nullable", "Nullable Entry", "https://example.com/nullable")
	require.NoError(t, entries.Create(t.Context(), entry))

	storedEntries, err := entries.ListByRunID(t.Context(), run.ID)
	require.NoError(t, err)
	require.Len(t, storedEntries, 1)
	assert.Nil(t, storedEntries[0].PublishedAt)
	assert.Nil(t, storedEntries[0].Rank)

	article := domain.NewCollectorArticle(entry.ID, run.ID, "baidu", "article-nullable", "Nullable Article", "https://example.com/article-nullable")
	require.NoError(t, articles.Create(t.Context(), article))

	storedArticle, err := articles.GetByID(t.Context(), article.ID)
	require.NoError(t, err)
	assert.Empty(t, storedArticle.WorkspaceID)
	assert.Nil(t, storedArticle.PublishedAt)
}

func TestCollectorAttemptAndSchedulerRepos_PersistOperationalState(t *testing.T) {
	provider := newCollectorProvider(t)
	sources := provider.CollectorSourceRepo()
	runs := provider.CollectorRunRepo()
	attempts := provider.CollectorAttemptRepo()
	scheduler := provider.CollectorSchedulerStateRepo()
	require.NoError(t, sources.Create(t.Context(), domain.NewCollectorSource("baidu", "百度热搜")))

	run := domain.NewCollectorRun("scheduled")
	require.NoError(t, runs.Create(t.Context(), run))
	sourceRun := domain.NewCollectorSourceRun(run.ID, "baidu", domain.CollectorStageDetail)
	require.NoError(t, runs.CreateSourceRun(t.Context(), sourceRun))
	entry := domain.NewCollectorEntry(run.ID, "baidu", "entry-1", "Entry", "https://example.com/entry-1")
	require.NoError(t, provider.CollectorEntryRepo().Create(t.Context(), entry))

	attempt := domain.NewCollectorAttempt(run.ID, sourceRun.ID, entry.ID, domain.CollectorStageDetail)
	attempt.Status = domain.CollectorAttemptFailed
	attempt.ErrorCode = domain.CollectorErrorParseFailed
	attempt.ErrorMessage = "detail payload changed"
	attempt.ResponseStatusCode = 502
	attempt.RawJSON = []byte(`{"response":"bad gateway"}`)

	require.NoError(t, attempts.Create(t.Context(), attempt))

	items, err := attempts.ListBySourceRunID(t.Context(), sourceRun.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, domain.CollectorAttemptFailed, items[0].Status)
	assert.Equal(t, domain.CollectorErrorParseFailed, items[0].ErrorCode)

	state := domain.NewCollectorSchedulerState(domain.DefaultCollectorSchedulerName)
	state.Status = domain.CollectorSchedulerRunning
	nextRunAt := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	state.NextRunAt = &nextRunAt
	state.LastRunID = run.ID

	require.NoError(t, scheduler.Upsert(t.Context(), state))

	stored, err := scheduler.GetByName(t.Context(), domain.DefaultCollectorSchedulerName)
	require.NoError(t, err)
	assert.Equal(t, domain.CollectorSchedulerRunning, stored.Status)
	assert.Equal(t, run.ID, stored.LastRunID)
	if assert.NotNil(t, stored.NextRunAt) {
		assert.True(t, stored.NextRunAt.Equal(nextRunAt))
	}
}

func TestCollectorAttemptRepo_CreateFailsOnMissingEntryForeignKey(t *testing.T) {
	provider := newCollectorProvider(t)
	sources := provider.CollectorSourceRepo()
	runs := provider.CollectorRunRepo()
	attempts := provider.CollectorAttemptRepo()
	require.NoError(t, sources.Create(t.Context(), domain.NewCollectorSource("baidu", "百度热搜")))

	run := domain.NewCollectorRun("scheduled")
	require.NoError(t, runs.Create(t.Context(), run))
	sourceRun := domain.NewCollectorSourceRun(run.ID, "baidu", domain.CollectorStageDetail)
	require.NoError(t, runs.CreateSourceRun(t.Context(), sourceRun))

	attempt := domain.NewCollectorAttempt(run.ID, sourceRun.ID, "missing-entry", domain.CollectorStageDetail)

	err := attempts.Create(t.Context(), attempt)

	require.Error(t, err)
	assert.ErrorContains(t, err, "FOREIGN KEY")
}

func TestCollectorSchedulerStateRepo_UpsertOverwritesExistingState(t *testing.T) {
	provider := newCollectorProvider(t)
	repo := provider.CollectorSchedulerStateRepo()

	initial := domain.NewCollectorSchedulerState(domain.DefaultCollectorSchedulerName)
	initial.Status = domain.CollectorSchedulerRunning
	initial.LastRunID = "run-1"
	require.NoError(t, repo.Upsert(t.Context(), initial))

	updated := domain.NewCollectorSchedulerState(domain.DefaultCollectorSchedulerName)
	updated.Status = domain.CollectorSchedulerFailed
	updated.LastRunID = "run-2"
	updated.ErrorMessage = "scheduler halted"
	require.NoError(t, repo.Upsert(t.Context(), updated))

	stored, err := repo.GetByName(t.Context(), domain.DefaultCollectorSchedulerName)

	require.NoError(t, err)
	assert.Equal(t, domain.CollectorSchedulerFailed, stored.Status)
	assert.Equal(t, "run-2", stored.LastRunID)
	assert.Equal(t, "scheduler halted", stored.ErrorMessage)
}

func TestCollectorSchedulerStateRepo_GetByNameReturnsNotFound(t *testing.T) {
	provider := newCollectorProvider(t)

	_, err := provider.CollectorSchedulerStateRepo().GetByName(t.Context(), "missing")

	require.Error(t, err)
	assert.ErrorContains(t, err, "collector_scheduler_state")
}
