package service

import (
	"content-hub/domain"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowResumeCommandCanReplayFromCheckpoint(t *testing.T) {
	var order []string
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"middle": &recordingWorkflowNode{label: "middle", order: &order},
		"end":    &recordingWorkflowNode{label: "end", order: &order},
	})

	wf := &domain.WorkflowDefinition{
		Name:        "resume-command-replay",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "middle",
		Nodes: []domain.WorkflowNode{
			{ID: "middle", Type: "action", Name: "Middle"},
			{ID: "end", Type: "action", Name: "End"},
		},
		Edges: []domain.WorkflowEdge{{FromNodeID: "middle", ToNodeID: "end", Priority: 1}},
	}
	runtimeCtx := &WorkflowExecutionContext{
		Workflow: wf,
		Context:  &domain.WorkflowContext{},
		Checkpoints: []domain.WorkflowCheckpoint{{
			ID:            "checkpoint-1",
			WorkflowRunID: "run-1",
			NodeID:        "middle",
			State:         domain.WorkflowCheckpointStateActive,
			Resumable:     true,
		}},
	}

	err := kernel.ResumeWithCommand(context.Background(), runtimeCtx, WorkflowResumeCommand{
		CheckpointID: "checkpoint-1",
		Mode:         WorkflowResumeModeReplayFromCheckpoint,
		OperatorNote: "replay after operator review",
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"middle", "end"}, order)
	assert.Equal(t, string(WorkflowResumeModeReplayFromCheckpoint), runtimeCtx.Checkpoints[0].Metadata[workflowResumeModeMetadataKey])
	assert.Equal(t, "replay after operator review", runtimeCtx.Checkpoints[0].Metadata[workflowResumeOperatorNoteMetadataKey])
}

func TestWorkflowResumeCommandRejectsBlankMode(t *testing.T) {
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"middle": &recordingWorkflowNode{label: "middle"},
	})

	wf := &domain.WorkflowDefinition{
		Name:        "resume-command-mode-required",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "middle",
		Nodes:       []domain.WorkflowNode{{ID: "middle", Type: "action", Name: "Middle"}},
	}
	runtimeCtx := &WorkflowExecutionContext{
		Workflow: wf,
		Context:  &domain.WorkflowContext{},
		Checkpoints: []domain.WorkflowCheckpoint{{
			ID:        "checkpoint-1",
			NodeID:    "middle",
			State:     domain.WorkflowCheckpointStateActive,
			Resumable: true,
			Metadata: map[string]any{
				workflowPauseAllowedResumeModesMetadataKey: []string{string(WorkflowResumeModeReplayFromCheckpoint)},
			},
		}},
	}

	err := kernel.ResumeWithCommand(context.Background(), runtimeCtx, WorkflowResumeCommand{CheckpointID: "checkpoint-1"})

	require.Error(t, err)
	assert.Equal(t, domain.ErrValidation, err.(*domain.AppError).Code)
	assert.Equal(t, "workflow resume mode is required", err.(*domain.AppError).Message)
}
