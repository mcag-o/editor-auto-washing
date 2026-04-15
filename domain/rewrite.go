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
	ID                 string         `json:"id"`
	ProfileID          string         `json:"profile_id"`
	ProfileVersion     string         `json:"profile_version"`
	WorkspaceArticleID string         `json:"workspace_article_id"`
	CollectorArticleID string         `json:"collector_article_id"`
	TargetType         string         `json:"target_type"`
	SourceProfile      string         `json:"source_profile"`
	Status             string         `json:"status"`
	CurrentStage       string         `json:"current_stage"`
	StartedAt          time.Time      `json:"started_at"`
	CompletedAt        *time.Time     `json:"completed_at"`
	FinalDraftID       string         `json:"final_draft_id"`
	ErrorSummary       string         `json:"error_summary"`
	Metadata           map[string]any `json:"metadata"`
}

type RewriteStageRun struct {
	ID            string         `json:"id"`
	PipelineRunID string         `json:"pipeline_run_id"`
	StageName     string         `json:"stage_name"`
	StageType     string         `json:"stage_type"`
	PromptRef     string         `json:"prompt_ref"`
	LLMProfileRef string         `json:"llm_profile_ref"`
	Status        string         `json:"status"`
	Attempt       int            `json:"attempt"`
	InputJSON     string         `json:"input_json"`
	OutputJSON    string         `json:"output_json"`
	ErrorSummary  string         `json:"error_summary"`
	Metadata      map[string]any `json:"metadata"`
	StartedAt     time.Time      `json:"started_at"`
	CompletedAt   *time.Time     `json:"completed_at"`
}

type PromptTemplate struct {
	Key            string         `json:"key"`
	Version        string         `json:"version"`
	SystemTemplate string         `json:"system_template"`
	UserTemplate   string         `json:"user_template"`
	Description    string         `json:"description"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type LLMProfile struct {
	Name        string         `json:"name"`
	Provider    string         `json:"provider"`
	Model       string         `json:"model"`
	APIKeyRef   string         `json:"api_key_ref"`
	BaseURLRef  string         `json:"base_url_ref"`
	Temperature float64        `json:"temperature"`
	MaxTokens   int            `json:"max_tokens"`
	TimeoutSec  int            `json:"timeout_sec"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
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
		CurrentStage:       "",
		StartedAt:          now,
		Metadata:           map[string]any{},
	}
}
