package service

import (
	"content-hub/domain"
	"content-hub/pkg/id"
	"content-hub/pkg/repo"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	rewritePendingNote = "rewrite queued"
	rewritingNote      = "rewrite in progress"
)

type RewriteRunRequest struct {
	WorkspaceArticleID string
	CollectorArticleID string
	Title              string
	TargetType         string
	SourceProfile      string
	Version            string
}

type RewriteOrchestrator struct {
	profiles    *RewriteProfileRegistry
	runs        repo.RewritePipelineRunRepo
	stageRuns   repo.RewriteStageRunRepo
	workspaces  repo.WorkspaceRepo
	executor    *RewriteStageExecutor
	materialize *DraftMaterializer
}

func NewRewriteOrchestrator(profiles *RewriteProfileRegistry, runs repo.RewritePipelineRunRepo, stageRuns repo.RewriteStageRunRepo, workspaces repo.WorkspaceRepo, executor *RewriteStageExecutor, materializer *DraftMaterializer) *RewriteOrchestrator {
	return &RewriteOrchestrator{
		profiles:    profiles,
		runs:        runs,
		stageRuns:   stageRuns,
		workspaces:  workspaces,
		executor:    executor,
		materialize: materializer,
	}
}

func (o *RewriteOrchestrator) Run(ctx context.Context, req RewriteRunRequest) (*domain.RewritePipelineRun, error) {
	if err := o.validate(); err != nil {
		return nil, err
	}

	profile, err := o.profiles.Resolve(ctx, req.TargetType, req.SourceProfile, req.Version)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, domain.NewNotFoundErr("rewrite profile", fmt.Sprintf("%s/%s@%s", req.TargetType, req.SourceProfile, req.Version))
	}

	run := domain.NewRewritePipelineRun(profile.ID, profile.Version, strings.TrimSpace(req.WorkspaceArticleID), strings.TrimSpace(req.CollectorArticleID), strings.TrimSpace(req.TargetType), strings.TrimSpace(req.SourceProfile))
	run.Status = domain.RewriteRunRunning
	run.Metadata = map[string]any{
		"title": req.Title,
	}
	if err := o.runs.Create(ctx, run); err != nil {
		return nil, err
	}

	if err := o.workspaces.TransitionStatus(ctx, req.WorkspaceArticleID, domain.ArticleWorkspaceStatusRewritePending, rewritePendingNote); err != nil {
		return nil, err
	}
	if err := o.workspaces.TransitionStatus(ctx, req.WorkspaceArticleID, domain.ArticleWorkspaceStatusRewriting, rewritingNote); err != nil {
		return nil, err
	}

	vars := map[string]any{"title": req.Title}
	var finalOutput map[string]any
	for _, stage := range profile.Stages {
		if !stage.Enabled {
			continue
		}

		run.CurrentStage = stage.Name
		if err := o.runs.Update(ctx, run); err != nil {
			return nil, err
		}

		inputVars := mergeStageVars(vars, stage.InputBindings)
		result, err := o.executor.Execute(ctx, stage, StageExecutionInput{Vars: inputVars})
		if err != nil {
			return nil, err
		}

		stageRun, err := buildRewriteStageRun(run.ID, stage, inputVars, result)
		if err != nil {
			return nil, err
		}
		if err := o.stageRuns.Create(ctx, stageRun); err != nil {
			return nil, err
		}

		for key, value := range result.StructuredOutput {
			vars[key] = value
		}
		finalOutput = result.StructuredOutput
	}

	draft, err := o.materialize.Materialize(ctx, req.WorkspaceArticleID, finalOutput)
	if err != nil {
		return nil, err
	}

	completedAt := time.Now().UTC()
	run.Status = domain.RewriteRunSucceeded
	run.FinalDraftID = draft.ID
	run.CompletedAt = &completedAt
	if err := o.runs.Update(ctx, run); err != nil {
		return nil, err
	}

	return run, nil
}

func (o *RewriteOrchestrator) validate() error {
	if o.profiles == nil || o.runs == nil || o.stageRuns == nil || o.workspaces == nil || o.executor == nil || o.materialize == nil {
		return domain.NewInternalErr("rewrite orchestrator is not configured", nil)
	}
	return nil
}

func mergeStageVars(vars map[string]any, bindings map[string]string) map[string]any {
	merged := make(map[string]any, len(vars)+len(bindings))
	for key, value := range vars {
		merged[key] = value
	}
	for key, sourceKey := range bindings {
		if value, ok := vars[sourceKey]; ok {
			merged[key] = value
		}
	}
	return merged
}

func buildRewriteStageRun(pipelineRunID string, stage domain.RewriteStageDefinition, inputVars map[string]any, result *StageExecutionResult) (*domain.RewriteStageRun, error) {
	inputJSON, err := json.Marshal(inputVars)
	if err != nil {
		return nil, fmt.Errorf("marshal rewrite stage input: %w", err)
	}
	outputJSON, err := json.Marshal(result.StructuredOutput)
	if err != nil {
		return nil, fmt.Errorf("marshal rewrite stage output: %w", err)
	}
	completedAt := time.Now().UTC()
	return &domain.RewriteStageRun{
		ID:            id.New(),
		PipelineRunID: pipelineRunID,
		StageName:     stage.Name,
		StageType:     stage.Type,
		PromptRef:     stage.PromptRef,
		LLMProfileRef: stage.ModelProfileRef,
		Status:        domain.RewriteStageSucceeded,
		Attempt:       1,
		InputJSON:     string(inputJSON),
		OutputJSON:    string(outputJSON),
		Metadata:      map[string]any{},
		StartedAt:     completedAt,
		CompletedAt:   &completedAt,
	}, nil
}
