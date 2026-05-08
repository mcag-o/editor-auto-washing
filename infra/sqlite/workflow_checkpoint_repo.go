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

var _ repo.WorkflowCheckpointRepo = (*workflowCheckpointRepo)(nil)

type workflowCheckpointRepo struct{ db *sql.DB }

func (r *workflowCheckpointRepo) Create(ctx context.Context, checkpoint *domain.WorkflowCheckpoint) error {
	metadataJSON, err := json.Marshal(checkpoint.Metadata)
	if err != nil {
		return fmt.Errorf("marshal workflow checkpoint metadata: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO workflow_checkpoints (id, workflow_run_id, node_execution_id, node_id, state, resumable, resume_token, payload_json, failure_class, created_at, consumed_at, metadata_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, checkpoint.ID, checkpoint.WorkflowRunID, checkpoint.NodeExecutionID, checkpoint.NodeID, checkpoint.State, boolToInt(checkpoint.Resumable), checkpoint.ResumeToken, checkpoint.PayloadJSON, checkpoint.FailureClass, checkpoint.CreatedAt.Format(time.RFC3339Nano), nullableTime(checkpoint.ConsumedAt), string(metadataJSON))
	if err != nil {
		return fmt.Errorf("insert workflow checkpoint: %w", err)
	}
	return nil
}

func (r *workflowCheckpointRepo) Update(ctx context.Context, checkpoint *domain.WorkflowCheckpoint) error {
	metadataJSON, err := json.Marshal(checkpoint.Metadata)
	if err != nil {
		return fmt.Errorf("marshal workflow checkpoint metadata: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `UPDATE workflow_checkpoints SET workflow_run_id = ?, node_execution_id = ?, node_id = ?, state = ?, resumable = ?, resume_token = ?, payload_json = ?, failure_class = ?, created_at = ?, consumed_at = ?, metadata_json = ? WHERE id = ?`, checkpoint.WorkflowRunID, checkpoint.NodeExecutionID, checkpoint.NodeID, checkpoint.State, boolToInt(checkpoint.Resumable), checkpoint.ResumeToken, checkpoint.PayloadJSON, checkpoint.FailureClass, checkpoint.CreatedAt.Format(time.RFC3339Nano), nullableTime(checkpoint.ConsumedAt), string(metadataJSON), checkpoint.ID)
	if err != nil {
		return fmt.Errorf("update workflow checkpoint: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check update workflow checkpoint result: %w", err)
	}
	if rows == 0 {
		return domain.NewNotFoundErr("workflow_checkpoint", checkpoint.ID)
	}
	return nil
}

func (r *workflowCheckpointRepo) DeleteByWorkflowRunID(ctx context.Context, workflowRunID string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM workflow_checkpoints WHERE workflow_run_id = ?`, workflowRunID); err != nil {
		return fmt.Errorf("delete workflow checkpoints by workflow run id: %w", err)
	}
	return nil
}

func (r *workflowCheckpointRepo) ListByWorkflowRunID(ctx context.Context, workflowRunID string) ([]domain.WorkflowCheckpoint, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, workflow_run_id, node_execution_id, node_id, state, resumable, resume_token, payload_json, failure_class, created_at, consumed_at, metadata_json FROM workflow_checkpoints WHERE workflow_run_id = ? ORDER BY created_at ASC, id ASC`, workflowRunID)
	if err != nil {
		return nil, fmt.Errorf("query workflow checkpoints: %w", err)
	}
	defer rows.Close()
	items := []domain.WorkflowCheckpoint{}
	for rows.Next() {
		checkpoint, scanErr := scanWorkflowCheckpoint(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *checkpoint)
	}
	return items, rows.Err()
}

type workflowCheckpointScanner interface{ Scan(dest ...any) error }

func scanWorkflowCheckpoint(row workflowCheckpointScanner) (*domain.WorkflowCheckpoint, error) {
	var checkpoint domain.WorkflowCheckpoint
	var consumedAt sql.NullString
	var createdAt, metadataJSON string
	var resumable int
	if err := row.Scan(&checkpoint.ID, &checkpoint.WorkflowRunID, &checkpoint.NodeExecutionID, &checkpoint.NodeID, &checkpoint.State, &resumable, &checkpoint.ResumeToken, &checkpoint.PayloadJSON, &checkpoint.FailureClass, &createdAt, &consumedAt, &metadataJSON); err != nil {
		return nil, err
	}
	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("decode workflow checkpoint created_at: %w", err)
	}
	checkpoint.CreatedAt = parsedCreatedAt
	if consumedAt.Valid {
		parsedConsumedAt, err := time.Parse(time.RFC3339Nano, consumedAt.String)
		if err != nil {
			return nil, fmt.Errorf("decode workflow checkpoint consumed_at: %w", err)
		}
		checkpoint.ConsumedAt = &parsedConsumedAt
	}
	if err := json.Unmarshal([]byte(metadataJSON), &checkpoint.Metadata); err != nil {
		return nil, fmt.Errorf("decode workflow checkpoint metadata: %w", err)
	}
	if checkpoint.Metadata == nil {
		checkpoint.Metadata = map[string]any{}
	}
	checkpoint.Resumable = resumable == 1
	return &checkpoint, nil
}
