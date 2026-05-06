package main

import (
	"content-hub/domain"
	"context"
	"sort"
	"time"
)

type stubSourceDocumentRepo struct {
	storedByID map[string]*domain.SourceDocument
}

func (r *stubSourceDocumentRepo) Create(_ context.Context, doc *domain.SourceDocument) error {
	if r.storedByID == nil {
		r.storedByID = map[string]*domain.SourceDocument{}
	}
	copyValue := *doc
	r.storedByID[doc.ID] = &copyValue
	return nil
}

func (r *stubSourceDocumentRepo) Update(_ context.Context, doc *domain.SourceDocument) error {
	if r.storedByID == nil {
		r.storedByID = map[string]*domain.SourceDocument{}
	}
	copyValue := *doc
	r.storedByID[doc.ID] = &copyValue
	return nil
}

func (r *stubSourceDocumentRepo) Delete(_ context.Context, id string) error {
	delete(r.storedByID, id)
	return nil
}

func (r *stubSourceDocumentRepo) GetByID(_ context.Context, id string) (*domain.SourceDocument, error) {
	doc, ok := r.storedByID[id]
	if !ok {
		return nil, domain.NewNotFoundErr("source_document", id)
	}
	copyValue := *doc
	return &copyValue, nil
}

func (r *stubSourceDocumentRepo) List(_ context.Context, limit int) ([]domain.SourceDocument, error) {
	out := make([]domain.SourceDocument, 0, len(r.storedByID))
	for _, doc := range r.storedByID {
		out = append(out, *doc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *stubSourceDocumentRepo) FindByHash(_ context.Context, hash string) (*domain.SourceDocument, error) {
	for _, doc := range r.storedByID {
		if doc.Hash == hash {
			copyValue := *doc
			return &copyValue, nil
		}
	}
	return nil, domain.NewNotFoundErr("source_document", hash)
}

func (r *stubSourceDocumentRepo) ClaimPending(context.Context, int, string, time.Time) ([]domain.SourceDocument, error) {
	return nil, nil
}

func (r *stubSourceDocumentRepo) ListByStatus(_ context.Context, status string, limit int) ([]domain.SourceDocument, error) {
	out := []domain.SourceDocument{}
	for _, doc := range r.storedByID {
		if doc.Status == status {
			out = append(out, *doc)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

type stubBusinessConfigRepo struct{}

func (r *stubBusinessConfigRepo) Upsert(context.Context, *domain.BusinessConfig) error { return nil }
func (r *stubBusinessConfigRepo) GetByCategoryAndKey(context.Context, string, string) (*domain.BusinessConfig, error) {
	return nil, domain.NewNotFoundErr("business_config", "settings")
}
func (r *stubBusinessConfigRepo) ListByCategory(context.Context, string) ([]domain.BusinessConfig, error) {
	return nil, nil
}

type stubSystemControlStateRepo struct{}

func (r *stubSystemControlStateRepo) Get(context.Context) (*domain.SystemControlState, error) {
	return nil, domain.NewNotFoundErr("system_control_state", "singleton")
}
func (r *stubSystemControlStateRepo) Upsert(context.Context, *domain.SystemControlState) error {
	return nil
}

type stubAuditLogRepo struct{}

func (r *stubAuditLogRepo) Create(context.Context, *domain.AuditLog) error { return nil }
func (r *stubAuditLogRepo) GetByID(context.Context, string) (*domain.AuditLog, error) {
	return nil, domain.NewNotFoundErr("audit_log", "missing")
}
func (r *stubAuditLogRepo) List(context.Context, int) ([]domain.AuditLog, error) { return nil, nil }
