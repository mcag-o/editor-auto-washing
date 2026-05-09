package service

import (
	"content-hub/domain"
	"fmt"
	"strings"
)

const workflowResumeModeMetadataKey = "resume_mode"
const workflowResumeOperatorNoteMetadataKey = "resume_operator_note"

type WorkflowResumeCommand struct {
	CheckpointID string
	Mode         WorkflowResumeMode
	HumanInput   map[string]any
	OperatorNote string
}

func ResolveWorkflowResumeMode(checkpoint *domain.WorkflowCheckpoint, requested WorkflowResumeMode) (WorkflowResumeMode, error) {
	if strings.TrimSpace(string(requested)) == "" {
		return defaultWorkflowResumeMode(checkpoint), nil
	}
	return resolveWorkflowResumeMode(checkpoint, requested)
}

func resolveWorkflowResumeMode(checkpoint *domain.WorkflowCheckpoint, requested WorkflowResumeMode) (WorkflowResumeMode, error) {
	requested = WorkflowResumeMode(strings.TrimSpace(string(requested)))
	allowed := workflowPauseAllowedResumeModes(checkpoint)
	if requested == "" {
		return "", domain.NewValidationErr("workflow resume mode is required", nil)
	}
	if !isSupportedWorkflowResumeMode(requested) {
		return "", domain.NewValidationErr("unsupported workflow resume mode", nil)
	}
	if len(allowed) == 0 {
		return requested, nil
	}
	for _, mode := range allowed {
		if mode == requested {
			return requested, nil
		}
	}
	return "", domain.NewConflictErr(fmt.Sprintf("workflow pause does not allow resume mode %s", requested))
}

func defaultWorkflowResumeMode(checkpoint *domain.WorkflowCheckpoint) WorkflowResumeMode {
	allowed := workflowPauseAllowedResumeModes(checkpoint)
	for _, mode := range allowed {
		if mode == WorkflowResumeModeContinueToken {
			return WorkflowResumeModeContinueToken
		}
	}
	for _, mode := range allowed {
		if mode == WorkflowResumeModeContinueActiveTokens || mode == WorkflowResumeModeReplayFromCheckpoint {
			return mode
		}
	}
	if len(workflowActiveTokensFromCheckpoint(checkpoint)) > 1 {
		return WorkflowResumeModeContinueActiveTokens
	}
	return WorkflowResumeModeContinueToken
}

func workflowPauseAllowedResumeModes(checkpoint *domain.WorkflowCheckpoint) []WorkflowResumeMode {
	if checkpoint == nil || len(checkpoint.Metadata) == 0 {
		return nil
	}
	rawModes, ok := checkpoint.Metadata[workflowPauseAllowedResumeModesMetadataKey]
	if !ok {
		return nil
	}
	if typed, ok := rawModes.([]string); ok {
		modes := make([]WorkflowResumeMode, 0, len(typed))
		for _, item := range typed {
			mode := WorkflowResumeMode(strings.TrimSpace(item))
			if isSupportedWorkflowResumeMode(mode) {
				modes = append(modes, mode)
			}
		}
		return modes
	}
	items, ok := rawModes.([]any)
	if !ok {
		return nil
	}
	modes := make([]WorkflowResumeMode, 0, len(items))
	for _, item := range items {
		mode := WorkflowResumeMode(strings.TrimSpace(domain.DraftString(item)))
		if isSupportedWorkflowResumeMode(mode) {
			modes = append(modes, mode)
		}
	}
	return modes
}

func isSupportedWorkflowResumeMode(mode WorkflowResumeMode) bool {
	switch mode {
	case WorkflowResumeModeContinueToken, WorkflowResumeModeContinueActiveTokens, WorkflowResumeModeReplayFromCheckpoint:
		return true
	default:
		return false
	}
}

func recordWorkflowResumeCommand(checkpoint *domain.WorkflowCheckpoint, mode WorkflowResumeMode, command WorkflowResumeCommand) {
	if checkpoint == nil {
		return
	}
	if checkpoint.Metadata == nil {
		checkpoint.Metadata = map[string]any{}
	}
	checkpoint.Metadata[workflowResumeModeMetadataKey] = strings.TrimSpace(string(mode))
	if note := strings.TrimSpace(command.OperatorNote); note != "" {
		checkpoint.Metadata[workflowResumeOperatorNoteMetadataKey] = note
	}
	checkpoint.Metadata[workflowLatestAuditHintMetadataKey] = workflowResumeAuditHint(mode, command)
	if command.HumanInput != nil {
		for key, value := range workflowHumanResumeInputMetadata(command.HumanInput, true) {
			checkpoint.Metadata[key] = value
		}
	}
}

func workflowResumeAuditHint(mode WorkflowResumeMode, command WorkflowResumeCommand) string {
	note := strings.TrimSpace(command.OperatorNote)
	if note != "" {
		return note
	}
	if mode == "" {
		return ""
	}
	return "resume:" + strings.TrimSpace(string(mode))
}

func workflowLatestAuditHint(checkpoint *domain.WorkflowCheckpoint) string {
	if checkpoint == nil || len(checkpoint.Metadata) == 0 {
		return ""
	}
	return strings.TrimSpace(domain.DraftString(checkpoint.Metadata[workflowLatestAuditHintMetadataKey]))
}

func applyWorkflowResumeTarget(runtimeCtx *WorkflowExecutionContext, checkpoint *domain.WorkflowCheckpoint, mode WorkflowResumeMode) {
	if runtimeCtx == nil {
		return
	}
	switch mode {
	case WorkflowResumeModeContinueToken:
		runtimeCtx.CurrentToken = workflowTokenFromCheckpoint(checkpoint)
		if runtimeCtx.CurrentToken == nil {
			active := workflowActiveTokensFromCheckpoint(checkpoint)
			if len(active) > 0 {
				runtimeCtx.CurrentToken = active[len(active)-1]
			}
		}
		if runtimeCtx.CurrentToken != nil {
			runtimeCtx.ActiveTokens = []*WorkflowToken{runtimeCtx.CurrentToken}
		} else {
			runtimeCtx.ActiveTokens = nil
		}
	default:
		runtimeCtx.ActiveTokens = workflowActiveTokensFromCheckpoint(checkpoint)
		if len(runtimeCtx.ActiveTokens) > 0 {
			runtimeCtx.CurrentToken = runtimeCtx.ActiveTokens[len(runtimeCtx.ActiveTokens)-1]
		} else {
			runtimeCtx.CurrentToken = workflowTokenFromCheckpoint(checkpoint)
			if runtimeCtx.CurrentToken != nil {
				runtimeCtx.ActiveTokens = []*WorkflowToken{runtimeCtx.CurrentToken}
			}
		}
	}
}
