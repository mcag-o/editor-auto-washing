package service

import (
	"content-hub/domain"
	"strings"
)

const workflowPauseSourceMetadataKey = "pause_source"
const workflowPauseScopeMetadataKey = "pause_scope"
const workflowPauseReasonMetadataKey = "pause_reason"
const workflowPausePayloadMetadataKey = "pause_payload"
const workflowPauseAllowedResumeModesMetadataKey = "pause_allowed_resume_modes"

type WorkflowPauseScope string

const (
	WorkflowPauseScopeRun   WorkflowPauseScope = "run"
	WorkflowPauseScopeToken WorkflowPauseScope = "token"
)

func MarkWorkflowRunPaused(run *domain.WorkflowRun, checkpointID string, pauseState WorkflowPauseState) error {
	if run == nil {
		return domain.NewValidationErr("workflow run is required", nil)
	}
	if strings.TrimSpace(run.ID) == "" {
		return domain.NewValidationErr("workflow run id is required", nil)
	}
	if run.Status == domain.WorkflowRunSucceeded || run.Status == domain.WorkflowRunFailed {
		return domain.NewConflictErr("workflow run is already in a terminal state")
	}
	if strings.TrimSpace(checkpointID) == "" {
		return domain.NewValidationErr("workflow pause checkpoint id is required", nil)
	}
	metadata, err := workflowPauseCheckpointMetadata(pauseState)
	if err != nil {
		return err
	}
	run.Status = domain.WorkflowRunPaused
	run.Resumable = true
	run.ResumeFromCheckpointID = strings.TrimSpace(checkpointID)
	run.CompletedAt = nil
	run.ErrorSummary = ""
	run.FinalFailureClass = ""
	if run.Metadata == nil {
		run.Metadata = map[string]any{}
	}
	for key, value := range metadata {
		run.Metadata[key] = value
	}
	return nil
}

func pauseToken(token *WorkflowToken) {
	if token == nil {
		return
	}
	token.State = WorkflowTokenStatePaused
}

func appendManualPauseCheckpoint(ctx *WorkflowExecutionContext, workflowRunID, nodeID, reason string, payload map[string]any) {
	appendPauseCheckpoint(ctx, workflowRunID, nodeID, WorkflowPauseState{
		Source:             WorkflowPauseSourceManual,
		Scope:              WorkflowPauseScopeRun,
		Reason:             reason,
		Payload:            cloneWorkflowPayload(payload),
		AllowedResumeModes: []WorkflowResumeMode{WorkflowResumeModeContinueActiveTokens, WorkflowResumeModeReplayFromCheckpoint},
	})
}

func appendPolicyPauseCheckpoint(ctx *WorkflowExecutionContext, workflowRunID, nodeID, reason string, triggerContext map[string]any) {
	payload := map[string]any{"trigger_context": cloneWorkflowPayload(triggerContext)}
	appendPauseCheckpoint(ctx, workflowRunID, nodeID, WorkflowPauseState{
		Source:             WorkflowPauseSourcePolicy,
		Scope:              WorkflowPauseScopeRun,
		Reason:             reason,
		Payload:            payload,
		AllowedResumeModes: []WorkflowResumeMode{WorkflowResumeModeContinueActiveTokens, WorkflowResumeModeReplayFromCheckpoint},
	})
}

func appendPauseCheckpoint(ctx *WorkflowExecutionContext, workflowRunID, nodeID string, pauseState WorkflowPauseState) {
	appendCheckpointWithSnapshot(ctx, workflowRunID, nodeID, workflowCheckpointSnapshot{PauseState: &pauseState})
}

func workflowPauseCheckpointMetadata(pauseState WorkflowPauseState) (map[string]any, error) {
	if strings.TrimSpace(string(pauseState.Source)) == "" {
		return nil, domain.NewValidationErr("workflow pause source is required", nil)
	}
	switch pauseState.Source {
	case WorkflowPauseSourceHumanNode, WorkflowPauseSourceManual, WorkflowPauseSourcePolicy:
	default:
		return nil, domain.NewValidationErr("unsupported workflow pause source", nil)
	}
	if strings.TrimSpace(string(pauseState.Scope)) == "" {
		return nil, domain.NewValidationErr("workflow pause scope is required", nil)
	}
	switch pauseState.Scope {
	case WorkflowPauseScopeRun, WorkflowPauseScopeToken:
	default:
		return nil, domain.NewValidationErr("unsupported workflow pause scope", nil)
	}
	metadata := map[string]any{
		workflowPauseSourceMetadataKey: strings.TrimSpace(string(pauseState.Source)),
		workflowPauseScopeMetadataKey:  strings.TrimSpace(string(pauseState.Scope)),
		workflowPauseReasonMetadataKey: strings.TrimSpace(pauseState.Reason),
	}
	if len(pauseState.Payload) > 0 {
		metadata[workflowPausePayloadMetadataKey] = cloneWorkflowPayload(pauseState.Payload)
	}
	if len(pauseState.AllowedResumeModes) > 0 {
		modes := make([]string, 0, len(pauseState.AllowedResumeModes))
		for _, mode := range pauseState.AllowedResumeModes {
			trimmed := strings.TrimSpace(string(mode))
			if trimmed == "" {
				return nil, domain.NewValidationErr("workflow pause resume mode is required", nil)
			}
			switch mode {
			case WorkflowResumeModeContinueToken, WorkflowResumeModeContinueActiveTokens, WorkflowResumeModeReplayFromCheckpoint:
				modes = append(modes, trimmed)
			default:
				return nil, domain.NewValidationErr("unsupported workflow pause resume mode", nil)
			}
		}
		if len(modes) > 0 {
			metadata[workflowPauseAllowedResumeModesMetadataKey] = modes
		}
	}
	return metadata, nil
}
