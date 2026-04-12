package sqlite

import (
	"content-hub/domain"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRuntimeProvider(t *testing.T) *Provider {
	t.Helper()
	dbPath := fmt.Sprintf("%s/test_runtime_%d.db", t.TempDir(), os.Getpid())
	provider, err := NewProvider(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	return provider
}

func TestSQLiteJobEventRepoListByJobOrdersEventsDeterministically(t *testing.T) {
	provider := newRuntimeProvider(t)
	job := domain.NewJobRun("ordering")
	require.NoError(t, provider.JobRepo().Create(context.Background(), job))
	base := time.Now().UTC().Truncate(time.Second)
	started := domain.NewJobEvent(job.ID, "started", "job started")
	started.CreatedAt = base
	completed := domain.NewJobEvent(job.ID, "completed", "job completed")
	completed.CreatedAt = base
	if started.ID > completed.ID {
		started.ID, completed.ID = completed.ID, started.ID
	}

	require.NoError(t, provider.JobEventRepo().Add(context.Background(), started))
	require.NoError(t, provider.JobEventRepo().Add(context.Background(), completed))

	events, err := provider.JobEventRepo().ListByJob(context.Background(), job.ID)

	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, started.ID, events[0].ID)
	assert.Equal(t, completed.ID, events[1].ID)
	assert.Equal(t, "started", events[0].Status)
	assert.Equal(t, "completed", events[1].Status)
}

func TestSQLiteJobEventRepoListByJobIgnoresTimestampLexicographicQuirks(t *testing.T) {
	provider := newRuntimeProvider(t)
	job := domain.NewJobRun("timestamp-ordering")
	require.NoError(t, provider.JobRepo().Create(context.Background(), job))
	first := domain.NewJobEvent(job.ID, "started", "first")
	first.CreatedAt = time.Date(2026, 4, 10, 2, 0, 0, 900, time.UTC)
	second := domain.NewJobEvent(job.ID, "completed", "second")
	second.CreatedAt = time.Date(2026, 4, 10, 2, 0, 0, 100, time.UTC)

	require.NoError(t, provider.JobEventRepo().Add(context.Background(), first))
	require.NoError(t, provider.JobEventRepo().Add(context.Background(), second))

	events, err := provider.JobEventRepo().ListByJob(context.Background(), job.ID)

	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, first.ID, events[0].ID)
	assert.Equal(t, second.ID, events[1].ID)
}
