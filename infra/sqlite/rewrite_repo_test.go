package sqlite

import (
	"content-hub/domain"
	"testing"

	"github.com/stretchr/testify/require"
)

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
