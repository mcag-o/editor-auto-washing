package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewWorkflowRunStartsAtPendingWithEntryNode(t *testing.T) {
	run, err := NewWorkflowRun(WorkflowRunSpec{
		WorkflowID:       "workflow-1",
		WorkflowVersion:  "v1",
		EntryNodeID:      "node-entry",
		WorkspaceArticleID: "workspace-1",
	})

	assert.NoError(t, err)
	assert.Equal(t, WorkflowRunPending, run.Status)
	assert.Equal(t, "node-entry", run.CurrentNodeID)
	assert.Equal(t, "workflow-1", run.WorkflowID)
	assert.Equal(t, "v1", run.WorkflowVersion)
	assert.Equal(t, "workspace-1", run.WorkspaceArticleID)
	assert.NotEmpty(t, run.ID)
	assert.False(t, run.StartedAt.IsZero())
	assert.Nil(t, run.CompletedAt)
	assert.Empty(t, run.FinalFailureClass)
	assert.NotNil(t, run.Metadata)
}

func TestWorkflowRunCanTransitionToSucceeded(t *testing.T) {
	run, err := NewWorkflowRun(WorkflowRunSpec{
		WorkflowID:      "workflow-1",
		WorkflowVersion: "v1",
		EntryNodeID:     "node-entry",
	})

	assert.NoError(t, err)

	before := time.Now().UTC()
	err = run.MarkSucceeded()

	assert.NoError(t, err)
	assert.Equal(t, WorkflowRunSucceeded, run.Status)
	assert.NotNil(t, run.CompletedAt)
	assert.False(t, run.CompletedAt.Before(before))
	assert.Empty(t, run.ErrorSummary)
	assert.Empty(t, run.FinalFailureClass)
}

func TestWorkflowRunValidateRequiresKnownStatusAndFailureClass(t *testing.T) {
	run := WorkflowRun{
		ID:              "run-1",
		WorkflowID:      "workflow-1",
		WorkflowVersion: "v1",
		Status:          "queued",
	}

	err := run.Validate()

	assert.Error(t, err)
	assert.Equal(t, ErrValidation, err.(*AppError).Code)
	assert.Equal(t, "unsupported workflow run status", err.(*AppError).Message)

	run.Status = WorkflowRunFailed
	run.FinalFailureClass = "unknown"
	err = run.Validate()

	assert.Error(t, err)
	assert.Equal(t, ErrValidation, err.(*AppError).Code)
	assert.Equal(t, "unsupported workflow run failure class", err.(*AppError).Message)

	run.FinalFailureClass = WorkflowFailureClassPermanent
	assert.NoError(t, run.Validate())
}

func TestWorkflowNodeExecutionValidateRequiresKnownStatusAndFailureClass(t *testing.T) {
	execution := WorkflowNodeExecution{
		ID:            "exec-1",
		WorkflowRunID: "run-1",
		NodeID:        "node-1",
		NodeType:      "action",
		Status:        "queued",
	}

	err := execution.Validate()

	assert.Error(t, err)
	assert.Equal(t, ErrValidation, err.(*AppError).Code)
	assert.Equal(t, "unsupported workflow node execution status", err.(*AppError).Message)

	execution.Status = WorkflowNodeExecutionFailed
	execution.FailureClass = "unknown"
	err = execution.Validate()

	assert.Error(t, err)
	assert.Equal(t, ErrValidation, err.(*AppError).Code)
	assert.Equal(t, "unsupported workflow node execution failure class", err.(*AppError).Message)

	execution.FailureClass = WorkflowFailureClassTransient
	assert.NoError(t, execution.Validate())
}

func TestWorkflowRunMarkSucceededRejectsAlreadyTerminalRun(t *testing.T) {
	now := time.Now().UTC()
	run := &WorkflowRun{
		ID:           "run-1",
		WorkflowID:   "workflow-1",
		Status:       WorkflowRunSucceeded,
		CompletedAt:  &now,
		CurrentNodeID: "node-1",
	}

	err := run.MarkSucceeded()

	assert.Error(t, err)
	assert.Equal(t, ErrConflict, err.(*AppError).Code)
	assert.Equal(t, "workflow run is already in a terminal state", err.(*AppError).Message)
}

func TestWorkflowCheckpointRequiresActiveOrTerminalState(t *testing.T) {
	err := (WorkflowCheckpoint{State: "queued"}).Validate()

	assert.Error(t, err)
	assert.Equal(t, ErrValidation, err.(*AppError).Code)
	assert.Equal(t, "workflow checkpoint state must be active or terminal", err.(*AppError).Message)

	assert.NoError(t, (WorkflowCheckpoint{State: WorkflowCheckpointStateActive}).Validate())
	assert.NoError(t, (WorkflowCheckpoint{State: WorkflowCheckpointStateTerminal}).Validate())
}
