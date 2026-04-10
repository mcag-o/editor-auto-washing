package sqlite

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"database/sql"
	"fmt"
	"time"
)

var _ repo.CollectorEntryRepo = (*collectorEntryRepo)(nil)

type collectorEntryRepo struct{ db *sql.DB }

func (r *collectorEntryRepo) Create(ctx context.Context, entry *domain.CollectorEntry) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO collector_entries (id, run_id, source_id, external_id, canonical_url, title, summary, author, status, rank, published_at, raw_json, normalized_json, metadata_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, entry.ID, entry.RunID, entry.SourceID, entry.ExternalID, entry.CanonicalURL, entry.Title, entry.Summary, entry.Author, entry.Status, nullableInt(entry.Rank), nullableTime(entry.PublishedAt), string(entry.RawJSON), string(entry.NormalizedJSON), string(entry.MetadataJSON), entry.CreatedAt.Format(time.RFC3339Nano), entry.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("insert collector entry: %w", err)
	}
	return nil
}

func (r *collectorEntryRepo) GetByID(ctx context.Context, id string) (*domain.CollectorEntry, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, run_id, source_id, external_id, canonical_url, title, summary, author, status, rank, published_at, raw_json, normalized_json, metadata_json, created_at, updated_at FROM collector_entries WHERE id = ?`, id)
	entry, err := scanCollectorEntry(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("collector_entry", id)
		}
		return nil, err
	}
	return entry, nil
}

func (r *collectorEntryRepo) Update(ctx context.Context, entry *domain.CollectorEntry) error {
	_, err := r.db.ExecContext(ctx, `UPDATE collector_entries SET source_id = ?, external_id = ?, canonical_url = ?, title = ?, summary = ?, author = ?, status = ?, rank = ?, published_at = ?, raw_json = ?, normalized_json = ?, metadata_json = ?, updated_at = ? WHERE id = ?`, entry.SourceID, entry.ExternalID, entry.CanonicalURL, entry.Title, entry.Summary, entry.Author, entry.Status, nullableInt(entry.Rank), nullableTime(entry.PublishedAt), string(entry.RawJSON), string(entry.NormalizedJSON), string(entry.MetadataJSON), entry.UpdatedAt.Format(time.RFC3339Nano), entry.ID)
	if err != nil {
		return fmt.Errorf("update collector entry: %w", err)
	}
	return nil
}

func (r *collectorEntryRepo) ListByRunID(ctx context.Context, runID string) ([]domain.CollectorEntry, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, run_id, source_id, external_id, canonical_url, title, summary, author, status, rank, published_at, raw_json, normalized_json, metadata_json, created_at, updated_at FROM collector_entries WHERE run_id = ? ORDER BY created_at ASC, id ASC`, runID)
	if err != nil {
		return nil, fmt.Errorf("query collector entries: %w", err)
	}
	defer rows.Close()
	var items []domain.CollectorEntry
	for rows.Next() {
		entry, scanErr := scanCollectorEntry(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *entry)
	}
	return items, rows.Err()
}

type collectorEntryScanner interface{ Scan(dest ...any) error }

func scanCollectorEntry(row collectorEntryScanner) (*domain.CollectorEntry, error) {
	var entry domain.CollectorEntry
	var rank sql.NullInt64
	var publishedAt sql.NullString
	var rawJSON, normalizedJSON, metadataJSON, createdAt, updatedAt string
	if err := row.Scan(&entry.ID, &entry.RunID, &entry.SourceID, &entry.ExternalID, &entry.CanonicalURL, &entry.Title, &entry.Summary, &entry.Author, &entry.Status, &rank, &publishedAt, &rawJSON, &normalizedJSON, &metadataJSON, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	if rank.Valid {
		value := int(rank.Int64)
		entry.Rank = &value
	}
	if publishedAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, publishedAt.String)
		if err != nil {
			return nil, fmt.Errorf("decode collector entry published_at: %w", err)
		}
		entry.PublishedAt = &parsed
	}
	entry.RawJSON = []byte(rawJSON)
	entry.NormalizedJSON = []byte(normalizedJSON)
	entry.MetadataJSON = []byte(metadataJSON)
	created, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("decode collector entry created_at: %w", err)
	}
	updated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("decode collector entry updated_at: %w", err)
	}
	entry.CreatedAt = created
	entry.UpdatedAt = updated
	return &entry, nil
}
