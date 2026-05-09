package handlers

import (
	"bytes"
	"content-hub/domain"
	"content-hub/service"
	"content-hub/transport/http/middleware"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAPIWorkflowRunsGetPausedRunReturnsPausePayload(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runRepo := &stubWorkflowRunRepo{runs: map[string]*domain.WorkflowRun{}}
	checkpointRepo := &stubWorkflowCheckpointRepo{checkpoints: map[string][]*domain.WorkflowCheckpoint{}}
	auditRepo := &stubAuditLogRepo{}
	run, err := domain.NewWorkflowRun(domain.WorkflowRunSpec{WorkflowID: "wf-1", WorkflowVersion: "v1", EntryNodeID: "human", WorkspaceArticleID: "article-1"})
	require.NoError(t, err)
	run.ID = "run-1"
	run.Status = domain.WorkflowRunPaused
	run.Resumable = true
	run.ResumeFromCheckpointID = "checkpoint-1"
	run.Metadata["pause_source"] = string(service.WorkflowPauseSourceHumanNode)
	run.Metadata["pause_reason"] = "awaiting human input"
	run.Metadata["pause_allowed_resume_modes"] = []string{string(service.WorkflowResumeModeContinueToken)}
	runRepo.runs[run.ID] = run
	checkpointRepo.checkpoints[run.ID] = []*domain.WorkflowCheckpoint{{
		ID:            "checkpoint-1",
		WorkflowRunID: run.ID,
		NodeID:        "human",
		State:         domain.WorkflowCheckpointStateActive,
		Resumable:     true,
		ResumeToken:   "token-1",
		CreatedAt:     time.Now().UTC(),
		Metadata: map[string]any{
			"pause_source":             string(service.WorkflowPauseSourceHumanNode),
			"pause_reason":             "awaiting human input",
			"pause_allowed_resume_modes": []string{string(service.WorkflowResumeModeContinueToken)},
		},
	}}
	require.NoError(t, auditRepo.Create(t.Context(), &domain.AuditLog{
		ID:         "audit-run-older",
		Actor:      "tester",
		Action:     "web_control.workflow_run.resume",
		Resource:   "workflow_run",
		ResourceID: run.ID,
		Result:     "success",
		Message:    "older event",
		Metadata: map[string]any{
			"workflow_run_id": run.ID,
		},
		CreatedAt: time.Now().UTC().Add(-2 * time.Minute),
	}))
	require.NoError(t, auditRepo.Create(t.Context(), &domain.AuditLog{
		ID:         "audit-run-newest",
		Actor:      "tester",
		Action:     "web_control.workflow_run.pause",
		Resource:   "workflow_run",
		ResourceID: run.ID,
		Result:     "success",
		Message:    "newest event",
		Metadata: map[string]any{
			"workflow_run_id": run.ID,
		},
		CreatedAt: time.Now().UTC().Add(-1 * time.Minute),
	}))
	require.NoError(t, auditRepo.Create(t.Context(), &domain.AuditLog{
		ID:         "audit-other-run",
		Actor:      "tester",
		Action:     "web_control.workflow_run.pause",
		Resource:   "workflow_run",
		ResourceID: "run-2",
		Result:     "success",
		Message:    "other run event",
		Metadata: map[string]any{
			"workflow_run_id": "run-2",
		},
		CreatedAt: time.Now().UTC(),
	}))

	handler := NewAPIWorkflowRunsHandler(runRepo, checkpointRepo, auditRepo)
	router := gin.New()
	router.Use(middleware.TraceID())
	router.GET("/api/workflow-runs/:id", handler.Get)
	router.GET("/api/workflow-runs/:id/audit", handler.Audit)

	req := httptest.NewRequest(http.MethodGet, "/api/workflow-runs/run-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "paused", resp["status"])
	require.Equal(t, "human_node", resp["pause_source"])
	require.Equal(t, "awaiting human input", resp["pause_reason"])
	require.Contains(t, resp["allowed_resume_modes"], "continue_token")

	auditReq := httptest.NewRequest(http.MethodGet, "/api/workflow-runs/run-1/audit?action_prefix=web_control.workflow_run&resource_id=run-1", nil)
	auditW := httptest.NewRecorder()
	router.ServeHTTP(auditW, auditReq)

	require.Equal(t, http.StatusOK, auditW.Code)
	var auditResp []domain.AuditLog
	require.NoError(t, json.Unmarshal(auditW.Body.Bytes(), &auditResp))
	require.Len(t, auditResp, 2)
	require.Equal(t, "audit-run-newest", auditResp[0].ID)
	require.Equal(t, "audit-run-older", auditResp[1].ID)
	require.Equal(t, run.ID, auditResp[0].ResourceID)
	require.Equal(t, run.ID, auditResp[1].ResourceID)
	require.Equal(t, "web_control.workflow_run.pause", auditResp[0].Action)
	require.Equal(t, "web_control.workflow_run.resume", auditResp[1].Action)
}

func TestAPIWorkflowRunsPauseViewReturnsSummaryTaskItemsAndFullAuditRefs(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runRepo := &stubWorkflowRunRepo{runs: map[string]*domain.WorkflowRun{}}
	checkpointRepo := &stubWorkflowCheckpointRepo{checkpoints: map[string][]*domain.WorkflowCheckpoint{}}
	auditRepo := &stubAuditLogRepo{}
	run, err := domain.NewWorkflowRun(domain.WorkflowRunSpec{WorkflowID: "wf-1", WorkflowVersion: "v2", EntryNodeID: "human_review", WorkspaceArticleID: "article-42"})
	require.NoError(t, err)
	run.ID = "run-42"
	run.Status = domain.WorkflowRunPaused
	run.CurrentNodeID = "human_review"
	run.Resumable = true
	run.ResumeFromCheckpointID = "checkpoint-2"
	run.Metadata["pause_source"] = string(service.WorkflowPauseSourceManual)
	run.Metadata["pause_reason"] = "fallback pause reason"
	run.Metadata["pause_allowed_resume_modes"] = []string{string(service.WorkflowResumeModeContinueActiveTokens)}
	runRepo.runs[run.ID] = run
	checkpointRepo.checkpoints[run.ID] = []*domain.WorkflowCheckpoint{{
		ID:            "checkpoint-1",
		WorkflowRunID: run.ID,
		NodeID:        "policy_gate",
		State:         domain.WorkflowCheckpointStateTerminal,
		Resumable:     false,
		ResumeToken:   "token-old",
		CreatedAt:     time.Date(2026, time.May, 9, 8, 45, 0, 0, time.UTC),
		Metadata: map[string]any{
			"pause_source":                string(service.WorkflowPauseSourcePolicy),
			"pause_reason":                "older pause",
			"pause_allowed_resume_modes":  []string{string(service.WorkflowResumeModeReplayFromCheckpoint)},
			"pause_payload":               map[string]any{"token_id": "token-old", "node_id": "policy_gate"},
			"active_token_set":            []map[string]any{{"token_id": "token-old", "node_id": "policy_gate"}},
			"workflow_latest_audit_hint":  "older hint",
		},
	}, {
		ID:            "checkpoint-2",
		WorkflowRunID: run.ID,
		NodeID:        "human_review",
		State:         domain.WorkflowCheckpointStateActive,
		Resumable:     true,
		ResumeToken:   "token-2",
		CreatedAt:     time.Date(2026, time.May, 9, 9, 0, 0, 0, time.UTC),
		Metadata: map[string]any{
			"pause_source":                string(service.WorkflowPauseSourceHumanNode),
			"pause_reason":                "awaiting editor approval",
			"pause_allowed_resume_modes":  []string{string(service.WorkflowResumeModeContinueToken), string(service.WorkflowResumeModeReplayFromCheckpoint)},
			"pause_payload":               map[string]any{"token_id": "token-2", "node_id": "human_review", "headline": "Review Graph Title"},
			"active_token_set":            []map[string]any{{"token_id": "token-2", "node_id": "human_review"}},
			"workflow_latest_audit_hint":  "latest pause visible in queue",
		},
	}}
	require.NoError(t, auditRepo.Create(t.Context(), &domain.AuditLog{
		ID:         "audit-resume",
		Actor:      "tester",
		Action:     "web_control.workflow_run.resume",
		Resource:   "workflow_run",
		ResourceID: run.ID,
		Result:     "success",
		Message:    "resumed before latest pause",
		Metadata: map[string]any{
			"workflow_run_id": run.ID,
		},
		CreatedAt: time.Date(2026, time.May, 9, 8, 59, 0, 0, time.UTC),
	}))
	require.NoError(t, auditRepo.Create(t.Context(), &domain.AuditLog{
		ID:         "audit-pause",
		Actor:      "tester",
		Action:     "web_control.workflow_run.pause",
		Resource:   "workflow_run",
		ResourceID: run.ID,
		Result:     "success",
		Message:    "paused for human review",
		Metadata: map[string]any{
			"workflow_run_id": run.ID,
			"pause_reason":    "awaiting editor approval",
		},
		CreatedAt: time.Date(2026, time.May, 9, 9, 1, 0, 0, time.UTC),
	}))
	require.NoError(t, auditRepo.Create(t.Context(), &domain.AuditLog{
		ID:         "audit-ignore",
		Actor:      "tester",
		Action:     "web_control.workflow_run.inspect",
		Resource:   "workflow_run",
		ResourceID: run.ID,
		Result:     "success",
		Message:    "non pause action",
		Metadata: map[string]any{
			"workflow_run_id": run.ID,
		},
		CreatedAt: time.Date(2026, time.May, 9, 9, 2, 0, 0, time.UTC),
	}))

	handler := NewAPIWorkflowRunsHandler(runRepo, checkpointRepo, auditRepo)
	router := gin.New()
	router.Use(middleware.TraceID())
	router.GET("/api/workflow-runs/:id/pause-view", handler.PauseView)

	req := httptest.NewRequest(http.MethodGet, "/api/workflow-runs/run-42/pause-view", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Summary struct {
			RunID              string   `json:"runId"`
			Status             string   `json:"status"`
			WorkflowID         string   `json:"workflowId"`
			WorkflowVersion    string   `json:"workflowVersion"`
			PauseSource        string   `json:"pauseSource"`
			PauseReason        string   `json:"pauseReason"`
			AffectedTokenCount float64  `json:"affectedTokenCount"`
			AffectedTokenIDs   []string `json:"affectedTokenIDs"`
			CurrentNodeIDs     []string `json:"currentNodeIDs"`
			CheckpointID       string   `json:"checkpointId"`
		} `json:"summary"`
		TaskItems []struct {
			TaskID             string         `json:"taskId"`
			RunID              string         `json:"runId"`
			TokenID            string         `json:"tokenId"`
			PauseSource        string         `json:"pauseSource"`
			NodeID             string         `json:"nodeId"`
			Title              string         `json:"title"`
			Summary            string         `json:"summary"`
			AvailableActions   []string       `json:"availableActions"`
			AllowedResumeModes []string       `json:"allowedResumeModes"`
			LatestAuditHint    string         `json:"latestAuditHint"`
			PausePayloadPreview map[string]any `json:"pausePayloadPreview"`
		} `json:"taskItems"`
		FullAuditRefs []domain.AuditLog `json:"fullAuditRefs"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "run-42", resp.Summary.RunID)
	require.Equal(t, "paused", resp.Summary.Status)
	require.Equal(t, "wf-1", resp.Summary.WorkflowID)
	require.Equal(t, "v2", resp.Summary.WorkflowVersion)
	require.Equal(t, "human_node", resp.Summary.PauseSource)
	require.Equal(t, "awaiting editor approval", resp.Summary.PauseReason)
	require.Equal(t, float64(1), resp.Summary.AffectedTokenCount)
	require.Equal(t, []string{"token-2"}, resp.Summary.AffectedTokenIDs)
	require.Equal(t, []string{"human_review"}, resp.Summary.CurrentNodeIDs)
	require.Equal(t, "checkpoint-2", resp.Summary.CheckpointID)
	require.Len(t, resp.TaskItems, 1)
	require.Equal(t, "run-42:token-2:human_review", resp.TaskItems[0].TaskID)
	require.Equal(t, "run-42", resp.TaskItems[0].RunID)
	require.Equal(t, "token-2", resp.TaskItems[0].TokenID)
	require.Equal(t, "human_node", resp.TaskItems[0].PauseSource)
	require.Equal(t, "human_review", resp.TaskItems[0].NodeID)
	require.Equal(t, "Human review required", resp.TaskItems[0].Title)
	require.Equal(t, "awaiting editor approval", resp.TaskItems[0].Summary)
	require.Contains(t, resp.TaskItems[0].AvailableActions, "submit")
	require.Equal(t, []string{"continue_token", "replay_from_checkpoint"}, resp.TaskItems[0].AllowedResumeModes)
	require.Equal(t, "latest pause visible in queue", resp.TaskItems[0].LatestAuditHint)
	require.Equal(t, "Review Graph Title", resp.TaskItems[0].PausePayloadPreview["headline"])
	require.Len(t, resp.FullAuditRefs, 2)
	require.Equal(t, "audit-pause", resp.FullAuditRefs[0].ID)
	require.Equal(t, "audit-resume", resp.FullAuditRefs[1].ID)
}

func TestAPIWorkflowRunsResumeHumanNodeAcceptsActionAndFormValues(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runRepo := &stubWorkflowRunRepo{runs: map[string]*domain.WorkflowRun{}}
	checkpointRepo := &stubWorkflowCheckpointRepo{checkpoints: map[string][]*domain.WorkflowCheckpoint{}}
	run, err := domain.NewWorkflowRun(domain.WorkflowRunSpec{WorkflowID: "wf-1", WorkflowVersion: "v1", EntryNodeID: "human", WorkspaceArticleID: "article-1"})
	require.NoError(t, err)
	run.ID = "run-1"
	run.Status = domain.WorkflowRunPaused
	run.Resumable = true
	run.ResumeFromCheckpointID = "checkpoint-1"
	run.Metadata["pause_source"] = string(service.WorkflowPauseSourceHumanNode)
	run.Metadata["pause_allowed_resume_modes"] = []string{string(service.WorkflowResumeModeReplayFromCheckpoint), string(service.WorkflowResumeModeContinueToken)}
	runRepo.runs[run.ID] = run
	checkpointRepo.checkpoints[run.ID] = []*domain.WorkflowCheckpoint{{
		ID:            "checkpoint-1",
		WorkflowRunID: run.ID,
		NodeID:        "human",
		State:         domain.WorkflowCheckpointStateActive,
		Resumable:     true,
		ResumeToken:   "token-1",
		CreatedAt:     time.Now().UTC(),
		Metadata: map[string]any{
			"pause_source":             string(service.WorkflowPauseSourceHumanNode),
			"pause_allowed_resume_modes": []string{string(service.WorkflowResumeModeContinueToken)},
		},
	}}

	handler := NewAPIWorkflowRunsHandler(runRepo, checkpointRepo, &stubAuditLogRepo{})
	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/workflow-runs/:id/resume", handler.Resume)

	req := httptest.NewRequest(http.MethodPost, "/api/workflow-runs/run-1/resume", bytes.NewBufferString(`{"action":"approve","form_values":{"title":"ok"}}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "resumed", resp["status"])
	require.Equal(t, "approve", resp["action"])
	require.Contains(t, resp["form_values"], "title")
	storedRun, getRunErr := runRepo.GetByID(t.Context(), run.ID)
	require.NoError(t, getRunErr)
	require.Empty(t, storedRun.ResumeFromCheckpointID)
	require.False(t, storedRun.Resumable)
	storedCheckpoints, listErr := checkpointRepo.ListByWorkflowRunID(t.Context(), run.ID)
	require.NoError(t, listErr)
		require.Len(t, storedCheckpoints, 1)
		require.Equal(t, domain.WorkflowCheckpointStateTerminal, storedCheckpoints[0].State)
		require.False(t, storedCheckpoints[0].Resumable)
		require.NotNil(t, storedCheckpoints[0].ConsumedAt)
	require.Equal(t, service.WorkflowResumeModeContinueToken, storedRun.Metadata["resume_mode"])
}

func TestAPIWorkflowRunsResumeHonorsRequestedMode(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runRepo := &stubWorkflowRunRepo{runs: map[string]*domain.WorkflowRun{}}
	checkpointRepo := &stubWorkflowCheckpointRepo{checkpoints: map[string][]*domain.WorkflowCheckpoint{}}
	auditRepo := &stubAuditLogRepo{}
	run, err := domain.NewWorkflowRun(domain.WorkflowRunSpec{WorkflowID: "wf-1", WorkflowVersion: "v1", EntryNodeID: "manual", WorkspaceArticleID: "article-1"})
	require.NoError(t, err)
	run.ID = "run-requested-mode"
	run.Status = domain.WorkflowRunPaused
	run.Resumable = true
	run.ResumeFromCheckpointID = "checkpoint-requested-mode"
	run.Metadata["pause_source"] = string(service.WorkflowPauseSourceManual)
	run.Metadata["pause_allowed_resume_modes"] = []string{string(service.WorkflowResumeModeContinueActiveTokens), string(service.WorkflowResumeModeReplayFromCheckpoint)}
	runRepo.runs[run.ID] = run
	checkpointRepo.checkpoints[run.ID] = []*domain.WorkflowCheckpoint{{
		ID:            "checkpoint-requested-mode",
		WorkflowRunID: run.ID,
		NodeID:        "manual",
		State:         domain.WorkflowCheckpointStateActive,
		Resumable:     true,
		ResumeToken:   "token-requested-mode",
		CreatedAt:     time.Now().UTC(),
		Metadata: map[string]any{
			"pause_source":                string(service.WorkflowPauseSourceManual),
			"pause_allowed_resume_modes":  []string{string(service.WorkflowResumeModeContinueActiveTokens), string(service.WorkflowResumeModeReplayFromCheckpoint)},
		},
	}}

	handler := NewAPIWorkflowRunsHandler(runRepo, checkpointRepo, auditRepo)
	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/workflow-runs/:id/resume", handler.Resume)

	req := httptest.NewRequest(http.MethodPost, "/api/workflow-runs/run-requested-mode/resume", bytes.NewBufferString(`{"action":"resume","resume_mode":"replay_from_checkpoint","form_values":{"editor":"ops"}}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "replay_from_checkpoint", resp["resume_mode"])
	storedRun, getErr := runRepo.GetByID(t.Context(), run.ID)
	require.NoError(t, getErr)
	require.Equal(t, service.WorkflowResumeModeReplayFromCheckpoint, storedRun.Metadata["resume_mode"])
	require.Len(t, auditRepo.logs, 1)
	require.Equal(t, service.WorkflowResumeModeReplayFromCheckpoint, auditRepo.logs[0].Metadata["resume_mode"])

	run, err = domain.NewWorkflowRun(domain.WorkflowRunSpec{WorkflowID: "wf-1", WorkflowVersion: "v1", EntryNodeID: "manual", WorkspaceArticleID: "article-1"})
	require.NoError(t, err)
	run.ID = "run-requested-mode-invalid"
	run.Status = domain.WorkflowRunPaused
	run.Resumable = true
	run.ResumeFromCheckpointID = "checkpoint-requested-mode-invalid"
	run.Metadata["pause_source"] = string(service.WorkflowPauseSourceManual)
	run.Metadata["pause_allowed_resume_modes"] = []string{string(service.WorkflowResumeModeContinueActiveTokens), string(service.WorkflowResumeModeReplayFromCheckpoint)}
	runRepo.runs[run.ID] = run
	checkpointRepo.checkpoints[run.ID] = []*domain.WorkflowCheckpoint{{
		ID:            "checkpoint-requested-mode-invalid",
		WorkflowRunID: run.ID,
		NodeID:        "manual",
		State:         domain.WorkflowCheckpointStateActive,
		Resumable:     true,
		ResumeToken:   "token-requested-mode-invalid",
		CreatedAt:     time.Now().UTC(),
		Metadata: map[string]any{
			"pause_source":               string(service.WorkflowPauseSourceManual),
			"pause_allowed_resume_modes": []string{string(service.WorkflowResumeModeContinueActiveTokens), string(service.WorkflowResumeModeReplayFromCheckpoint)},
		},
	}}

	req = httptest.NewRequest(http.MethodPost, "/api/workflow-runs/run-requested-mode-invalid/resume", bytes.NewBufferString(`{"action":"resume","resume_mode":"continue_token","form_values":{"editor":"ops"}}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code)
	var errResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	require.Equal(t, string(domain.ErrConflict), errResp["code"])
}

func TestAPIWorkflowRunsGetPausedRunSelectsNewestResumableCheckpoint(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runRepo := &stubWorkflowRunRepo{runs: map[string]*domain.WorkflowRun{}}
	checkpointRepo := &stubWorkflowCheckpointRepo{checkpoints: map[string][]*domain.WorkflowCheckpoint{}}
	auditRepo := &stubAuditLogRepo{}
	run, err := domain.NewWorkflowRun(domain.WorkflowRunSpec{WorkflowID: "wf-1", WorkflowVersion: "v1", EntryNodeID: "human", WorkspaceArticleID: "article-1"})
	require.NoError(t, err)
	run.ID = "run-newest-checkpoint"
	run.Status = domain.WorkflowRunPaused
	run.Resumable = true
	run.ResumeFromCheckpointID = ""
	run.Metadata["pause_source"] = string(service.WorkflowPauseSourceHumanNode)
	run.Metadata["pause_allowed_resume_modes"] = []string{string(service.WorkflowResumeModeContinueToken)}
	runRepo.runs[run.ID] = run
	checkpointRepo.checkpoints[run.ID] = []*domain.WorkflowCheckpoint{{
		ID:            "checkpoint-z-oldest",
		WorkflowRunID: run.ID,
		NodeID:        "human-oldest",
		State:         domain.WorkflowCheckpointStateActive,
		Resumable:     true,
		ResumeToken:   "token-oldest",
		CreatedAt:     time.Date(2026, time.May, 9, 9, 0, 0, 0, time.UTC),
		Metadata: map[string]any{
			"pause_source":                string(service.WorkflowPauseSourceHumanNode),
			"pause_reason":                "oldest pause",
			"pause_allowed_resume_modes":  []string{string(service.WorkflowResumeModeContinueToken)},
			"pause_payload":               map[string]any{"token_id": "token-oldest", "node_id": "human-oldest"},
		},
	}, {
		ID:            "checkpoint-a-newest",
		WorkflowRunID: run.ID,
		NodeID:        "human-newest",
		State:         domain.WorkflowCheckpointStateActive,
		Resumable:     true,
		ResumeToken:   "token-newest",
		CreatedAt:     time.Date(2026, time.May, 9, 10, 0, 0, 0, time.UTC),
		Metadata: map[string]any{
			"pause_source":                string(service.WorkflowPauseSourceHumanNode),
			"pause_reason":                "newest pause",
			"pause_allowed_resume_modes":  []string{string(service.WorkflowResumeModeContinueToken)},
			"pause_payload":               map[string]any{"token_id": "token-newest", "node_id": "human-newest"},
		},
	}, {
		ID:            "checkpoint-z-newest",
		WorkflowRunID: run.ID,
		NodeID:        "human-newest-tiebreak",
		State:         domain.WorkflowCheckpointStateActive,
		Resumable:     true,
		ResumeToken:   "token-newest-tiebreak",
		CreatedAt:     time.Date(2026, time.May, 9, 10, 0, 0, 0, time.UTC),
		Metadata: map[string]any{
			"pause_source":                string(service.WorkflowPauseSourceHumanNode),
			"pause_reason":                "newest pause tiebreak",
			"pause_allowed_resume_modes":  []string{string(service.WorkflowResumeModeContinueToken)},
			"pause_payload":               map[string]any{"token_id": "token-newest-tiebreak", "node_id": "human-newest-tiebreak"},
		},
	}}

	handler := NewAPIWorkflowRunsHandler(runRepo, checkpointRepo, auditRepo)
	router := gin.New()
	router.Use(middleware.TraceID())
	router.GET("/api/workflow-runs/:id", handler.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/workflow-runs/run-newest-checkpoint", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "checkpoint-z-newest", resp["checkpoint_id"])
	require.Equal(t, "newest pause tiebreak", resp["pause_reason"])
	affected, ok := resp["affected_token"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "token-newest-tiebreak", affected["token_id"])
	require.Equal(t, "human-newest-tiebreak", affected["node_id"])
}

func TestAPIWorkflowRunsManualPauseCreatesAuditEntry(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runRepo := &stubWorkflowRunRepo{runs: map[string]*domain.WorkflowRun{}}
	checkpointRepo := &stubWorkflowCheckpointRepo{checkpoints: map[string][]*domain.WorkflowCheckpoint{}}
	auditRepo := &stubAuditLogRepo{}
	run, err := domain.NewWorkflowRun(domain.WorkflowRunSpec{WorkflowID: "wf-1", WorkflowVersion: "v1", EntryNodeID: "human", WorkspaceArticleID: "article-1"})
	require.NoError(t, err)
	run.ID = "run-1"
	run.Status = domain.WorkflowRunRunning
	runRepo.runs[run.ID] = run

	handler := NewAPIWorkflowRunsHandler(runRepo, checkpointRepo, auditRepo)
	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/workflow-runs/:id/pause", handler.Pause)

	req := httptest.NewRequest(http.MethodPost, "/api/workflow-runs/run-1/pause", bytes.NewBufferString(`{"reason":"manual review"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	require.Len(t, auditRepo.logs, 1)
	require.Equal(t, "web_control.workflow_run.pause", auditRepo.logs[0].Action)
	require.Equal(t, "success", auditRepo.logs[0].Result)
}

func TestAPIWorkflowRunsPauseFailsWhenCheckpointPersistenceFails(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	runRepo := &stubWorkflowRunRepo{runs: map[string]*domain.WorkflowRun{}}
	checkpointRepo := &stubWorkflowCheckpointRepo{checkpoints: map[string][]*domain.WorkflowCheckpoint{}}
	auditRepo := &stubAuditLogRepo{}
	run, err := domain.NewWorkflowRun(domain.WorkflowRunSpec{WorkflowID: "wf-1", WorkflowVersion: "v1", EntryNodeID: "human", WorkspaceArticleID: "article-1"})
	require.NoError(t, err)
	run.ID = "run-1"
	run.Status = domain.WorkflowRunRunning
	runRepo.runs[run.ID] = run
	checkpointRepo.createErr = domain.NewInternalErr("checkpoint create failed", nil)

	handler := NewAPIWorkflowRunsHandler(runRepo, checkpointRepo, auditRepo)
	router := gin.New()
	router.Use(middleware.TraceID())
	router.POST("/api/workflow-runs/:id/pause", handler.Pause)

	req := httptest.NewRequest(http.MethodPost, "/api/workflow-runs/run-1/pause", bytes.NewBufferString(`{"reason":"manual review"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Empty(t, auditRepo.logs)
	stored, getErr := runRepo.GetByID(t.Context(), run.ID)
	require.NoError(t, getErr)
	require.Equal(t, domain.WorkflowRunRunning, stored.Status)
}
