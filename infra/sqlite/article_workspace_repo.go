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

var _ repo.WorkspaceRepo = (*articleWorkspaceRepo)(nil)

type articleWorkspaceRepo struct {
	db *sql.DB
}

func (r *articleWorkspaceRepo) Create(ctx context.Context, record *domain.ArticleWorkspaceRecord) error {
	statusHistory, err := json.Marshal(record.StatusHistory)
	if err != nil {
		return fmt.Errorf("marshal status history: %w", err)
	}
	lifecycleHistory, err := json.Marshal(record.LifecycleHistory)
	if err != nil {
		return fmt.Errorf("marshal lifecycle history: %w", err)
	}
	source, err := json.Marshal(record.Source)
	if err != nil {
		return fmt.Errorf("marshal workspace source: %w", err)
	}
	metadata, err := json.Marshal(record.Metadata)
	if err != nil {
		return fmt.Errorf("marshal workspace metadata: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO workspace_articles (id, title, summary, status, status_history, lifecycle_history, source, metadata, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, record.ID, record.Title, record.Summary, record.Status, string(statusHistory), string(lifecycleHistory), string(source), string(metadata), record.Notes, record.CreatedAt.Format(time.RFC3339), record.UpdatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert workspace article: %w", err)
	}
	return nil
}

func (r *articleWorkspaceRepo) Update(ctx context.Context, record *domain.ArticleWorkspaceRecord) error {
	statusHistory, err := json.Marshal(record.StatusHistory)
	if err != nil {
		return fmt.Errorf("marshal status history: %w", err)
	}
	lifecycleHistory, err := json.Marshal(record.LifecycleHistory)
	if err != nil {
		return fmt.Errorf("marshal lifecycle history: %w", err)
	}
	source, err := json.Marshal(record.Source)
	if err != nil {
		return fmt.Errorf("marshal workspace source: %w", err)
	}
	metadata, err := json.Marshal(record.Metadata)
	if err != nil {
		return fmt.Errorf("marshal workspace metadata: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `UPDATE workspace_articles SET title = ?, summary = ?, status = ?, status_history = ?, lifecycle_history = ?, source = ?, metadata = ?, notes = ?, created_at = ?, updated_at = ? WHERE id = ?`, record.Title, record.Summary, record.Status, string(statusHistory), string(lifecycleHistory), string(source), string(metadata), record.Notes, record.CreatedAt.Format(time.RFC3339), record.UpdatedAt.Format(time.RFC3339), record.ID)
	if err != nil {
		return fmt.Errorf("update workspace article: %w", err)
	}
	rows, err := result.RowsAffected()
	if err == nil && rows == 0 {
		return domain.NewNotFoundErr("workspace", record.ID)
	}
	return nil
	}

func (r *articleWorkspaceRepo) GetByID(ctx context.Context, id string) (*domain.ArticleWorkspaceRecord, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, title, summary, status, status_history, lifecycle_history, source, metadata, notes, created_at, updated_at FROM workspace_articles WHERE id = ?`, id)
	record, err := scanWorkspaceArticle(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("workspace", id)
		}
		return nil, err
	}
	return record, nil
}

func (r *articleWorkspaceRepo) List(ctx context.Context, status *string) ([]domain.ArticleWorkspaceRecord, error) {
	query := `SELECT id, title, summary, status, status_history, lifecycle_history, source, metadata, notes, created_at, updated_at FROM workspace_articles`
	args := []any{}
	if status != nil && *status != "" {
		query += ` WHERE status = ?`
		args = append(args, *status)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query workspace articles: %w", err)
	}
	defer rows.Close()

	var records []domain.ArticleWorkspaceRecord
	for rows.Next() {
		record, err := scanWorkspaceArticle(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, *record)
	}
	return records, rows.Err()
}

func (r *articleWorkspaceRepo) ListByIngestionID(ctx context.Context, ingestionID string) ([]domain.ArticleWorkspaceRecord, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, title, summary, status, status_history, lifecycle_history, source, metadata, notes, created_at, updated_at FROM workspace_articles WHERE json_extract(source, '$.ingestion_id') = ? ORDER BY created_at DESC`, ingestionID)
	if err != nil {
		return nil, fmt.Errorf("query workspace articles by ingestion: %w", err)
	}
	defer rows.Close()
	var records []domain.ArticleWorkspaceRecord
	for rows.Next() {
		record, err := scanWorkspaceArticle(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, *record)
	}
	return records, rows.Err()
}

func (r *articleWorkspaceRepo) TransitionStatus(ctx context.Context, id string, newStatus, notes string) error {
	record, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := domain.ValidateWorkspaceTransition(record.Status, newStatus); err != nil {
		return err
	}
	now := time.Now().UTC()
	record.Status = newStatus
	record.StatusHistory = append(record.StatusHistory, newStatus)
	record.LifecycleHistory = append(record.LifecycleHistory, domain.ArticleWorkspaceLifecycleEntry{Status: newStatus, Notes: notes, CreatedAt: now})
	record.Notes = notes
	record.UpdatedAt = now
	statusHistory, err := json.Marshal(record.StatusHistory)
	if err != nil {
		return fmt.Errorf("marshal status history: %w", err)
	}
	lifecycleHistory, err := json.Marshal(record.LifecycleHistory)
	if err != nil {
		return fmt.Errorf("marshal lifecycle history: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `UPDATE workspace_articles SET status = ?, status_history = ?, lifecycle_history = ?, notes = ?, updated_at = ? WHERE id = ?`, record.Status, string(statusHistory), string(lifecycleHistory), record.Notes, record.UpdatedAt.Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("update workspace article: %w", err)
	}
	return nil
}

func (r *articleWorkspaceRepo) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM workspace_articles WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete workspace article: %w", err)
	}
	rows, err := result.RowsAffected()
	if err == nil && rows == 0 {
		return domain.NewNotFoundErr("workspace", id)
	}
	return nil
}

func scanWorkspaceArticle(row scanner) (*domain.ArticleWorkspaceRecord, error) {
	var record domain.ArticleWorkspaceRecord
	var statusHistory string
	var lifecycleHistory string
	var source string
	var metadata string
	var createdAt string
	var updatedAt string
	if err := row.Scan(&record.ID, &record.Title, &record.Summary, &record.Status, &statusHistory, &lifecycleHistory, &source, &metadata, &record.Notes, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(statusHistory), &record.StatusHistory); err != nil {
		return nil, fmt.Errorf("decode workspace status history: %w", err)
	}
	if err := json.Unmarshal([]byte(lifecycleHistory), &record.LifecycleHistory); err != nil {
		return nil, fmt.Errorf("decode workspace lifecycle history: %w", err)
	}
	if err := json.Unmarshal([]byte(source), &record.Source); err != nil {
		return nil, fmt.Errorf("decode workspace source: %w", err)
	}
	if err := json.Unmarshal([]byte(metadata), &record.Metadata); err != nil {
		return nil, fmt.Errorf("decode workspace metadata: %w", err)
	}
	if record.Metadata == nil {
		record.Metadata = map[string]any{}
	}
	parsedCreatedAt, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("decode workspace created_at: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("decode workspace updated_at: %w", err)
	}
	record.CreatedAt = parsedCreatedAt
	record.UpdatedAt = parsedUpdatedAt
	return &record, nil
}
