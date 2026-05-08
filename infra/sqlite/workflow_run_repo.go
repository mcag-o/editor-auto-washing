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

var _ repo.WorkflowRunRepo = (*workflowRunRepo)(nil)

type workflowRunRepo struct{ db *sql.DB }

func (r *workflowRunRepo) Create(ctx context.Context, run *domain.WorkflowRun) error {
	metadataJSON, err := json.Marshal(run.Metadata)
	if err != nil {
		return fmt.Errorf("marshal workflow run metadata: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO workflow_runs (id, workflow_id, workflow_version, workspace_article_id, status, current_node_id, started_at, completed_at, error_summary, final_failure_class, resumable, resume_from_checkpoint_id, metadata_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, run.ID, run.WorkflowID, run.WorkflowVersion, run.WorkspaceArticleID, run.Status, run.CurrentNodeID, run.StartedAt.Format(time.RFC3339Nano), nullableTime(run.CompletedAt), run.ErrorSummary, run.FinalFailureClass, boolToInt(run.Resumable), run.ResumeFromCheckpointID, string(metadataJSON))
	if err != nil {
		return fmt.Errorf("insert workflow run: %w", err)
	}
	return nil
}

func (r *workflowRunRepo) Update(ctx context.Context, run *domain.WorkflowRun) error {
	metadataJSON, err := json.Marshal(run.Metadata)
	if err != nil {
		return fmt.Errorf("marshal workflow run metadata: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `UPDATE workflow_runs SET workflow_id = ?, workflow_version = ?, workspace_article_id = ?, status = ?, current_node_id = ?, started_at = ?, completed_at = ?, error_summary = ?, final_failure_class = ?, resumable = ?, resume_from_checkpoint_id = ?, metadata_json = ? WHERE id = ?`, run.WorkflowID, run.WorkflowVersion, run.WorkspaceArticleID, run.Status, run.CurrentNodeID, run.StartedAt.Format(time.RFC3339Nano), nullableTime(run.CompletedAt), run.ErrorSummary, run.FinalFailureClass, boolToInt(run.Resumable), run.ResumeFromCheckpointID, string(metadataJSON), run.ID)
	if err != nil {
		return fmt.Errorf("update workflow run: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check update workflow run result: %w", err)
	}
	if rows == 0 {
		return domain.NewNotFoundErr("workflow_run", run.ID)
	}
	return nil
}

func (r *workflowRunRepo) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM workflow_runs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete workflow run: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check delete workflow run result: %w", err)
	}
	if rows == 0 {
		return domain.NewNotFoundErr("workflow_run", id)
	}
	return nil
}

func (r *workflowRunRepo) GetByID(ctx context.Context, id string) (*domain.WorkflowRun, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, workflow_id, workflow_version, workspace_article_id, status, current_node_id, started_at, completed_at, error_summary, final_failure_class, resumable, resume_from_checkpoint_id, metadata_json FROM workflow_runs WHERE id = ?`, id)
	run, err := scanWorkflowRun(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("workflow_run", id)
		}
		return nil, err
	}
	return run, nil
}

func (r *workflowRunRepo) List(ctx context.Context, limit int) ([]domain.WorkflowRun, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, workflow_id, workflow_version, workspace_article_id, status, current_node_id, started_at, completed_at, error_summary, final_failure_class, resumable, resume_from_checkpoint_id, metadata_json FROM workflow_runs ORDER BY started_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query workflow runs: %w", err)
	}
	defer rows.Close()
	items := []domain.WorkflowRun{}
	for rows.Next() {
		run, scanErr := scanWorkflowRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *run)
	}
	return items, rows.Err()
}

type workflowRunScanner interface{ Scan(dest ...any) error }

func scanWorkflowRun(row workflowRunScanner) (*domain.WorkflowRun, error) {
	var run domain.WorkflowRun
	var completedAt sql.NullString
	var startedAt, metadataJSON string
	var resumable int
	if err := row.Scan(&run.ID, &run.WorkflowID, &run.WorkflowVersion, &run.WorkspaceArticleID, &run.Status, &run.CurrentNodeID, &startedAt, &completedAt, &run.ErrorSummary, &run.FinalFailureClass, &resumable, &run.ResumeFromCheckpointID, &metadataJSON); err != nil {
		return nil, err
	}
	parsedStartedAt, err := time.Parse(time.RFC3339Nano, startedAt)
	if err != nil {
		return nil, fmt.Errorf("decode workflow run started_at: %w", err)
	}
	run.StartedAt = parsedStartedAt
	if completedAt.Valid {
		parsedCompletedAt, err := time.Parse(time.RFC3339Nano, completedAt.String)
		if err != nil {
			return nil, fmt.Errorf("decode workflow run completed_at: %w", err)
		}
		run.CompletedAt = &parsedCompletedAt
	}
	if err := json.Unmarshal([]byte(metadataJSON), &run.Metadata); err != nil {
		return nil, fmt.Errorf("decode workflow run metadata: %w", err)
	}
	if run.Metadata == nil {
		run.Metadata = map[string]any{}
	}
	run.Resumable = resumable == 1
	return &run, nil
}
