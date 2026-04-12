package sqlite

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"database/sql"
	"fmt"
	"time"
)

var _ repo.CollectorRunRepo = (*collectorRunRepo)(nil)

type collectorRunRepo struct{ db *sql.DB }

func (r *collectorRunRepo) Create(ctx context.Context, run *domain.CollectorRun) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO collector_runs (id, trigger, status, started_at, completed_at, error_code, error_message, metadata_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, run.ID, run.Trigger, run.Status, nullableTime(run.StartedAt), nullableTime(run.CompletedAt), nullableString(emptyToNil(run.ErrorCode)), nullableString(emptyToNil(run.ErrorMessage)), string(run.MetadataJSON), run.CreatedAt.Format(time.RFC3339Nano), run.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("insert collector run: %w", err)
	}
	return nil
}

func (r *collectorRunRepo) GetByID(ctx context.Context, id string) (*domain.CollectorRun, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, trigger, status, started_at, completed_at, error_code, error_message, metadata_json, created_at, updated_at FROM collector_runs WHERE id = ?`, id)
	run, err := scanCollectorRun(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("collector_run", id)
		}
		return nil, err
	}
	return run, nil
}

func (r *collectorRunRepo) Update(ctx context.Context, run *domain.CollectorRun) error {
	run.UpdatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `UPDATE collector_runs SET trigger = ?, status = ?, started_at = ?, completed_at = ?, error_code = ?, error_message = ?, metadata_json = ?, updated_at = ? WHERE id = ?`, run.Trigger, run.Status, nullableTime(run.StartedAt), nullableTime(run.CompletedAt), nullableString(emptyToNil(run.ErrorCode)), nullableString(emptyToNil(run.ErrorMessage)), string(run.MetadataJSON), run.UpdatedAt.Format(time.RFC3339Nano), run.ID)
	if err != nil {
		return fmt.Errorf("update collector run: %w", err)
	}
	return nil
}

func (r *collectorRunRepo) ListRecent(ctx context.Context, limit int) ([]domain.CollectorRun, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, trigger, status, started_at, completed_at, error_code, error_message, metadata_json, created_at, updated_at FROM collector_runs ORDER BY created_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query collector runs: %w", err)
	}
	defer rows.Close()
	var items []domain.CollectorRun
	for rows.Next() {
		run, scanErr := scanCollectorRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *run)
	}
	return items, rows.Err()
}

func (r *collectorRunRepo) CreateSourceRun(ctx context.Context, sourceRun *domain.CollectorSourceRun) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO collector_source_runs (id, run_id, source_id, stage, status, started_at, completed_at, error_code, error_message, discovered_count, stored_count, metadata_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, sourceRun.ID, sourceRun.RunID, sourceRun.SourceID, sourceRun.Stage, sourceRun.Status, nullableTime(sourceRun.StartedAt), nullableTime(sourceRun.CompletedAt), nullableString(emptyToNil(sourceRun.ErrorCode)), nullableString(emptyToNil(sourceRun.ErrorMessage)), sourceRun.DiscoveredCount, sourceRun.StoredCount, string(sourceRun.MetadataJSON), sourceRun.CreatedAt.Format(time.RFC3339Nano), sourceRun.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("insert collector source run: %w", err)
	}
	return nil
}

func (r *collectorRunRepo) UpdateSourceRun(ctx context.Context, sourceRun *domain.CollectorSourceRun) error {
	sourceRun.UpdatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `UPDATE collector_source_runs SET status = ?, started_at = ?, completed_at = ?, error_code = ?, error_message = ?, discovered_count = ?, stored_count = ?, metadata_json = ?, updated_at = ? WHERE id = ?`, sourceRun.Status, nullableTime(sourceRun.StartedAt), nullableTime(sourceRun.CompletedAt), nullableString(emptyToNil(sourceRun.ErrorCode)), nullableString(emptyToNil(sourceRun.ErrorMessage)), sourceRun.DiscoveredCount, sourceRun.StoredCount, string(sourceRun.MetadataJSON), sourceRun.UpdatedAt.Format(time.RFC3339Nano), sourceRun.ID)
	if err != nil {
		return fmt.Errorf("update collector source run: %w", err)
	}
	return nil
}

func (r *collectorRunRepo) ListSourceRuns(ctx context.Context, runID string) ([]domain.CollectorSourceRun, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, run_id, source_id, stage, status, started_at, completed_at, error_code, error_message, discovered_count, stored_count, metadata_json, created_at, updated_at FROM collector_source_runs WHERE run_id = ? ORDER BY created_at ASC, id ASC`, runID)
	if err != nil {
		return nil, fmt.Errorf("query collector source runs: %w", err)
	}
	defer rows.Close()
	var items []domain.CollectorSourceRun
	for rows.Next() {
		sourceRun, scanErr := scanCollectorSourceRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *sourceRun)
	}
	return items, rows.Err()
}

type collectorRunScanner interface{ Scan(dest ...any) error }

func scanCollectorRun(row collectorRunScanner) (*domain.CollectorRun, error) {
	var run domain.CollectorRun
	var startedAt, completedAt, errorCode, errorMessage sql.NullString
	var metadataJSON, createdAt, updatedAt string
	if err := row.Scan(&run.ID, &run.Trigger, &run.Status, &startedAt, &completedAt, &errorCode, &errorMessage, &metadataJSON, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	if startedAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, startedAt.String)
		if err != nil {
			return nil, fmt.Errorf("decode collector run started_at: %w", err)
		}
		run.StartedAt = &parsed
	}
	if completedAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, completedAt.String)
		if err != nil {
			return nil, fmt.Errorf("decode collector run completed_at: %w", err)
		}
		run.CompletedAt = &parsed
	}
	if errorCode.Valid {
		run.ErrorCode = errorCode.String
	}
	if errorMessage.Valid {
		run.ErrorMessage = errorMessage.String
	}
	run.MetadataJSON = []byte(metadataJSON)
	created, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("decode collector run created_at: %w", err)
	}
	updated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("decode collector run updated_at: %w", err)
	}
	run.CreatedAt = created
	run.UpdatedAt = updated
	return &run, nil
}

func scanCollectorSourceRun(row collectorRunScanner) (*domain.CollectorSourceRun, error) {
	var sourceRun domain.CollectorSourceRun
	var startedAt, completedAt, errorCode, errorMessage sql.NullString
	var metadataJSON, createdAt, updatedAt string
	if err := row.Scan(&sourceRun.ID, &sourceRun.RunID, &sourceRun.SourceID, &sourceRun.Stage, &sourceRun.Status, &startedAt, &completedAt, &errorCode, &errorMessage, &sourceRun.DiscoveredCount, &sourceRun.StoredCount, &metadataJSON, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	if startedAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, startedAt.String)
		if err != nil {
			return nil, fmt.Errorf("decode collector source run started_at: %w", err)
		}
		sourceRun.StartedAt = &parsed
	}
	if completedAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, completedAt.String)
		if err != nil {
			return nil, fmt.Errorf("decode collector source run completed_at: %w", err)
		}
		sourceRun.CompletedAt = &parsed
	}
	if errorCode.Valid {
		sourceRun.ErrorCode = errorCode.String
	}
	if errorMessage.Valid {
		sourceRun.ErrorMessage = errorMessage.String
	}
	sourceRun.MetadataJSON = []byte(metadataJSON)
	created, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("decode collector source run created_at: %w", err)
	}
	updated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("decode collector source run updated_at: %w", err)
	}
	sourceRun.CreatedAt = created
	sourceRun.UpdatedAt = updated
	return &sourceRun, nil
}
