package http

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"sort"
	"strings"
	"time"
)

type testWebControlRepos struct {
	SourceDocuments *stubSourceDocumentRepo
	Workspaces      repo.WorkspaceRepo
	AuditLogs       *stubAuditLogRepo
	Configs         *stubBusinessConfigRepo
	ControlStates   *stubSystemControlStateRepo
}

type stubSourceDocumentRepo struct {
	created    []*domain.SourceDocument
	updated    []*domain.SourceDocument
	storedByID map[string]*domain.SourceDocument
	createErr  error
	updateErr  error
	deleteErr  error
}

func (r *stubSourceDocumentRepo) Create(_ context.Context, doc *domain.SourceDocument) error {
	if r.createErr != nil {
		return r.createErr
	}
	copyValue := cloneSourceDocument(doc)
	r.created = append(r.created, copyValue)
	if r.storedByID == nil {
		r.storedByID = map[string]*domain.SourceDocument{}
	}
	r.storedByID[doc.ID] = copyValue
	return nil
}

func (r *stubSourceDocumentRepo) Update(_ context.Context, doc *domain.SourceDocument) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	copyValue := cloneSourceDocument(doc)
	r.updated = append(r.updated, copyValue)
	if r.storedByID == nil {
		r.storedByID = map[string]*domain.SourceDocument{}
	}
	r.storedByID[doc.ID] = copyValue
	return nil
}

func (r *stubSourceDocumentRepo) UpdateIfStatus(_ context.Context, doc *domain.SourceDocument, expectedStatuses ...string) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	stored, ok := r.storedByID[doc.ID]
	if !ok {
		return domain.NewNotFoundErr("source_document", doc.ID)
	}
	for _, status := range expectedStatuses {
		if stored.Status == status {
			copyValue := cloneSourceDocument(doc)
			r.updated = append(r.updated, copyValue)
			r.storedByID[doc.ID] = copyValue
			return nil
		}
	}
	return domain.NewConflictErr("source document state changed")
}

func (r *stubSourceDocumentRepo) GetByID(_ context.Context, id string) (*domain.SourceDocument, error) {
	if doc, ok := r.storedByID[id]; ok {
		return cloneSourceDocument(doc), nil
	}
	return nil, domain.NewNotFoundErr("source_document", id)
}

func (r *stubSourceDocumentRepo) Delete(_ context.Context, id string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	if _, ok := r.storedByID[id]; !ok {
		return domain.NewNotFoundErr("source_document", id)
	}
	delete(r.storedByID, id)
	return nil
}

func (r *stubSourceDocumentRepo) DeleteIfStatus(_ context.Context, id string, expectedStatuses ...string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	stored, ok := r.storedByID[id]
	if !ok {
		return domain.NewNotFoundErr("source_document", id)
	}
	for _, status := range expectedStatuses {
		if stored.Status == status {
			delete(r.storedByID, id)
			return nil
		}
	}
	return domain.NewConflictErr("source document state changed")
}

func (r *stubSourceDocumentRepo) List(_ context.Context, limit int) ([]domain.SourceDocument, error) {
	items := make([]domain.SourceDocument, 0, len(r.storedByID))
	for _, doc := range r.storedByID {
		items = append(items, *cloneSourceDocument(doc))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *stubSourceDocumentRepo) FindByHash(_ context.Context, hash string) (*domain.SourceDocument, error) {
	for _, doc := range r.storedByID {
		if doc.Hash == hash {
			return cloneSourceDocument(doc), nil
		}
	}
	return nil, domain.NewNotFoundErr("source_document", hash)
}

func (r *stubSourceDocumentRepo) ClaimPending(context.Context, int, string, time.Time) ([]domain.SourceDocument, error) {
	return nil, nil
}

func (r *stubSourceDocumentRepo) ListByStatus(_ context.Context, status string, limit int) ([]domain.SourceDocument, error) {
	items := []domain.SourceDocument{}
	for _, doc := range r.storedByID {
		if doc.Status == status {
			items = append(items, *cloneSourceDocument(doc))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func cloneSourceDocument(doc *domain.SourceDocument) *domain.SourceDocument {
	if doc == nil {
		return nil
	}
	copyValue := *doc
	if doc.Metadata != nil {
		copyValue.Metadata = map[string]any{}
		for k, v := range doc.Metadata {
			copyValue.Metadata[k] = v
		}
	}
	if doc.ImportedAt != nil {
		copied := *doc.ImportedAt
		copyValue.ImportedAt = &copied
	}
	if doc.ClaimedAt != nil {
		copied := *doc.ClaimedAt
		copyValue.ClaimedAt = &copied
	}
	if doc.ProcessingStartedAt != nil {
		copied := *doc.ProcessingStartedAt
		copyValue.ProcessingStartedAt = &copied
	}
	if doc.CompletedAt != nil {
		copied := *doc.CompletedAt
		copyValue.CompletedAt = &copied
	}
	return &copyValue
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
