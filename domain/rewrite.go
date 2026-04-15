package domain

import (
	"content-hub/pkg/id"
	"time"
)

const (
	RewriteRunPending   = "pending"
	RewriteRunRunning   = "running"
	RewriteRunSucceeded = "succeeded"
	RewriteRunFailed    = "failed"

	RewriteStagePending   = "pending"
	RewriteStageRunning   = "running"
	RewriteStageSucceeded = "succeeded"
	RewriteStageFailed    = "failed"
)

type RewriteRetryPolicy struct {
	MaxAttempts int    `json:"max_attempts"`
	Strategy    string `json:"strategy"`
}

type RewriteFailurePolicy struct {
	Action      string `json:"action"`
	RepairStage string `json:"repair_stage"`
}

type RewriteStageDefinition struct {
	Name            string               `json:"name"`
	Type            string               `json:"type"`
	PromptRef       string               `json:"prompt_ref"`
	ModelProfileRef string               `json:"model_profile_ref"`
	InputBindings   map[string]string    `json:"input_bindings"`
	OutputSchema    string               `json:"output_schema"`
	RetryPolicy     RewriteRetryPolicy   `json:"retry_policy"`
	OnFailure       RewriteFailurePolicy `json:"on_failure"`
	QualityChecks   []string             `json:"quality_checks"`
	Enabled         bool                 `json:"enabled"`
}

type RewritePipelineProfile struct {
	ID                    string                   `json:"id"`
	Name                  string                   `json:"name"`
	TargetType            string                   `json:"target_type"`
	SourceProfile         string                   `json:"source_profile"`
	Version               string                   `json:"version"`
	Description           string                   `json:"description"`
	Stages                []RewriteStageDefinition `json:"stages"`
	DefaultLLMProfile     string                   `json:"default_llm_profile"`
	QualityPolicyRef      string                   `json:"quality_policy_ref"`
	MaterializationPolicy string                   `json:"materialization_policy"`
	Enabled               bool                     `json:"enabled"`
}

type RewritePipelineRun struct {
	ID                 string     `json:"id"`
	ProfileID          string     `json:"profile_id"`
	ProfileVersion     string     `json:"profile_version"`
	WorkspaceArticleID string     `json:"workspace_article_id"`
	CollectorArticleID string     `json:"collector_article_id"`
	TargetType         string     `json:"target_type"`
	SourceProfile      string     `json:"source_profile"`
	Status             string     `json:"status"`
	CurrentStage       string     `json:"current_stage"`
	StartedAt          time.Time  `json:"started_at"`
	CompletedAt        *time.Time `json:"completed_at"`
	FinalDraftID       string     `json:"final_draft_id"`
	ErrorSummary       string     `json:"error_summary"`
	MetadataJSON       []byte     `json:"metadata_json"`
}

func NewRewritePipelineRun(profileID, profileVersion, workspaceArticleID, collectorArticleID, targetType, sourceProfile string) *RewritePipelineRun {
	now := time.Now().UTC()
	return &RewritePipelineRun{
		ID:                 id.New(),
		ProfileID:          profileID,
		ProfileVersion:     profileVersion,
		WorkspaceArticleID: workspaceArticleID,
		CollectorArticleID: collectorArticleID,
		TargetType:         targetType,
		SourceProfile:      sourceProfile,
		Status:             RewriteRunPending,
		StartedAt:          now,
		MetadataJSON:       []byte("{}"),
	}
}
