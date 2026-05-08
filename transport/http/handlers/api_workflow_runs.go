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
		FormValues map[string]any `json:"form_values"`
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
	rawModes := workflowResumeModes(run, checkpoint)
	resumeMode := string(service.WorkflowResumeModeContinueToken)
	if len(rawModes) > 0 {
		if mode, ok := rawModes[0].(string); ok && strings.TrimSpace(mode) != "" {
			resumeMode = mode
		}
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
	for i := len(checkpoints) - 1; i >= 0; i-- {
		if checkpoints[i].Resumable {
			checkpoint := checkpoints[i]
			return run, &checkpoint, nil
		}
	}
	return nil, nil, domain.NewNotFoundErr("workflow_checkpoint", checkpointID)
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
