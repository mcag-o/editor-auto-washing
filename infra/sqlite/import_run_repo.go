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

var _ repo.ImportRunRepo = (*importRunRepo)(nil)

type importRunRepo struct{ db *sql.DB }

func (r *importRunRepo) Create(ctx context.Context, run *domain.ImportRun) error {
	if err := run.Validate(); err != nil {
		return err
	}
	metadataJSON, err := json.Marshal(run.Metadata)
	if err != nil {
		return fmt.Errorf("marshal import run metadata: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO import_runs (id, source_type, status, started_at, completed_at, imported_count, failed_count, error_summary, metadata_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, run.ID, run.SourceType, run.Status, run.StartedAt.Format(time.RFC3339Nano), nullableTimeNano(run.CompletedAt), run.ImportedCount, run.FailedCount, run.ErrorSummary, string(metadataJSON))
	if err != nil {
		return fmt.Errorf("insert import run: %w", err)
	}
	return nil
}

func (r *importRunRepo) Update(ctx context.Context, run *domain.ImportRun) error {
	if err := run.Validate(); err != nil {
		return err
	}
	metadataJSON, err := json.Marshal(run.Metadata)
	if err != nil {
		return fmt.Errorf("marshal import run metadata: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `UPDATE import_runs SET source_type = ?, status = ?, started_at = ?, completed_at = ?, imported_count = ?, failed_count = ?, error_summary = ?, metadata_json = ? WHERE id = ?`, run.SourceType, run.Status, run.StartedAt.Format(time.RFC3339Nano), nullableTimeNano(run.CompletedAt), run.ImportedCount, run.FailedCount, run.ErrorSummary, string(metadataJSON), run.ID)
	if err != nil {
		return fmt.Errorf("update import run: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check update import run result: %w", err)
	}
	if rows == 0 {
		return domain.NewNotFoundErr("import_run", run.ID)
	}
	return nil
}

func (r *importRunRepo) GetByID(ctx context.Context, id string) (*domain.ImportRun, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, source_type, status, started_at, completed_at, imported_count, failed_count, error_summary, metadata_json FROM import_runs WHERE id = ?`, id)
	run, err := scanImportRun(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("import_run", id)
		}
		return nil, err
	}
	return run, nil
}

func (r *importRunRepo) List(ctx context.Context, limit int) ([]domain.ImportRun, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, source_type, status, started_at, completed_at, imported_count, failed_count, error_summary, metadata_json FROM import_runs ORDER BY started_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query import runs: %w", err)
	}
	defer rows.Close()

	items := []domain.ImportRun{}
	for rows.Next() {
		run, err := scanImportRun(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *run)
	}
	return items, rows.Err()
}

type importRunScanner interface{ Scan(dest ...any) error }

func scanImportRun(row importRunScanner) (*domain.ImportRun, error) {
	var run domain.ImportRun
	var startedAt string
	var completedAt sql.NullString
	var metadataJSON string
	if err := row.Scan(&run.ID, &run.SourceType, &run.Status, &startedAt, &completedAt, &run.ImportedCount, &run.FailedCount, &run.ErrorSummary, &metadataJSON); err != nil {
		return nil, err
	}
	parsedStartedAt, err := time.Parse(time.RFC3339Nano, startedAt)
	if err != nil {
		return nil, fmt.Errorf("decode import run started_at: %w", err)
	}
	run.StartedAt = parsedStartedAt
	run.CompletedAt, err = parseNullTime(completedAt, "completed_at")
	if err != nil {
		return nil, fmt.Errorf("decode import run %w", err)
	}
	if err := json.Unmarshal([]byte(metadataJSON), &run.Metadata); err != nil {
		return nil, fmt.Errorf("decode import run metadata: %w", err)
	}
	if run.Metadata == nil {
		run.Metadata = map[string]any{}
	}
	return &run, nil
}
