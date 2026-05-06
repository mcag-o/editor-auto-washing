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

var _ repo.TemplateDefinitionRepo = (*templateDefinitionRepo)(nil)

type templateDefinitionRepo struct{ db *sql.DB }

func (r *templateDefinitionRepo) Create(ctx context.Context, template *domain.TemplateDefinition) error {
	return r.write(ctx, template, false)
}

func (r *templateDefinitionRepo) Update(ctx context.Context, template *domain.TemplateDefinition) error {
	if err := template.Validate(); err != nil {
		return err
	}
	variablesJSON, err := normalizeTemplateVariablesJSON(template.VariablesJSON)
	if err != nil {
		return err
	}
	template.VariablesJSON = variablesJSON
	template.UpdatedAt = time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
		UPDATE template_definitions
		SET name = ?, type = ?, version = ?, enabled = ?, content = ?, variables_json = ?, updated_by = ?, updated_at = ?
		WHERE id = ?
	`, template.Name, template.Type, template.Version, boolToInt(template.Enabled), template.Content, template.VariablesJSON, template.UpdatedBy, template.UpdatedAt.Format(time.RFC3339Nano), template.ID)
	if err != nil {
		return fmt.Errorf("update template definition: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update template definition rows affected: %w", err)
	}
	if affected == 0 {
		return domain.NewNotFoundErr("template_definition", template.ID)
	}
	return nil
}

func (r *templateDefinitionRepo) Upsert(ctx context.Context, template *domain.TemplateDefinition) error {
	return r.write(ctx, template, true)
}

func (r *templateDefinitionRepo) write(ctx context.Context, template *domain.TemplateDefinition, upsert bool) error {
	if err := template.Validate(); err != nil {
		return err
	}
	variablesJSON, err := normalizeTemplateVariablesJSON(template.VariablesJSON)
	if err != nil {
		return err
	}
	template.VariablesJSON = variablesJSON
	template.UpdatedAt = time.Now().UTC()
	query := `
		INSERT INTO template_definitions (id, name, type, version, enabled, content, variables_json, updated_by, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if upsert {
		query += `
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			type = excluded.type,
			version = excluded.version,
			enabled = excluded.enabled,
			content = excluded.content,
			variables_json = excluded.variables_json,
			updated_by = excluded.updated_by,
			updated_at = excluded.updated_at`
	}
	_, err = r.db.ExecContext(ctx, query,
		template.ID, template.Name, template.Type, template.Version, boolToInt(template.Enabled), template.Content, template.VariablesJSON, template.UpdatedBy, template.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		if !upsert && isSQLitePrimaryKeyConflict(err) {
			return domain.NewConflictErr("template definition already exists")
		}
		if upsert {
			return fmt.Errorf("upsert template definition: %w", err)
		}
		return fmt.Errorf("create template definition: %w", err)
	}
	return nil
}

func (r *templateDefinitionRepo) GetByID(ctx context.Context, id string) (*domain.TemplateDefinition, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, name, type, version, enabled, content, variables_json, updated_by, updated_at FROM template_definitions WHERE id = ?`, id)
	template, err := scanTemplateDefinition(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("template_definition", id)
		}
		return nil, err
	}
	return template, nil
}

func (r *templateDefinitionRepo) List(ctx context.Context, limit int) ([]domain.TemplateDefinition, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, type, version, enabled, content, variables_json, updated_by, updated_at FROM template_definitions ORDER BY updated_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query template definitions: %w", err)
	}
	defer rows.Close()
	items := []domain.TemplateDefinition{}
	for rows.Next() {
		template, err := scanTemplateDefinition(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *template)
	}
	return items, rows.Err()
}

func (r *templateDefinitionRepo) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM template_definitions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete template definition: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete template definition rows affected: %w", err)
	}
	if affected == 0 {
		return domain.NewNotFoundErr("template_definition", id)
	}
	return nil
}

type templateDefinitionScanner interface{ Scan(dest ...any) error }

func scanTemplateDefinition(row templateDefinitionScanner) (*domain.TemplateDefinition, error) {
	var template domain.TemplateDefinition
	var enabled int
	var updatedAt string
	if err := row.Scan(&template.ID, &template.Name, &template.Type, &template.Version, &enabled, &template.Content, &template.VariablesJSON, &template.UpdatedBy, &updatedAt); err != nil {
		return nil, err
	}
	variablesJSON, err := normalizeTemplateVariablesJSON(template.VariablesJSON)
	if err != nil {
		return nil, fmt.Errorf("decode template definition variables_json: %w", err)
	}
	template.VariablesJSON = variablesJSON
	template.Enabled = enabled == 1
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("decode template definition updated_at: %w", err)
	}
	template.UpdatedAt = parsedUpdatedAt
	return &template, nil
}

func normalizeTemplateVariablesJSON(value []byte) ([]byte, error) {
	if len(value) == 0 {
		return []byte(`{}`), nil
	}
	if !json.Valid(value) {
		return nil, domain.NewValidationErr("template variables json must be valid json", nil)
	}
	return value, nil
}
