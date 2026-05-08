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
