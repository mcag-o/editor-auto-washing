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

type BundleImportTx struct {
	tx *sql.Tx
}

var _ repo.BundleImportTxStarter = (*Provider)(nil)
var _ repo.BundleImportTx = (*BundleImportTx)(nil)

func (p *Provider) BeginBundleImport(ctx context.Context) (repo.BundleImportTx, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin bundle import tx: %w", err)
	}
	return &BundleImportTx{tx: tx}, nil
}

func (t *BundleImportTx) CreateWorkspaceArticle(ctx context.Context, record *domain.ArticleWorkspaceRecord) error {
	statusHistory, err := json.Marshal(record.StatusHistory)
	if err != nil {
		return fmt.Errorf("marshal status history: %w", err)
	}
	lifecycleHistory, err := json.Marshal(record.LifecycleHistory)
	if err != nil {
		return fmt.Errorf("marshal lifecycle history: %w", err)
	}
	source, err := json.Marshal(record.Source)
	if err != nil {
		return fmt.Errorf("marshal workspace source: %w", err)
	}
	metadata, err := json.Marshal(record.Metadata)
	if err != nil {
		return fmt.Errorf("marshal workspace metadata: %w", err)
	}
	_, err = t.tx.ExecContext(ctx, `INSERT INTO workspace_articles (id, title, summary, status, status_history, lifecycle_history, source, metadata, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, record.ID, record.Title, record.Summary, record.Status, string(statusHistory), string(lifecycleHistory), string(source), string(metadata), record.Notes, record.CreatedAt.Format(time.RFC3339), record.UpdatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert workspace article: %w", err)
	}
	return nil
}

func (t *BundleImportTx) RecordIngestion(ctx context.Context, rec *domain.IngestionRecord) error {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO ingestions (id, source_type, bundle_file, original_location, routed_path, payload, status, error_message, imported_items, created_articles, retried, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, rec.ID, rec.SourceType, rec.BundleFile, rec.OriginalLocation, rec.RoutedPath, string(rec.Payload), rec.Status, rec.ErrorMessage, rec.ImportedItems, rec.CreatedArticles, boolToInt(rec.Retried), rec.CreatedAt.Format(time.RFC3339), rec.UpdatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert ingestion: %w", err)
	}
	return nil
}

func (t *BundleImportTx) Commit() error {
	return t.tx.Commit()
}

func (t *BundleImportTx) Rollback() error {
	return t.tx.Rollback()
}
