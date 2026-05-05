package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
)

type AuditLogCreateInput struct {
	Actor      string
	Action     string
	Resource   string
	ResourceID string
	Result     string
	Message    string
	Metadata   map[string]any
}

type AuditLogService struct {
	repo repo.AuditLogRepo
}

func NewAuditLogService(r repo.AuditLogRepo) *AuditLogService {
	return &AuditLogService{repo: r}
}

func (s *AuditLogService) Create(ctx context.Context, input AuditLogCreateInput) (*domain.AuditLog, error) {
	log := domain.NewAuditLog(input.Actor, input.Action)
	log.Resource = input.Resource
	log.ResourceID = input.ResourceID
	log.Result = input.Result
	log.Message = input.Message
	if input.Metadata != nil {
		log.Metadata = input.Metadata
	}
	if err := s.repo.Create(ctx, log); err != nil {
		return nil, err
	}
	return log, nil
}

func (s *AuditLogService) List(ctx context.Context, limit int) ([]domain.AuditLog, error) {
	return s.repo.List(ctx, limit)
}
