package sqlite

import (
	"content-hub/domain"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProviderExposesRewriteRepos(t *testing.T) {
	provider := newRuntimeProvider(t)

	require.NotNil(t, provider.RewritePipelineProfileRepo())
	require.NotNil(t, provider.RewritePipelineRunRepo())
	require.NotNil(t, provider.RewriteStageRunRepo())
	require.NotNil(t, provider.PromptTemplateRepo())
	require.NotNil(t, provider.LLMProfileRepo())
}

func TestRewritePipelineProfileRepo_UpsertGetListKeepsLogicalIDStable(t *testing.T) {
	provider := newRuntimeProvider(t)
	profile := &domain.RewritePipelineProfile{
		ID:            "profile-1",
		Name:          "WeChat SSPAI",
		TargetType:    "wechat-longform",
		SourceProfile: "sspai",
		Version:       "v1",
		Description:   "first version",
		Stages: []domain.RewriteStageDefinition{{
			Name:        "plan",
			Type:        "planning",
			PromptRef:   "plan_rewrite",
			Enabled:     true,
			RetryPolicy: domain.RewriteRetryPolicy{MaxAttempts: 2, Strategy: "fixed"},
		}},
		DefaultLLMProfile:     "gpt-4o",
		QualityPolicyRef:      "quality-default",
		MaterializationPolicy: "draft",
		Enabled:               true,
	}

	require.NoError(t, provider.RewritePipelineProfileRepo().Upsert(t.Context(), profile))

	replacement := *profile
	replacement.ID = "profile-2"
	replacement.Description = "updated description"
	require.NoError(t, provider.RewritePipelineProfileRepo().Upsert(t.Context(), &replacement))

	stored, err := provider.RewritePipelineProfileRepo().Get(t.Context(), profile.TargetType, profile.SourceProfile, profile.Version)
	require.NoError(t, err)
	require.Equal(t, "profile-1", stored.ID)
	require.Equal(t, "updated description", stored.Description)

	profiles, err := provider.RewritePipelineProfileRepo().List(t.Context())
	require.NoError(t, err)
	require.Len(t, profiles, 1)
	require.Equal(t, "profile-1", profiles[0].ID)
}

func TestRewritePipelineRunRepo_CreateAndGetByID(t *testing.T) {
	provider := newRuntimeProvider(t)
	run := domain.NewRewritePipelineRun("profile-1", "v1", "workspace-1", "collector-1", "wechat-longform", "sspai")

	require.NoError(t, provider.RewritePipelineRunRepo().Create(t.Context(), run))

	stored, err := provider.RewritePipelineRunRepo().GetByID(t.Context(), run.ID)
	require.NoError(t, err)
	require.Equal(t, run.ProfileID, stored.ProfileID)
	require.Equal(t, domain.RewriteRunPending, stored.Status)
	require.Equal(t, run.Metadata, stored.Metadata)
}

func TestRewritePipelineRunRepo_UpdateAndList(t *testing.T) {
	provider := newRuntimeProvider(t)
	older := domain.NewRewritePipelineRun("profile-1", "v1", "workspace-1", "collector-1", "wechat-longform", "sspai")
	older.StartedAt = time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	newer := domain.NewRewritePipelineRun("profile-1", "v1", "workspace-2", "collector-2", "wechat-longform", "sspai")
	newer.StartedAt = time.Now().UTC().Truncate(time.Second)
	newer.Status = domain.RewriteRunRunning
	newer.CurrentStage = "draft"
	newer.FinalDraftID = "draft-42"
	newer.ErrorSummary = ""
	newer.Metadata = map[string]any{"attempt": float64(2)}

	require.NoError(t, provider.RewritePipelineRunRepo().Create(t.Context(), older))
	require.NoError(t, provider.RewritePipelineRunRepo().Create(t.Context(), newer))

	completedAt := newer.StartedAt.Add(10 * time.Minute)
	newer.Status = domain.RewriteRunSucceeded
	newer.CompletedAt = &completedAt
	newer.CurrentStage = "finalize"
	newer.Metadata["result"] = "ok"
	require.NoError(t, provider.RewritePipelineRunRepo().Update(t.Context(), newer))

	stored, err := provider.RewritePipelineRunRepo().GetByID(t.Context(), newer.ID)
	require.NoError(t, err)
	require.Equal(t, domain.RewriteRunSucceeded, stored.Status)
	require.Equal(t, "finalize", stored.CurrentStage)
	require.Equal(t, newer.FinalDraftID, stored.FinalDraftID)
	require.Equal(t, "ok", stored.Metadata["result"])
	require.NotNil(t, stored.CompletedAt)

	runs, err := provider.RewritePipelineRunRepo().List(t.Context(), 10)
	require.NoError(t, err)
	require.Len(t, runs, 2)
	require.Equal(t, newer.ID, runs[0].ID)
	require.Equal(t, older.ID, runs[1].ID)
}

func TestRewriteStageRunRepo_CreateAndListByPipelineRunID(t *testing.T) {
	provider := newRuntimeProvider(t)
	pipelineRun := domain.NewRewritePipelineRun("profile-1", "v1", "workspace-1", "collector-1", "wechat-longform", "sspai")
	require.NoError(t, provider.RewritePipelineRunRepo().Create(t.Context(), pipelineRun))

	first := &domain.RewriteStageRun{
		ID:            "stage-1",
		PipelineRunID: pipelineRun.ID,
		StageName:     "plan",
		StageType:     "planning",
		PromptRef:     "plan_rewrite",
		LLMProfileRef: "gpt-4o",
		Status:        domain.RewriteStagePending,
		Attempt:       1,
		InputJSON:     `{"title":"hello"}`,
		OutputJSON:    `{}`,
		Metadata:      map[string]any{"order": float64(1)},
		StartedAt:     time.Now().UTC().Add(-time.Minute).Truncate(time.Second),
	}
	second := &domain.RewriteStageRun{
		ID:            "stage-2",
		PipelineRunID: pipelineRun.ID,
		StageName:     "draft",
		StageType:     "generation",
		PromptRef:     "draft_rewrite",
		LLMProfileRef: "gpt-4o-mini",
		Status:        domain.RewriteStageRunning,
		Attempt:       2,
		InputJSON:     `{"outline":"x"}`,
		OutputJSON:    `{"body":"y"}`,
		Metadata:      map[string]any{"order": float64(2)},
		StartedAt:     time.Now().UTC().Truncate(time.Second),
	}

	require.NoError(t, provider.RewriteStageRunRepo().Create(t.Context(), first))
	require.NoError(t, provider.RewriteStageRunRepo().Create(t.Context(), second))

	runs, err := provider.RewriteStageRunRepo().ListByPipelineRunID(t.Context(), pipelineRun.ID)
	require.NoError(t, err)
	require.Len(t, runs, 2)
	require.Equal(t, first.ID, runs[0].ID)
	require.Equal(t, second.ID, runs[1].ID)
	require.Equal(t, "draft_rewrite", runs[1].PromptRef)
}

func TestPromptTemplateRepo_CreateAndResolveVersion(t *testing.T) {
	provider := newRuntimeProvider(t)
	tpl := domain.PromptTemplate{
		Key:            "plan_rewrite",
		Version:        "v1",
		SystemTemplate: "sys",
		UserTemplate:   "user",
		Description:    "rewrite planner",
		Metadata: map[string]any{
			"stage": "plan",
		},
	}

	require.NoError(t, provider.PromptTemplateRepo().Upsert(t.Context(), &tpl))

	stored, err := provider.PromptTemplateRepo().Get(t.Context(), "plan_rewrite", "v1")
	require.NoError(t, err)
	require.Equal(t, "sys", stored.SystemTemplate)
	require.Equal(t, "user", stored.UserTemplate)
	require.Equal(t, "plan", stored.Metadata["stage"])
}

func TestLLMProfileRepo_UpsertGetAndList(t *testing.T) {
	provider := newRuntimeProvider(t)
	profile := &domain.LLMProfile{
		Name:        "gpt-4o",
		Provider:    "openai",
		Model:       "gpt-4o",
		APIKeyRef:   "env.OPENAI_API_KEY",
		BaseURLRef:  "env.OPENAI_BASE_URL",
		Temperature: 0.4,
		MaxTokens:   4096,
		TimeoutSec:  60,
		Metadata:    map[string]any{"tier": "primary"},
	}

	require.NoError(t, provider.LLMProfileRepo().Upsert(t.Context(), profile))

	stored, err := provider.LLMProfileRepo().GetByName(t.Context(), profile.Name)
	require.NoError(t, err)
	require.Equal(t, "openai", stored.Provider)
	require.Equal(t, "primary", stored.Metadata["tier"])

	profiles, err := provider.LLMProfileRepo().List(t.Context())
	require.NoError(t, err)
	require.Len(t, profiles, 1)
	require.Equal(t, profile.Name, profiles[0].Name)
}
