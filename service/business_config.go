package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"encoding/json"
)

type BusinessConfigValue struct {
	ID        string
	Category  string
	Key       string
	ValueJSON []byte
	UpdatedBy string
}

type BusinessConfigService struct {
	repo repo.BusinessConfigRepo
}

func NewBusinessConfigService(r repo.BusinessConfigRepo) *BusinessConfigService {
	return &BusinessConfigService{repo: r}
}

func (s *BusinessConfigService) SetJSON(ctx context.Context, category, key string, valueJSON []byte, updatedBy string) error {
	if !json.Valid(valueJSON) {
		return domain.NewValidationErr("config value must be valid json", nil)
	}
	cfg := domain.NewBusinessConfig(category, key, string(valueJSON), updatedBy)
	return s.repo.Upsert(ctx, cfg)
}

func (s *BusinessConfigService) Get(ctx context.Context, category, key string) (*BusinessConfigValue, error) {
	cfg, err := s.repo.GetByCategoryAndKey(ctx, category, key)
	if err != nil {
		return nil, err
	}
	return &BusinessConfigValue{
		ID:        cfg.ID,
		Category:  cfg.Category,
		Key:       cfg.Key,
		ValueJSON: []byte(cfg.Value),
		UpdatedBy: cfg.UpdatedBy,
	}, nil
}

func (s *BusinessConfigService) ListByCategory(ctx context.Context, category string) ([]BusinessConfigValue, error) {
	configs, err := s.repo.ListByCategory(ctx, category)
	if err != nil {
		return nil, err
	}
	values := make([]BusinessConfigValue, 0, len(configs))
	for _, cfg := range configs {
		values = append(values, BusinessConfigValue{
			ID:        cfg.ID,
			Category:  cfg.Category,
			Key:       cfg.Key,
			ValueJSON: []byte(cfg.Value),
			UpdatedBy: cfg.UpdatedBy,
		})
	}
	return values, nil
}
