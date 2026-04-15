package sqlite

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

var _ repo.RewritePipelineProfileRepo = (*rewritePipelineProfileRepo)(nil)

type rewritePipelineProfileRepo struct{ db *sql.DB }

func (r *rewritePipelineProfileRepo) Upsert(ctx context.Context, profile *domain.RewritePipelineProfile) error {
	stagesJSON, err := json.Marshal(profile.Stages)
	if err != nil {
		return fmt.Errorf("marshal rewrite pipeline profile stages: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO rewrite_pipeline_profiles (
			id, name, target_type, source_profile, version, description,
			stages_json, default_llm_profile, quality_policy_ref, materialization_policy, enabled
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(target_type, source_profile, version) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			stages_json = excluded.stages_json,
			default_llm_profile = excluded.default_llm_profile,
			quality_policy_ref = excluded.quality_policy_ref,
			materialization_policy = excluded.materialization_policy,
			enabled = excluded.enabled
	`, profile.ID, profile.Name, profile.TargetType, profile.SourceProfile, profile.Version, profile.Description, string(stagesJSON), profile.DefaultLLMProfile, profile.QualityPolicyRef, profile.MaterializationPolicy, boolToInt(profile.Enabled))
	if err != nil {
		return fmt.Errorf("upsert rewrite pipeline profile: %w", err)
	}
	return nil
}

func (r *rewritePipelineProfileRepo) Get(ctx context.Context, targetType, sourceProfile, version string) (*domain.RewritePipelineProfile, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, name, target_type, source_profile, version, description, stages_json, default_llm_profile, quality_policy_ref, materialization_policy, enabled FROM rewrite_pipeline_profiles WHERE target_type = ? AND source_profile = ? AND version = ?`, targetType, sourceProfile, version)
	profile, err := scanRewritePipelineProfile(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("rewrite_pipeline_profile", fmt.Sprintf("%s/%s/%s", targetType, sourceProfile, version))
		}
		return nil, err
	}
	return profile, nil
}

func (r *rewritePipelineProfileRepo) List(ctx context.Context) ([]domain.RewritePipelineProfile, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, target_type, source_profile, version, description, stages_json, default_llm_profile, quality_policy_ref, materialization_policy, enabled FROM rewrite_pipeline_profiles ORDER BY target_type ASC, source_profile ASC, version DESC`)
	if err != nil {
		return nil, fmt.Errorf("query rewrite pipeline profiles: %w", err)
	}
	defer rows.Close()
	var profiles []domain.RewritePipelineProfile
	for rows.Next() {
		profile, scanErr := scanRewritePipelineProfile(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		profiles = append(profiles, *profile)
	}
	return profiles, rows.Err()
}

type rewriteProfileScanner interface{ Scan(dest ...any) error }

func scanRewritePipelineProfile(row rewriteProfileScanner) (*domain.RewritePipelineProfile, error) {
	var profile domain.RewritePipelineProfile
	var stagesJSON string
	var enabled int
	if err := row.Scan(&profile.ID, &profile.Name, &profile.TargetType, &profile.SourceProfile, &profile.Version, &profile.Description, &stagesJSON, &profile.DefaultLLMProfile, &profile.QualityPolicyRef, &profile.MaterializationPolicy, &enabled); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(stagesJSON), &profile.Stages); err != nil {
		return nil, fmt.Errorf("decode rewrite pipeline profile stages: %w", err)
	}
	profile.Enabled = enabled == 1
	if profile.Stages == nil {
		profile.Stages = []domain.RewriteStageDefinition{}
	}
	return &profile, nil
}
