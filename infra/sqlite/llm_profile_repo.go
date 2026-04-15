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

var _ repo.LLMProfileRepo = (*llmProfileRepo)(nil)

type llmProfileRepo struct{ db *sql.DB }

func (r *llmProfileRepo) Upsert(ctx context.Context, profile *domain.LLMProfile) error {
	now := time.Now().UTC()
	if profile.CreatedAt.IsZero() {
		profile.CreatedAt = now
	}
	profile.UpdatedAt = now
	metadataJSON, err := json.Marshal(profile.Metadata)
	if err != nil {
		return fmt.Errorf("marshal llm profile metadata: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO llm_profiles (name, provider, model, api_key_ref, base_url_ref, temperature, max_tokens, timeout_sec, metadata_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			provider = excluded.provider,
			model = excluded.model,
			api_key_ref = excluded.api_key_ref,
			base_url_ref = excluded.base_url_ref,
			temperature = excluded.temperature,
			max_tokens = excluded.max_tokens,
			timeout_sec = excluded.timeout_sec,
			metadata_json = excluded.metadata_json,
			updated_at = excluded.updated_at
	`, profile.Name, profile.Provider, profile.Model, profile.APIKeyRef, profile.BaseURLRef, profile.Temperature, profile.MaxTokens, profile.TimeoutSec, string(metadataJSON), profile.CreatedAt.Format(time.RFC3339), profile.UpdatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("upsert llm profile: %w", err)
	}
	return nil
}

func (r *llmProfileRepo) GetByName(ctx context.Context, name string) (*domain.LLMProfile, error) {
	row := r.db.QueryRowContext(ctx, `SELECT name, provider, model, api_key_ref, base_url_ref, temperature, max_tokens, timeout_sec, metadata_json, created_at, updated_at FROM llm_profiles WHERE name = ?`, name)
	profile, err := scanLLMProfile(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("llm_profile", name)
		}
		return nil, err
	}
	return profile, nil
}

func (r *llmProfileRepo) List(ctx context.Context) ([]domain.LLMProfile, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT name, provider, model, api_key_ref, base_url_ref, temperature, max_tokens, timeout_sec, metadata_json, created_at, updated_at FROM llm_profiles ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("query llm profiles: %w", err)
	}
	defer rows.Close()
	var profiles []domain.LLMProfile
	for rows.Next() {
		profile, scanErr := scanLLMProfile(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		profiles = append(profiles, *profile)
	}
	return profiles, rows.Err()
}

type llmProfileScanner interface{ Scan(dest ...any) error }

func scanLLMProfile(row llmProfileScanner) (*domain.LLMProfile, error) {
	var profile domain.LLMProfile
	var metadataJSON, createdAt, updatedAt string
	if err := row.Scan(&profile.Name, &profile.Provider, &profile.Model, &profile.APIKeyRef, &profile.BaseURLRef, &profile.Temperature, &profile.MaxTokens, &profile.TimeoutSec, &metadataJSON, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(metadataJSON), &profile.Metadata); err != nil {
		return nil, fmt.Errorf("decode llm profile metadata: %w", err)
	}
	if profile.Metadata == nil {
		profile.Metadata = map[string]any{}
	}
	parsedCreatedAt, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("decode llm profile created_at: %w", err)
	}
	profile.CreatedAt = parsedCreatedAt
	parsedUpdatedAt, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("decode llm profile updated_at: %w", err)
	}
	profile.UpdatedAt = parsedUpdatedAt
	return &profile, nil
}
