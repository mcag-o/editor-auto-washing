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

var _ repo.TemplateRepo = (*templateRepo)(nil)

type templateRepo struct {
	db *sql.DB
}

func (r *templateRepo) Create(ctx context.Context, t *domain.TemplateAsset) error {
	meta, err := json.Marshal(t.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO templates (id, category, name, content, metadata, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.ID,
		t.Category,
		t.Name,
		t.Content,
		string(meta),
		t.CreatedAt.Format(time.RFC3339),
		t.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert template: %w", err)
	}
	return nil
}

func (r *templateRepo) GetByID(ctx context.Context, id string) (*domain.TemplateAsset, error) {
	var t domain.TemplateAsset
	var metaStr, createdAt, updatedAt string

	err := r.db.QueryRowContext(ctx,
		`SELECT id, category, name, content, metadata, created_at, updated_at FROM templates WHERE id = ?`,
		id,
	).Scan(&t.ID, &t.Category, &t.Name, &t.Content, &metaStr, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("template", id)
		}
		return nil, fmt.Errorf("query template: %w", err)
	}

	if err := json.Unmarshal([]byte(metaStr), &t.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}
	if t.Metadata == nil {
		t.Metadata = make(map[string]any)
	}

	if timeVal, err := time.Parse(time.RFC3339, createdAt); err == nil {
		t.CreatedAt = timeVal
	}
	if timeVal, err := time.Parse(time.RFC3339, updatedAt); err == nil {
		t.UpdatedAt = timeVal
	}

	return &t, nil
}

func (r *templateRepo) List(ctx context.Context, category string) ([]domain.TemplateAsset, error) {
	var query string
	var args []any

	if category != "" {
		query = `SELECT id, category, name, content, metadata, created_at, updated_at FROM templates WHERE category = ? ORDER BY name`
		args = append(args, category)
	} else {
		query = `SELECT id, category, name, content, metadata, created_at, updated_at FROM templates ORDER BY category, name`
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query templates: %w", err)
	}
	defer rows.Close()

	var templates []domain.TemplateAsset
	for rows.Next() {
		var t domain.TemplateAsset
		var metaStr, createdAt, updatedAt string

		if err := rows.Scan(&t.ID, &t.Category, &t.Name, &t.Content, &metaStr, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan template: %w", err)
		}

		if err := json.Unmarshal([]byte(metaStr), &t.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
		if t.Metadata == nil {
			t.Metadata = make(map[string]any)
		}

		if timeVal, err := time.Parse(time.RFC3339, createdAt); err == nil {
			t.CreatedAt = timeVal
		}
		if timeVal, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			t.UpdatedAt = timeVal
		}

		templates = append(templates, t)
	}

	return templates, rows.Err()
}

func (r *templateRepo) ListCategories(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT category FROM templates ORDER BY category`)
	if err != nil {
		return nil, fmt.Errorf("query categories: %w", err)
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var cat string
		if err := rows.Scan(&cat); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		categories = append(categories, cat)
	}

	return categories, rows.Err()
}

func (r *templateRepo) Update(ctx context.Context, id string, content string) error {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx,
		`UPDATE templates SET content = ?, updated_at = ? WHERE id = ?`,
		content, now.Format(time.RFC3339), id,
	)
	if err != nil {
		return fmt.Errorf("update template: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check update result: %w", err)
	}
	if rows == 0 {
		return domain.NewNotFoundErr("template", id)
	}
	return nil
}

func (r *templateRepo) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM templates WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete template: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check delete result: %w", err)
	}
	if rows == 0 {
		return domain.NewNotFoundErr("template", id)
	}
	return nil
}
