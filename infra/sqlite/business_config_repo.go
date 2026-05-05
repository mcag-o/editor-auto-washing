package sqlite

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

var _ repo.BusinessConfigRepo = (*businessConfigRepo)(nil)

type businessConfigRepo struct{ db *sql.DB }

func (r *businessConfigRepo) Upsert(ctx context.Context, cfg *domain.BusinessConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	now := time.Now().UTC()
	if cfg.CreatedAt.IsZero() {
		cfg.CreatedAt = now
	}
	if cfg.UpdatedAt.IsZero() {
		cfg.UpdatedAt = now
	}
	metadataJSON, err := json.Marshal(cfg.Metadata)
	if err != nil {
		return fmt.Errorf("marshal business config metadata: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO business_configs (id, category, key, value, metadata_json, updated_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(category, key) DO UPDATE SET
			value = excluded.value,
			metadata_json = excluded.metadata_json,
			updated_by = excluded.updated_by,
			updated_at = excluded.updated_at
	`, cfg.ID, cfg.Category, cfg.Key, cfg.Value, string(metadataJSON), cfg.UpdatedBy, cfg.CreatedAt.Format(time.RFC3339Nano), cfg.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("upsert business config: %w", err)
	}
	return nil
}

func (r *businessConfigRepo) GetByCategoryAndKey(ctx context.Context, category, key string) (*domain.BusinessConfig, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, category, key, value, metadata_json, updated_by, created_at, updated_at FROM business_configs WHERE category = ? AND key = ?`, category, key)
	cfg, err := scanBusinessConfig(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("business_config", fmt.Sprintf("%s/%s", category, key))
		}
		return nil, err
	}
	return cfg, nil
}

func (r *businessConfigRepo) ListByCategory(ctx context.Context, category string) ([]domain.BusinessConfig, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, category, key, value, metadata_json, updated_by, created_at, updated_at FROM business_configs WHERE category = ? ORDER BY key ASC`, category)
	if err != nil {
		return nil, fmt.Errorf("query business configs: %w", err)
	}
	defer rows.Close()
	var configs []domain.BusinessConfig
	for rows.Next() {
		cfg, scanErr := scanBusinessConfig(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		configs = append(configs, *cfg)
	}
	return configs, rows.Err()
}

type businessConfigScanner interface{ Scan(dest ...any) error }

func scanBusinessConfig(row businessConfigScanner) (*domain.BusinessConfig, error) {
	var cfg domain.BusinessConfig
	var metadataJSON string
	var createdAt string
	var updatedAt string
	if err := row.Scan(&cfg.ID, &cfg.Category, &cfg.Key, &cfg.Value, &metadataJSON, &cfg.UpdatedBy, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(metadataJSON), &cfg.Metadata); err != nil {
		return nil, fmt.Errorf("decode business config metadata: %w", err)
	}
	if cfg.Metadata == nil {
		cfg.Metadata = map[string]any{}
	}
	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("decode business config created_at: %w", err)
	}
	cfg.CreatedAt = parsedCreatedAt
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("decode business config updated_at: %w", err)
	}
	cfg.UpdatedAt = parsedUpdatedAt
	return &cfg, nil
}
