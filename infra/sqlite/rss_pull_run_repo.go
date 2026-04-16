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

var _ repo.RSSPullRunRepo = (*rssPullRunRepo)(nil)

type rssPullRunRepo struct{ db *sql.DB }

func (r *rssPullRunRepo) Create(ctx context.Context, run *domain.RSSPullRun) error {
	if err := run.Validate(); err != nil {
		return err
	}
	metadataJSON, err := json.Marshal(run.Metadata)
	if err != nil {
		return fmt.Errorf("marshal rss pull run metadata: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO rss_pull_runs (id, subscription_id, status, started_at, completed_at, error_summary, metadata_json) VALUES (?, ?, ?, ?, ?, ?, ?)`, run.ID, run.SubscriptionID, run.Status, run.StartedAt.Format(time.RFC3339Nano), nullableTimeNano(run.CompletedAt), run.ErrorSummary, string(metadataJSON))
	if err != nil {
		return fmt.Errorf("insert rss pull run: %w", err)
	}
	return nil
}

func (r *rssPullRunRepo) Update(ctx context.Context, run *domain.RSSPullRun) error {
	if err := run.Validate(); err != nil {
		return err
	}
	metadataJSON, err := json.Marshal(run.Metadata)
	if err != nil {
		return fmt.Errorf("marshal rss pull run metadata: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `UPDATE rss_pull_runs SET subscription_id = ?, status = ?, started_at = ?, completed_at = ?, error_summary = ?, metadata_json = ? WHERE id = ?`, run.SubscriptionID, run.Status, run.StartedAt.Format(time.RFC3339Nano), nullableTimeNano(run.CompletedAt), run.ErrorSummary, string(metadataJSON), run.ID)
	if err != nil {
		return fmt.Errorf("update rss pull run: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check update rss pull run result: %w", err)
	}
	if rows == 0 {
		return domain.NewNotFoundErr("rss_pull_run", run.ID)
	}
	return nil
}

func (r *rssPullRunRepo) GetByID(ctx context.Context, id string) (*domain.RSSPullRun, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, subscription_id, status, started_at, completed_at, error_summary, metadata_json FROM rss_pull_runs WHERE id = ?`, id)
	run, err := scanRSSPullRun(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("rss_pull_run", id)
		}
		return nil, err
	}
	return run, nil
}

func (r *rssPullRunRepo) List(ctx context.Context, limit int) ([]domain.RSSPullRun, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, subscription_id, status, started_at, completed_at, error_summary, metadata_json FROM rss_pull_runs ORDER BY started_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query rss pull runs: %w", err)
	}
	defer rows.Close()
	var items []domain.RSSPullRun
	for rows.Next() {
		run, scanErr := scanRSSPullRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *run)
	}
	return items, rows.Err()
}

type rssPullRunScanner interface{ Scan(dest ...any) error }

func scanRSSPullRun(row rssPullRunScanner) (*domain.RSSPullRun, error) {
	var run domain.RSSPullRun
	var completedAt sql.NullString
	var startedAt, metadataJSON string
	if err := row.Scan(&run.ID, &run.SubscriptionID, &run.Status, &startedAt, &completedAt, &run.ErrorSummary, &metadataJSON); err != nil {
		return nil, err
	}
	parsedStartedAt, err := time.Parse(time.RFC3339Nano, startedAt)
	if err != nil {
		return nil, fmt.Errorf("decode rss pull run started_at: %w", err)
	}
	run.StartedAt = parsedStartedAt
	if completedAt.Valid {
		parsedCompletedAt, err := time.Parse(time.RFC3339Nano, completedAt.String)
		if err != nil {
			return nil, fmt.Errorf("decode rss pull run completed_at: %w", err)
		}
		run.CompletedAt = &parsedCompletedAt
	}
	if err := json.Unmarshal([]byte(metadataJSON), &run.Metadata); err != nil {
		return nil, fmt.Errorf("decode rss pull run metadata: %w", err)
	}
	if run.Metadata == nil {
		run.Metadata = map[string]any{}
	}
	return &run, nil
}
