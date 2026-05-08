package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubAuditLogRepo struct {
	logs []*domain.AuditLog
}

func (r *stubAuditLogRepo) Create(_ context.Context, log *domain.AuditLog) error {
	if err := log.Validate(); err != nil {
		return err
	}
	copyLog := *log
	r.logs = append(r.logs, &copyLog)
	return nil
}

func (r *stubAuditLogRepo) GetByID(_ context.Context, id string) (*domain.AuditLog, error) {
	for _, log := range r.logs {
		if log.ID == id {
			copyLog := *log
			return &copyLog, nil
		}
	}
	return nil, domain.NewNotFoundErr("audit_log", id)
}

func (r *stubAuditLogRepo) List(_ context.Context, limit int) ([]domain.AuditLog, error) {
	if limit <= 0 || limit > len(r.logs) {
		limit = len(r.logs)
	}
	out := make([]domain.AuditLog, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, *r.logs[i])
	}
	return out, nil
}

func (r *stubAuditLogRepo) ListByQuery(_ context.Context, query repo.AuditLogQuery) ([]domain.AuditLog, error) {
	out := make([]domain.AuditLog, 0, len(r.logs))
	for _, log := range r.logs {
		if query.Resource != "" && log.Resource != query.Resource {
			continue
		}
		if query.WorkflowRunID != "" {
			workflowRunID, _ := log.Metadata["workflow_run_id"].(string)
			if workflowRunID != query.WorkflowRunID {
				continue
			}
		}
		if query.ActionPrefix != "" && !strings.HasPrefix(log.Action, query.ActionPrefix) {
			continue
		}
		if query.ResourceID != "" && log.ResourceID != query.ResourceID {
			continue
		}
		out = append(out, *log)
	}
	return out, nil
}

func TestAuditLogServiceCreateAndList(t *testing.T) {
	repo := &stubAuditLogRepo{}
	svc := NewAuditLogService(repo)

	first, err := svc.Create(t.Context(), AuditLogCreateInput{
		Actor:      "local-admin",
		Action:     "upload_article",
		Resource:   "article",
		ResourceID: "article-1",
		Result:     "success",
		Message:    "uploaded source article",
	})
	require.NoError(t, err)

	_, err = svc.Create(t.Context(), AuditLogCreateInput{
		Actor:   "local-admin",
		Action:  "pause_system",
		Result:  "success",
		Message: "paused processing",
	})
	require.NoError(t, err)

	logs, err := svc.List(t.Context(), 10)
	require.NoError(t, err)
	require.Len(t, logs, 2)
	require.Equal(t, first.ID, logs[0].ID)
	require.Equal(t, "pause_system", logs[1].Action)
}
