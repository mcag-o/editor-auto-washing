package sqlite

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"database/sql"
	"fmt"
	"time"
)

var _ repo.IngestionRepo = (*ingestionRepo)(nil)

type ingestionRepo struct {
	db *sql.DB
}

func (r *ingestionRepo) Record(ctx context.Context, rec *domain.IngestionRecord) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO ingestions (id, source_type, bundle_file, original_location, routed_path, payload, status, error_message, imported_items, created_articles, retried, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, rec.ID, rec.SourceType, rec.BundleFile, rec.OriginalLocation, rec.RoutedPath, string(rec.Payload), rec.Status, rec.ErrorMessage, rec.ImportedItems, rec.CreatedArticles, boolToInt(rec.Retried), rec.CreatedAt.Format(time.RFC3339), rec.UpdatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert ingestion: %w", err)
	}
	return nil
}

func (r *ingestionRepo) GetByID(ctx context.Context, id string) (*domain.IngestionRecord, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, source_type, bundle_file, original_location, routed_path, payload, status, error_message, imported_items, created_articles, retried, created_at, updated_at FROM ingestions WHERE id = ?`, id)
	rec, err := scanIngestionRecord(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("ingestion", id)
		}
		return nil, err
	}
	return rec, nil
}

func (r *ingestionRepo) List(ctx context.Context, status string) ([]domain.IngestionRecord, error) {
	query := `SELECT id, source_type, bundle_file, original_location, routed_path, payload, status, error_message, imported_items, created_articles, retried, created_at, updated_at FROM ingestions`
	args := []any{}
	if status != "" {
		query += ` WHERE status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query ingestions: %w", err)
	}
	defer rows.Close()

	var records []domain.IngestionRecord
	for rows.Next() {
		rec, err := scanIngestionRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, *rec)
	}
	return records, rows.Err()
}

func (r *ingestionRepo) Update(ctx context.Context, id string, fn func(*domain.IngestionRecord)) error {
	rec, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	fn(rec)
	_, err = r.db.ExecContext(ctx, `UPDATE ingestions SET source_type = ?, bundle_file = ?, original_location = ?, routed_path = ?, payload = ?, status = ?, error_message = ?, imported_items = ?, created_articles = ?, retried = ?, updated_at = ? WHERE id = ?`, rec.SourceType, rec.BundleFile, rec.OriginalLocation, rec.RoutedPath, string(rec.Payload), rec.Status, rec.ErrorMessage, rec.ImportedItems, rec.CreatedArticles, boolToInt(rec.Retried), rec.UpdatedAt.Format(time.RFC3339), rec.ID)
	if err != nil {
		return fmt.Errorf("update ingestion: %w", err)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanIngestionRecord(row scanner) (*domain.IngestionRecord, error) {
	var rec domain.IngestionRecord
	var payload string
	var retried int
	var createdAt string
	var updatedAt string
	if err := row.Scan(&rec.ID, &rec.SourceType, &rec.BundleFile, &rec.OriginalLocation, &rec.RoutedPath, &payload, &rec.Status, &rec.ErrorMessage, &rec.ImportedItems, &rec.CreatedArticles, &retried, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	rec.Payload = []byte(payload)
	rec.Retried = retried == 1
	parsedCreatedAt, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("decode ingestion created_at: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("decode ingestion updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreatedAt
	rec.UpdatedAt = parsedUpdatedAt
	return &rec, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
