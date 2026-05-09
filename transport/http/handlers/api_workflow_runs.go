package handlers

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"content-hub/service"
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const workflowRunOperationsActor = "local-admin"

const workflowPauseSourceMetadataKey = "pause_source"
const workflowPauseReasonMetadataKey = "pause_reason"
const workflowPausePayloadMetadataKey = "pause_payload"
const workflowPauseAllowedResumeModesMetadataKey = "pause_allowed_resume_modes"
const workflowHumanResumeInputMetadataKey = "human_resume_input"

type APIWorkflowRunsHandler struct {
	runs        repo.WorkflowRunRepo
	checkpoints repo.WorkflowCheckpointRepo
	audit       repo.AuditLogRepo
}

func NewAPIWorkflowRunsHandler(runs repo.WorkflowRunRepo, checkpoints repo.WorkflowCheckpointRepo, audit repo.AuditLogRepo) *APIWorkflowRunsHandler {
	return &APIWorkflowRunsHandler{runs: runs, checkpoints: checkpoints, audit: audit}
}

func (h *APIWorkflowRunsHandler) Get(c *gin.Context) {
	run, checkpoint, err := h.loadPausedRun(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}

	pausePayload, _ := checkpoint.Metadata[workflowPausePayloadMetadataKey].(map[string]any)
	c.JSON(http.StatusOK, gin.H{
		"id":                   run.ID,
		"status":               run.Status,
		"workflow_id":          run.WorkflowID,
		"workflow_version":     run.WorkflowVersion,
		"workspace_article_id": run.WorkspaceArticleID,
		"current_node_id":      run.CurrentNodeID,
		"checkpoint_id":        checkpoint.ID,
		"pause_source":         draftString(run.Metadata[workflowPauseSourceMetadataKey], checkpoint.Metadata[workflowPauseSourceMetadataKey]),
		"pause_reason":         draftString(run.Metadata[workflowPauseReasonMetadataKey], checkpoint.Metadata[workflowPauseReasonMetadataKey]),
		"pause_payload":        pausePayload,
		"affected_token": map[string]any{
			"token_id":       draftString(valueFromMap(pausePayload, "token_id"), checkpoint.ResumeToken),
			"node_id":        draftString(valueFromMap(pausePayload, "node_id"), checkpoint.NodeID),
			"resume_token":   checkpoint.ResumeToken,
			"checkpoint_id":  checkpoint.ID,
			"checkpoint_node": checkpoint.NodeID,
		},
		"allowed_resume_modes": workflowResumeModes(run, checkpoint),
	})
}

func (h *APIWorkflowRunsHandler) PauseView(c *gin.Context) {
	run, checkpoint, err := h.loadPausedRun(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	checkpoints, err := h.checkpoints.ListByWorkflowRunID(c.Request.Context(), run.ID)
	if err != nil {
		HandleError(c, err)
		return
	}
	auditLogs, err := h.audit.ListByQuery(c.Request.Context(), repo.AuditLogQuery{WorkflowRunID: run.ID})
	if err != nil {
		HandleError(c, err)
		return
	}
	view, err := service.BuildWorkflowPauseView(run, auditLogs, checkpointsToPointers(checkpoints)...)
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, workflowPauseViewResponse(run, checkpoint, view))
}

func (h *APIWorkflowRunsHandler) Audit(c *gin.Context) {
	workflowRunID := strings.TrimSpace(c.Param("id"))
	if workflowRunID == "" {
		HandleError(c, domain.NewValidationErr("workflow run id is required", nil))
		return
	}
	if h == nil || h.audit == nil {
		HandleError(c, domain.NewInternalErr("audit log repo is not configured", nil))
		return
	}
	logs, err := h.audit.ListByQuery(c.Request.Context(), repo.AuditLogQuery{
		WorkflowRunID: workflowRunID,
		ActionPrefix:  strings.TrimSpace(c.Query("action_prefix")),
		ResourceID:    strings.TrimSpace(c.Query("resource_id")),
	})
	if err != nil {
		HandleError(c, err)
		return
	}
	sort.SliceStable(logs, func(i, j int) bool {
		if logs[i].CreatedAt.Equal(logs[j].CreatedAt) {
			return strings.TrimSpace(logs[i].ID) > strings.TrimSpace(logs[j].ID)
		}
		return logs[i].CreatedAt.After(logs[j].CreatedAt)
	})
	c.JSON(http.StatusOK, logs)
}

func (h *APIWorkflowRunsHandler) Pause(c *gin.Context) {
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	run, err := h.runs.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	if run.Status != domain.WorkflowRunRunning && run.Status != domain.WorkflowRunPending {
		HandleError(c, domain.NewValidationErr("pause is only allowed from pending or running state", nil))
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "manual pause requested"
	}
	checkpointID := strings.TrimSpace(run.ResumeFromCheckpointID)
	if checkpointID == "" {
		checkpointID = "manual-pause-" + run.ID
	}
	if err := service.MarkWorkflowRunPaused(run, checkpointID, service.WorkflowPauseState{
		Source:             service.WorkflowPauseSourceManual,
		Scope:              service.WorkflowPauseScopeRun,
		Reason:             reason,
		Payload:            map[string]any{"workflow_run_id": run.ID},
		AllowedResumeModes: []service.WorkflowResumeMode{service.WorkflowResumeModeContinueActiveTokens, service.WorkflowResumeModeReplayFromCheckpoint},
	}); err != nil {
		HandleError(c, err)
		return
	}
	now := time.Now().UTC()
	checkpoint := &domain.WorkflowCheckpoint{
		ID:            checkpointID,
		WorkflowRunID: run.ID,
		NodeID:        strings.TrimSpace(run.CurrentNodeID),
		State:         domain.WorkflowCheckpointStateActive,
		Resumable:     true,
		ResumeToken:   checkpointID,
		CreatedAt:     now,
		Metadata: map[string]any{
			workflowPauseSourceMetadataKey:             string(service.WorkflowPauseSourceManual),
			workflowPauseReasonMetadataKey:             reason,
			workflowPausePayloadMetadataKey:            map[string]any{"workflow_run_id": run.ID},
			workflowPauseAllowedResumeModesMetadataKey: []string{string(service.WorkflowResumeModeContinueActiveTokens), string(service.WorkflowResumeModeReplayFromCheckpoint)},
		},
	}
	items, listErr := h.checkpoints.ListByWorkflowRunID(c.Request.Context(), run.ID)
	if listErr != nil {
		HandleError(c, listErr)
		return
	}
	found := false
	for _, item := range items {
		if item.ID == checkpoint.ID {
			found = true
			break
		}
	}
	if found {
		if err := h.checkpoints.Update(c.Request.Context(), checkpoint); err != nil {
			HandleError(c, err)
			return
		}
	} else {
		if err := h.checkpoints.Create(c.Request.Context(), checkpoint); err != nil {
			HandleError(c, err)
			return
		}
	}
	if err := h.runs.Update(c.Request.Context(), run); err != nil {
		HandleError(c, err)
		return
	}
	h.recordAuditBestEffort(c.Request.Context(), run, "pause", "success", "paused workflow run", map[string]any{
		"workflow_run_id":      run.ID,
		"pause_source":         string(service.WorkflowPauseSourceManual),
		"pause_reason":         reason,
		"allowed_resume_modes": []string{string(service.WorkflowResumeModeContinueActiveTokens), string(service.WorkflowResumeModeReplayFromCheckpoint)},
		"checkpoint_id":        checkpointID,
	})
	c.JSON(http.StatusAccepted, gin.H{
		"status":               domain.WorkflowRunPaused,
		"pause_source":         string(service.WorkflowPauseSourceManual),
		"pause_reason":         reason,
		"allowed_resume_modes": []string{string(service.WorkflowResumeModeContinueActiveTokens), string(service.WorkflowResumeModeReplayFromCheckpoint)},
		"workflow_run":         run,
	})
}

func (h *APIWorkflowRunsHandler) Resume(c *gin.Context) {
	var req struct {
		Action     string         `json:"action"`
		ResumeMode  string         `json:"resume_mode"`
		FormValues  map[string]any `json:"form_values"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	run, checkpoint, err := h.loadPausedRun(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	requestedMode := service.WorkflowResumeMode(strings.TrimSpace(req.ResumeMode))
	resumeMode, err := service.ResolveWorkflowResumeMode(checkpoint, requestedMode)
	if err != nil {
		HandleError(c, err)
		return
	}
	if checkpoint.Metadata == nil {
		checkpoint.Metadata = map[string]any{}
	}
	checkpoint.Metadata[workflowHumanResumeInputMetadataKey] = map[string]any{
		"submitted": true,
		"action": map[string]any{
			"value": strings.TrimSpace(req.Action),
		},
		"form": cloneMap(req.FormValues),
	}
	checkpoint.State = domain.WorkflowCheckpointStateTerminal
	checkpoint.Resumable = false
	checkpoint.ConsumedAt = timePointer(time.Now().UTC())
	if err := h.checkpoints.Update(c.Request.Context(), checkpoint); err != nil {
		HandleError(c, err)
		return
	}
	run.Status = domain.WorkflowRunRunning
	run.Resumable = false
	run.ResumeFromCheckpointID = ""
	run.CompletedAt = nil
	if run.Metadata == nil {
		run.Metadata = map[string]any{}
	}
	run.Metadata["resume_mode"] = resumeMode
	if err := h.runs.Update(c.Request.Context(), run); err != nil {
		HandleError(c, err)
		return
	}
	h.recordAuditBestEffort(c.Request.Context(), run, "resume", "success", "resumed workflow run", map[string]any{
		"workflow_run_id": run.ID,
		"checkpoint_id": checkpoint.ID,
		"resume_mode":   resumeMode,
		"action":        strings.TrimSpace(req.Action),
		"form_values":   cloneMap(req.FormValues),
	})
	c.JSON(http.StatusAccepted, gin.H{
		"status":       "resumed",
		"id":           run.ID,
		"action":       strings.TrimSpace(req.Action),
		"form_values":  cloneMap(req.FormValues),
		"resume_mode":  resumeMode,
		"checkpoint_id": checkpoint.ID,
	})
}

func (h *APIWorkflowRunsHandler) loadPausedRun(ctx context.Context, id string) (*domain.WorkflowRun, *domain.WorkflowCheckpoint, error) {
	run, err := h.runs.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if run.Status != domain.WorkflowRunPaused {
		return nil, nil, domain.NewValidationErr("workflow run is not paused", nil)
	}
	if h.checkpoints == nil {
		return nil, nil, domain.NewInternalErr("workflow checkpoint repo is not configured", nil)
	}
	checkpoints, err := h.checkpoints.ListByWorkflowRunID(ctx, run.ID)
	if err != nil {
		return nil, nil, err
	}
	checkpointID := strings.TrimSpace(run.ResumeFromCheckpointID)
	for i := range checkpoints {
		if strings.TrimSpace(checkpoints[i].ID) == checkpointID {
			checkpoint := checkpoints[i]
			return run, &checkpoint, nil
		}
	}
	if selected := newestResumableCheckpoint(checkpoints); selected != nil {
		return run, selected, nil
	}
	return nil, nil, domain.NewNotFoundErr("workflow_checkpoint", checkpointID)
}

func newestResumableCheckpoint(checkpoints []domain.WorkflowCheckpoint) *domain.WorkflowCheckpoint {
	var selected *domain.WorkflowCheckpoint
	for i := range checkpoints {
		candidate := checkpoints[i]
		if candidate.State != domain.WorkflowCheckpointStateActive || !candidate.Resumable {
			continue
		}
		if selected == nil || candidate.CreatedAt.After(selected.CreatedAt) || (candidate.CreatedAt.Equal(selected.CreatedAt) && strings.TrimSpace(candidate.ID) > strings.TrimSpace(selected.ID)) {
			copyValue := candidate
			selected = &copyValue
		}
	}
	return selected
}

func (h *APIWorkflowRunsHandler) recordAuditBestEffort(ctx context.Context, run *domain.WorkflowRun, action, result, message string, metadata map[string]any) {
	if h == nil || h.audit == nil || run == nil {
		return
	}
	_, _ = service.NewAuditLogService(h.audit).Create(ctx, service.AuditLogCreateInput{
		Actor:      workflowRunOperationsActor,
		Action:     "web_control.workflow_run." + strings.TrimSpace(action),
		Resource:   "workflow_run",
		ResourceID: run.ID,
		Result:     strings.TrimSpace(result),
		Message:    strings.TrimSpace(message),
		Metadata:   metadata,
	})
}

func workflowResumeModes(run *domain.WorkflowRun, checkpoint *domain.WorkflowCheckpoint) []any {
	if checkpoint != nil && checkpoint.Metadata != nil {
		if modes, ok := checkpoint.Metadata[workflowPauseAllowedResumeModesMetadataKey].([]string); ok {
			out := make([]any, 0, len(modes))
			for _, mode := range modes {
				out = append(out, mode)
			}
			return out
		}
		if modes, ok := checkpoint.Metadata[workflowPauseAllowedResumeModesMetadataKey].([]any); ok {
			return modes
		}
	}
	if run != nil && run.Metadata != nil {
		if modes, ok := run.Metadata[workflowPauseAllowedResumeModesMetadataKey].([]string); ok {
			out := make([]any, 0, len(modes))
			for _, mode := range modes {
				out = append(out, mode)
			}
			return out
		}
		if modes, ok := run.Metadata[workflowPauseAllowedResumeModesMetadataKey].([]any); ok {
			return modes
		}
	}
	return nil
}

func draftString(values ...any) string {
	for _, value := range values {
		text, _ := value.(string)
		if strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func valueFromMap(values map[string]any, key string) any {
	if len(values) == 0 {
		return nil
	}
	return values[key]
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	out := map[string]any{}
	for key, value := range input {
		out[key] = value
	}
	return out
}

func timePointer(value time.Time) *time.Time {
	copyValue := value
	return &copyValue
}

func workflowPauseViewResponse(run *domain.WorkflowRun, checkpoint *domain.WorkflowCheckpoint, view service.WorkflowPauseView) gin.H {
	taskItems := view.TaskItems
	if len(taskItems) == 0 {
		if fallback, ok := workflowPauseViewFallbackTaskItem(run, checkpoint, view.Summary); ok {
			taskItems = []service.WorkflowPausedTaskItem{fallback}
		}
	}
	serializedTaskItems := make([]gin.H, 0, len(taskItems))
	for _, item := range taskItems {
		allowedResumeModes := make([]string, 0, len(item.AllowedResumeModes))
		for _, mode := range item.AllowedResumeModes {
			allowedResumeModes = append(allowedResumeModes, string(mode))
		}
		serializedTaskItems = append(serializedTaskItems, gin.H{
			"taskId":              item.TaskID,
			"runId":               item.RunID,
			"tokenId":             item.TokenID,
			"pauseSource":         string(item.PauseSource),
			"pausedAt":            item.PausedAt,
			"nodeId":              item.NodeID,
			"title":               item.Title,
			"summary":             item.Summary,
			"allowedResumeModes":  allowedResumeModes,
			"availableActions":    append([]string(nil), item.AvailableActions...),
			"pausePayloadPreview": cloneMap(item.PausePayloadPreview),
			"latestAuditHint":     item.LatestAuditHint,
		})
	}
	allowedResumeModes := make([]string, 0, len(view.Summary.AllowedResumeModes))
	for _, mode := range view.Summary.AllowedResumeModes {
		allowedResumeModes = append(allowedResumeModes, string(mode))
	}
	return gin.H{
		"summary": gin.H{
			"runId":              view.Summary.RunID,
			"status":             view.Summary.Status,
			"workflowId":         view.Summary.WorkflowID,
			"workflowVersion":    view.Summary.WorkflowVersion,
			"pauseSource":        string(view.Summary.PauseSource),
			"pauseReason":        view.Summary.PauseReason,
			"allowedResumeModes": allowedResumeModes,
			"affectedTokenCount": view.Summary.AffectedTokenCount,
			"affectedTokenIDs":   append([]string(nil), view.Summary.AffectedTokenIDs...),
			"currentNodeIDs":     append([]string(nil), view.Summary.CurrentNodeIDs...),
			"checkpointId":       view.Summary.CheckpointID,
			"workspaceArticleId": view.Summary.WorkspaceArticleID,
		},
		"taskItems":     serializedTaskItems,
		"fullAuditRefs": view.FullAuditRefs,
	}
}

func workflowPauseViewFallbackTaskItem(run *domain.WorkflowRun, checkpoint *domain.WorkflowCheckpoint, summary service.WorkflowPausedRunSummary) (service.WorkflowPausedTaskItem, bool) {
	if checkpoint == nil {
		return service.WorkflowPausedTaskItem{}, false
	}
	pauseSource := summary.PauseSource
	if pauseSource == "" {
		pauseSource = service.WorkflowPauseSource(draftString(checkpoint.Metadata[workflowPauseSourceMetadataKey], run.Metadata[workflowPauseSourceMetadataKey]))
	}
	if pauseSource == "" {
		return service.WorkflowPausedTaskItem{}, false
	}
	return service.WorkflowPausedTaskItem{
		TaskID:              workflowPauseViewTaskID(strings.TrimSpace(summary.RunID), strings.TrimSpace(checkpoint.ResumeToken), strings.TrimSpace(checkpoint.NodeID), strings.TrimSpace(summary.CheckpointID)),
		RunID:               strings.TrimSpace(summary.RunID),
		TokenID:             strings.TrimSpace(checkpoint.ResumeToken),
		PauseSource:         pauseSource,
		PausedAt:            checkpoint.CreatedAt,
		NodeID:              strings.TrimSpace(checkpoint.NodeID),
		Title:               workflowPauseViewTaskTitle(pauseSource, ""),
		Summary:             summary.PauseReason,
		AllowedResumeModes:  append([]service.WorkflowResumeMode(nil), summary.AllowedResumeModes...),
		AvailableActions:    workflowPauseViewAvailableActions(summary.AllowedResumeModes),
		PausePayloadPreview: cloneMap(workflowPauseViewPayloadPreview(checkpoint)),
		LatestAuditHint:     draftString(checkpoint.Metadata["workflow_latest_audit_hint"]),
	}, true
}

func workflowPauseViewTaskID(runID, tokenID, nodeID, checkpointID string) string {
	runID = strings.TrimSpace(runID)
	tokenID = strings.TrimSpace(tokenID)
	nodeID = strings.TrimSpace(nodeID)
	checkpointID = strings.TrimSpace(checkpointID)
	if tokenID != "" {
		return runID + ":" + tokenID
	}
	if checkpointID != "" {
		return runID + ":" + checkpointID
	}
	if nodeID != "" {
		return runID + ":" + nodeID
	}
	return runID
}

func workflowPauseViewTaskTitle(source service.WorkflowPauseSource, fallback string) string {
	switch source {
	case service.WorkflowPauseSourceHumanNode:
		return "Human input required"
	case service.WorkflowPauseSourceManual:
		return "Manual pause"
	case service.WorkflowPauseSourcePolicy:
		return "Policy pause"
	}
	if strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback)
	}
	return "Paused task"
}

func workflowPauseViewAvailableActions(modes []service.WorkflowResumeMode) []string {
	actions := make([]string, 0, len(modes))
	seen := map[string]struct{}{}
	for _, mode := range modes {
		action := ""
		switch mode {
		case service.WorkflowResumeModeContinueToken:
			action = "submit"
		case service.WorkflowResumeModeContinueActiveTokens:
			action = "resume"
		case service.WorkflowResumeModeReplayFromCheckpoint:
			action = "replay"
		}
		if action == "" {
			continue
		}
		if _, ok := seen[action]; ok {
			continue
		}
		seen[action] = struct{}{}
		actions = append(actions, action)
	}
	return actions
}

func workflowPauseViewPayloadPreview(checkpoint *domain.WorkflowCheckpoint) map[string]any {
	if checkpoint == nil {
		return nil
	}
	pausePayload, _ := checkpoint.Metadata[workflowPausePayloadMetadataKey].(map[string]any)
	if pausePayload == nil {
		return nil
	}
	return pausePayload
}

func checkpointsToPointers(checkpoints []domain.WorkflowCheckpoint) []*domain.WorkflowCheckpoint {
	items := make([]*domain.WorkflowCheckpoint, 0, len(checkpoints))
	seen := map[string]struct{}{}
	for i := range checkpoints {
		checkpoint := checkpoints[i]
		checkpointID := strings.TrimSpace(checkpoint.ID)
		if checkpointID != "" {
			if _, ok := seen[checkpointID]; ok {
				continue
			}
			seen[checkpointID] = struct{}{}
		}
		copyValue := checkpoint
		items = append(items, &copyValue)
	}
	return items
}
