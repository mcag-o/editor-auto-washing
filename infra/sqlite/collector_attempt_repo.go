package sqlite

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"database/sql"
	"fmt"
	"time"
)

var _ repo.CollectorAttemptRepo = (*collectorAttemptRepo)(nil)

type collectorAttemptRepo struct{ db *sql.DB }

func (r *collectorAttemptRepo) Create(ctx context.Context, attempt *domain.CollectorAttempt) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO collector_attempts (id, run_id, source_run_id, entry_id, article_id, stage, attempt_number, status, request_url, request_method, response_status_code, error_code, error_message, raw_json, started_at, completed_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, attempt.ID, attempt.RunID, attempt.SourceRunID, nullableString(emptyToNil(attempt.EntryID)), nullableString(emptyToNil(attempt.ArticleID)), attempt.Stage, attempt.AttemptNumber, attempt.Status, attempt.RequestURL, attempt.RequestMethod, attempt.ResponseStatusCode, nullableString(emptyToNil(attempt.ErrorCode)), nullableString(emptyToNil(attempt.ErrorMessage)), string(attempt.RawJSON), nullableTime(attempt.StartedAt), nullableTime(attempt.CompletedAt), attempt.CreatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("insert collector attempt: %w", err)
	}
	return nil
}

func (r *collectorAttemptRepo) ListBySourceRunID(ctx context.Context, sourceRunID string) ([]domain.CollectorAttempt, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, run_id, source_run_id, entry_id, article_id, stage, attempt_number, status, request_url, request_method, response_status_code, error_code, error_message, raw_json, started_at, completed_at, created_at FROM collector_attempts WHERE source_run_id = ? ORDER BY created_at ASC, id ASC`, sourceRunID)
	if err != nil {
		return nil, fmt.Errorf("query collector attempts: %w", err)
	}
	defer rows.Close()
	var items []domain.CollectorAttempt
	for rows.Next() {
		attempt, scanErr := scanCollectorAttempt(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *attempt)
	}
	return items, rows.Err()
}

type collectorAttemptScanner interface{ Scan(dest ...any) error }

func scanCollectorAttempt(row collectorAttemptScanner) (*domain.CollectorAttempt, error) {
	var attempt domain.CollectorAttempt
	var entryID, articleID, errorCode, errorMessage, startedAt, completedAt sql.NullString
	var rawJSON, createdAt string
	if err := row.Scan(&attempt.ID, &attempt.RunID, &attempt.SourceRunID, &entryID, &articleID, &attempt.Stage, &attempt.AttemptNumber, &attempt.Status, &attempt.RequestURL, &attempt.RequestMethod, &attempt.ResponseStatusCode, &errorCode, &errorMessage, &rawJSON, &startedAt, &completedAt, &createdAt); err != nil {
		return nil, err
	}
	if entryID.Valid {
		attempt.EntryID = entryID.String
	}
	if articleID.Valid {
		attempt.ArticleID = articleID.String
	}
	if errorCode.Valid {
		attempt.ErrorCode = errorCode.String
	}
	if errorMessage.Valid {
		attempt.ErrorMessage = errorMessage.String
	}
	if startedAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, startedAt.String)
		if err != nil {
			return nil, fmt.Errorf("decode collector attempt started_at: %w", err)
		}
		attempt.StartedAt = &parsed
	}
	if completedAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, completedAt.String)
		if err != nil {
			return nil, fmt.Errorf("decode collector attempt completed_at: %w", err)
		}
		attempt.CompletedAt = &parsed
	}
	attempt.RawJSON = []byte(rawJSON)
	created, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("decode collector attempt created_at: %w", err)
	}
	attempt.CreatedAt = created
	return &attempt, nil
}
