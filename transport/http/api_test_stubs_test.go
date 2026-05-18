package http

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"strings"
)

type testWebControlRepos struct {
	Workspaces      repo.WorkspaceRepo
	AuditLogs       *stubAuditLogRepo
	Configs         *stubBusinessConfigRepo
	ControlStates   *stubSystemControlStateRepo
}

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

type stubSystemControlStateRepo struct {
	state *domain.SystemControlState
}

func (r *stubSystemControlStateRepo) Get(context.Context) (*domain.SystemControlState, error) {
	if r.state == nil {
		return nil, domain.NewNotFoundErr("system_control_state", "singleton")
	}
	copyState := *r.state
	if r.state.Metadata != nil {
		copyState.Metadata = map[string]any{}
		for k, v := range r.state.Metadata {
			copyState.Metadata[k] = v
		}
	}
	return &copyState, nil
}

func (r *stubSystemControlStateRepo) Upsert(_ context.Context, state *domain.SystemControlState) error {
	if err := state.Validate(); err != nil {
		return err
	}
	copyState := *state
	if state.Metadata != nil {
		copyState.Metadata = map[string]any{}
		for k, v := range state.Metadata {
			copyState.Metadata[k] = v
		}
	}
	r.state = &copyState
	return nil
}

type stubBusinessConfigRepo struct {
	stored map[string]*domain.BusinessConfig
}

func (r *stubBusinessConfigRepo) Upsert(_ context.Context, cfg *domain.BusinessConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if r.stored == nil {
		r.stored = map[string]*domain.BusinessConfig{}
	}
	copyCfg := *cfg
	r.stored[cfg.Category+"/"+cfg.Key] = &copyCfg
	return nil
}

func (r *stubBusinessConfigRepo) GetByCategoryAndKey(_ context.Context, category, key string) (*domain.BusinessConfig, error) {
	cfg, ok := r.stored[category+"/"+key]
	if !ok {
		return nil, domain.NewNotFoundErr("business_config", category+"/"+key)
	}
	copyCfg := *cfg
	return &copyCfg, nil
}

func (r *stubBusinessConfigRepo) ListByCategory(_ context.Context, category string) ([]domain.BusinessConfig, error) {
	out := []domain.BusinessConfig{}
	for _, cfg := range r.stored {
		if cfg.Category == category {
			out = append(out, *cfg)
		}
	}
	return out, nil
}
