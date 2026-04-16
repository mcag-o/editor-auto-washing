package sqlite

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

var _ repo.RSSItemRepo = (*rssItemRepo)(nil)

type rssItemRepo struct{ db *sql.DB }

func (r *rssItemRepo) Create(ctx context.Context, item *domain.RSSItemRecord) error {
	if err := item.Validate(); err != nil {
		return err
	}
	metadataJSON, err := json.Marshal(item.Metadata)
	if err != nil {
		return fmt.Errorf("marshal rss item metadata: %w", err)
	}
	rawPayloadJSON := item.RawPayloadJSON
	if len(rawPayloadJSON) == 0 {
		rawPayloadJSON = []byte(`{}`)
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO rss_items (id, subscription_id, pull_run_id, guid, link, content_hash, title, status, published_at, imported_at, workspace_article_id, metadata_json, raw_payload_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, item.ID, item.SubscriptionID, item.PullRunID, item.GUID, item.Link, item.ContentHash, item.Title, item.Status, nullableTimeNano(item.PublishedAt), nullableTimeNano(item.ImportedAt), nullableString(emptyToNil(item.WorkspaceArticleID)), string(metadataJSON), string(rawPayloadJSON), item.CreatedAt.Format(time.RFC3339Nano), item.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("insert rss item: %w", err)
	}
	return nil
}

func (r *rssItemRepo) Update(ctx context.Context, item *domain.RSSItemRecord) error {
	if err := item.Validate(); err != nil {
		return err
	}
	metadataJSON, err := json.Marshal(item.Metadata)
	if err != nil {
		return fmt.Errorf("marshal rss item metadata: %w", err)
	}
	rawPayloadJSON := item.RawPayloadJSON
	if len(rawPayloadJSON) == 0 {
		rawPayloadJSON = []byte(`{}`)
	}
	result, err := r.db.ExecContext(ctx, `UPDATE rss_items SET subscription_id = ?, pull_run_id = ?, guid = ?, link = ?, content_hash = ?, title = ?, status = ?, published_at = ?, imported_at = ?, workspace_article_id = ?, metadata_json = ?, raw_payload_json = ?, created_at = ?, updated_at = ? WHERE id = ?`, item.SubscriptionID, item.PullRunID, item.GUID, item.Link, item.ContentHash, item.Title, item.Status, nullableTimeNano(item.PublishedAt), nullableTimeNano(item.ImportedAt), nullableString(emptyToNil(item.WorkspaceArticleID)), string(metadataJSON), string(rawPayloadJSON), item.CreatedAt.Format(time.RFC3339Nano), item.UpdatedAt.Format(time.RFC3339Nano), item.ID)
	if err != nil {
		return fmt.Errorf("update rss item: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check update rss item result: %w", err)
	}
	if rows == 0 {
		return domain.NewNotFoundErr("rss_item", item.ID)
	}
	return nil
}

func (r *rssItemRepo) FindDuplicate(ctx context.Context, key domain.RSSDuplicateKey) (*domain.RSSItemRecord, error) {
	if err := key.Validate(); err != nil {
		return nil, err
	}
	clauses := make([]string, 0, 3)
	args := []any{key.SubscriptionID}
	if guid := strings.TrimSpace(key.GUID); guid != "" {
		clauses = append(clauses, "guid = ?")
		args = append(args, guid)
	}
	if link := strings.TrimSpace(key.Link); link != "" {
		clauses = append(clauses, "link = ?")
		args = append(args, link)
	}
	if contentHash := strings.TrimSpace(key.ContentHash); contentHash != "" {
		clauses = append(clauses, "content_hash = ?")
		args = append(args, contentHash)
	}
	query := fmt.Sprintf(`SELECT id, subscription_id, pull_run_id, guid, link, content_hash, title, status, published_at, imported_at, workspace_article_id, metadata_json, raw_payload_json, created_at, updated_at FROM rss_items WHERE subscription_id = ? AND (%s) ORDER BY created_at DESC, id DESC LIMIT 1`, strings.Join(clauses, " OR "))
	row := r.db.QueryRowContext(ctx, query, args...)
	item, err := scanRSSItem(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func (r *rssItemRepo) GetByID(ctx context.Context, id string) (*domain.RSSItemRecord, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, subscription_id, pull_run_id, guid, link, content_hash, title, status, published_at, imported_at, workspace_article_id, metadata_json, raw_payload_json, created_at, updated_at FROM rss_items WHERE id = ?`, id)
	item, err := scanRSSItem(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("rss_item", id)
		}
		return nil, err
	}
	return item, nil
}

func (r *rssItemRepo) List(ctx context.Context, limit int) ([]domain.RSSItemRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, subscription_id, pull_run_id, guid, link, content_hash, title, status, published_at, imported_at, workspace_article_id, metadata_json, raw_payload_json, created_at, updated_at FROM rss_items ORDER BY created_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query rss items: %w", err)
	}
	defer rows.Close()
	var items []domain.RSSItemRecord
	for rows.Next() {
		item, scanErr := scanRSSItem(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

type rssItemScanner interface{ Scan(dest ...any) error }

func scanRSSItem(row rssItemScanner) (*domain.RSSItemRecord, error) {
	var item domain.RSSItemRecord
	var workspaceArticleID sql.NullString
	var publishedAt, importedAt sql.NullString
	var metadataJSON, rawPayloadJSON, createdAt, updatedAt string
	if err := row.Scan(&item.ID, &item.SubscriptionID, &item.PullRunID, &item.GUID, &item.Link, &item.ContentHash, &item.Title, &item.Status, &publishedAt, &importedAt, &workspaceArticleID, &metadataJSON, &rawPayloadJSON, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	if publishedAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, publishedAt.String)
		if err != nil {
			return nil, fmt.Errorf("decode rss item published_at: %w", err)
		}
		item.PublishedAt = &parsed
	}
	if importedAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, importedAt.String)
		if err != nil {
			return nil, fmt.Errorf("decode rss item imported_at: %w", err)
		}
		item.ImportedAt = &parsed
	}
	if workspaceArticleID.Valid {
		item.WorkspaceArticleID = workspaceArticleID.String
	}
	if err := json.Unmarshal([]byte(metadataJSON), &item.Metadata); err != nil {
		return nil, fmt.Errorf("decode rss item metadata: %w", err)
	}
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}
	item.RawPayloadJSON = []byte(rawPayloadJSON)
	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("decode rss item created_at: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("decode rss item updated_at: %w", err)
	}
	item.CreatedAt = parsedCreatedAt
	item.UpdatedAt = parsedUpdatedAt
	return &item, nil
}
