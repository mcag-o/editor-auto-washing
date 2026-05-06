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

var _ repo.RewriteStageRunRepo = (*rewriteStageRunRepo)(nil)

type rewriteStageRunRepo struct{ db *sql.DB }

func (r *rewriteStageRunRepo) Create(ctx context.Context, run *domain.RewriteStageRun) error {
	metadataJSON, err := json.Marshal(run.Metadata)
	if err != nil {
		return fmt.Errorf("marshal rewrite stage run metadata: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO rewrite_stage_runs (id, pipeline_run_id, stage_name, stage_type, prompt_ref, llm_profile_ref, status, attempt, input_json, output_json, error_summary, metadata_json, started_at, completed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, run.ID, run.PipelineRunID, run.StageName, run.StageType, run.PromptRef, run.LLMProfileRef, run.Status, run.Attempt, run.InputJSON, run.OutputJSON, run.ErrorSummary, string(metadataJSON), run.StartedAt.Format(time.RFC3339), nullableTime(run.CompletedAt))
	if err != nil {
		return fmt.Errorf("insert rewrite stage run: %w", err)
	}
	return nil
}

func (r *rewriteStageRunRepo) ListByPipelineRunID(ctx context.Context, pipelineRunID string) ([]domain.RewriteStageRun, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, pipeline_run_id, stage_name, stage_type, prompt_ref, llm_profile_ref, status, attempt, input_json, output_json, error_summary, metadata_json, started_at, completed_at FROM rewrite_stage_runs WHERE pipeline_run_id = ? ORDER BY started_at ASC, id ASC`, pipelineRunID)
	if err != nil {
		return nil, fmt.Errorf("query rewrite stage runs: %w", err)
	}
	defer rows.Close()
	var runs []domain.RewriteStageRun
	for rows.Next() {
		run, scanErr := scanRewriteStageRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		runs = append(runs, *run)
	}
	return runs, rows.Err()
}

func (r *rewriteStageRunRepo) Update(ctx context.Context, run *domain.RewriteStageRun) error {
	metadataJSON, err := json.Marshal(run.Metadata)
	if err != nil {
		return fmt.Errorf("marshal rewrite stage run metadata: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `UPDATE rewrite_stage_runs SET pipeline_run_id = ?, stage_name = ?, stage_type = ?, prompt_ref = ?, llm_profile_ref = ?, status = ?, attempt = ?, input_json = ?, output_json = ?, error_summary = ?, metadata_json = ?, started_at = ?, completed_at = ? WHERE id = ?`, run.PipelineRunID, run.StageName, run.StageType, run.PromptRef, run.LLMProfileRef, run.Status, run.Attempt, run.InputJSON, run.OutputJSON, run.ErrorSummary, string(metadataJSON), run.StartedAt.Format(time.RFC3339), nullableTime(run.CompletedAt), run.ID)
	if err != nil {
		return fmt.Errorf("update rewrite stage run: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check update rewrite stage run result: %w", err)
	}
	if rows == 0 {
		return domain.NewNotFoundErr("rewrite_stage_run", run.ID)
	}
	return nil
}

func (r *rewriteStageRunRepo) DeleteByPipelineRunID(ctx context.Context, pipelineRunID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM rewrite_stage_runs WHERE pipeline_run_id = ?`, pipelineRunID)
	if err != nil {
		return fmt.Errorf("delete rewrite stage runs by pipeline run id: %w", err)
	}
	return nil
}

type rewriteStageRunScanner interface{ Scan(dest ...any) error }

func scanRewriteStageRun(row rewriteStageRunScanner) (*domain.RewriteStageRun, error) {
	var run domain.RewriteStageRun
	var startedAt string
	var completedAt sql.NullString
	var metadataJSON string
	if err := row.Scan(&run.ID, &run.PipelineRunID, &run.StageName, &run.StageType, &run.PromptRef, &run.LLMProfileRef, &run.Status, &run.Attempt, &run.InputJSON, &run.OutputJSON, &run.ErrorSummary, &metadataJSON, &startedAt, &completedAt); err != nil {
		return nil, err
	}
	parsedStartedAt, err := time.Parse(time.RFC3339, startedAt)
	if err != nil {
		return nil, fmt.Errorf("decode rewrite stage run started_at: %w", err)
	}
	run.StartedAt = parsedStartedAt
	if completedAt.Valid {
		parsedCompletedAt, err := time.Parse(time.RFC3339, completedAt.String)
		if err != nil {
			return nil, fmt.Errorf("decode rewrite stage run completed_at: %w", err)
		}
		run.CompletedAt = &parsedCompletedAt
	}
	if err := json.Unmarshal([]byte(metadataJSON), &run.Metadata); err != nil {
		return nil, fmt.Errorf("decode rewrite stage run metadata: %w", err)
	}
	if run.Metadata == nil {
		run.Metadata = map[string]any{}
	}
	return &run, nil
}
