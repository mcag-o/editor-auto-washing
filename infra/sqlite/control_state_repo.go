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

const controlStateSingletonKey = 1

var _ repo.SystemControlStateRepo = (*controlStateRepo)(nil)

type controlStateRepo struct{ db *sql.DB }

func (r *controlStateRepo) Get(ctx context.Context) (*domain.SystemControlState, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, state, reason, metadata_json, updated_by, requested_at, updated_at FROM system_control_state WHERE singleton_key = ?`, controlStateSingletonKey)
	state, err := scanControlState(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("system_control_state", "singleton")
		}
		return nil, err
	}
	return state, nil
}

func (r *controlStateRepo) Upsert(ctx context.Context, state *domain.SystemControlState) error {
	if err := state.Validate(); err != nil {
		return err
	}
	existingID, existingUpdatedAt, err := r.lookupIdentity(ctx)
	if err != nil {
		return err
	}
	if existingID != "" {
		state.ID = existingID
	}
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = time.Now().UTC()
	}
	metadataJSON, err := json.Marshal(state.Metadata)
	if err != nil {
		return fmt.Errorf("marshal system control state metadata: %w", err)
	}
	createdUpdatedAt := state.UpdatedAt
	if !existingUpdatedAt.IsZero() {
		createdUpdatedAt = existingUpdatedAt
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO system_control_state (singleton_key, id, state, reason, metadata_json, updated_by, requested_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(singleton_key) DO UPDATE SET
			id = excluded.id,
			state = excluded.state,
			reason = excluded.reason,
			metadata_json = excluded.metadata_json,
			updated_by = excluded.updated_by,
			requested_at = excluded.requested_at,
			updated_at = excluded.updated_at
	`, controlStateSingletonKey, state.ID, state.State, state.Reason, string(metadataJSON), state.UpdatedBy, nullableTimeNano(state.RequestedAt), createdUpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("upsert system control state: %w", err)
	}
	state.UpdatedAt = createdUpdatedAt
	return nil
}

func (r *controlStateRepo) lookupIdentity(ctx context.Context) (string, time.Time, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, updated_at FROM system_control_state WHERE singleton_key = ?`, controlStateSingletonKey)
	var id string
	var updatedAt string
	if err := row.Scan(&id, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return "", time.Time{}, nil
		}
		return "", time.Time{}, fmt.Errorf("lookup system control state identity: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("decode system control state updated_at: %w", err)
	}
	return id, parsedUpdatedAt, nil
}

type controlStateScanner interface{ Scan(dest ...any) error }

func scanControlState(row controlStateScanner) (*domain.SystemControlState, error) {
	var state domain.SystemControlState
	var metadataJSON string
	var requestedAt sql.NullString
	var updatedAt string
	if err := row.Scan(&state.ID, &state.State, &state.Reason, &metadataJSON, &state.UpdatedBy, &requestedAt, &updatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(metadataJSON), &state.Metadata); err != nil {
		return nil, fmt.Errorf("decode system control state metadata: %w", err)
	}
	if state.Metadata == nil {
		state.Metadata = map[string]any{}
	}
	if requestedAt.Valid {
		parsedRequestedAt, err := time.Parse(time.RFC3339Nano, requestedAt.String)
		if err != nil {
			return nil, fmt.Errorf("decode system control state requested_at: %w", err)
		}
		state.RequestedAt = &parsedRequestedAt
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("decode system control state updated_at: %w", err)
	}
	state.UpdatedAt = parsedUpdatedAt
	return &state, nil
}
