package service

import (
	"content-hub/domain"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowRunCanTransitionToPausedWithSourceAndReason(t *testing.T) {
	run, err := domain.NewWorkflowRun(domain.WorkflowRunSpec{
		WorkflowID:      "workflow-1",
		WorkflowVersion: "v1",
		EntryNodeID:     "node-entry",
	})
	require.NoError(t, err)
	completedAt := time.Date(2026, time.May, 8, 12, 0, 0, 0, time.UTC)
	run.CompletedAt = &completedAt

	pauseState := WorkflowPauseState{
		Source:             WorkflowPauseSourceHumanNode,
		Scope:              WorkflowPauseScopeToken,
		Reason:             "awaiting editor review",
		AllowedResumeModes: []WorkflowResumeMode{WorkflowResumeModeContinueToken, WorkflowResumeModeReplayFromCheckpoint},
		Payload: map[string]any{
			"node_id": "review-node",
		},
	}

	err = MarkWorkflowRunPaused(run, "checkpoint-1", pauseState)

	require.NoError(t, err)
	assert.Equal(t, domain.WorkflowRunPaused, run.Status)
	assert.True(t, run.Resumable)
	assert.Equal(t, "checkpoint-1", run.ResumeFromCheckpointID)
	assert.Nil(t, run.CompletedAt)
	require.NotNil(t, run.Metadata)
	assert.Equal(t, string(WorkflowPauseSourceHumanNode), run.Metadata[workflowPauseSourceMetadataKey])
	assert.Equal(t, string(WorkflowPauseScopeToken), run.Metadata[workflowPauseScopeMetadataKey])
	assert.Equal(t, "awaiting editor review", run.Metadata[workflowPauseReasonMetadataKey])
	assert.Equal(t, []string{string(WorkflowResumeModeContinueToken), string(WorkflowResumeModeReplayFromCheckpoint)}, run.Metadata[workflowPauseAllowedResumeModesMetadataKey])
	assert.Equal(t, map[string]any{"node_id": "review-node"}, run.Metadata[workflowPausePayloadMetadataKey])
	assert.NoError(t, run.Validate())
}

func TestWorkflowRunPauseRejectsUnknownSource(t *testing.T) {
	run, err := domain.NewWorkflowRun(domain.WorkflowRunSpec{
		WorkflowID:      "workflow-1",
		WorkflowVersion: "v1",
		EntryNodeID:     "node-entry",
	})
	require.NoError(t, err)

	err = MarkWorkflowRunPaused(run, "checkpoint-1", WorkflowPauseState{Source: WorkflowPauseSource("operator")})

	require.Error(t, err)
	assert.Equal(t, domain.ErrValidation, err.(*domain.AppError).Code)
	assert.Equal(t, "unsupported workflow pause source", err.(*domain.AppError).Message)
	assert.Equal(t, domain.WorkflowRunPending, run.Status)
}

func TestWorkflowRunPauseRejectsBlankCheckpointID(t *testing.T) {
	run, err := domain.NewWorkflowRun(domain.WorkflowRunSpec{
		WorkflowID:      "workflow-1",
		WorkflowVersion: "v1",
		EntryNodeID:     "node-entry",
	})
	require.NoError(t, err)

	err = MarkWorkflowRunPaused(run, "   ", WorkflowPauseState{Source: WorkflowPauseSourceManual})

	require.Error(t, err)
	assert.Equal(t, domain.ErrValidation, err.(*domain.AppError).Code)
	assert.Equal(t, "workflow pause checkpoint id is required", err.(*domain.AppError).Message)
	assert.Equal(t, domain.WorkflowRunPending, run.Status)
	assert.False(t, run.Resumable)
	assert.Empty(t, run.ResumeFromCheckpointID)
}

func TestWorkflowRunPauseRejectsTerminalRun(t *testing.T) {
	run := &domain.WorkflowRun{
		ID:              "run-1",
		WorkflowID:      "workflow-1",
		WorkflowVersion: "v1",
		Status:          domain.WorkflowRunSucceeded,
	}

	err := MarkWorkflowRunPaused(run, "checkpoint-1", WorkflowPauseState{Source: WorkflowPauseSourceManual})

	require.Error(t, err)
	assert.Equal(t, domain.ErrConflict, err.(*domain.AppError).Code)
	assert.Equal(t, "workflow run is already in a terminal state", err.(*domain.AppError).Message)
	assert.Equal(t, domain.WorkflowRunSucceeded, run.Status)
	assert.False(t, run.Resumable)
	assert.Empty(t, run.ResumeFromCheckpointID)
}

func TestWorkflowTokenCanEnterPausedState(t *testing.T) {
	token := newWorkflowRootToken("review-node")

	pauseToken(token)

	require.NotNil(t, token)
	assert.Equal(t, WorkflowTokenStatePaused, token.State)
	assert.Equal(t, "review-node", token.NodeID)
	assert.NotEmpty(t, token.ID)
}

func TestWorkflowPauseCheckpointMetadataCarriesAllowedResumeModes(t *testing.T) {
	metadata, err := workflowPauseCheckpointMetadata(WorkflowPauseState{
		Source:             WorkflowPauseSourceManual,
		Scope:              WorkflowPauseScopeRun,
		Reason:             "operator paused run",
		AllowedResumeModes: []WorkflowResumeMode{WorkflowResumeModeContinueActiveTokens, WorkflowResumeModeReplayFromCheckpoint},
		Payload: map[string]any{
			"operator_id": "ops-1",
		},
	})
	require.NoError(t, err)

	assert.Equal(t, string(WorkflowPauseSourceManual), metadata[workflowPauseSourceMetadataKey])
	assert.Equal(t, string(WorkflowPauseScopeRun), metadata[workflowPauseScopeMetadataKey])
	assert.Equal(t, "operator paused run", metadata[workflowPauseReasonMetadataKey])
	assert.Equal(t, []string{string(WorkflowResumeModeContinueActiveTokens), string(WorkflowResumeModeReplayFromCheckpoint)}, metadata[workflowPauseAllowedResumeModesMetadataKey])
	assert.Equal(t, map[string]any{"operator_id": "ops-1"}, metadata[workflowPausePayloadMetadataKey])
}

func TestWorkflowPauseCheckpointMetadataRejectsBlankScope(t *testing.T) {
	metadata, err := workflowPauseCheckpointMetadata(WorkflowPauseState{
		Source:             WorkflowPauseSourceManual,
		AllowedResumeModes: []WorkflowResumeMode{WorkflowResumeModeContinueActiveTokens},
	})

	require.Error(t, err)
	assert.Nil(t, metadata)
	assert.Equal(t, domain.ErrValidation, err.(*domain.AppError).Code)
	assert.Equal(t, "workflow pause scope is required", err.(*domain.AppError).Message)
}

func TestWorkflowPauseCheckpointMetadataRejectsUnknownResumeMode(t *testing.T) {
	metadata, err := workflowPauseCheckpointMetadata(WorkflowPauseState{
		Source:             WorkflowPauseSourceManual,
		Scope:              WorkflowPauseScopeRun,
		AllowedResumeModes: []WorkflowResumeMode{WorkflowResumeMode("resume_anywhere")},
	})

	require.Error(t, err)
	assert.Nil(t, metadata)
	assert.Equal(t, domain.ErrValidation, err.(*domain.AppError).Code)
	assert.Equal(t, "unsupported workflow pause resume mode", err.(*domain.AppError).Message)
}

func TestManualPauseProducesRunLevelPauseCheckpoint(t *testing.T) {
	runtimeCtx := &WorkflowExecutionContext{
		ActiveTokens: []*WorkflowToken{newWorkflowRootToken("review")},
	}

	appendManualPauseCheckpoint(runtimeCtx, "run-1", "review", "operator paused run", map[string]any{"operator_id": "ops-1"})

	require.Len(t, runtimeCtx.Checkpoints, 1)
	checkpoint := runtimeCtx.Checkpoints[0]
	assert.Equal(t, "run-1", checkpoint.WorkflowRunID)
	assert.Equal(t, "review", checkpoint.NodeID)
	assert.Equal(t, domain.WorkflowCheckpointStateActive, checkpoint.State)
	assert.True(t, checkpoint.Resumable)
	assert.Equal(t, string(WorkflowPauseSourceManual), checkpoint.Metadata[workflowPauseSourceMetadataKey])
	assert.Equal(t, string(WorkflowPauseScopeRun), checkpoint.Metadata[workflowPauseScopeMetadataKey])
	assert.Equal(t, "operator paused run", checkpoint.Metadata[workflowPauseReasonMetadataKey])
	assert.Equal(t, []string{string(WorkflowResumeModeContinueActiveTokens), string(WorkflowResumeModeReplayFromCheckpoint)}, checkpoint.Metadata[workflowPauseAllowedResumeModesMetadataKey])
	assert.Equal(t, map[string]any{"operator_id": "ops-1"}, checkpoint.Metadata[workflowPausePayloadMetadataKey])
	require.Contains(t, checkpoint.Metadata, workflowActiveTokenSetMetadataKey)
}

func TestPolicyPauseCarriesTriggerContext(t *testing.T) {
	runtimeCtx := &WorkflowExecutionContext{
		ActiveTokens: []*WorkflowToken{newWorkflowRootToken("rewrite")},
	}
	triggerContext := map[string]any{
		"policy_id":    "dup-1",
		"matched_rule": "duplicate_content",
	}

	appendPolicyPauseCheckpoint(runtimeCtx, "run-9", "rewrite", "policy paused run", triggerContext)

	require.Len(t, runtimeCtx.Checkpoints, 1)
	checkpoint := runtimeCtx.Checkpoints[0]
	assert.Equal(t, string(WorkflowPauseSourcePolicy), checkpoint.Metadata[workflowPauseSourceMetadataKey])
	assert.Equal(t, string(WorkflowPauseScopeRun), checkpoint.Metadata[workflowPauseScopeMetadataKey])
	assert.Equal(t, "policy paused run", checkpoint.Metadata[workflowPauseReasonMetadataKey])
	assert.Equal(t, map[string]any{"trigger_context": triggerContext}, checkpoint.Metadata[workflowPausePayloadMetadataKey])
	assert.Equal(t, []string{string(WorkflowResumeModeContinueActiveTokens), string(WorkflowResumeModeReplayFromCheckpoint)}, checkpoint.Metadata[workflowPauseAllowedResumeModesMetadataKey])
}
