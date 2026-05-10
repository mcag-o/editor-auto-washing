package service

import (
	"content-hub/domain"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowKernelKeepsSiblingBranchVarsIsolated(t *testing.T) {
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"start": &branchFanoutNode{},
		"left":  &branchMutationNode{label: "left", value: "left"},
		"right": &branchMutationNode{label: "right", value: "right"},
	})

	wf := &domain.WorkflowDefinition{
		Name:        "branch-vars-isolated",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "start",
		Nodes: []domain.WorkflowNode{
			{ID: "start", Type: "action", Name: "Start"},
			{ID: "left", Type: "action", Name: "Left"},
			{ID: "right", Type: "action", Name: "Right"},
		},
		Edges: []domain.WorkflowEdge{
			{FromNodeID: "start", ToNodeID: "left", Condition: "result.branch == start", Priority: 1},
			{FromNodeID: "start", ToNodeID: "right", Condition: "result.branch == start", Priority: 1},
		},
	}
	runtimeCtx := &WorkflowExecutionContext{
		Workflow: wf,
		Context: &domain.WorkflowContext{Payload: map[string]any{
			"title": "shared-input",
		}},
		Metadata: map[string]any{"source": "upload"},
	}

	err := kernel.executeFrom(context.Background(), runtimeCtx, wf.EntryNodeID)

	require.NoError(t, err)
	require.Len(t, runtimeCtx.CompletedTokens, 3)
	require.NotNil(t, runtimeCtx.RootToken)
	assert.Equal(t, map[string]any{"title": "shared-input"}, runtimeCtx.Input)

	byNode := make(map[string]*WorkflowToken)
	for i := range runtimeCtx.CompletedTokens {
		token := runtimeCtx.CompletedTokens[i]
		byNode[token.NodeID] = token
	}

	left := byNode["left"]
	right := byNode["right"]
	require.NotNil(t, left)
	require.NotNil(t, right)
	require.NotNil(t, left.Branch)
	require.NotNil(t, right.Branch)

	assert.Equal(t, "left", left.Branch.Variables["branch"])
	assert.Equal(t, "left", left.Branch.Result["branch"])
	assert.Equal(t, "left", left.Branch.Artifacts["branch"])
	assert.Equal(t, "right", right.Branch.Variables["branch"])
	assert.Equal(t, "right", right.Branch.Result["branch"])
	assert.Equal(t, "right", right.Branch.Artifacts["branch"])

	assert.Equal(t, "shared-input", runtimeCtx.Input["title"])
	assert.Equal(t, map[string]any{"title": "shared-input", "branch": "right"}, runtimeCtx.Context.Payload)
	assert.Nil(t, runtimeCtx.Input["branch"])
	assert.Equal(t, "shared-input", runtimeCtx.Input["title"])
	assert.Equal(t, map[string]any{"title": "shared-input", "branch": "right"}, runtimeCtx.Context.Payload)
	assert.Equal(t, map[string]any{"source": "upload"}, runtimeCtx.Metadata)
	assert.Nil(t, runtimeCtx.Result)
	assert.Nil(t, runtimeCtx.Variables)
	assert.Nil(t, runtimeCtx.Artifacts)

	resumeWorkflow := &domain.WorkflowDefinition{
		Name:        "resume-branch-state",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "resume",
		Nodes: []domain.WorkflowNode{
			{ID: "resume", Type: "action", Name: "Resume"},
			{ID: "vars", Type: "action", Name: "Vars"},
			{ID: "result", Type: "action", Name: "Result"},
			{ID: "artifacts", Type: "action", Name: "Artifacts"},
		},
		Edges: []domain.WorkflowEdge{
			{FromNodeID: "resume", ToNodeID: "vars", Condition: "vars.branch == left", Priority: 1},
			{FromNodeID: "resume", ToNodeID: "result", Condition: "result.branch == left", Priority: 1},
			{FromNodeID: "resume", ToNodeID: "artifacts", Condition: "artifacts.branch == left", Priority: 1},
		},
	}
	resumeKernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"resume":    &resumeRouteNode{},
		"vars":      &branchMutationNode{label: "vars", value: "vars"},
		"result":    &branchMutationNode{label: "result", value: "result"},
		"artifacts": &branchMutationNode{label: "artifacts", value: "artifacts"},
	})
	resumeCtx := &WorkflowExecutionContext{
		Workflow: resumeWorkflow,
		Context:  &domain.WorkflowContext{Payload: map[string]any{"title": "shared-input"}},
		Metadata: map[string]any{"source": "upload"},
		Checkpoints: []domain.WorkflowCheckpoint{{
			NodeID:    "resume",
			State:     domain.WorkflowCheckpointStateActive,
			Resumable: true,
			Metadata: mergeCheckpointMetadata(workflowTokenMetadata(WorkflowToken{
				ID:            "token-left",
				ParentTokenID: "token-root",
				OriginTokenID: "token-root",
				Branch: &WorkflowBranchContext{
					Variables: map[string]any{"branch": "left"},
					Result:    map[string]any{"branch": "left"},
					Artifacts: map[string]any{"branch": "left"},
				},
			})),
		}},
	}

	err = resumeKernel.Resume(context.Background(), resumeCtx)

	require.NoError(t, err)
	require.Len(t, resumeCtx.CompletedTokens, 4)
	resumeByNode := make(map[string]*WorkflowToken)
	for i := range resumeCtx.CompletedTokens {
		token := resumeCtx.CompletedTokens[i]
		resumeByNode[token.NodeID] = token
	}
	assert.Equal(t, "vars", resumeByNode["vars"].Branch.Result["branch"])
	assert.Equal(t, "result", resumeByNode["result"].Branch.Result["branch"])
	assert.Equal(t, "artifacts", resumeByNode["artifacts"].Branch.Result["branch"])
}

type branchFanoutNode struct{}

func (n *branchFanoutNode) Name() string {
	return "start"
}

func (n *branchFanoutNode) Execute(_ context.Context, _ *domain.WorkflowContext) error {
	return nil
}

func (n *branchFanoutNode) ExecuteWorkflow(_ context.Context, runtimeCtx *WorkflowExecutionContext, _ domain.WorkflowNode) (WorkflowNodeResult, error) {
	runtimeCtx.Variables["branch"] = "start"
	runtimeCtx.Result["branch"] = "stale"
	runtimeCtx.Artifacts["branch"] = "start"
	return WorkflowNodeResult{Output: map[string]any{"branch": "start"}}, nil
}

type branchMutationNode struct {
	label string
	value string
}

func (n *branchMutationNode) Name() string {
	return n.label
}

func (n *branchMutationNode) Execute(_ context.Context, _ *domain.WorkflowContext) error {
	return nil
}

func (n *branchMutationNode) ExecuteWorkflow(_ context.Context, runtimeCtx *WorkflowExecutionContext, _ domain.WorkflowNode) (WorkflowNodeResult, error) {
	runtimeCtx.Input["title"] = "mutated-active-input"
	runtimeCtx.Variables["branch"] = n.value
	runtimeCtx.Artifacts["branch"] = n.value
	return WorkflowNodeResult{Output: map[string]any{"branch": n.value}}, nil
}

type resumeRouteNode struct{}

func (n *resumeRouteNode) Name() string {
	return "resume"
}

func (n *resumeRouteNode) Execute(_ context.Context, _ *domain.WorkflowContext) error {
	return nil
}

func (n *resumeRouteNode) ExecuteWorkflow(_ context.Context, runtimeCtx *WorkflowExecutionContext, _ domain.WorkflowNode) (WorkflowNodeResult, error) {
	return WorkflowNodeResult{Output: cloneWorkflowPayload(runtimeCtx.Result)}, nil
}
