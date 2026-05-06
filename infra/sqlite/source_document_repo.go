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

var _ repo.SourceDocumentRepo = (*sourceDocumentRepo)(nil)

type sourceDocumentRepo struct{ db *sql.DB }

func (r *sourceDocumentRepo) Create(ctx context.Context, doc *domain.SourceDocument) error {
	if err := doc.Validate(); err != nil {
		return err
	}
	metadataJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		return fmt.Errorf("marshal source document metadata: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO source_documents (id, source_type, original_filename, original_path, archived_path, file_type, title, body, summary, metadata_json, hash, imported_at, status, workspace_article_id, rewrite_run_id, claimed_by, claimed_at, processing_started_at, completed_at, error_summary) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, doc.ID, doc.SourceType, doc.OriginalFilename, doc.OriginalPath, doc.ArchivedPath, doc.FileType, doc.Title, doc.Body, doc.Summary, string(metadataJSON), doc.Hash, nullableTimeNano(doc.ImportedAt), doc.Status, doc.WorkspaceArticleID, doc.RewriteRunID, doc.ClaimedBy, nullableTimeNano(doc.ClaimedAt), nullableTimeNano(doc.ProcessingStartedAt), nullableTimeNano(doc.CompletedAt), doc.ErrorSummary)
	if err != nil {
		return fmt.Errorf("insert source document: %w", err)
	}
	return nil
}

func (r *sourceDocumentRepo) Update(ctx context.Context, doc *domain.SourceDocument) error {
	if err := doc.Validate(); err != nil {
		return err
	}
	metadataJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		return fmt.Errorf("marshal source document metadata: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `UPDATE source_documents SET source_type = ?, original_filename = ?, original_path = ?, archived_path = ?, file_type = ?, title = ?, body = ?, summary = ?, metadata_json = ?, hash = ?, imported_at = ?, status = ?, workspace_article_id = ?, rewrite_run_id = ?, claimed_by = ?, claimed_at = ?, processing_started_at = ?, completed_at = ?, error_summary = ? WHERE id = ?`, doc.SourceType, doc.OriginalFilename, doc.OriginalPath, doc.ArchivedPath, doc.FileType, doc.Title, doc.Body, doc.Summary, string(metadataJSON), doc.Hash, nullableTimeNano(doc.ImportedAt), doc.Status, doc.WorkspaceArticleID, doc.RewriteRunID, doc.ClaimedBy, nullableTimeNano(doc.ClaimedAt), nullableTimeNano(doc.ProcessingStartedAt), nullableTimeNano(doc.CompletedAt), doc.ErrorSummary, doc.ID)
	if err != nil {
		return fmt.Errorf("update source document: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check update source document result: %w", err)
	}
	if rows == 0 {
		return domain.NewNotFoundErr("source_document", doc.ID)
	}
	return nil
}

func (r *sourceDocumentRepo) UpdateIfStatus(ctx context.Context, doc *domain.SourceDocument, expectedStatuses ...string) error {
	if err := doc.Validate(); err != nil {
		return err
	}
	if len(expectedStatuses) == 0 {
		return domain.NewValidationErr("expected source document statuses are required", nil)
	}
	metadataJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		return fmt.Errorf("marshal source document metadata: %w", err)
	}
	args := []any{doc.SourceType, doc.OriginalFilename, doc.OriginalPath, doc.ArchivedPath, doc.FileType, doc.Title, doc.Body, doc.Summary, string(metadataJSON), doc.Hash, nullableTimeNano(doc.ImportedAt), doc.Status, doc.WorkspaceArticleID, doc.RewriteRunID, doc.ClaimedBy, nullableTimeNano(doc.ClaimedAt), nullableTimeNano(doc.ProcessingStartedAt), nullableTimeNano(doc.CompletedAt), doc.ErrorSummary, doc.ID}
	placeholders := make([]string, 0, len(expectedStatuses))
	for _, status := range expectedStatuses {
		placeholders = append(placeholders, "?")
		args = append(args, status)
	}
	query := fmt.Sprintf(`UPDATE source_documents SET source_type = ?, original_filename = ?, original_path = ?, archived_path = ?, file_type = ?, title = ?, body = ?, summary = ?, metadata_json = ?, hash = ?, imported_at = ?, status = ?, workspace_article_id = ?, rewrite_run_id = ?, claimed_by = ?, claimed_at = ?, processing_started_at = ?, completed_at = ?, error_summary = ? WHERE id = ? AND status IN (%s)`, strings.Join(placeholders, ", "))
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("guarded update source document: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check guarded update source document result: %w", err)
	}
	if rows == 0 {
		return domain.NewConflictErr(fmt.Sprintf("source document %s state changed", doc.ID))
	}
	return nil
}

func (r *sourceDocumentRepo) GetByID(ctx context.Context, id string) (*domain.SourceDocument, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, source_type, original_filename, original_path, archived_path, file_type, title, body, summary, metadata_json, hash, imported_at, status, workspace_article_id, rewrite_run_id, claimed_by, claimed_at, processing_started_at, completed_at, error_summary FROM source_documents WHERE id = ?`, id)
	doc, err := scanSourceDocument(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("source_document", id)
		}
		return nil, err
	}
	return doc, nil
}

func (r *sourceDocumentRepo) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM source_documents WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete source document: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check delete source document result: %w", err)
	}
	if rows == 0 {
		return domain.NewNotFoundErr("source_document", id)
	}
	return nil
}

func (r *sourceDocumentRepo) DeleteIfStatus(ctx context.Context, id string, expectedStatuses ...string) error {
	if len(expectedStatuses) == 0 {
		return domain.NewValidationErr("expected source document statuses are required", nil)
	}
	args := []any{id}
	placeholders := make([]string, 0, len(expectedStatuses))
	for _, status := range expectedStatuses {
		placeholders = append(placeholders, "?")
		args = append(args, status)
	}
	query := fmt.Sprintf(`DELETE FROM source_documents WHERE id = ? AND status IN (%s)`, strings.Join(placeholders, ", "))
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("guarded delete source document: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check guarded delete source document result: %w", err)
	}
	if rows == 0 {
		return domain.NewConflictErr(fmt.Sprintf("source document %s state changed", id))
	}
	return nil
}

func (r *sourceDocumentRepo) List(ctx context.Context, limit int) ([]domain.SourceDocument, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, source_type, original_filename, original_path, archived_path, file_type, title, body, summary, metadata_json, hash, imported_at, status, workspace_article_id, rewrite_run_id, claimed_by, claimed_at, processing_started_at, completed_at, error_summary FROM source_documents ORDER BY claimed_at DESC, imported_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query source documents: %w", err)
	}
	defer rows.Close()

	items := []domain.SourceDocument{}
	for rows.Next() {
		doc, err := scanSourceDocument(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *doc)
	}
	return items, rows.Err()
}

func (r *sourceDocumentRepo) FindByHash(ctx context.Context, hash string) (*domain.SourceDocument, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, source_type, original_filename, original_path, archived_path, file_type, title, body, summary, metadata_json, hash, imported_at, status, workspace_article_id, rewrite_run_id, claimed_by, claimed_at, processing_started_at, completed_at, error_summary FROM source_documents WHERE hash = ? ORDER BY imported_at DESC, id DESC LIMIT 1`, hash)
	doc, err := scanSourceDocument(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("source_document_hash", hash)
		}
		return nil, err
	}
	return doc, nil
}

func (r *sourceDocumentRepo) ClaimPending(ctx context.Context, limit int, claimedBy string, now time.Time) ([]domain.SourceDocument, error) {
	if limit <= 0 {
		return []domain.SourceDocument{}, nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin claim source documents tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `SELECT id FROM source_documents WHERE status = ? ORDER BY id ASC LIMIT ?`, domain.SourceDocumentStatusPending, limit)
	if err != nil {
		return nil, fmt.Errorf("query pending source documents: %w", err)
	}
	defer rows.Close()

	ids := make([]string, 0, limit)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan pending source document id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending source document ids: %w", err)
	}
	if len(ids) == 0 {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit empty claim source documents tx: %w", err)
		}
		return []domain.SourceDocument{}, nil
	}

	claimedAt := now.UTC().Format(time.RFC3339Nano)
	claimedIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		result, err := tx.ExecContext(ctx, `UPDATE source_documents SET status = ?, claimed_by = ?, claimed_at = ? WHERE id = ? AND status = ?`, domain.SourceDocumentStatusClaimed, claimedBy, claimedAt, id, domain.SourceDocumentStatusPending)
		if err != nil {
			return nil, fmt.Errorf("claim source document %s: %w", id, err)
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("check claim source document %s result: %w", id, err)
		}
		if rowsAffected == 1 {
			claimedIDs = append(claimedIDs, id)
		}
	}

	claimed := make([]domain.SourceDocument, 0, len(claimedIDs))
	for _, id := range claimedIDs {
		row := tx.QueryRowContext(ctx, `SELECT id, source_type, original_filename, original_path, archived_path, file_type, title, body, summary, metadata_json, hash, imported_at, status, workspace_article_id, rewrite_run_id, claimed_by, claimed_at, processing_started_at, completed_at, error_summary FROM source_documents WHERE id = ? AND claimed_by = ?`, id, claimedBy)
		doc, err := scanSourceDocument(row)
		if err != nil {
			if err == sql.ErrNoRows {
				continue
			}
			return nil, fmt.Errorf("load claimed source document %s: %w", id, err)
		}
		claimed = append(claimed, *doc)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit claim source documents tx: %w", err)
	}
	return claimed, nil
}

func (r *sourceDocumentRepo) ListByStatus(ctx context.Context, status string, limit int) ([]domain.SourceDocument, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, source_type, original_filename, original_path, archived_path, file_type, title, body, summary, metadata_json, hash, imported_at, status, workspace_article_id, rewrite_run_id, claimed_by, claimed_at, processing_started_at, completed_at, error_summary FROM source_documents WHERE status = ? ORDER BY claimed_at DESC, imported_at DESC, id DESC LIMIT ?`, status, limit)
	if err != nil {
		return nil, fmt.Errorf("query source documents by status: %w", err)
	}
	defer rows.Close()

	items := []domain.SourceDocument{}
	for rows.Next() {
		doc, err := scanSourceDocument(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *doc)
	}
	return items, rows.Err()
}

type sourceDocumentScanner interface{ Scan(dest ...any) error }

func scanSourceDocument(row sourceDocumentScanner) (*domain.SourceDocument, error) {
	var doc domain.SourceDocument
	var metadataJSON string
	var importedAt, claimedAt, processingStartedAt, completedAt sql.NullString
	if err := row.Scan(&doc.ID, &doc.SourceType, &doc.OriginalFilename, &doc.OriginalPath, &doc.ArchivedPath, &doc.FileType, &doc.Title, &doc.Body, &doc.Summary, &metadataJSON, &doc.Hash, &importedAt, &doc.Status, &doc.WorkspaceArticleID, &doc.RewriteRunID, &doc.ClaimedBy, &claimedAt, &processingStartedAt, &completedAt, &doc.ErrorSummary); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(metadataJSON), &doc.Metadata); err != nil {
		return nil, fmt.Errorf("decode source document metadata: %w", err)
	}
	if doc.Metadata == nil {
		doc.Metadata = map[string]any{}
	}
	var err error
	if doc.ImportedAt, err = parseNullTime(importedAt, "imported_at"); err != nil {
		return nil, fmt.Errorf("decode source document %w", err)
	}
	if doc.ClaimedAt, err = parseNullTime(claimedAt, "claimed_at"); err != nil {
		return nil, fmt.Errorf("decode source document %w", err)
	}
	if doc.ProcessingStartedAt, err = parseNullTime(processingStartedAt, "processing_started_at"); err != nil {
		return nil, fmt.Errorf("decode source document %w", err)
	}
	if doc.CompletedAt, err = parseNullTime(completedAt, "completed_at"); err != nil {
		return nil, fmt.Errorf("decode source document %w", err)
	}
	return &doc, nil
}

func parseNullTime(value sql.NullString, field string) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", field, err)
	}
	return &parsed, nil
}
