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

func TestWorkflowKernelAllowsNaturalTerminationOnNoMatchForTerminalNode(t *testing.T) {
	var order []string
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"start": &resultWorkflowNode{label: "start", order: &order, allowNaturalTermination: true},
	})

	wf := &domain.WorkflowDefinition{
		Name:        "terminal-no-match",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "start",
		Nodes: []domain.WorkflowNode{
			{ID: "start", Type: "action", Name: "Start"},
			{ID: "optional", Type: "action", Name: "Optional"},
		},
		Edges: []domain.WorkflowEdge{{FromNodeID: "start", ToNodeID: "optional", Condition: "payload.route == approved", Priority: 1}},
	}

	err := kernel.Execute(context.Background(), wf, &domain.WorkflowContext{})

	require.NoError(t, err)
	assert.Equal(t, []string{"start"}, order)
}

func TestWorkflowKernelRequiresExplicitSignalToAllowNaturalTerminationOnNoMatch(t *testing.T) {
	var order []string
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"start": &resultWorkflowNode{label: "start", order: &order},
	})

	wf := &domain.WorkflowDefinition{
		Name:        "runtime-route-required-default",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "start",
		Nodes: []domain.WorkflowNode{
			{ID: "start", Type: "action", Name: "Start"},
			{ID: "optional", Type: "action", Name: "Optional"},
		},
		Edges: []domain.WorkflowEdge{{FromNodeID: "start", ToNodeID: "optional", Condition: "payload.route == approved", Priority: 1}},
	}

	err := kernel.Execute(context.Background(), wf, &domain.WorkflowContext{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no matching route")
	assert.Equal(t, []string{"start"}, order)
}

func TestValidateWorkflowRuntimeGraphAllowsNonStructuralBranchingShapes(t *testing.T) {
	wf := &domain.WorkflowDefinition{
		Name:        "branching-shape",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "start",
		Nodes: []domain.WorkflowNode{
			{ID: "start", Type: "action", Name: "Start"},
			{ID: "left", Type: "action", Name: "Left"},
			{ID: "right", Type: "action", Name: "Right"},
		},
		Edges: []domain.WorkflowEdge{
			{FromNodeID: "start", ToNodeID: "left", Priority: 1},
			{FromNodeID: "start", ToNodeID: "right", Priority: 2},
		},
	}

	err := validateWorkflowRuntimeGraph(wf)

	require.NoError(t, err)
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

func TestWorkflowKernelAllowsRewriteStyleSingleNodeMainlineToTerminateNaturally(t *testing.T) {
	var order []string
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"generate_draft": &recordingWorkflowNode{label: "generate_draft", order: &order},
	})

	wf := &domain.WorkflowDefinition{
		Name:        "rewrite-mainline-single-node",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "generate_draft",
		Nodes: []domain.WorkflowNode{{
			ID:   "generate_draft",
			Type: "rewrite_stage",
			Name: "generate_draft",
		}},
	}

	err := kernel.Execute(context.Background(), wf, &domain.WorkflowContext{})

	require.NoError(t, err)
	assert.Equal(t, []string{"generate_draft"}, order)
}

func TestWorkflowKernelRoutesUsingNodeResultOutput(t *testing.T) {
	var order []string
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"start":    &resultWorkflowNode{label: "start", order: &order, output: map[string]any{"decision": "go"}},
		"matched":  &recordingWorkflowNode{label: "matched", order: &order},
		"fallback": &recordingWorkflowNode{label: "fallback", order: &order},
	})

	wf := &domain.WorkflowDefinition{
		Name:        "result-route",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "start",
		Nodes: []domain.WorkflowNode{
			{ID: "start", Type: "action", Name: "Start"},
			{ID: "matched", Type: "action", Name: "Matched"},
			{ID: "fallback", Type: "action", Name: "Fallback"},
		},
		Edges: []domain.WorkflowEdge{
			{FromNodeID: "start", ToNodeID: "matched", Condition: "result.decision == go", Priority: 1},
			{FromNodeID: "start", ToNodeID: "fallback", Condition: "always", Priority: 99},
		},
	}

	err := kernel.Execute(context.Background(), wf, &domain.WorkflowContext{})

	require.NoError(t, err)
	assert.Equal(t, []string{"start", "matched"}, order)
}

func TestWorkflowKernelCreatesChildTokensForWinningRouteGroup(t *testing.T) {
	var order []string
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"start":  &resultWorkflowNode{label: "start", order: &order},
		"left":   &recordingWorkflowNode{label: "left", order: &order},
		"right":  &recordingWorkflowNode{label: "right", order: &order},
		"finish": &recordingWorkflowNode{label: "finish", order: &order},
	})

	wf := &domain.WorkflowDefinition{
		Name:        "route-fanout",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "start",
		Nodes: []domain.WorkflowNode{
			{ID: "start", Type: "action", Name: "Start"},
			{ID: "left", Type: "action", Name: "Left"},
			{ID: "right", Type: "action", Name: "Right"},
			{ID: "finish", Type: "action", Name: "Finish"},
		},
		Edges: []domain.WorkflowEdge{
			{FromNodeID: "start", ToNodeID: "left", Condition: "payload.route == approved", Priority: 1},
			{FromNodeID: "start", ToNodeID: "right", Condition: "payload.route == approved", Priority: 1},
			{FromNodeID: "left", ToNodeID: "finish", Priority: 1},
			{FromNodeID: "right", ToNodeID: "finish", Priority: 1},
		},
	}
	runtimeCtx := &WorkflowExecutionContext{
		Workflow: wf,
		Context:  &domain.WorkflowContext{Payload: map[string]any{"route": "approved"}},
	}

	err := kernel.executeFrom(context.Background(), runtimeCtx, wf.EntryNodeID)

	require.NoError(t, err)
	assert.Equal(t, []string{"start", "left", "finish", "right", "finish"}, order)
	require.Len(t, runtimeCtx.ActiveTokens, 0)
	assert.NotNil(t, runtimeCtx.RootToken)
	require.Len(t, runtimeCtx.CompletedTokens, 5)
	assert.Equal(t, runtimeCtx.RootToken.ID, runtimeCtx.CompletedTokens[0].ID)
	assert.Equal(t, "start", runtimeCtx.CompletedTokens[0].NodeID)

	byNode := make(map[string][]*WorkflowToken)
	for i := range runtimeCtx.CompletedTokens {
		token := runtimeCtx.CompletedTokens[i]
		byNode[token.NodeID] = append(byNode[token.NodeID], token)
	}

	require.Len(t, byNode["left"], 1)
	require.Len(t, byNode["right"], 1)
	require.Len(t, byNode["finish"], 2)

	left := byNode["left"][0]
	right := byNode["right"][0]
	finishA := byNode["finish"][0]
	finishB := byNode["finish"][1]

	assert.Equal(t, runtimeCtx.RootToken.ID, left.ParentTokenID)
	assert.Equal(t, runtimeCtx.RootToken.ID, right.ParentTokenID)
	assert.Equal(t, runtimeCtx.RootToken.ID, left.OriginTokenID)
	assert.Equal(t, runtimeCtx.RootToken.ID, right.OriginTokenID)
	assert.Equal(t, "left", left.OriginRoute.SelectedNodeID)
	assert.Equal(t, "right", right.OriginRoute.SelectedNodeID)

	finishByParent := map[string]*WorkflowToken{
		finishA.ParentTokenID: finishA,
		finishB.ParentTokenID: finishB,
	}
	require.Contains(t, finishByParent, left.ID)
	require.Contains(t, finishByParent, right.ID)
	assert.Equal(t, left.OriginRoute, finishByParent[left.ID].OriginRoute)
	assert.Equal(t, right.OriginRoute, finishByParent[right.ID].OriginRoute)
}

func TestWorkflowKernelPreservesBranchSpecificRouteSummaryOnFanout(t *testing.T) {
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"start":  &resultWorkflowNode{label: "start"},
		"left":   &recordingWorkflowNode{label: "left"},
		"right":  &recordingWorkflowNode{label: "right"},
		"finish": &recordingWorkflowNode{label: "finish"},
	})

	wf := &domain.WorkflowDefinition{
		Name:        "route-fanout-checkpoints",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "start",
		Nodes: []domain.WorkflowNode{
			{ID: "start", Type: "action", Name: "Start"},
			{ID: "left", Type: "action", Name: "Left"},
			{ID: "right", Type: "action", Name: "Right"},
			{ID: "finish", Type: "action", Name: "Finish"},
		},
		Edges: []domain.WorkflowEdge{
			{FromNodeID: "start", ToNodeID: "left", Condition: "payload.route == approved", Priority: 1},
			{FromNodeID: "start", ToNodeID: "right", Condition: "payload.route == approved", Priority: 1},
			{FromNodeID: "left", ToNodeID: "finish", Priority: 1},
			{FromNodeID: "right", ToNodeID: "finish", Priority: 1},
		},
	}
	runtimeCtx := &WorkflowExecutionContext{
		Workflow: wf,
		Context:  &domain.WorkflowContext{Payload: map[string]any{"route": "approved"}},
	}

	err := kernel.executeFrom(context.Background(), runtimeCtx, wf.EntryNodeID)

	require.NoError(t, err)
	var startRouteCheckpoints []domain.WorkflowCheckpoint
	for _, checkpoint := range runtimeCtx.Checkpoints {
		if checkpoint.Metadata["route_node_id"] == "start" {
			startRouteCheckpoints = append(startRouteCheckpoints, checkpoint)
		}
	}
	require.Len(t, startRouteCheckpoints, 2)

	byTarget := make(map[string]domain.WorkflowCheckpoint)
	for _, checkpoint := range startRouteCheckpoints {
		byTarget[checkpoint.NodeID] = checkpoint
	}
	require.Contains(t, byTarget, "left")
	require.Contains(t, byTarget, "right")
	assert.Equal(t, "left", byTarget["left"].Metadata["route_selected_node_id"])
	assert.Equal(t, "1:start->left@1[payload.route == approved]", byTarget["left"].Metadata["route_selected_edge_id"])
	assert.Equal(t, "right", byTarget["right"].Metadata["route_selected_node_id"])
	assert.Equal(t, "2:start->right@1[payload.route == approved]", byTarget["right"].Metadata["route_selected_edge_id"])
}

func TestWorkflowKernelResumePreservesTokenLineage(t *testing.T) {
	var order []string
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"left":   &recordingWorkflowNode{label: "left", order: &order},
		"finish": &recordingWorkflowNode{label: "finish", order: &order},
	})

	wf := &domain.WorkflowDefinition{
		Name:        "resume-lineage",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "start",
		Nodes: []domain.WorkflowNode{
			{ID: "start", Type: "action", Name: "Start"},
			{ID: "left", Type: "action", Name: "Left"},
			{ID: "finish", Type: "action", Name: "Finish"},
		},
		Edges: []domain.WorkflowEdge{
			{FromNodeID: "start", ToNodeID: "left", Condition: "payload.route == approved", Priority: 1},
			{FromNodeID: "left", ToNodeID: "finish", Priority: 1},
		},
	}
	metadata := map[string]any{}
	storeRewriteWorkflowCheckpoint(metadata, "left", map[string]any{"title": "Source"}, &WorkflowToken{
		ID:            "token-left",
		NodeID:        "left",
		ParentTokenID: "token-root",
		OriginTokenID: "token-root",
		OriginRoute: WorkflowTokenRouteLineage{
			SourceNodeID:   "start",
			SelectedEdgeID: "start->left@1[payload.route == approved]",
			SelectedNodeID: "left",
		},
	})
	payload, checkpoints, err := buildRewriteWorkflowExecutionState(metadata, "Source", "run-1", true)
	require.NoError(t, err)
	runtimeCtx := &WorkflowExecutionContext{
		Workflow:    wf,
		Context:     &domain.WorkflowContext{Payload: payload},
		Checkpoints: checkpoints,
	}

	err = kernel.Resume(context.Background(), runtimeCtx)

	require.NoError(t, err)
	assert.Equal(t, []string{"left", "finish"}, order)
	require.Len(t, runtimeCtx.CompletedTokens, 2)
	assert.Equal(t, "token-left", runtimeCtx.CompletedTokens[0].ID)
	assert.Equal(t, "token-root", runtimeCtx.CompletedTokens[0].ParentTokenID)
	assert.Equal(t, "token-root", runtimeCtx.CompletedTokens[0].OriginTokenID)
	assert.Equal(t, WorkflowTokenRouteLineage{
		SourceNodeID:   "start",
		SelectedEdgeID: "start->left@1[payload.route == approved]",
		SelectedNodeID: "left",
	}, runtimeCtx.CompletedTokens[0].OriginRoute)
	assert.Equal(t, runtimeCtx.CompletedTokens[0].ID, runtimeCtx.CompletedTokens[1].ParentTokenID)
	assert.Equal(t, runtimeCtx.CompletedTokens[0].OriginTokenID, runtimeCtx.CompletedTokens[1].OriginTokenID)
	assert.Equal(t, runtimeCtx.CompletedTokens[0].OriginRoute, runtimeCtx.CompletedTokens[1].OriginRoute)
}

func TestWorkflowKernelResumeRestoresMultiBranchActiveTokenSet(t *testing.T) {
	var order []string
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"left":  &recordingWorkflowNode{label: "left", order: &order},
		"right": &recordingWorkflowNode{label: "right", order: &order},
	})

	wf := &domain.WorkflowDefinition{
		Name:        "resume-multi-branch",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "start",
		Nodes: []domain.WorkflowNode{
			{ID: "start", Type: "action", Name: "Start"},
			{ID: "left", Type: "action", Name: "Left"},
			{ID: "right", Type: "action", Name: "Right"},
		},
		Edges: []domain.WorkflowEdge{
			{FromNodeID: "start", ToNodeID: "left", Priority: 1},
			{FromNodeID: "start", ToNodeID: "right", Priority: 1},
		},
	}
	runtimeCtx := &WorkflowExecutionContext{
		Workflow: wf,
		Context:  &domain.WorkflowContext{},
		Checkpoints: []domain.WorkflowCheckpoint{{
			WorkflowRunID: "run-1",
			NodeID:        "left",
			State:         domain.WorkflowCheckpointStateActive,
			Resumable:     true,
			Metadata: map[string]any{
				"token_id":                            "token-left",
				"token_parent_id":                     "token-root",
				"token_origin_id":                     "token-root",
				"token_origin_route_node_id":          "start",
				"token_origin_route_edge_id":          "start->left@1[payload.route == approved]",
				"token_origin_route_selected_node_id": "left",
				"active_token_set": []map[string]any{
					{
						"token_id":                            "token-left",
						"token_parent_id":                     "token-root",
						"token_origin_id":                     "token-root",
						"token_origin_route_node_id":          "start",
						"token_origin_route_edge_id":          "start->left@1[payload.route == approved]",
						"token_origin_route_selected_node_id": "left",
						"node_id":                             "left",
					},
					{
						"token_id":                            "token-right",
						"token_parent_id":                     "token-root",
						"token_origin_id":                     "token-root",
						"token_origin_route_node_id":          "start",
						"token_origin_route_edge_id":          "start->right@1[payload.route == approved]",
						"token_origin_route_selected_node_id": "right",
						"node_id":                             "right",
					},
				},
			},
		}},
	}

	err := kernel.Resume(context.Background(), runtimeCtx)

	require.NoError(t, err)
	assert.Equal(t, []string{"right", "left"}, order)
	require.Len(t, runtimeCtx.CompletedTokens, 2)
	assert.Equal(t, "token-right", runtimeCtx.CompletedTokens[0].ID)
	assert.Equal(t, "right", runtimeCtx.CompletedTokens[0].NodeID)
	assert.Equal(t, "token-left", runtimeCtx.CompletedTokens[1].ID)
	assert.Equal(t, "left", runtimeCtx.CompletedTokens[1].NodeID)
}

func TestWorkflowKernelResumeRestoresEmittedActiveTokenSetCheckpoint(t *testing.T) {
	var firstRunOrder []string
	rightNode := &failOnceWorkflowNode{label: "right", order: &firstRunOrder}
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"start": &resultWorkflowNode{label: "start", order: &firstRunOrder, output: map[string]any{"route": "approved"}},
		"left":  &recordingWorkflowNode{label: "left", order: &firstRunOrder},
		"right": rightNode,
	})

	wf := &domain.WorkflowDefinition{
		Name:        "resume-emitted-active-set",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "start",
		Nodes: []domain.WorkflowNode{
			{ID: "start", Type: "action", Name: "Start"},
			{ID: "left", Type: "action", Name: "Left"},
			{ID: "right", Type: "action", Name: "Right"},
		},
		Edges: []domain.WorkflowEdge{
			{FromNodeID: "start", ToNodeID: "left", Condition: "result.route == approved", Priority: 1},
			{FromNodeID: "start", ToNodeID: "right", Condition: "result.route == approved", Priority: 1},
		},
	}
	runtimeCtx := &WorkflowExecutionContext{
		Workflow: wf,
		Context:  &domain.WorkflowContext{},
	}

	err := kernel.executeFrom(context.Background(), runtimeCtx, wf.EntryNodeID)

	require.Error(t, err)
	assert.Equal(t, []string{"start", "left", "right"}, firstRunOrder)
	checkpoint, checkpointErr := latestResumableCheckpoint(runtimeCtx.Checkpoints)
	require.NoError(t, checkpointErr)
	require.NotNil(t, checkpoint)
	assert.True(t, checkpoint.Resumable)
	assert.Equal(t, domain.WorkflowCheckpointStateActive, checkpoint.State)
	activeSet, ok := checkpoint.Metadata[workflowActiveTokenSetMetadataKey].([]map[string]any)
	require.True(t, ok)
	require.Len(t, activeSet, 2)

	var resumedOrder []string
	resumeKernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"start": &resultWorkflowNode{label: "start", order: &resumedOrder, output: map[string]any{"route": "approved"}},
		"left":  &recordingWorkflowNode{label: "left", order: &resumedOrder},
		"right": &recordingWorkflowNode{label: "right", order: &resumedOrder},
	})
	resumeCtx := &WorkflowExecutionContext{
		Workflow:    wf,
		Context:     &domain.WorkflowContext{},
		Checkpoints: runtimeCtx.Checkpoints,
	}

	err = resumeKernel.Resume(context.Background(), resumeCtx)

	require.NoError(t, err)
	assert.Equal(t, []string{"left", "right"}, resumedOrder)
	require.Len(t, resumeCtx.CompletedTokens, 2)
	assert.ElementsMatch(t, []string{"left", "right"}, []string{resumeCtx.CompletedTokens[0].NodeID, resumeCtx.CompletedTokens[1].NodeID})
}

type resultWorkflowNode struct {
	label                   string
	order                   *[]string
	output                  map[string]any
	allowNaturalTermination bool
}

func (n *resultWorkflowNode) Name() string {
	return n.label
}

func (n *resultWorkflowNode) Execute(_ context.Context, _ *domain.WorkflowContext) error {
	if n.order != nil {
		*n.order = append(*n.order, n.label)
	}
	return nil
}

func (n *resultWorkflowNode) ExecuteWorkflow(_ context.Context, _ *WorkflowExecutionContext, _ domain.WorkflowNode) (WorkflowNodeResult, error) {
	if n.order != nil {
		*n.order = append(*n.order, n.label)
	}
	return WorkflowNodeResult{Output: n.output, AllowNaturalTermination: n.allowNaturalTermination}, nil
}

type failOnceWorkflowNode struct {
	label  string
	order  *[]string
	failed bool
}

func (n *failOnceWorkflowNode) Name() string {
	return n.label
}

func (n *failOnceWorkflowNode) Execute(_ context.Context, _ *domain.WorkflowContext) error {
	return nil
}

func (n *failOnceWorkflowNode) ExecuteWorkflow(_ context.Context, _ *WorkflowExecutionContext, _ domain.WorkflowNode) (WorkflowNodeResult, error) {
	if n.order != nil {
		*n.order = append(*n.order, n.label)
	}
	if !n.failed {
		n.failed = true
		return WorkflowNodeResult{}, assert.AnError
	}
	return WorkflowNodeResult{}, nil
}
