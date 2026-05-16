package sqlite

import (
	"content-hub/domain"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

func TestNewProviderDropsRetiredCollectorTablesDuringMigration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "upgrade.db")
	legacyDB, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = legacyDB.Close() })

	legacyTables := []string{
		"collector_sources",
		"collector_runs",
		"collector_source_runs",
		"collector_entries",
		"collector_articles",
		"collector_attempts",
		"collector_scheduler_state",
	}
	for _, table := range legacyTables {
		_, err := legacyDB.Exec(`CREATE TABLE ` + table + ` (id TEXT PRIMARY KEY)`)
		require.NoError(t, err)
	}
	require.NoError(t, legacyDB.Close())

	provider, err := NewProvider(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })

	for _, table := range legacyTables {
		row := provider.DB().QueryRowContext(t.Context(), `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table)
		var count int
		require.NoError(t, row.Scan(&count))
		require.Zero(t, count, table)
	}
}

func TestProviderExposesFolderIntakeRepos(t *testing.T) {
	provider := newRuntimeProvider(t)

	require.NotNil(t, provider.SourceDocumentRepo())
	require.NotNil(t, provider.ImportRunRepo())
}

func TestSourceDocumentRepoCreateAndFindByHash(t *testing.T) {
	provider := newRuntimeProvider(t)
	doc := domain.NewSourceDocument("article.md", "/inbox/article.md", "md", "Title", "Body", "hash-1")
	doc.Status = domain.SourceDocumentStatusPending

	require.NoError(t, provider.SourceDocumentRepo().Create(t.Context(), doc))

	stored, err := provider.SourceDocumentRepo().FindByHash(t.Context(), "hash-1")
	require.NoError(t, err)
	require.Equal(t, doc.ID, stored.ID)
	require.Equal(t, doc.OriginalFilename, stored.OriginalFilename)
	require.Equal(t, doc.OriginalPath, stored.OriginalPath)
	require.Equal(t, doc.FileType, stored.FileType)
	require.Equal(t, doc.Title, stored.Title)
	require.Equal(t, doc.Body, stored.Body)
	require.Equal(t, doc.Status, stored.Status)
	require.Equal(t, doc.Metadata, stored.Metadata)
}

func TestSourceDocumentRepoClaimPending(t *testing.T) {
	provider := newRuntimeProvider(t)
	pending := domain.NewSourceDocument("article.md", "/inbox/article.md", "md", "Title", "Body", "hash-1")
	pending.Status = domain.SourceDocumentStatusPending
	other := domain.NewSourceDocument("other.md", "/inbox/other.md", "md", "Other", "Body", "hash-2")
	other.Status = domain.SourceDocumentStatusCompleted
	completedAt := time.Now().UTC().Truncate(time.Second)
	other.CompletedAt = &completedAt

	require.NoError(t, provider.SourceDocumentRepo().Create(t.Context(), pending))
	require.NoError(t, provider.SourceDocumentRepo().Create(t.Context(), other))

	claimedAt := time.Now().UTC().Truncate(time.Second)
	claimed, err := provider.SourceDocumentRepo().ClaimPending(t.Context(), 1, "worker-1", claimedAt)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, pending.ID, claimed[0].ID)
	require.Equal(t, domain.SourceDocumentStatusClaimed, claimed[0].Status)
	require.Equal(t, "worker-1", claimed[0].ClaimedBy)
	require.NotNil(t, claimed[0].ClaimedAt)
	require.True(t, claimed[0].ClaimedAt.Equal(claimedAt))

	stored, err := provider.SourceDocumentRepo().GetByID(t.Context(), pending.ID)
	require.NoError(t, err)
	require.Equal(t, domain.SourceDocumentStatusClaimed, stored.Status)
	require.Equal(t, "worker-1", stored.ClaimedBy)
	require.NotNil(t, stored.ClaimedAt)
	require.True(t, stored.ClaimedAt.Equal(claimedAt))

	remaining, err := provider.SourceDocumentRepo().ListByStatus(t.Context(), domain.SourceDocumentStatusPending, 10)
	require.NoError(t, err)
	require.Empty(t, remaining)
	completed, err := provider.SourceDocumentRepo().ListByStatus(t.Context(), domain.SourceDocumentStatusCompleted, 10)
	require.NoError(t, err)
	require.Len(t, completed, 1)
	require.Equal(t, other.ID, completed[0].ID)
}

func TestSourceDocumentRepoClaimPendingSkipsRowsNotActuallyClaimed(t *testing.T) {
	provider := newRuntimeProvider(t)
	doc := domain.NewSourceDocument("article.md", "/inbox/article.md", "md", "Title", "Body", "hash-race")
	doc.Status = domain.SourceDocumentStatusPending
	require.NoError(t, provider.SourceDocumentRepo().Create(t.Context(), doc))

	// Simulate a race where the guarded claim UPDATE matches the candidate id,
	// but SQLite ignores the write before it can be applied.
	_, err := provider.DB().ExecContext(t.Context(), `
		CREATE TRIGGER source_documents_ignore_claim_update
		BEFORE UPDATE ON source_documents
		FOR EACH ROW
		WHEN OLD.id = '`+doc.ID+`' AND OLD.status = 'pending' AND NEW.status = 'claimed'
		BEGIN
			SELECT RAISE(IGNORE);
		END;
	`)
	require.NoError(t, err)

	claimedAt := time.Now().UTC().Truncate(time.Second)
	claimed, err := provider.SourceDocumentRepo().ClaimPending(t.Context(), 1, "worker-1", claimedAt)
	require.NoError(t, err)
	require.Empty(t, claimed)

	stored, err := provider.SourceDocumentRepo().GetByID(t.Context(), doc.ID)
	require.NoError(t, err)
	require.Equal(t, domain.SourceDocumentStatusPending, stored.Status)
	require.Empty(t, stored.ClaimedBy)
	require.Nil(t, stored.ClaimedAt)
}

func TestSourceDocumentRepoListReturnsDocumentsAcrossStatuses(t *testing.T) {
	provider := newRuntimeProvider(t)
	pending := domain.NewSourceDocument("pending.md", "/inbox/pending.md", "md", "Pending", "Body A", "hash-list-a")
	pending.Status = domain.SourceDocumentStatusPending
	completed := domain.NewSourceDocument("completed.md", "/inbox/completed.md", "md", "Completed", "Body B", "hash-list-b")
	completed.Status = domain.SourceDocumentStatusCompleted
	completedAt := time.Now().UTC().Truncate(time.Second)
	completed.CompletedAt = &completedAt

	require.NoError(t, provider.SourceDocumentRepo().Create(t.Context(), pending))
	require.NoError(t, provider.SourceDocumentRepo().Create(t.Context(), completed))

	docs, err := provider.SourceDocumentRepo().List(t.Context(), 10)
	require.NoError(t, err)
	require.Len(t, docs, 2)
	ids := []string{docs[0].ID, docs[1].ID}
	require.Contains(t, ids, pending.ID)
	require.Contains(t, ids, completed.ID)
}

func TestSourceDocumentRepoDelete(t *testing.T) {
	provider := newRuntimeProvider(t)
	doc := domain.NewSourceDocument("article.md", "/inbox/article.md", "md", "Title", "Body", "hash-delete")
	doc.Status = domain.SourceDocumentStatusPending
	require.NoError(t, provider.SourceDocumentRepo().Create(t.Context(), doc))

	require.NoError(t, provider.SourceDocumentRepo().Delete(t.Context(), doc.ID))

	_, err := provider.SourceDocumentRepo().GetByID(t.Context(), doc.ID)
	require.Error(t, err)
	appErr, ok := err.(*domain.AppError)
	require.True(t, ok)
	require.Equal(t, domain.ErrNotFound, appErr.Code)
}

func TestSourceDocumentRepoUpdateIfStatusRejectsStaleState(t *testing.T) {
	provider := newRuntimeProvider(t)
	doc := domain.NewSourceDocument("article.md", "/inbox/article.md", "md", "Title", "Body", "hash-update-if-status")
	doc.Status = domain.SourceDocumentStatusPending
	require.NoError(t, provider.SourceDocumentRepo().Create(t.Context(), doc))

	startedAt := time.Now().UTC().Truncate(time.Second)
	doc.Status = domain.SourceDocumentStatusProcessing
	doc.ProcessingStartedAt = &startedAt
	err := provider.SourceDocumentRepo().UpdateIfStatus(t.Context(), doc, domain.SourceDocumentStatusProcessing)
	require.Error(t, err)
	appErr, ok := err.(*domain.AppError)
	require.True(t, ok)
	require.Equal(t, domain.ErrConflict, appErr.Code)

	stored, getErr := provider.SourceDocumentRepo().GetByID(t.Context(), doc.ID)
	require.NoError(t, getErr)
	require.Equal(t, domain.SourceDocumentStatusPending, stored.Status)
}

func TestSourceDocumentRepoDeleteIfStatusRejectsStaleState(t *testing.T) {
	provider := newRuntimeProvider(t)
	doc := domain.NewSourceDocument("article.md", "/inbox/article.md", "md", "Title", "Body", "hash-delete-if-status")
	doc.Status = domain.SourceDocumentStatusPending
	require.NoError(t, provider.SourceDocumentRepo().Create(t.Context(), doc))

	err := provider.SourceDocumentRepo().DeleteIfStatus(t.Context(), doc.ID, domain.SourceDocumentStatusPaused)
	require.Error(t, err)
	appErr, ok := err.(*domain.AppError)
	require.True(t, ok)
	require.Equal(t, domain.ErrConflict, appErr.Code)

	stored, getErr := provider.SourceDocumentRepo().GetByID(t.Context(), doc.ID)
	require.NoError(t, getErr)
	require.Equal(t, doc.ID, stored.ID)
}

func TestImportRunRepoCreateUpdateAndList(t *testing.T) {
	provider := newRuntimeProvider(t)
	run := domain.NewImportRun("folder")
	run.Metadata = map[string]any{"batch": "nightly"}

	require.NoError(t, provider.ImportRunRepo().Create(t.Context(), run))

	stored, err := provider.ImportRunRepo().GetByID(t.Context(), run.ID)
	require.NoError(t, err)
	require.Equal(t, run.ID, stored.ID)
	require.Equal(t, run.SourceType, stored.SourceType)
	require.Equal(t, run.Status, stored.Status)
	require.Equal(t, "nightly", stored.Metadata["batch"])

	completedAt := run.StartedAt.Add(2 * time.Minute)
	run.Status = domain.ImportRunStatusCompleted
	run.ImportedCount = 3
	run.FailedCount = 1
	run.CompletedAt = &completedAt
	run.Metadata["result"] = "ok"
	require.NoError(t, provider.ImportRunRepo().Update(t.Context(), run))

	updated, err := provider.ImportRunRepo().GetByID(t.Context(), run.ID)
	require.NoError(t, err)
	require.Equal(t, domain.ImportRunStatusCompleted, updated.Status)
	require.Equal(t, 3, updated.ImportedCount)
	require.Equal(t, 1, updated.FailedCount)
	require.NotNil(t, updated.CompletedAt)
	require.True(t, updated.CompletedAt.Equal(completedAt))
	require.Equal(t, "ok", updated.Metadata["result"])

	runs, err := provider.ImportRunRepo().List(t.Context(), 10)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	require.Equal(t, run.ID, runs[0].ID)
}
