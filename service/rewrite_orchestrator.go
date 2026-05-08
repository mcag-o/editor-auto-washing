package service

import (
	"content-hub/domain"
	"content-hub/pkg/id"
	"content-hub/pkg/repo"
	"context"
	"encoding/json"
	"errors"
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
	WorkspaceArticleID string         `json:"workspace_article_id" binding:"required"`
	CollectorArticleID string         `json:"collector_article_id" binding:"required"`
	Title              string         `json:"title" binding:"required"`
	TargetType         string         `json:"target_type" binding:"required"`
	SourceProfile      string         `json:"source_profile" binding:"required"`
	Version            string         `json:"version" binding:"required"`
	Metadata           map[string]any `json:"metadata"`
}

type RewriteOrchestrator struct {
	profiles    *RewriteProfileRegistry
	runs        repo.RewritePipelineRunRepo
	stageRuns   repo.RewriteStageRunRepo
	workspaces  repo.WorkspaceRepo
	executor    *RewriteStageExecutor
	materialize *DraftMaterializer
	kernel      *RewriteKernelRunner
}

func NewRewriteOrchestrator(profiles *RewriteProfileRegistry, runs repo.RewritePipelineRunRepo, stageRuns repo.RewriteStageRunRepo, workspaces repo.WorkspaceRepo, executor *RewriteStageExecutor, materializer *DraftMaterializer) *RewriteOrchestrator {
	return &RewriteOrchestrator{
		profiles:    profiles,
		runs:        runs,
		stageRuns:   stageRuns,
		workspaces:  workspaces,
		executor:    executor,
		materialize: materializer,
		kernel:      NewRewriteKernelRunner(runs, stageRuns, executor, materializer),
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
	workspaceSnapshot, err := o.workspaces.GetByID(ctx, strings.TrimSpace(req.WorkspaceArticleID))
	if err != nil {
		return nil, err
	}
	workspaceSnapshot = cloneArticleWorkspaceRecord(workspaceSnapshot)

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
		return nil, o.rollbackCreatedRun(ctx, run, err)
	}
	if err := o.workspaces.TransitionStatus(ctx, req.WorkspaceArticleID, domain.ArticleWorkspaceStatusRewriting, rewritingNote); err != nil {
		workspaceErr := o.restoreWorkspaceSnapshot(ctx, workspaceSnapshot)
		return nil, o.rollbackCreatedRun(ctx, run, combineRewriteRollbackError(err, workspaceErr))
	}

	return o.executeRun(ctx, run, profile, strings.TrimSpace(req.WorkspaceArticleID), req.Title, false)
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
		appErr, ok := err.(*domain.AppError)
		if !ok || appErr.Code != domain.ErrConflict {
			return nil, err
		}
	}
	if err := o.runs.Update(ctx, run); err != nil {
		return nil, err
	}
	return o.executeRun(ctx, run, profile, run.WorkspaceArticleID, title, true)
}

func (o *RewriteOrchestrator) rollbackCreatedRun(ctx context.Context, run *domain.RewritePipelineRun, cause error) error {
	if run == nil {
		return cause
	}
	if err := o.runs.Delete(ctx, run.ID); err != nil {
		return fmt.Errorf("%w: rollback rewrite run create: %v", cause, err)
	}
	return cause
}

func combineRewriteRollbackError(cause error, rollbackErr error) error {
	if rollbackErr == nil {
		return cause
	}
	if cause == nil {
		return rollbackErr
	}
	return fmt.Errorf("%w: rollback workspace transition: %v", cause, rollbackErr)
}

func (o *RewriteOrchestrator) restoreWorkspaceSnapshot(ctx context.Context, workspace *domain.ArticleWorkspaceRecord) error {
	if workspace == nil {
		return nil
	}
	if err := o.workspaces.Delete(ctx, workspace.ID); err != nil {
		return err
	}
	restored := cloneArticleWorkspaceRecord(workspace)
	if restored == nil {
		return nil
	}
	return o.workspaces.Create(ctx, restored)
}

func cloneArticleWorkspaceRecord(workspace *domain.ArticleWorkspaceRecord) *domain.ArticleWorkspaceRecord {
	if workspace == nil {
		return nil
	}
	cloned := *workspace
	cloned.StatusHistory = append([]string(nil), workspace.StatusHistory...)
	cloned.LifecycleHistory = append([]domain.ArticleWorkspaceLifecycleEntry(nil), workspace.LifecycleHistory...)
	if workspace.Metadata != nil {
		cloned.Metadata = make(map[string]any, len(workspace.Metadata))
		for key, value := range workspace.Metadata {
			cloned.Metadata[key] = value
		}
	}
	return &cloned
}

func (o *RewriteOrchestrator) executeRun(ctx context.Context, run *domain.RewritePipelineRun, profile *domain.RewritePipelineProfile, workspaceArticleID string, title string, resume bool) (*domain.RewritePipelineRun, error) {
	var (
		draftID string
		err     error
	)
	if resume {
		draftID, err = o.kernel.Resume(ctx, run, profile, workspaceArticleID, title)
	} else {
		draftID, err = o.kernel.Run(ctx, run, profile, workspaceArticleID, title)
	}
	if err != nil {
		stage, inputVars, cause := rewriteKernelFailureContext(err, run.CurrentStage)
		return o.failRun(ctx, run, stage, inputVars, cause)
	}
	completedAt := time.Now().UTC()
	run.Status = domain.RewriteRunSucceeded
	run.FinalDraftID = draftID
	run.CurrentStage = rewriteWorkflowMaterializeNodeID
	run.CompletedAt = &completedAt
	if err := o.runs.Update(ctx, run); err != nil {
		return nil, err
	}

	return run, nil
}

func rewriteKernelFailureContext(err error, fallbackStageName string) (domain.RewriteStageDefinition, map[string]any, error) {
	var stageErr *rewriteKernelStageFailure
	if errors.As(err, &stageErr) && stageErr != nil {
		return stageErr.Stage, cloneWorkflowPayload(stageErr.InputVars), stageErr.Cause
	}
	return domain.RewriteStageDefinition{Name: fallbackStageName}, nil, err
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
	if o.profiles == nil || o.runs == nil || o.stageRuns == nil || o.workspaces == nil || o.executor == nil || o.materialize == nil || o.kernel == nil {
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
