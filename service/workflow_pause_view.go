package service

import (
	"content-hub/domain"
	"fmt"
	"sort"
	"strings"
	"time"
)

type WorkflowPauseView struct {
	Summary       WorkflowPausedRunSummary
	TaskItems     []WorkflowPausedTaskItem
	FullAuditRefs []domain.AuditLog
}

type WorkflowPausedRunSummary struct {
	RunID              string
	Status             string
	WorkflowID         string
	WorkflowVersion    string
	PauseSource        WorkflowPauseSource
	PauseReason        string
	AllowedResumeModes []WorkflowResumeMode
	AffectedTokenCount int
	AffectedTokenIDs   []string
	CurrentNodeIDs     []string
	CheckpointID       string
	WorkspaceArticleID string
}

type WorkflowPausedTaskItem struct {
	TaskID              string
	RunID               string
	TokenID             string
	PauseSource         WorkflowPauseSource
	PausedAt            time.Time
	NodeID              string
	Title               string
	Summary             string
	AllowedResumeModes  []WorkflowResumeMode
	AvailableActions    []string
	PausePayloadPreview map[string]any
	LatestAuditHint     string
}

func BuildWorkflowPauseView(run *domain.WorkflowRun, auditLogs []domain.AuditLog, checkpoints ...*domain.WorkflowCheckpoint) (WorkflowPauseView, error) {
	if run == nil {
		return WorkflowPauseView{}, domain.NewValidationErr("workflow run is required", nil)
	}
	_ = auditLogs
	for _, item := range checkpoints {
		if item == nil {
			continue
		}
		if checkpointRunID := strings.TrimSpace(item.WorkflowRunID); checkpointRunID != "" && checkpointRunID != strings.TrimSpace(run.ID) {
			return WorkflowPauseView{}, domain.NewValidationErr("workflow checkpoint does not belong to workflow run", nil)
		}
	}
	checkpoint, err := workflowPauseSummaryCheckpoint(run, checkpoints)
	if err != nil {
		return WorkflowPauseView{}, err
	}
	if checkpoint == nil {
		return WorkflowPauseView{}, domain.NewValidationErr("workflow checkpoint is required", nil)
	}

	pauseState := workflowPauseStateFromCheckpoint(checkpoint)
	activeTokens := workflowActiveTokensFromCheckpoint(checkpoint)
	affectedTokenIDs := make([]string, 0, len(activeTokens))
	currentNodeIDs := make([]string, 0, len(activeTokens)+1)
	seenNodeIDs := map[string]struct{}{}
	for _, token := range activeTokens {
		if token == nil {
			continue
		}
		if tokenID := strings.TrimSpace(token.ID); tokenID != "" {
			affectedTokenIDs = append(affectedTokenIDs, tokenID)
		}
		if nodeID := strings.TrimSpace(token.NodeID); nodeID != "" {
			if _, seen := seenNodeIDs[nodeID]; !seen {
				seenNodeIDs[nodeID] = struct{}{}
				currentNodeIDs = append(currentNodeIDs, nodeID)
			}
		}
	}
	if len(currentNodeIDs) == 0 {
		for _, nodeID := range []string{strings.TrimSpace(checkpoint.NodeID), strings.TrimSpace(run.CurrentNodeID)} {
			if nodeID == "" {
				continue
			}
			if _, seen := seenNodeIDs[nodeID]; seen {
				continue
			}
			seenNodeIDs[nodeID] = struct{}{}
			currentNodeIDs = append(currentNodeIDs, nodeID)
		}
	}
	summary := WorkflowPausedRunSummary{
		RunID:              strings.TrimSpace(run.ID),
		Status:             strings.TrimSpace(run.Status),
		WorkflowID:         strings.TrimSpace(run.WorkflowID),
		WorkflowVersion:    strings.TrimSpace(run.WorkflowVersion),
		PauseReason:        firstDraftString(checkpoint.Metadata, run.Metadata, workflowPauseReasonMetadataKey),
		AffectedTokenCount: len(affectedTokenIDs),
		AffectedTokenIDs:   affectedTokenIDs,
		CurrentNodeIDs:     currentNodeIDs,
		CheckpointID:       strings.TrimSpace(checkpoint.ID),
		WorkspaceArticleID: strings.TrimSpace(run.WorkspaceArticleID),
	}
	if pauseState != nil {
		summary.PauseSource = pauseState.Source
		summary.AllowedResumeModes = append([]WorkflowResumeMode(nil), pauseState.AllowedResumeModes...)
	}
	if summary.PauseSource == "" {
		summary.PauseSource = WorkflowPauseSource(firstDraftString(checkpoint.Metadata, run.Metadata, workflowPauseSourceMetadataKey))
	}
	if len(summary.AllowedResumeModes) == 0 {
		summary.AllowedResumeModes = append(summary.AllowedResumeModes, workflowPauseAllowedResumeModes(&domain.WorkflowCheckpoint{Metadata: run.Metadata})...)
	}
	if len(summary.AllowedResumeModes) == 0 {
		summary.AllowedResumeModes = append(summary.AllowedResumeModes, workflowPauseAllowedResumeModes(checkpoint)...)
	}
	taskItems := buildWorkflowPausedTaskItems(strings.TrimSpace(run.ID), checkpoints)
	fullAuditRefs := filterWorkflowPauseAuditLogs(auditLogs, strings.TrimSpace(run.ID))
	sort.SliceStable(fullAuditRefs, func(i, j int) bool {
		if fullAuditRefs[i].CreatedAt.Equal(fullAuditRefs[j].CreatedAt) {
			return strings.TrimSpace(fullAuditRefs[i].ID) > strings.TrimSpace(fullAuditRefs[j].ID)
		}
		return fullAuditRefs[i].CreatedAt.After(fullAuditRefs[j].CreatedAt)
	})
	sort.Strings(currentNodeIDs)
	return WorkflowPauseView{
		Summary:       summary,
		TaskItems:     taskItems,
		FullAuditRefs: fullAuditRefs,
	}, nil
}

func firstDraftString(primary, fallback map[string]any, key string) string {
	if value := strings.TrimSpace(domain.DraftString(primary[key])); value != "" {
		return value
	}
	return strings.TrimSpace(domain.DraftString(fallback[key]))
}

func workflowPauseSummaryCheckpoint(run *domain.WorkflowRun, checkpoints []*domain.WorkflowCheckpoint) (*domain.WorkflowCheckpoint, error) {
	resumeFromCheckpointID := ""
	if run != nil {
		resumeFromCheckpointID = strings.TrimSpace(run.ResumeFromCheckpointID)
	}
	if resumeFromCheckpointID != "" {
		for _, checkpoint := range checkpoints {
			if checkpoint == nil {
				continue
			}
			if strings.TrimSpace(checkpoint.ID) == resumeFromCheckpointID {
				return checkpoint, nil
			}
		}
		return nil, domain.NewValidationErr("workflow resume checkpoint not found", nil)
	}
	var selected *domain.WorkflowCheckpoint
	for _, checkpoint := range checkpoints {
		if checkpoint == nil || checkpoint.State != domain.WorkflowCheckpointStateActive || !checkpoint.Resumable {
			continue
		}
		if selected == nil || checkpoint.CreatedAt.After(selected.CreatedAt) || (checkpoint.CreatedAt.Equal(selected.CreatedAt) && strings.TrimSpace(checkpoint.ID) > strings.TrimSpace(selected.ID)) {
			selected = checkpoint
		}
	}
	if selected != nil {
		return selected, nil
	}
	for _, checkpoint := range checkpoints {
		if checkpoint != nil {
			return checkpoint, nil
		}
	}
	return nil, nil
}

func buildWorkflowPausedTaskItems(runID string, checkpoints []*domain.WorkflowCheckpoint) []WorkflowPausedTaskItem {
	items := make([]WorkflowPausedTaskItem, 0, len(checkpoints))
	for _, checkpoint := range checkpoints {
		projected := workflowPausedTaskItemsFromCheckpoint(runID, checkpoint)
		if len(projected) == 0 {
			continue
		}
		items = append(items, projected...)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].PausedAt.Equal(items[j].PausedAt) {
			return items[i].TaskID < items[j].TaskID
		}
		return items[i].PausedAt.After(items[j].PausedAt)
	})
	return items
}

func workflowPausedTaskItemsFromCheckpoint(runID string, checkpoint *domain.WorkflowCheckpoint) []WorkflowPausedTaskItem {
	if checkpoint == nil || checkpoint.State != domain.WorkflowCheckpointStateActive || !checkpoint.Resumable {
		return nil
	}
	pauseState := workflowPauseStateFromCheckpoint(checkpoint)
	if pauseState == nil {
		return nil
	}
	tokens := workflowActiveTokensFromCheckpoint(checkpoint)
	if len(tokens) == 0 && pauseState.Scope == WorkflowPauseScopeRun {
		return []WorkflowPausedTaskItem{workflowPausedRunScopedTaskItem(runID, checkpoint, pauseState)}
	}
	if len(tokens) == 0 {
		return nil
	}
	return workflowPausedTaskItemsFromTokens(runID, checkpoint, pauseState, tokens)
}

func workflowPausedRunScopedTaskItem(runID string, checkpoint *domain.WorkflowCheckpoint, pauseState *WorkflowPauseState) WorkflowPausedTaskItem {
	return WorkflowPausedTaskItem{
		TaskID:              workflowPausedTaskID(runID, strings.TrimSpace(checkpoint.ID), strings.TrimSpace(checkpoint.NodeID)),
		RunID:               strings.TrimSpace(runID),
		PauseSource:         pauseState.Source,
		PausedAt:            checkpoint.CreatedAt,
		NodeID:              strings.TrimSpace(checkpoint.NodeID),
		Title:               workflowPauseTaskTitle(pauseState.Source),
		Summary:             strings.TrimSpace(pauseState.Reason),
		AllowedResumeModes:  append([]WorkflowResumeMode(nil), pauseState.AllowedResumeModes...),
		AvailableActions:    workflowPauseAvailableActions(pauseState.Source, pauseState.AllowedResumeModes),
		PausePayloadPreview: cloneWorkflowPayload(pauseState.Payload),
		LatestAuditHint:     workflowLatestAuditHint(checkpoint),
	}
}

func workflowPausedTaskItemsFromTokens(runID string, checkpoint *domain.WorkflowCheckpoint, pauseState *WorkflowPauseState, tokens []*WorkflowToken) []WorkflowPausedTaskItem {
	items := make([]WorkflowPausedTaskItem, 0, len(tokens))
	for _, token := range tokens {
		if token == nil {
			continue
		}
		tokenID := strings.TrimSpace(token.ID)
		nodeID := strings.TrimSpace(token.NodeID)
		if nodeID == "" {
			nodeID = strings.TrimSpace(checkpoint.NodeID)
		}
		if nodeID == "" {
			continue
		}
		preview := cloneWorkflowPayload(pauseState.Payload)
		if pauseState.Source == WorkflowPauseSourceHumanNode {
			preview = workflowHumanPausePayloadPreview(pauseState.Payload)
		}
		item := WorkflowPausedTaskItem{
			TaskID:              workflowPausedTaskID(runID, tokenID, nodeID),
			RunID:               runID,
			TokenID:             tokenID,
			PauseSource:         pauseState.Source,
			PausedAt:            checkpoint.CreatedAt,
			NodeID:              nodeID,
			Title:               workflowPauseTaskTitle(pauseState.Source),
			Summary:             strings.TrimSpace(pauseState.Reason),
			AllowedResumeModes:  append([]WorkflowResumeMode(nil), pauseState.AllowedResumeModes...),
			AvailableActions:    workflowPauseAvailableActions(pauseState.Source, pauseState.AllowedResumeModes),
			PausePayloadPreview: preview,
			LatestAuditHint:     workflowLatestAuditHint(checkpoint),
		}
		items = append(items, item)
	}
	return items
}

func workflowPausedTaskID(runID, tokenID, nodeID string) string {
	parts := []string{strings.TrimSpace(runID), strings.TrimSpace(tokenID), strings.TrimSpace(nodeID)}
	for i, part := range parts {
		if part == "" {
			parts[i] = "unknown"
		}
	}
	return fmt.Sprintf("%s:%s:%s", parts[0], parts[1], parts[2])
}
