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
	rewriteFailedNote  = "rewrite failed"
)

type RewriteRunRequest struct {
	WorkspaceArticleID string `json:"workspace_article_id" binding:"required"`
	CollectorArticleID string `json:"collector_article_id" binding:"required"`
	Title              string `json:"title" binding:"required"`
	TargetType         string `json:"target_type" binding:"required"`
	SourceProfile      string `json:"source_profile" binding:"required"`
	Version            string `json:"version" binding:"required"`
	Metadata           map[string]any `json:"metadata"`
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
	if !profile.Enabled {
		return nil, domain.NewValidationErr(fmt.Sprintf("rewrite profile %s/%s@%s is disabled", req.TargetType, req.SourceProfile, req.Version), nil)
	}

	run := domain.NewRewritePipelineRun(profile.ID, profile.Version, strings.TrimSpace(req.WorkspaceArticleID), strings.TrimSpace(req.CollectorArticleID), strings.TrimSpace(req.TargetType), strings.TrimSpace(req.SourceProfile))
	run.Status = domain.RewriteRunRunning
	run.Metadata = map[string]any{
		"title": req.Title,
	}
	for key, value := range req.Metadata {
		run.Metadata[key] = value
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

	return o.executeRun(ctx, run, profile, strings.TrimSpace(req.WorkspaceArticleID), req.Title, nil)
}

func (o *RewriteOrchestrator) Resume(ctx context.Context, rewriteRunID string, title string) (*domain.RewritePipelineRun, error) {
	if err := o.validate(); err != nil {
		return nil, err
	}
	rewriteRunID = strings.TrimSpace(rewriteRunID)
	if rewriteRunID == "" {
		return nil, domain.NewValidationErr("rewrite run id is required", nil)
	}
	run, err := o.runs.GetByID(ctx, rewriteRunID)
	if err != nil {
		return nil, err
	}
	profile, err := o.profiles.Resolve(ctx, run.TargetType, run.SourceProfile, run.ProfileVersion)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, domain.NewNotFoundErr("rewrite profile", fmt.Sprintf("%s/%s@%s", run.TargetType, run.SourceProfile, run.ProfileVersion))
	}
	if !profile.Enabled {
		return nil, domain.NewValidationErr(fmt.Sprintf("rewrite profile %s/%s@%s is disabled", run.TargetType, run.SourceProfile, run.ProfileVersion), nil)
	}
	if run.Metadata == nil {
		run.Metadata = map[string]any{}
	}
	if strings.TrimSpace(title) != "" {
		run.Metadata["title"] = strings.TrimSpace(title)
	}
	run.Status = domain.RewriteRunRunning
	run.CompletedAt = nil
	run.ErrorSummary = ""
	if err := o.workspaces.TransitionStatus(ctx, run.WorkspaceArticleID, domain.ArticleWorkspaceStatusRewritePending, rewritePendingNote); err != nil {
		appErr, ok := err.(*domain.AppError)
		if !ok || appErr.Code != domain.ErrConflict {
			return nil, err
		}
	}
	if err := o.workspaces.TransitionStatus(ctx, run.WorkspaceArticleID, domain.ArticleWorkspaceStatusRewriting, rewritingNote); err != nil {
		return nil, err
	}
	if err := o.runs.Update(ctx, run); err != nil {
		return nil, err
	}
	stageRuns, err := o.stageRuns.ListByPipelineRunID(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	return o.executeRun(ctx, run, profile, run.WorkspaceArticleID, title, stageRuns)
}

func (o *RewriteOrchestrator) executeRun(ctx context.Context, run *domain.RewritePipelineRun, profile *domain.RewritePipelineProfile, workspaceArticleID string, title string, existingStageRuns []domain.RewriteStageRun) (*domain.RewritePipelineRun, error) {
	vars, completedStages, finalOutput, err := buildRewriteResumeState(existingStageRuns, title)
	if err != nil {
		return o.failRun(ctx, run, domain.RewriteStageDefinition{Name: run.CurrentStage}, nil, err)
	}

	stagesByName := indexRewriteStages(profile.Stages)
	skippedStages := map[string]bool{}
	executedAny := false
	for _, stage := range profile.Stages {
		if skippedStages[stage.Name] {
			continue
		}
		if completedStages[stage.Name] {
			continue
		}
		if !stage.Enabled {
			continue
		}
		stage = applyProfileDefaultsToStage(profile, stage)
		executedAny = true

		run.CurrentStage = stage.Name
		if err := o.runs.Update(ctx, run); err != nil {
			return nil, err
		}

		inputVars := mergeStageVars(vars, stage.InputBindings)
		for key, value := range run.Metadata {
			inputVars[key] = value
		}
		stage = applyWorkflowOverride(stage, run.Metadata, inputVars)
		result, err := o.executor.Execute(ctx, stage, StageExecutionInput{Vars: inputVars})
		if err != nil {
			return o.failRun(ctx, run, stage, inputVars, err)
		}

		stageRun, err := buildRewriteStageRun(run.ID, stage, inputVars, result)
		if err != nil {
			return o.failRun(ctx, run, stage, inputVars, err)
		}
		if err := o.stageRuns.Create(ctx, stageRun); err != nil {
			return o.failRun(ctx, run, stage, inputVars, err)
		}

		if shouldRouteStageToRepair(stage, result.Quality) {
			repairName := strings.TrimSpace(stage.OnFailure.RepairStage)
			repairStage, ok := stagesByName[repairName]
			if !ok {
				return o.failRun(ctx, run, stage, inputVars, domain.NewNotFoundErr("repair stage", repairName))
			}
			if !repairStage.Enabled {
				return o.failRun(ctx, run, stage, inputVars, domain.NewValidationErr(fmt.Sprintf("repair stage %s is disabled", repairName), nil))
			}

			repairStage = applyProfileDefaultsToStage(profile, repairStage)
			repairInputVars := mergeStageVars(vars, repairStage.InputBindings)
			for key, value := range run.Metadata {
				repairInputVars[key] = value
			}
			for key, value := range result.StructuredOutput {
				repairInputVars[key] = value
			}
			repairStage = applyWorkflowOverride(repairStage, run.Metadata, repairInputVars)

			run.CurrentStage = repairStage.Name
			if err := o.runs.Update(ctx, run); err != nil {
				return nil, err
			}

			repairResult, err := o.executor.Execute(ctx, repairStage, StageExecutionInput{Vars: repairInputVars})
			if err != nil {
				return o.failRun(ctx, run, repairStage, repairInputVars, err)
			}

			repairStageRun, err := buildRewriteStageRun(run.ID, repairStage, repairInputVars, repairResult)
			if err != nil {
				return o.failRun(ctx, run, repairStage, repairInputVars, err)
			}
			if err := o.stageRuns.Create(ctx, repairStageRun); err != nil {
				return o.failRun(ctx, run, repairStage, repairInputVars, err)
			}
			if repairResult.Quality.Action != QualityDecisionPass {
				return o.failRun(ctx, run, repairStage, repairInputVars, domain.NewValidationErr(repairResult.Quality.Message, nil))
			}

			result = repairResult
			completedStages[repairStage.Name] = true
			skippedStages[repairStage.Name] = true
		} else if result.Quality.Action == QualityDecisionRepair {
			return o.failRun(ctx, run, stage, inputVars, domain.NewValidationErr(result.Quality.Message, nil))
		}

		completedStages[stage.Name] = true
		for key, value := range result.StructuredOutput {
			vars[key] = value
		}
		finalOutput = result.StructuredOutput
	}
	if !executedAny && strings.TrimSpace(run.FinalDraftID) != "" {
		completedAt := time.Now().UTC()
		run.Status = domain.RewriteRunSucceeded
		run.CompletedAt = &completedAt
		if err := o.runs.Update(ctx, run); err != nil {
			return nil, err
		}
		return run, nil
	}
	if finalOutput == nil {
		return o.failRun(ctx, run, domain.RewriteStageDefinition{Name: run.CurrentStage}, nil, domain.NewInternalErr("rewrite run has no output to materialize", nil))
	}

	draft, err := o.materialize.Materialize(ctx, workspaceArticleID, finalOutput)
	if err != nil {
		return o.failRun(ctx, run, domain.RewriteStageDefinition{Name: run.CurrentStage}, nil, err)
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

func buildRewriteResumeState(stageRuns []domain.RewriteStageRun, title string) (map[string]any, map[string]bool, map[string]any, error) {
	vars := map[string]any{}
	if strings.TrimSpace(title) != "" {
		vars["title"] = strings.TrimSpace(title)
	}
	completedStages := map[string]bool{}
	var finalOutput map[string]any
	for _, stageRun := range stageRuns {
		if stageRun.Status != domain.RewriteStageSucceeded {
			continue
		}
		completedStages[stageRun.StageName] = true
		if strings.TrimSpace(stageRun.OutputJSON) == "" {
			continue
		}
		output := map[string]any{}
		if err := json.Unmarshal([]byte(stageRun.OutputJSON), &output); err != nil {
			return nil, nil, nil, fmt.Errorf("decode existing rewrite stage output: %w", err)
		}
		for key, value := range output {
			vars[key] = value
		}
		finalOutput = output
	}
	return vars, completedStages, finalOutput, nil
}

func (o *RewriteOrchestrator) failRun(ctx context.Context, run *domain.RewritePipelineRun, stage domain.RewriteStageDefinition, inputVars map[string]any, runErr error) (*domain.RewritePipelineRun, error) {
	if run == nil {
		return nil, runErr
	}

	if strings.TrimSpace(stage.Name) != "" {
		run.CurrentStage = stage.Name
	}
	completedAt := time.Now().UTC()
	run.Status = domain.RewriteRunFailed
	run.ErrorSummary = runErr.Error()
	run.CompletedAt = &completedAt

	stageRunErr := o.persistFailedStageRun(ctx, run.ID, stage, inputVars, runErr)
	workspaceErr := o.workspaces.TransitionStatus(ctx, run.WorkspaceArticleID, domain.ArticleWorkspaceStatusRewriteFailed, rewriteFailedNote)
	runUpdateErr := o.runs.Update(ctx, run)

	if stageRunErr != nil {
		return run, fmt.Errorf("%w: persist failed stage run: %v", runErr, stageRunErr)
	}
	if workspaceErr != nil {
		return run, fmt.Errorf("%w: mark workspace rewrite failed: %v", runErr, workspaceErr)
	}
	if runUpdateErr != nil {
		return run, fmt.Errorf("%w: update rewrite pipeline run: %v", runErr, runUpdateErr)
	}
	return run, runErr
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

func applyProfileDefaultsToStage(profile *domain.RewritePipelineProfile, stage domain.RewriteStageDefinition) domain.RewriteStageDefinition {
	if profile == nil {
		return stage
	}
	if strings.TrimSpace(stage.ModelProfileRef) == "" {
		stage.ModelProfileRef = strings.TrimSpace(profile.DefaultLLMProfile)
	}
	return stage
}

func indexRewriteStages(stages []domain.RewriteStageDefinition) map[string]domain.RewriteStageDefinition {
	indexed := make(map[string]domain.RewriteStageDefinition, len(stages))
	for _, stage := range stages {
		indexed[stage.Name] = stage
	}
	return indexed
}

func shouldRouteStageToRepair(stage domain.RewriteStageDefinition, quality QualityResult) bool {
	return quality.Action == QualityDecisionRepair &&
		strings.TrimSpace(stage.OnFailure.Action) == QualityDecisionRepair &&
		strings.TrimSpace(stage.OnFailure.RepairStage) != ""
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

func (o *RewriteOrchestrator) persistFailedStageRun(ctx context.Context, pipelineRunID string, stage domain.RewriteStageDefinition, inputVars map[string]any, runErr error) error {
	if strings.TrimSpace(stage.Name) == "" {
		return nil
	}

	inputJSON, err := json.Marshal(inputVars)
	if err != nil {
		return fmt.Errorf("marshal rewrite stage input: %w", err)
	}
	completedAt := time.Now().UTC()
	stageRun := &domain.RewriteStageRun{
		ID:            id.New(),
		PipelineRunID: pipelineRunID,
		StageName:     stage.Name,
		StageType:     stage.Type,
		PromptRef:     stage.PromptRef,
		LLMProfileRef: stage.ModelProfileRef,
		Status:        domain.RewriteStageFailed,
		Attempt:       1,
		InputJSON:     string(inputJSON),
		OutputJSON:    "{}",
		ErrorSummary:  runErr.Error(),
		Metadata:      map[string]any{},
		StartedAt:     completedAt,
		CompletedAt:   &completedAt,
	}
	if err := o.stageRuns.Create(ctx, stageRun); err != nil {
		return err
	}
	return nil
}
