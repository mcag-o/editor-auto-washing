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

var _ repo.AuditLogRepo = (*auditLogRepo)(nil)

type auditLogRepo struct{ db *sql.DB }

func (r *auditLogRepo) Create(ctx context.Context, log *domain.AuditLog) error {
	if err := log.Validate(); err != nil {
		return err
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now().UTC()
	}
	metadataJSON, err := json.Marshal(log.Metadata)
	if err != nil {
		return fmt.Errorf("marshal audit log metadata: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO audit_logs (id, actor, action, resource, resource_id, result, message, metadata_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, log.ID, log.Actor, log.Action, log.Resource, log.ResourceID, log.Result, log.Message, string(metadataJSON), log.CreatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

func (r *auditLogRepo) GetByID(ctx context.Context, id string) (*domain.AuditLog, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, actor, action, resource, resource_id, result, message, metadata_json, created_at FROM audit_logs WHERE id = ?`, id)
	log, err := scanAuditLog(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("audit_log", id)
		}
		return nil, err
	}
	return log, nil
}

func (r *auditLogRepo) List(ctx context.Context, limit int) ([]domain.AuditLog, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, actor, action, resource, resource_id, result, message, metadata_json, created_at FROM audit_logs ORDER BY created_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query audit logs: %w", err)
	}
	defer rows.Close()
	var logs []domain.AuditLog
	for rows.Next() {
		log, scanErr := scanAuditLog(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		logs = append(logs, *log)
	}
	return logs, rows.Err()
}

func (r *auditLogRepo) ListByQuery(ctx context.Context, query repo.AuditLogQuery) ([]domain.AuditLog, error) {
	where := make([]string, 0, 3)
	args := make([]any, 0, 3)
	if query.WorkflowRunID != "" {
		where = append(where, `json_extract(metadata_json, '$.workflow_run_id') = ?`)
		args = append(args, query.WorkflowRunID)
	}
	if query.Resource != "" {
		where = append(where, `resource = ?`)
		args = append(args, query.Resource)
	}
	if query.ActionPrefix != "" {
		where = append(where, `action LIKE ?`)
		args = append(args, strings.TrimSpace(query.ActionPrefix)+"%")
	}
	if query.ResourceID != "" {
		where = append(where, `resource_id = ?`)
		args = append(args, query.ResourceID)
	}

	querySQL := `SELECT id, actor, action, resource, resource_id, result, message, metadata_json, created_at FROM audit_logs`
	if len(where) > 0 {
		querySQL += " WHERE " + strings.Join(where, " AND ")
	}
	querySQL += ` ORDER BY created_at DESC, id DESC`

	rows, err := r.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, fmt.Errorf("query audit logs by filter: %w", err)
	}
	defer rows.Close()

	var logs []domain.AuditLog
	for rows.Next() {
		log, scanErr := scanAuditLog(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		logs = append(logs, *log)
	}
	return logs, rows.Err()
}

type auditLogScanner interface{ Scan(dest ...any) error }

func scanAuditLog(row auditLogScanner) (*domain.AuditLog, error) {
	var log domain.AuditLog
	var metadataJSON string
	var createdAt string
	if err := row.Scan(&log.ID, &log.Actor, &log.Action, &log.Resource, &log.ResourceID, &log.Result, &log.Message, &metadataJSON, &createdAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(metadataJSON), &log.Metadata); err != nil {
		return nil, fmt.Errorf("decode audit log metadata: %w", err)
	}
	if log.Metadata == nil {
		log.Metadata = map[string]any{}
	}
	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("decode audit log created_at: %w", err)
	}
	log.CreatedAt = parsedCreatedAt
	return &log, nil
}
