package sqlite

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

var _ repo.ArticleRepo = (*articleRepo)(nil)

type articleRepo struct {
	db *sql.DB
}

func (r *articleRepo) Create(ctx context.Context, doc *domain.ContentDocument) error {
	meta, err := json.Marshal(doc.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO articles (id, title, body, format, summary, metadata, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		doc.ID,
		doc.Title,
		doc.Body,
		doc.Format,
		doc.Summary,
		string(meta),
		doc.CreatedAt.Format(time.RFC3339),
		doc.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert article: %w", err)
	}
	return nil
}

func (r *articleRepo) GetByID(ctx context.Context, id string) (*domain.ContentDocument, error) {
	var doc domain.ContentDocument
	var metaStr, createdAt, updatedAt string

	err := r.db.QueryRowContext(ctx,
		`SELECT id, title, body, format, summary, metadata, created_at, updated_at FROM articles WHERE id = ?`,
		id,
	).Scan(&doc.ID, &doc.Title, &doc.Body, &doc.Format, &doc.Summary, &metaStr, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("article", id)
		}
		return nil, fmt.Errorf("query article: %w", err)
	}

	if err := json.Unmarshal([]byte(metaStr), &doc.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}
	if doc.Metadata == nil {
		doc.Metadata = make(map[string]any)
	}

	if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
		doc.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
		doc.UpdatedAt = t
	}

	return &doc, nil
}

func (r *articleRepo) List(ctx context.Context, q domain.ListQuery) ([]domain.ContentDocument, error) {
	args := []any{}
	conditions := []string{}

	if q.TitleQuery != "" {
		conditions = append(conditions, "title LIKE ?")
		args = append(args, "%"+q.TitleQuery+"%")
	}
	if q.Published != nil {
		conditions = append(conditions, "metadata LIKE ?")
		if *q.Published {
			args = append(args, `%"published":true%`)
		} else {
			args = append(args, `%"published":false%`)
		}
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	limit := q.Limit
	offset := q.Offset
	if limit <= 0 {
		limit = 50
	}

	query := fmt.Sprintf(
		`SELECT id, title, body, format, summary, metadata, created_at, updated_at FROM articles %s ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		where,
	)
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query articles: %w", err)
	}
	defer rows.Close()

	var docs []domain.ContentDocument
	for rows.Next() {
		var doc domain.ContentDocument
		var metaStr, createdAt, updatedAt string

		if err := rows.Scan(&doc.ID, &doc.Title, &doc.Body, &doc.Format, &doc.Summary, &metaStr, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan article: %w", err)
		}

		if err := json.Unmarshal([]byte(metaStr), &doc.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
		if doc.Metadata == nil {
			doc.Metadata = make(map[string]any)
		}

		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			doc.CreatedAt = t
		}
		if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			doc.UpdatedAt = t
		}

		docs = append(docs, doc)
	}

	return docs, rows.Err()
}

func (r *articleRepo) Update(ctx context.Context, id string, body string) error {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx,
		`UPDATE articles SET body = ?, updated_at = ? WHERE id = ?`,
		body, now.Format(time.RFC3339), id,
	)
	if err != nil {
		return fmt.Errorf("update article: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check update result: %w", err)
	}
	if rows == 0 {
		return domain.NewNotFoundErr("article", id)
	}
	return nil
}

func (r *articleRepo) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM articles WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete article: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check delete result: %w", err)
	}
	if rows == 0 {
		return domain.NewNotFoundErr("article", id)
	}
	return nil
}
