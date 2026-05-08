package service

import (
	"content-hub/domain"
	llminfra "content-hub/infra/llm"
)

type RewriteRuntime struct {
	orchestrator *RewriteOrchestrator
}

type rewriteAssembly struct {
	orchestrator *RewriteOrchestrator
}

func BuildRewriteRuntime(repos *RuntimeRepos) (*RewriteRuntime, error) {
	if repos == nil {
		return nil, domain.NewInternalErr("rewrite runtime repos are required", nil)
	}

	assembly := buildRewriteAssembly(repos)

	return &RewriteRuntime{
		orchestrator: assembly.orchestrator,
	}, nil
}

func buildRewriteAssembly(repos *RuntimeRepos) rewriteAssembly {
	promptRegistry := NewPromptRegistry(repos.PromptTemplateRepo)
	profileRegistry := NewRewriteProfileRegistry(repos.RewritePipelineProfileRepo)
	qualityGate := NewQualityGateEngine()
	stageExecutor := NewRewriteStageExecutorWithProfileResolver(promptRegistry, repos.LLMProfileRepo, rewriteLLMClient(repos), qualityGate)
	draftMaterializer := NewDraftMaterializer(repos.DraftRepo, repos.WorkspaceRepo)

	return rewriteAssembly{
		orchestrator: NewRewriteOrchestrator(profileRegistry, repos.RewritePipelineRunRepo, repos.RewriteStageRunRepo, repos.WorkspaceRepo, stageExecutor, draftMaterializer),
	}
}

func (r *RewriteRuntime) Orchestrator() *RewriteOrchestrator {
	if r == nil {
		return nil
	}
	return r.orchestrator
}

func rewriteLLMClient(repos *RuntimeRepos) llminfra.Client {
	if repos == nil {
		return nil
	}
	return repos.LLMClient
}
