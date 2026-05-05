package service

import (
	"content-hub/domain"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

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
	var out []domain.BusinessConfig
	for _, cfg := range r.stored {
		if cfg.Category == category {
			out = append(out, *cfg)
		}
	}
	return out, nil
}

func TestBusinessConfigServiceSetAndGet(t *testing.T) {
	repo := &stubBusinessConfigRepo{}
	svc := NewBusinessConfigService(repo)

	require.NoError(t, svc.SetJSON(t.Context(), "processing", "default_target_type", []byte(`"wechat-longform"`), "local-admin"))

	value, err := svc.Get(t.Context(), "processing", "default_target_type")
	require.NoError(t, err)
	require.Equal(t, "processing", value.Category)
	require.Equal(t, "default_target_type", value.Key)
	require.Equal(t, `"wechat-longform"`, string(value.ValueJSON))
	require.Equal(t, "local-admin", value.UpdatedBy)
}
