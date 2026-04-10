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

var _ repo.CollectorSourceRepo = (*collectorSourceRepo)(nil)

type collectorSourceRepo struct{ db *sql.DB }

func (r *collectorSourceRepo) Create(ctx context.Context, source *domain.CollectorSource) error {
	metadata, err := json.Marshal(source.Metadata)
	if err != nil {
		return fmt.Errorf("marshal collector source metadata: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO collector_sources (id, display_name, enabled, schedule_enabled, interval_minutes, auth_mode, timeout_ms, headers_json, cookie_secret_ref, header_secret_ref, hotlist_limit, detail_fetch_enabled, concurrency, retry_policy_json, options_json, metadata_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, source.ID, source.DisplayName, boolToInt(source.Enabled), boolToInt(source.ScheduleEnabled), source.IntervalMinutes, source.AuthMode, source.TimeoutMS, string(source.HeadersJSON), source.CookieSecretRef, source.HeaderSecretRef, source.HotlistLimit, boolToInt(source.DetailFetchEnabled), source.Concurrency, string(source.RetryPolicyJSON), string(source.OptionsJSON), string(metadata), source.CreatedAt.Format(time.RFC3339Nano), source.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("insert collector source: %w", err)
	}
	return nil
}

func (r *collectorSourceRepo) GetByID(ctx context.Context, id string) (*domain.CollectorSource, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, display_name, enabled, schedule_enabled, interval_minutes, auth_mode, timeout_ms, headers_json, cookie_secret_ref, header_secret_ref, hotlist_limit, detail_fetch_enabled, concurrency, retry_policy_json, options_json, metadata_json, created_at, updated_at FROM collector_sources WHERE id = ?`, id)
	source, err := scanCollectorSource(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("collector_source", id)
		}
		return nil, err
	}
	return source, nil
}

func (r *collectorSourceRepo) Update(ctx context.Context, source *domain.CollectorSource) error {
	metadata, err := json.Marshal(source.Metadata)
	if err != nil {
		return fmt.Errorf("marshal collector source metadata: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `UPDATE collector_sources SET display_name = ?, enabled = ?, schedule_enabled = ?, interval_minutes = ?, auth_mode = ?, timeout_ms = ?, headers_json = ?, cookie_secret_ref = ?, header_secret_ref = ?, hotlist_limit = ?, detail_fetch_enabled = ?, concurrency = ?, retry_policy_json = ?, options_json = ?, metadata_json = ?, updated_at = ? WHERE id = ?`, source.DisplayName, boolToInt(source.Enabled), boolToInt(source.ScheduleEnabled), source.IntervalMinutes, source.AuthMode, source.TimeoutMS, string(source.HeadersJSON), source.CookieSecretRef, source.HeaderSecretRef, source.HotlistLimit, boolToInt(source.DetailFetchEnabled), source.Concurrency, string(source.RetryPolicyJSON), string(source.OptionsJSON), string(metadata), source.UpdatedAt.Format(time.RFC3339Nano), source.ID)
	if err != nil {
		return fmt.Errorf("update collector source: %w", err)
	}
	return nil
}

func (r *collectorSourceRepo) ListAll(ctx context.Context) ([]domain.CollectorSource, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, display_name, enabled, schedule_enabled, interval_minutes, auth_mode, timeout_ms, headers_json, cookie_secret_ref, header_secret_ref, hotlist_limit, detail_fetch_enabled, concurrency, retry_policy_json, options_json, metadata_json, created_at, updated_at FROM collector_sources ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query collector sources: %w", err)
	}
	defer rows.Close()
	var items []domain.CollectorSource
	for rows.Next() {
		source, scanErr := scanCollectorSource(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *source)
	}
	return items, rows.Err()
}

func (r *collectorSourceRepo) ListEnabled(ctx context.Context) ([]domain.CollectorSource, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, display_name, enabled, schedule_enabled, interval_minutes, auth_mode, timeout_ms, headers_json, cookie_secret_ref, header_secret_ref, hotlist_limit, detail_fetch_enabled, concurrency, retry_policy_json, options_json, metadata_json, created_at, updated_at FROM collector_sources WHERE enabled = 1 ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query enabled collector sources: %w", err)
	}
	defer rows.Close()
	var items []domain.CollectorSource
	for rows.Next() {
		source, scanErr := scanCollectorSource(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *source)
	}
	return items, rows.Err()
}

type collectorSourceScanner interface{ Scan(dest ...any) error }

func scanCollectorSource(row collectorSourceScanner) (*domain.CollectorSource, error) {
	var source domain.CollectorSource
	var enabled, scheduleEnabled, detailFetchEnabled int
	var headersJSON, retryPolicyJSON, optionsJSON, metadataJSON string
	var createdAt, updatedAt string
	if err := row.Scan(&source.ID, &source.DisplayName, &enabled, &scheduleEnabled, &source.IntervalMinutes, &source.AuthMode, &source.TimeoutMS, &headersJSON, &source.CookieSecretRef, &source.HeaderSecretRef, &source.HotlistLimit, &detailFetchEnabled, &source.Concurrency, &retryPolicyJSON, &optionsJSON, &metadataJSON, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	source.Enabled = enabled == 1
	source.ScheduleEnabled = scheduleEnabled == 1
	source.DetailFetchEnabled = detailFetchEnabled == 1
	source.HeadersJSON = []byte(headersJSON)
	source.RetryPolicyJSON = []byte(retryPolicyJSON)
	source.OptionsJSON = []byte(optionsJSON)
	if err := json.Unmarshal([]byte(metadataJSON), &source.Metadata); err != nil {
		return nil, fmt.Errorf("decode collector source metadata: %w", err)
	}
	if source.Metadata == nil {
		source.Metadata = map[string]any{}
	}
	created, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("decode collector source created_at: %w", err)
	}
	updated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("decode collector source updated_at: %w", err)
	}
	source.CreatedAt = created
	source.UpdatedAt = updated
	return &source, nil
}
