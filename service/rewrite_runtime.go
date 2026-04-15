package service

import (
	"content-hub/domain"
	llminfra "content-hub/infra/llm"
)

type RewriteRuntime struct {
	PromptRegistry    *PromptRegistry
	ProfileRegistry   *RewriteProfileRegistry
	QualityGate       *QualityGateEngine
	StageExecutor     *RewriteStageExecutor
	DraftMaterializer *DraftMaterializer
	Orchestrator      *RewriteOrchestrator
}

func BuildRewriteRuntime(repos *RuntimeRepos) (*RewriteRuntime, error) {
	if repos == nil {
		return nil, domain.NewInternalErr("rewrite runtime repos are required", nil)
	}

	promptRegistry := NewPromptRegistry(repos.PromptTemplateRepo)
	profileRegistry := NewRewriteProfileRegistry(repos.RewritePipelineProfileRepo)
	qualityGate := NewQualityGateEngine()
	stageExecutor := NewRewriteStageExecutorWithProfileResolver(promptRegistry, repos.LLMProfileRepo, rewriteLLMClient(repos), qualityGate)
	draftMaterializer := NewDraftMaterializer(repos.DraftRepo, repos.WorkspaceRepo)
	orchestrator := NewRewriteOrchestrator(profileRegistry, repos.RewritePipelineRunRepo, repos.RewriteStageRunRepo, repos.WorkspaceRepo, stageExecutor, draftMaterializer)

	return &RewriteRuntime{
		PromptRegistry:    promptRegistry,
		ProfileRegistry:   profileRegistry,
		QualityGate:       qualityGate,
		StageExecutor:     stageExecutor,
		DraftMaterializer: draftMaterializer,
		Orchestrator:      orchestrator,
	}, nil
}

func rewriteLLMClient(repos *RuntimeRepos) llminfra.Client {
	if repos == nil {
		return nil
	}
	return repos.LLMClient
}
