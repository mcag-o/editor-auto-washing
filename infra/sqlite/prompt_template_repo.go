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

var _ repo.PromptTemplateRepo = (*promptTemplateRepo)(nil)

type promptTemplateRepo struct{ db *sql.DB }

func (r *promptTemplateRepo) Upsert(ctx context.Context, prompt *domain.PromptTemplate) error {
	now := time.Now().UTC()
	if prompt.CreatedAt.IsZero() {
		prompt.CreatedAt = now
	}
	prompt.UpdatedAt = now
	metadataJSON, err := json.Marshal(prompt.Metadata)
	if err != nil {
		return fmt.Errorf("marshal prompt template metadata: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO prompt_templates (key, version, system_template, user_template, description, metadata_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(key, version) DO UPDATE SET
			system_template = excluded.system_template,
			user_template = excluded.user_template,
			description = excluded.description,
			metadata_json = excluded.metadata_json,
			updated_at = excluded.updated_at
	`, prompt.Key, prompt.Version, prompt.SystemTemplate, prompt.UserTemplate, prompt.Description, string(metadataJSON), prompt.CreatedAt.Format(time.RFC3339), prompt.UpdatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("upsert prompt template: %w", err)
	}
	return nil
}

func (r *promptTemplateRepo) Get(ctx context.Context, key, version string) (*domain.PromptTemplate, error) {
	row := r.db.QueryRowContext(ctx, `SELECT key, version, system_template, user_template, description, metadata_json, created_at, updated_at FROM prompt_templates WHERE key = ? AND version = ?`, key, version)
	prompt, err := scanPromptTemplate(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("prompt_template", fmt.Sprintf("%s/%s", key, version))
		}
		return nil, err
	}
	return prompt, nil
}

func (r *promptTemplateRepo) List(ctx context.Context) ([]domain.PromptTemplate, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT key, version, system_template, user_template, description, metadata_json, created_at, updated_at FROM prompt_templates ORDER BY key ASC, version DESC`)
	if err != nil {
		return nil, fmt.Errorf("query prompt templates: %w", err)
	}
	defer rows.Close()
	var prompts []domain.PromptTemplate
	for rows.Next() {
		prompt, scanErr := scanPromptTemplate(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		prompts = append(prompts, *prompt)
	}
	return prompts, rows.Err()
}

type promptTemplateScanner interface{ Scan(dest ...any) error }

func scanPromptTemplate(row promptTemplateScanner) (*domain.PromptTemplate, error) {
	var prompt domain.PromptTemplate
	var metadataJSON, createdAt, updatedAt string
	if err := row.Scan(&prompt.Key, &prompt.Version, &prompt.SystemTemplate, &prompt.UserTemplate, &prompt.Description, &metadataJSON, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(metadataJSON), &prompt.Metadata); err != nil {
		return nil, fmt.Errorf("decode prompt template metadata: %w", err)
	}
	if prompt.Metadata == nil {
		prompt.Metadata = map[string]any{}
	}
	parsedCreatedAt, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("decode prompt template created_at: %w", err)
	}
	prompt.CreatedAt = parsedCreatedAt
	parsedUpdatedAt, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("decode prompt template updated_at: %w", err)
	}
	prompt.UpdatedAt = parsedUpdatedAt
	return &prompt, nil
}
