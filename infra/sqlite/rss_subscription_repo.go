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

var _ repo.RSSSubscriptionRepo = (*rssSubscriptionRepo)(nil)

type rssSubscriptionRepo struct{ db *sql.DB }

func (r *rssSubscriptionRepo) Create(ctx context.Context, subscription *domain.RSSSubscription) error {
	if err := subscription.Validate(); err != nil {
		return err
	}
	metadataJSON, err := json.Marshal(subscription.Metadata)
	if err != nil {
		return fmt.Errorf("marshal rss subscription metadata: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO rss_subscriptions (id, name, feed_url, target_type, source_profile, rewrite_profile_version, enabled, poll_interval_sec, last_pulled_at, metadata_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, subscription.ID, subscription.Name, subscription.FeedURL, subscription.TargetType, subscription.SourceProfile, subscription.RewriteProfileVersion, boolToInt(subscription.Enabled), subscription.PollIntervalSec, nullableTimeNano(subscription.LastPulledAt), string(metadataJSON), subscription.CreatedAt.Format(time.RFC3339Nano), subscription.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("insert rss subscription: %w", err)
	}
	return nil
}

func (r *rssSubscriptionRepo) Update(ctx context.Context, subscription *domain.RSSSubscription) error {
	if err := subscription.Validate(); err != nil {
		return err
	}
	metadataJSON, err := json.Marshal(subscription.Metadata)
	if err != nil {
		return fmt.Errorf("marshal rss subscription metadata: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `UPDATE rss_subscriptions SET name = ?, feed_url = ?, target_type = ?, source_profile = ?, rewrite_profile_version = ?, enabled = ?, poll_interval_sec = ?, last_pulled_at = ?, metadata_json = ?, created_at = ?, updated_at = ? WHERE id = ?`, subscription.Name, subscription.FeedURL, subscription.TargetType, subscription.SourceProfile, subscription.RewriteProfileVersion, boolToInt(subscription.Enabled), subscription.PollIntervalSec, nullableTimeNano(subscription.LastPulledAt), string(metadataJSON), subscription.CreatedAt.Format(time.RFC3339Nano), subscription.UpdatedAt.Format(time.RFC3339Nano), subscription.ID)
	if err != nil {
		return fmt.Errorf("update rss subscription: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check update rss subscription result: %w", err)
	}
	if rows == 0 {
		return domain.NewNotFoundErr("rss_subscription", subscription.ID)
	}
	return nil
}

func (r *rssSubscriptionRepo) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM rss_subscriptions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete rss subscription: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check delete rss subscription result: %w", err)
	}
	if rows == 0 {
		return domain.NewNotFoundErr("rss_subscription", id)
	}
	return nil
}

func (r *rssSubscriptionRepo) GetByID(ctx context.Context, id string) (*domain.RSSSubscription, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, name, feed_url, target_type, source_profile, rewrite_profile_version, enabled, poll_interval_sec, last_pulled_at, metadata_json, created_at, updated_at FROM rss_subscriptions WHERE id = ?`, id)
	subscription, err := scanRSSSubscription(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("rss_subscription", id)
		}
		return nil, err
	}
	return subscription, nil
}

func (r *rssSubscriptionRepo) List(ctx context.Context) ([]domain.RSSSubscription, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, feed_url, target_type, source_profile, rewrite_profile_version, enabled, poll_interval_sec, last_pulled_at, metadata_json, created_at, updated_at FROM rss_subscriptions ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("query rss subscriptions: %w", err)
	}
	defer rows.Close()
	var items []domain.RSSSubscription
	for rows.Next() {
		subscription, scanErr := scanRSSSubscription(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *subscription)
	}
	return items, rows.Err()
}

type rssSubscriptionScanner interface{ Scan(dest ...any) error }

func scanRSSSubscription(row rssSubscriptionScanner) (*domain.RSSSubscription, error) {
	var subscription domain.RSSSubscription
	var enabled int
	var lastPulledAt sql.NullString
	var metadataJSON, createdAt, updatedAt string
	if err := row.Scan(&subscription.ID, &subscription.Name, &subscription.FeedURL, &subscription.TargetType, &subscription.SourceProfile, &subscription.RewriteProfileVersion, &enabled, &subscription.PollIntervalSec, &lastPulledAt, &metadataJSON, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	subscription.Enabled = enabled == 1
	if lastPulledAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, lastPulledAt.String)
		if err != nil {
			return nil, fmt.Errorf("decode rss subscription last_pulled_at: %w", err)
		}
		subscription.LastPulledAt = &parsed
	}
	if err := json.Unmarshal([]byte(metadataJSON), &subscription.Metadata); err != nil {
		return nil, fmt.Errorf("decode rss subscription metadata: %w", err)
	}
	if subscription.Metadata == nil {
		subscription.Metadata = map[string]any{}
	}
	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("decode rss subscription created_at: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("decode rss subscription updated_at: %w", err)
	}
	subscription.CreatedAt = parsedCreatedAt
	subscription.UpdatedAt = parsedUpdatedAt
	return &subscription, nil
}
