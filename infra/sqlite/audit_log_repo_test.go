package sqlite

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newTestAuditLogRepo(t *testing.T) *auditLogRepo {
	t.Helper()
	dbPath := fmt.Sprintf("%s/test_%d.db", t.TempDir(), os.Getpid())
	p, err := NewProvider(dbPath)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	t.Cleanup(func() {
		p.Close()
	})
	return p.AuditLogRepo().(*auditLogRepo)
}

func createTestAuditLog(t *testing.T, r *auditLogRepo, ctx context.Context, id, actor, action, resource, resourceID string, createdAt time.Time) {
	t.Helper()
	log := &domain.AuditLog{
		ID:         id,
		Actor:      actor,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Result:     "success",
		Message:    "test audit log",
		Metadata:   map[string]any{"workflow_run_id": resourceID},
		CreatedAt:  createdAt,
	}
	if err := r.Create(ctx, log); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestAuditLogRepoListByQueryReturnsNewestFirstWithoutTruncationLoss(t *testing.T) {
	r := newTestAuditLogRepo(t)
	ctx := context.Background()
	base := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 120; i++ {
		createTestAuditLog(
			t,
			r,
			ctx,
			fmt.Sprintf("audit-%03d", i),
			"local-admin",
			"web_control.workflow_run.resume",
			"workflow_run",
			"run-1",
			base.Add(time.Duration(i)*time.Second),
		)
	}

	logs, err := r.ListByQuery(ctx, repo.AuditLogQuery{WorkflowRunID: "run-1", ActionPrefix: "web_control.workflow_run"})
	require.NoError(t, err)
	require.Len(t, logs, 120)
	require.Equal(t, "audit-119", logs[0].ID)
	require.Equal(t, "audit-000", logs[len(logs)-1].ID)
}

func TestAuditLogRepoListByQueryFiltersByExactResourceID(t *testing.T) {
	r := newTestAuditLogRepo(t)
	ctx := context.Background()
	base := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)

	createTestAuditLog(t, r, ctx, "audit-1", "local-admin", "web_control.workflow_run.resume", "workflow_run", "run-1", base)
	createTestAuditLog(t, r, ctx, "audit-2", "local-admin", "web_control.workflow_run.resume", "workflow_run", "run-10", base.Add(time.Second))
	createTestAuditLog(t, r, ctx, "audit-3", "local-admin", "web_control.workflow_run.resume", "workflow_run", "run-1", base.Add(2*time.Second))

	logs, err := r.ListByQuery(ctx, repo.AuditLogQuery{Resource: "workflow_run", ResourceID: "run-1"})
	require.NoError(t, err)
	require.Len(t, logs, 2)
	require.Equal(t, "audit-3", logs[0].ID)
	require.Equal(t, "audit-1", logs[1].ID)
}
