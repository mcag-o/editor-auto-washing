package service

import (
	"content-hub/domain"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowKernelFailsWhenNoRequiredRouteMatches(t *testing.T) {
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"start": &recordingWorkflowNode{label: "start"},
	})

	wf := &domain.WorkflowDefinition{
		Name:        "route-required",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "start",
		Nodes: []domain.WorkflowNode{
			{ID: "start", Type: "action", Name: "Start"},
			{ID: "next", Type: "action", Name: "Next"},
		},
		Edges: []domain.WorkflowEdge{{FromNodeID: "start", ToNodeID: "next", Condition: "payload.route == approved", Priority: 1}},
	}

	err := kernel.Execute(context.Background(), wf, &domain.WorkflowContext{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no matching route")
}

func TestWorkflowKernelResumeContinuesFromLatestCheckpoint(t *testing.T) {
	var order []string
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"start":  &recordingWorkflowNode{label: "start", order: &order},
		"middle": &recordingWorkflowNode{label: "middle", order: &order},
		"end":    &recordingWorkflowNode{label: "end", order: &order},
	})

	wf := &domain.WorkflowDefinition{
		Name:        "resume",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "start",
		Nodes: []domain.WorkflowNode{
			{ID: "start", Type: "action", Name: "Start"},
			{ID: "middle", Type: "action", Name: "Middle"},
			{ID: "end", Type: "action", Name: "End"},
		},
		Edges: []domain.WorkflowEdge{
			{FromNodeID: "start", ToNodeID: "middle", Priority: 1},
			{FromNodeID: "middle", ToNodeID: "end", Priority: 1},
		},
	}
	runtimeCtx := &WorkflowExecutionContext{
		Workflow: wf,
		Context:  &domain.WorkflowContext{},
		Checkpoints: []domain.WorkflowCheckpoint{{
			WorkflowRunID: "run-1",
			NodeID:        "middle",
			State:         domain.WorkflowCheckpointStateActive,
			Resumable:     true,
		}},
	}

	err := kernel.Resume(context.Background(), runtimeCtx)

	require.NoError(t, err)
	assert.Equal(t, []string{"middle", "end"}, order)
	assert.Equal(t, "end", runtimeCtx.CurrentNodeID)
	assert.Len(t, runtimeCtx.Checkpoints, 2)
	assert.Equal(t, domain.WorkflowCheckpointStateTerminal, runtimeCtx.Checkpoints[0].State)
	assert.False(t, runtimeCtx.Checkpoints[0].Resumable)
	assert.Equal(t, domain.WorkflowCheckpointStateTerminal, runtimeCtx.Checkpoints[1].State)
	assert.False(t, runtimeCtx.Checkpoints[1].Resumable)
}

func TestWorkflowKernelAcceptsConditionalRouteWithAlwaysFallback(t *testing.T) {
	var order []string
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"start":    &recordingWorkflowNode{label: "start", order: &order},
		"approved": &recordingWorkflowNode{label: "approved", order: &order},
		"fallback": &recordingWorkflowNode{label: "fallback", order: &order},
	})

	wf := &domain.WorkflowDefinition{
		Name:        "fallback",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "start",
		Nodes: []domain.WorkflowNode{
			{ID: "start", Type: "action", Name: "Start"},
			{ID: "approved", Type: "action", Name: "Approved"},
			{ID: "fallback", Type: "action", Name: "Fallback"},
		},
		Edges: []domain.WorkflowEdge{
			{FromNodeID: "start", ToNodeID: "approved", Condition: "payload.route == approved", Priority: 1},
			{FromNodeID: "start", ToNodeID: "fallback", Condition: "always", Priority: 99},
		},
	}

	err := kernel.Execute(context.Background(), wf, &domain.WorkflowContext{Payload: map[string]any{"route": "approved"}})

	require.NoError(t, err)
	assert.Equal(t, []string{"start", "approved"}, order)
}
