package sqlite

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"database/sql"
	"fmt"
	"time"
)

var _ repo.CollectorSchedulerStateRepo = (*collectorSchedulerStateRepo)(nil)

type collectorSchedulerStateRepo struct{ db *sql.DB }

func (r *collectorSchedulerStateRepo) Upsert(ctx context.Context, state *domain.CollectorSchedulerState) error {
	state.UpdatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `INSERT INTO collector_scheduler_state (name, status, last_run_id, last_heartbeat, last_run_at, next_run_at, error_message, metadata_json, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(name) DO UPDATE SET status = excluded.status, last_run_id = excluded.last_run_id, last_heartbeat = excluded.last_heartbeat, last_run_at = excluded.last_run_at, next_run_at = excluded.next_run_at, error_message = excluded.error_message, metadata_json = excluded.metadata_json, updated_at = excluded.updated_at`, state.Name, state.Status, nullableString(emptyToNil(state.LastRunID)), nullableTime(state.LastHeartbeat), nullableTime(state.LastRunAt), nullableTime(state.NextRunAt), nullableString(emptyToNil(state.ErrorMessage)), string(state.MetadataJSON), state.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("upsert collector scheduler state: %w", err)
	}
	return nil
}

func (r *collectorSchedulerStateRepo) GetByName(ctx context.Context, name string) (*domain.CollectorSchedulerState, error) {
	row := r.db.QueryRowContext(ctx, `SELECT name, status, last_run_id, last_heartbeat, last_run_at, next_run_at, error_message, metadata_json, updated_at FROM collector_scheduler_state WHERE name = ?`, name)
	state, err := scanCollectorSchedulerState(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("collector_scheduler_state", name)
		}
		return nil, err
	}
	return state, nil
}

type collectorSchedulerScanner interface{ Scan(dest ...any) error }

func scanCollectorSchedulerState(row collectorSchedulerScanner) (*domain.CollectorSchedulerState, error) {
	var state domain.CollectorSchedulerState
	var lastRunID, lastHeartbeat, lastRunAt, nextRunAt, errorMessage sql.NullString
	var metadataJSON, updatedAt string
	if err := row.Scan(&state.Name, &state.Status, &lastRunID, &lastHeartbeat, &lastRunAt, &nextRunAt, &errorMessage, &metadataJSON, &updatedAt); err != nil {
		return nil, err
	}
	if lastRunID.Valid {
		state.LastRunID = lastRunID.String
	}
	if errorMessage.Valid {
		state.ErrorMessage = errorMessage.String
	}
	if lastHeartbeat.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, lastHeartbeat.String)
		if err != nil {
			return nil, fmt.Errorf("decode collector scheduler last_heartbeat: %w", err)
		}
		state.LastHeartbeat = &parsed
	}
	if lastRunAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, lastRunAt.String)
		if err != nil {
			return nil, fmt.Errorf("decode collector scheduler last_run_at: %w", err)
		}
		state.LastRunAt = &parsed
	}
	if nextRunAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, nextRunAt.String)
		if err != nil {
			return nil, fmt.Errorf("decode collector scheduler next_run_at: %w", err)
		}
		state.NextRunAt = &parsed
	}
	state.MetadataJSON = []byte(metadataJSON)
	updated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("decode collector scheduler updated_at: %w", err)
	}
	state.UpdatedAt = updated
	return &state, nil
}
