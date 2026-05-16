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

var _ repo.RewritePipelineRunRepo = (*rewritePipelineRunRepo)(nil)

type rewritePipelineRunRepo struct{ db *sql.DB }

func (r *rewritePipelineRunRepo) Create(ctx context.Context, run *domain.RewritePipelineRun) error {
	metadataJSON, err := json.Marshal(run.Metadata)
	if err != nil {
		return fmt.Errorf("marshal rewrite pipeline run metadata: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO rewrite_pipeline_runs (id, profile_id, profile_version, workspace_article_id, target_type, source_profile, status, current_stage, started_at, completed_at, final_draft_id, error_summary, metadata_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, run.ID, run.ProfileID, run.ProfileVersion, run.WorkspaceArticleID, run.TargetType, run.SourceProfile, run.Status, run.CurrentStage, run.StartedAt.Format(time.RFC3339), nullableTime(run.CompletedAt), run.FinalDraftID, run.ErrorSummary, string(metadataJSON))
	if err != nil {
		return fmt.Errorf("insert rewrite pipeline run: %w", err)
	}
	return nil
}

func (r *rewritePipelineRunRepo) Update(ctx context.Context, run *domain.RewritePipelineRun) error {
	metadataJSON, err := json.Marshal(run.Metadata)
	if err != nil {
		return fmt.Errorf("marshal rewrite pipeline run metadata: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `UPDATE rewrite_pipeline_runs SET profile_id = ?, profile_version = ?, workspace_article_id = ?, target_type = ?, source_profile = ?, status = ?, current_stage = ?, started_at = ?, completed_at = ?, final_draft_id = ?, error_summary = ?, metadata_json = ? WHERE id = ?`, run.ProfileID, run.ProfileVersion, run.WorkspaceArticleID, run.TargetType, run.SourceProfile, run.Status, run.CurrentStage, run.StartedAt.Format(time.RFC3339), nullableTime(run.CompletedAt), run.FinalDraftID, run.ErrorSummary, string(metadataJSON), run.ID)
	if err != nil {
		return fmt.Errorf("update rewrite pipeline run: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check update rewrite pipeline run result: %w", err)
	}
	if rows == 0 {
		return domain.NewNotFoundErr("rewrite_pipeline_run", run.ID)
	}
	return nil
}

func (r *rewritePipelineRunRepo) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM rewrite_pipeline_runs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete rewrite pipeline run: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check delete rewrite pipeline run result: %w", err)
	}
	if rows == 0 {
		return domain.NewNotFoundErr("rewrite_pipeline_run", id)
	}
	return nil
}

func (r *rewritePipelineRunRepo) GetByID(ctx context.Context, id string) (*domain.RewritePipelineRun, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, profile_id, profile_version, workspace_article_id, target_type, source_profile, status, current_stage, started_at, completed_at, final_draft_id, error_summary, metadata_json FROM rewrite_pipeline_runs WHERE id = ?`, id)
	run, err := scanRewritePipelineRun(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("rewrite_pipeline_run", id)
		}
		return nil, err
	}
	return run, nil
}

func (r *rewritePipelineRunRepo) List(ctx context.Context, limit int) ([]domain.RewritePipelineRun, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, profile_id, profile_version, workspace_article_id, target_type, source_profile, status, current_stage, started_at, completed_at, final_draft_id, error_summary, metadata_json FROM rewrite_pipeline_runs ORDER BY started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query rewrite pipeline runs: %w", err)
	}
	defer rows.Close()
	var runs []domain.RewritePipelineRun
	for rows.Next() {
		run, scanErr := scanRewritePipelineRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		runs = append(runs, *run)
	}
	return runs, rows.Err()
}

type rewriteRunScanner interface{ Scan(dest ...any) error }

func scanRewritePipelineRun(row rewriteRunScanner) (*domain.RewritePipelineRun, error) {
	var run domain.RewritePipelineRun
	var completedAt sql.NullString
	var startedAt, metadataJSON string
	if err := row.Scan(&run.ID, &run.ProfileID, &run.ProfileVersion, &run.WorkspaceArticleID, &run.TargetType, &run.SourceProfile, &run.Status, &run.CurrentStage, &startedAt, &completedAt, &run.FinalDraftID, &run.ErrorSummary, &metadataJSON); err != nil {
		return nil, err
	}
	parsedStartedAt, err := time.Parse(time.RFC3339, startedAt)
	if err != nil {
		return nil, fmt.Errorf("decode rewrite pipeline run started_at: %w", err)
	}
	run.StartedAt = parsedStartedAt
	if completedAt.Valid {
		parsedCompletedAt, err := time.Parse(time.RFC3339, completedAt.String)
		if err != nil {
			return nil, fmt.Errorf("decode rewrite pipeline run completed_at: %w", err)
		}
		run.CompletedAt = &parsedCompletedAt
	}
	if err := json.Unmarshal([]byte(metadataJSON), &run.Metadata); err != nil {
		return nil, fmt.Errorf("decode rewrite pipeline run metadata: %w", err)
	}
	if run.Metadata == nil {
		run.Metadata = map[string]any{}
	}
	return &run, nil
}
