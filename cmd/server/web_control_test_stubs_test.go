package main

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
)

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
func (r *stubAuditLogRepo) ListByQuery(context.Context, repo.AuditLogQuery) ([]domain.AuditLog, error) {
	return nil, nil
}
