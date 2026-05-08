package service

import (
	"content-hub/domain"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAuditLogServiceListByQueryReturnsWorkflowRunMatchesNewestFirst(t *testing.T) {
	repos, cleanup, err := BuildRuntimeRepos(t.TempDir())
	if cleanup != nil {
		defer func() {
			require.NoError(t, cleanup())
		}()
	}
	require.NoError(t, err)

	svc := NewAuditLogService(repos.AuditLogRepo)
	base := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 120; i++ {
		log := &domain.AuditLog{
			ID:         fmt.Sprintf("audit-%03d", i),
			Actor:      "local-admin",
			Action:     "web_control.workflow_run.resume",
			Resource:   "workflow_run",
			ResourceID: "run-1",
			Result:     "success",
			Message:    "workflow resumed",
			Metadata:   map[string]any{"workflow_run_id": "run-1"},
			CreatedAt:  base.Add(time.Duration(i) * time.Second),
		}
		require.NoError(t, repos.AuditLogRepo.Create(t.Context(), log))
	}

	logs, err := svc.ListByQuery(t.Context(), AuditLogQuery{
		Resource:      "workflow_run",
		WorkflowRunID: "run-1",
		ActionPrefix:  "web_control.workflow_run",
	})

	require.NoError(t, err)
	require.Len(t, logs, 120)
	require.Equal(t, "audit-119", logs[0].ID)
	require.Equal(t, "audit-000", logs[len(logs)-1].ID)
	for _, log := range logs {
		require.Equal(t, "workflow_run", log.Resource)
		require.Equal(t, "run-1", log.ResourceID)
		require.Contains(t, log.Action, "web_control.workflow_run")
	}
}
