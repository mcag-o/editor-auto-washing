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

type rewriteAssembly struct {
	promptRegistry    *PromptRegistry
	profileRegistry   *RewriteProfileRegistry
	qualityGate       *QualityGateEngine
	stageExecutor     *RewriteStageExecutor
	draftMaterializer *DraftMaterializer
	orchestrator      *RewriteOrchestrator
}

func BuildRewriteRuntime(repos *RuntimeRepos) (*RewriteRuntime, error) {
	if repos == nil {
		return nil, domain.NewInternalErr("rewrite runtime repos are required", nil)
	}

	assembly := buildRewriteAssembly(repos)

	return &RewriteRuntime{
		PromptRegistry:    assembly.promptRegistry,
		ProfileRegistry:   assembly.profileRegistry,
		QualityGate:       assembly.qualityGate,
		StageExecutor:     assembly.stageExecutor,
		DraftMaterializer: assembly.draftMaterializer,
		Orchestrator:      assembly.orchestrator,
	}, nil
}

func buildRewriteAssembly(repos *RuntimeRepos) rewriteAssembly {
	promptRegistry := NewPromptRegistry(repos.PromptTemplateRepo)
	profileRegistry := NewRewriteProfileRegistry(repos.RewritePipelineProfileRepo)
	qualityGate := NewQualityGateEngine()
	stageExecutor := NewRewriteStageExecutorWithProfileResolver(promptRegistry, repos.LLMProfileRepo, rewriteLLMClient(repos), qualityGate)
	draftMaterializer := NewDraftMaterializer(repos.DraftRepo, repos.WorkspaceRepo)
	orchestrator := NewRewriteOrchestrator(profileRegistry, repos.RewritePipelineRunRepo, repos.RewriteStageRunRepo, repos.WorkspaceRepo, stageExecutor, draftMaterializer)

	return rewriteAssembly{
		promptRegistry:    promptRegistry,
		profileRegistry:   profileRegistry,
		qualityGate:       qualityGate,
		stageExecutor:     stageExecutor,
		draftMaterializer: draftMaterializer,
		orchestrator:      orchestrator,
	}
}

func rewriteLLMClient(repos *RuntimeRepos) llminfra.Client {
	if repos == nil {
		return nil
	}
	return repos.LLMClient
}
