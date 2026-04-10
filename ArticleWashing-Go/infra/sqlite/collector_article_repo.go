package sqlite

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"database/sql"
	"fmt"
	"time"
)

var _ repo.CollectorArticleRepo = (*collectorArticleRepo)(nil)

type collectorArticleRepo struct{ db *sql.DB }

func (r *collectorArticleRepo) Create(ctx context.Context, article *domain.CollectorArticle) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO collector_articles (id, entry_id, run_id, source_id, external_id, canonical_url, title, body, summary, author, bridge_status, workspace_id, published_at, raw_json, normalized_json, metadata_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, article.ID, article.EntryID, article.RunID, article.SourceID, article.ExternalID, article.CanonicalURL, article.Title, article.Body, article.Summary, article.Author, article.BridgeStatus, nullableString(emptyToNil(article.WorkspaceID)), nullableTime(article.PublishedAt), string(article.RawJSON), string(article.NormalizedJSON), string(article.MetadataJSON), article.CreatedAt.Format(time.RFC3339Nano), article.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("insert collector article: %w", err)
	}
	return nil
}

func (r *collectorArticleRepo) GetByID(ctx context.Context, id string) (*domain.CollectorArticle, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, entry_id, run_id, source_id, external_id, canonical_url, title, body, summary, author, bridge_status, workspace_id, published_at, raw_json, normalized_json, metadata_json, created_at, updated_at FROM collector_articles WHERE id = ?`, id)
	article, err := scanCollectorArticle(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("collector_article", id)
		}
		return nil, err
	}
	return article, nil
}

func (r *collectorArticleRepo) GetByEntryID(ctx context.Context, entryID string) (*domain.CollectorArticle, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, entry_id, run_id, source_id, external_id, canonical_url, title, body, summary, author, bridge_status, workspace_id, published_at, raw_json, normalized_json, metadata_json, created_at, updated_at FROM collector_articles WHERE entry_id = ? ORDER BY created_at ASC LIMIT 1`, entryID)
	article, err := scanCollectorArticle(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("collector_article", entryID)
		}
		return nil, err
	}
	return article, nil
}

func (r *collectorArticleRepo) Update(ctx context.Context, article *domain.CollectorArticle) error {
	_, err := r.db.ExecContext(ctx, `UPDATE collector_articles SET entry_id = ?, run_id = ?, source_id = ?, external_id = ?, canonical_url = ?, title = ?, body = ?, summary = ?, author = ?, bridge_status = ?, workspace_id = ?, published_at = ?, raw_json = ?, normalized_json = ?, metadata_json = ?, updated_at = ? WHERE id = ?`, article.EntryID, article.RunID, article.SourceID, article.ExternalID, article.CanonicalURL, article.Title, article.Body, article.Summary, article.Author, article.BridgeStatus, nullableString(emptyToNil(article.WorkspaceID)), nullableTime(article.PublishedAt), string(article.RawJSON), string(article.NormalizedJSON), string(article.MetadataJSON), article.UpdatedAt.Format(time.RFC3339Nano), article.ID)
	if err != nil {
		return fmt.Errorf("update collector article: %w", err)
	}
	return nil
}

func (r *collectorArticleRepo) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM collector_articles WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete collector article: %w", err)
	}
	rows, err := result.RowsAffected()
	if err == nil && rows == 0 {
		return domain.NewNotFoundErr("collector_article", id)
	}
	return nil
}

type collectorArticleScanner interface{ Scan(dest ...any) error }

func scanCollectorArticle(row collectorArticleScanner) (*domain.CollectorArticle, error) {
	var article domain.CollectorArticle
	var workspaceID, publishedAt sql.NullString
	var rawJSON, normalizedJSON, metadataJSON, createdAt, updatedAt string
	if err := row.Scan(&article.ID, &article.EntryID, &article.RunID, &article.SourceID, &article.ExternalID, &article.CanonicalURL, &article.Title, &article.Body, &article.Summary, &article.Author, &article.BridgeStatus, &workspaceID, &publishedAt, &rawJSON, &normalizedJSON, &metadataJSON, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	if workspaceID.Valid {
		article.WorkspaceID = workspaceID.String
	}
	if publishedAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, publishedAt.String)
		if err != nil {
			return nil, fmt.Errorf("decode collector article published_at: %w", err)
		}
		article.PublishedAt = &parsed
	}
	article.RawJSON = []byte(rawJSON)
	article.NormalizedJSON = []byte(normalizedJSON)
	article.MetadataJSON = []byte(metadataJSON)
	created, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("decode collector article created_at: %w", err)
	}
	updated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("decode collector article updated_at: %w", err)
	}
	article.CreatedAt = created
	article.UpdatedAt = updated
	return &article, nil
}
