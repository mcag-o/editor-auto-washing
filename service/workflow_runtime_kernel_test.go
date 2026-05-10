package service

import (
	"content-hub/domain"
	"context"
	"sync"
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

func TestWorkflowKernelResumeDerivesCompatibilityModeFromPauseMetadata(t *testing.T) {
	var order []string
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"review": &humanWorkflowNode{actionSchema: map[string]any{"type": "object"}, formSchema: map[string]any{"type": "object"}},
		"end":    &recordingWorkflowNode{label: "end", order: &order},
	})

	wf := &domain.WorkflowDefinition{
		Name:        "resume-compatibility-mode",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "review",
		Nodes: []domain.WorkflowNode{
			{ID: "review", Type: "human", Name: "Review"},
			{ID: "end", Type: "action", Name: "End"},
		},
		Edges: []domain.WorkflowEdge{{FromNodeID: "review", ToNodeID: "end", Priority: 1}},
	}
	runtimeCtx := &WorkflowExecutionContext{Workflow: wf, Context: &domain.WorkflowContext{}}

	err := kernel.executeFrom(context.Background(), runtimeCtx, wf.EntryNodeID)
	require.NoError(t, err)
	require.Len(t, runtimeCtx.Checkpoints, 1)
	runtimeCtx.Checkpoints[0].Metadata[workflowHumanResumeInputMetadataKey] = workflowCheckpointPayload(workflowHumanResumeInputMetadata(map[string]any{}, true)[workflowHumanResumeInputMetadataKey])

	err = kernel.Resume(context.Background(), runtimeCtx)

	require.NoError(t, err)
	assert.Equal(t, []string{"end"}, order)
	assert.Equal(t, string(WorkflowResumeModeContinueToken), runtimeCtx.Checkpoints[0].Metadata[workflowResumeModeMetadataKey])
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
	assert.Equal(t, "start", order[0])
	assert.ElementsMatch(t, []string{"left", "right", "finish"}, order[1:])
	require.Len(t, runtimeCtx.ActiveTokens, 0)
	assert.NotNil(t, runtimeCtx.RootToken)
	require.Len(t, runtimeCtx.CompletedTokens, 4)
	assert.Equal(t, runtimeCtx.RootToken.ID, runtimeCtx.CompletedTokens[0].ID)
	assert.Equal(t, "start", runtimeCtx.CompletedTokens[0].NodeID)

	byNode := make(map[string][]*WorkflowToken)
	for i := range runtimeCtx.CompletedTokens {
		token := runtimeCtx.CompletedTokens[i]
		byNode[token.NodeID] = append(byNode[token.NodeID], token)
	}

	require.Len(t, byNode["left"], 1)
	require.Len(t, byNode["right"], 1)
	require.Len(t, byNode["finish"], 1)

	left := byNode["left"][0]
	right := byNode["right"][0]
	finish := byNode["finish"][0]

	assert.Equal(t, runtimeCtx.RootToken.ID, left.ParentTokenID)
	assert.Equal(t, runtimeCtx.RootToken.ID, right.ParentTokenID)
	assert.Equal(t, runtimeCtx.RootToken.ID, left.OriginTokenID)
	assert.Equal(t, runtimeCtx.RootToken.ID, right.OriginTokenID)
	assert.Equal(t, "left", left.OriginRoute.SelectedNodeID)
	assert.Equal(t, "right", right.OriginRoute.SelectedNodeID)
	require.NotNil(t, finish.Branch)
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
	assert.Equal(t, "start", firstRunOrder[0])
	assert.ElementsMatch(t, []string{"left", "right"}, firstRunOrder[1:])
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
	assert.ElementsMatch(t, []string{"left", "right"}, resumedOrder)
	require.Len(t, resumeCtx.CompletedTokens, 2)
	assert.ElementsMatch(t, []string{"left", "right"}, []string{resumeCtx.CompletedTokens[0].NodeID, resumeCtx.CompletedTokens[1].NodeID})
}

func TestWorkflowKernelCanAdvanceMultipleActiveTokensConcurrently(t *testing.T) {
	tracker := &concurrentTokenTracker{release: make(chan struct{})}
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"start": &resultWorkflowNode{output: map[string]any{"route": "approved"}},
		"left":  &concurrentGateWorkflowNode{label: "left", tracker: tracker},
		"right": &concurrentGateWorkflowNode{label: "right", tracker: tracker},
	})

	wf := &domain.WorkflowDefinition{
		Name:        "concurrent-active-tokens",
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
		Context:  &domain.WorkflowContext{Payload: map[string]any{"title": "shared"}},
	}

	err := kernel.executeFrom(context.Background(), runtimeCtx, wf.EntryNodeID)

	require.NoError(t, err)
	assert.True(t, tracker.concurrent)
	assert.Equal(t, map[string]any{"title": "shared"}, runtimeCtx.Input)
	require.Len(t, runtimeCtx.CompletedTokens, 3)
	for _, token := range runtimeCtx.CompletedTokens {
		if token.NodeID != "left" && token.NodeID != "right" {
			continue
		}
		require.NotNil(t, token.Branch)
		assert.Equal(t, token.NodeID, token.Branch.Result["branch"])
	}
}

func TestWorkflowKernelJoinBarrierWaitsForAllIncomingBranches(t *testing.T) {
	var order []string
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"start": &resultWorkflowNode{label: "start", order: &order, output: map[string]any{"route": "approved"}},
		"left":  &joinBranchWorkflowNode{label: "left", order: &order},
		"right": &joinBranchWorkflowNode{label: "right", order: &order},
		"join":  &recordingWorkflowNode{label: "join", order: &order},
	})

	wf := &domain.WorkflowDefinition{
		Name:        "join-barrier-runtime",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "start",
		Nodes: []domain.WorkflowNode{
			{ID: "start", Type: "action", Name: "Start"},
			{ID: "left", Type: "action", Name: "Left"},
			{ID: "right", Type: "action", Name: "Right"},
			{ID: "join", Type: "action", Name: "Join"},
		},
		Edges: []domain.WorkflowEdge{
			{FromNodeID: "start", ToNodeID: "left", Condition: "result.route == approved", Priority: 1},
			{FromNodeID: "start", ToNodeID: "right", Condition: "result.route == approved", Priority: 1},
			{FromNodeID: "left", ToNodeID: "join", Priority: 1},
			{FromNodeID: "right", ToNodeID: "join", Priority: 1},
		},
	}
	runtimeCtx := &WorkflowExecutionContext{
		Workflow: wf,
		Context:  &domain.WorkflowContext{Payload: map[string]any{"title": "shared"}},
	}

	err := kernel.executeFrom(context.Background(), runtimeCtx, wf.EntryNodeID)

	require.NoError(t, err)
	assert.Equal(t, "start", order[0])
	assert.ElementsMatch(t, []string{"left", "right", "join"}, order[1:])
	joinCount := 0
	for _, item := range order {
		if item == "join" {
			joinCount++
		}
	}
	assert.Equal(t, 1, joinCount)
	require.Len(t, runtimeCtx.CompletedTokens, 4)
	byNode := make(map[string][]*WorkflowToken)
	for i := range runtimeCtx.CompletedTokens {
		token := runtimeCtx.CompletedTokens[i]
		byNode[token.NodeID] = append(byNode[token.NodeID], token)
	}
	require.Len(t, byNode["join"], 1)
	require.NotNil(t, byNode["join"][0].Branch)
	assert.Equal(t, "left", byNode["join"][0].Branch.Variables["shared"])
	assert.Equal(t, "right", byNode["join"][0].Branch.Result["from"])
	assert.Equal(t, "right", byNode["join"][0].Branch.Artifacts["artifact"])
}

func TestWorkflowKernelSubflowRunsInlineAndReturnsToParentToken(t *testing.T) {
	var order []string
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"start":         &recordingWorkflowNode{label: "start", order: &order},
		"subflow":       &inlineSubflowWorkflowNode{},
		"after-subflow": &recordingWorkflowNode{label: "after-subflow", order: &order},
	})

	wf := &domain.WorkflowDefinition{
		Name:        "inline-subflow-runtime",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "start",
		Nodes: []domain.WorkflowNode{
			{ID: "start", Type: "action", Name: "Start"},
			{ID: "subflow", Type: "action", Name: "Subflow"},
			{ID: "after-subflow", Type: "action", Name: "AfterSubflow"},
		},
		Edges: []domain.WorkflowEdge{
			{FromNodeID: "start", ToNodeID: "subflow", Priority: 1},
			{FromNodeID: "subflow", ToNodeID: "after-subflow", Priority: 1},
		},
	}
	runtimeCtx := &WorkflowExecutionContext{
		Workflow: wf,
		Context:  &domain.WorkflowContext{Payload: map[string]any{"title": "Parent Title"}},
	}

	err := kernel.executeFrom(context.Background(), runtimeCtx, wf.EntryNodeID)

	require.NoError(t, err)
	assert.Equal(t, []string{"start", "after-subflow"}, order)
	require.Len(t, runtimeCtx.CompletedTokens, 3)
	byNode := make(map[string]*WorkflowToken)
	for i := range runtimeCtx.CompletedTokens {
		byNode[runtimeCtx.CompletedTokens[i].NodeID] = runtimeCtx.CompletedTokens[i]
	}
	require.NotNil(t, byNode["subflow"])
	require.NotNil(t, byNode["after-subflow"])
	assert.Equal(t, byNode["subflow"].ID, byNode["after-subflow"].ParentTokenID)
	assert.Equal(t, "Child Title", byNode["after-subflow"].Branch.Variables["title"])
	assert.Nil(t, byNode["after-subflow"].Branch.Variables["ignored"])
}



func TestWorkflowKernelBindsTokenLocalFrameBeforeNodeExecution(t *testing.T) {
	probe := &frameProbeWorkflowNode{}
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"start": probe,
	})

	wf := &domain.WorkflowDefinition{
		Name:        "frame-binding",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "start",
		Nodes: []domain.WorkflowNode{{
			ID:   "start",
			Type: "action",
			Name: "Start",
		}},
	}
	runtimeCtx := &WorkflowExecutionContext{
		Workflow: wf,
		Context:  &domain.WorkflowContext{Payload: map[string]any{"title": "shared"}},
		Metadata: map[string]any{"source": "upload"},
	}

	err := kernel.executeFrom(context.Background(), runtimeCtx, wf.EntryNodeID)

	require.NoError(t, err)
	require.NotNil(t, probe.frame)
	assert.Equal(t, "mutated", probe.frame.Input["title"])
	assert.Equal(t, "mutated", probe.frame.Metadata["source"])
	assert.Equal(t, "shared", runtimeCtx.Input["title"])
	assert.Equal(t, map[string]any{"title": "shared", "branch": "frame"}, runtimeCtx.Context.Payload)
	assert.Equal(t, map[string]any{"source": "upload"}, runtimeCtx.Metadata)
	require.NotNil(t, runtimeCtx.RootToken)
	require.NotNil(t, runtimeCtx.RootToken.Branch)
	assert.Equal(t, "frame", runtimeCtx.RootToken.Branch.Variables["branch"])
	assert.Equal(t, "frame", runtimeCtx.RootToken.Branch.Result["branch"])
	assert.Equal(t, "frame", runtimeCtx.RootToken.Branch.Artifacts["branch"])
	assert.Nil(t, runtimeCtx.Variables)
	assert.Nil(t, runtimeCtx.Result)
	assert.Nil(t, runtimeCtx.Artifacts)
}

func TestWorkflowKernelChildTokensDoNotInheritMutatedFrameBaseline(t *testing.T) {
	collector := &childFrameBaselineWorkflowNode{}
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"start": &mutatingFrameParentWorkflowNode{},
		"left":  collector,
		"right": collector,
	})

	wf := &domain.WorkflowDefinition{
		Name:        "child-frame-baseline",
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
		Context:  &domain.WorkflowContext{Payload: map[string]any{"title": "shared"}},
		Metadata: map[string]any{"source": "upload"},
	}

	err := kernel.executeFrom(context.Background(), runtimeCtx, wf.EntryNodeID)

	require.NoError(t, err)
	require.Len(t, collector.inputs, 2)
	require.Len(t, collector.metadata, 2)
	assert.Equal(t, []string{"shared", "shared"}, collector.inputs)
	assert.Equal(t, []string{"upload", "upload"}, collector.metadata)
	for _, checkpoint := range runtimeCtx.Checkpoints {
		if checkpoint.NodeID != "left" && checkpoint.NodeID != "right" {
			continue
		}
		rawInput, hasInput := checkpoint.Metadata["token_frame_input"]
		rawMetadata, hasMetadata := checkpoint.Metadata["token_frame_metadata"]
		if !hasInput && !hasMetadata {
			continue
		}
		input, ok := rawInput.(map[string]any)
		require.True(t, ok)
		metadata, ok := rawMetadata.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "shared", input["title"])
		assert.Equal(t, "upload", metadata["source"])
	}
}

func TestWorkflowKernelPreservesFinalTokenPayloadView(t *testing.T) {
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"start": &resultWorkflowNode{output: map[string]any{"decision": "approved"}},
	})

	wf := &domain.WorkflowDefinition{
		Name:        "final-payload-view",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "start",
		Nodes: []domain.WorkflowNode{{
			ID:   "start",
			Type: "action",
			Name: "Start",
		}},
	}
	runtimeCtx := &WorkflowExecutionContext{
		Workflow: wf,
		Context:  &domain.WorkflowContext{Payload: map[string]any{"title": "shared"}},
	}

	err := kernel.executeFrom(context.Background(), runtimeCtx, wf.EntryNodeID)

	require.NoError(t, err)
	assert.Equal(t, map[string]any{"title": "shared", "decision": "approved"}, runtimeCtx.Context.Payload)
	assert.Equal(t, map[string]any{"title": "shared"}, runtimeCtx.Input)
	assert.Nil(t, runtimeCtx.Result)
}

func TestWorkflowKernelResumeUsesSharedBaselineWhenCheckpointLacksFrameSnapshot(t *testing.T) {
	var seenInputs []string
	var seenMetadata []string
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"resume": &resumeFrameBaselineWorkflowNode{inputs: &seenInputs, metadata: &seenMetadata},
	})

	wf := &domain.WorkflowDefinition{
		Name:        "resume-frame-baseline",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "resume",
		Nodes: []domain.WorkflowNode{{
			ID:   "resume",
			Type: "action",
			Name: "Resume",
		}},
	}
	runtimeCtx := &WorkflowExecutionContext{
		Workflow: wf,
		Context:  &domain.WorkflowContext{Payload: map[string]any{"title": "shared"}},
		Metadata: map[string]any{"source": "upload"},
		Checkpoints: []domain.WorkflowCheckpoint{{
			WorkflowRunID: "run-1",
			NodeID:        "resume",
			State:         domain.WorkflowCheckpointStateActive,
			Resumable:     true,
			Metadata: map[string]any{
				"token_id":                            "token-resume",
				"token_parent_id":                     "token-root",
				"token_origin_id":                     "token-root",
				"token_origin_route_node_id":          "start",
				"token_origin_route_edge_id":          "start->resume@1[always]",
				"token_origin_route_selected_node_id": "resume",
				"active_token_set": []map[string]any{{
					"token_id":                            "token-resume",
					"token_parent_id":                     "token-root",
					"token_origin_id":                     "token-root",
					"token_origin_route_node_id":          "start",
					"token_origin_route_edge_id":          "start->resume@1[always]",
					"token_origin_route_selected_node_id": "resume",
					"node_id":                             "resume",
				}},
			},
		}},
	}

	err := kernel.Resume(context.Background(), runtimeCtx)

	require.NoError(t, err)
	require.Equal(t, []string{"shared"}, seenInputs)
	require.Equal(t, []string{"upload"}, seenMetadata)
	assert.Equal(t, map[string]any{"title": "shared", "decision": "resume"}, runtimeCtx.Context.Payload)
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

type frameProbeWorkflowNode struct {
	frame *WorkflowExecutionFrame
}

type resumeFrameBaselineWorkflowNode struct {
	inputs   *[]string
	metadata *[]string
}

type mutatingFrameParentWorkflowNode struct{}

type childFrameBaselineWorkflowNode struct {
	inputs   []string
	metadata []string
}

type joinBranchWorkflowNode struct {
	label string
	order *[]string
}

type inlineSubflowWorkflowNode struct{}

type loopControllerWorkflowNode struct {
	maxIterations int
}

type concurrentTokenTracker struct {
	mu         sync.Mutex
	started    int
	concurrent bool
	release    chan struct{}
	once       sync.Once
}

type concurrentGateWorkflowNode struct {
	label   string
	tracker *concurrentTokenTracker
}

func (n *frameProbeWorkflowNode) Name() string {
	return "start"
}

func (n *frameProbeWorkflowNode) Execute(_ context.Context, _ *domain.WorkflowContext) error {
	return nil
}

func (n *frameProbeWorkflowNode) ExecuteWorkflow(_ context.Context, runtimeCtx *WorkflowExecutionContext, _ domain.WorkflowNode) (WorkflowNodeResult, error) {
	n.frame = runtimeCtx.CurrentFrame
	runtimeCtx.Input["title"] = "mutated"
	runtimeCtx.Metadata["source"] = "mutated"
	runtimeCtx.Variables["branch"] = "frame"
	runtimeCtx.Artifacts["branch"] = "frame"
	return WorkflowNodeResult{Output: map[string]any{"branch": "frame"}}, nil
}

func (n *resumeFrameBaselineWorkflowNode) Name() string {
	return "resume"
}

func (n *resumeFrameBaselineWorkflowNode) Execute(_ context.Context, _ *domain.WorkflowContext) error {
	return nil
}

func (n *resumeFrameBaselineWorkflowNode) ExecuteWorkflow(_ context.Context, runtimeCtx *WorkflowExecutionContext, _ domain.WorkflowNode) (WorkflowNodeResult, error) {
	if n.inputs != nil {
		*n.inputs = append(*n.inputs, domain.DraftString(runtimeCtx.Input["title"]))
	}
	if n.metadata != nil {
		*n.metadata = append(*n.metadata, domain.DraftString(runtimeCtx.Metadata["source"]))
	}
	runtimeCtx.Result["decision"] = "resume"
	return WorkflowNodeResult{Output: map[string]any{"decision": "resume"}}, nil
}

func (n *mutatingFrameParentWorkflowNode) Name() string {
	return "start"
}

func (n *mutatingFrameParentWorkflowNode) Execute(_ context.Context, _ *domain.WorkflowContext) error {
	return nil
}

func (n *mutatingFrameParentWorkflowNode) ExecuteWorkflow(_ context.Context, runtimeCtx *WorkflowExecutionContext, _ domain.WorkflowNode) (WorkflowNodeResult, error) {
	runtimeCtx.Input["title"] = "mutated-parent"
	runtimeCtx.Metadata["source"] = "mutated-parent"
	return WorkflowNodeResult{Output: map[string]any{"route": "approved"}}, nil
}

func (n *childFrameBaselineWorkflowNode) Name() string {
	return "child"
}

func (n *childFrameBaselineWorkflowNode) Execute(_ context.Context, _ *domain.WorkflowContext) error {
	return nil
}

func (n *childFrameBaselineWorkflowNode) ExecuteWorkflow(_ context.Context, runtimeCtx *WorkflowExecutionContext, _ domain.WorkflowNode) (WorkflowNodeResult, error) {
	n.inputs = append(n.inputs, domain.DraftString(runtimeCtx.Input["title"]))
	n.metadata = append(n.metadata, domain.DraftString(runtimeCtx.Metadata["source"]))
	return WorkflowNodeResult{}, nil
}

func (n *joinBranchWorkflowNode) Name() string {
	return n.label
}

func (n *joinBranchWorkflowNode) Execute(_ context.Context, _ *domain.WorkflowContext) error {
	return nil
}

func (n *joinBranchWorkflowNode) ExecuteWorkflow(_ context.Context, runtimeCtx *WorkflowExecutionContext, _ domain.WorkflowNode) (WorkflowNodeResult, error) {
	if n.order != nil {
		*n.order = append(*n.order, n.label)
	}
	runtimeCtx.Variables["owner"] = n.label
	runtimeCtx.Variables["shared"] = n.label
	runtimeCtx.Artifacts["artifact"] = n.label
	return WorkflowNodeResult{Output: map[string]any{"from": n.label}}, nil
}

func (n *inlineSubflowWorkflowNode) Name() string {
	return "subflow"
}

func (n *inlineSubflowWorkflowNode) Execute(_ context.Context, _ *domain.WorkflowContext) error {
	return nil
}

func (n *inlineSubflowWorkflowNode) ExecuteWorkflow(_ context.Context, runtimeCtx *WorkflowExecutionContext, _ domain.WorkflowNode) (WorkflowNodeResult, error) {
	frame := &workflowSubflowFrame{
		ParentTokenID:   runtimeCtx.CurrentToken.ID,
		ParentNodeID:    "subflow",
		ChildWorkflowID: "child-flow",
		EntryNodeID:     "child-start",
		ReturnNodeID:    "after-subflow",
		ReturnMapping:   map[string]string{"headline": "title"},
		ParentBranch:    cloneWorkflowBranchContext(runtimeCtx.CurrentToken.Branch),
		State:           workflowSubflowStateRunning,
	}
	runtimeCtx.CurrentToken.Subflow = frame
	runtimeCtx.Variables["headline"] = "Child Title"
	runtimeCtx.Variables["ignored"] = "drop"
	return WorkflowNodeResult{}, nil
}

func (n *loopControllerWorkflowNode) Name() string {
	return "loop"
}

func (n *loopControllerWorkflowNode) Execute(_ context.Context, _ *domain.WorkflowContext) error {
	return nil
}

func (n *loopControllerWorkflowNode) ExecuteWorkflow(_ context.Context, runtimeCtx *WorkflowExecutionContext, _ domain.WorkflowNode) (WorkflowNodeResult, error) {
	frame := ensureWorkflowLoopFrame(runtimeCtx, "loop", "run-1", n.maxIterations)
	return workflowLoopApplyDecision(runtimeCtx, frame, workflowLoopDecisionRepeat), nil
}

func (n *concurrentGateWorkflowNode) Name() string {
	return n.label
}

func (n *concurrentGateWorkflowNode) Execute(_ context.Context, _ *domain.WorkflowContext) error {
	return nil
}

func (n *concurrentGateWorkflowNode) ExecuteWorkflow(_ context.Context, runtimeCtx *WorkflowExecutionContext, _ domain.WorkflowNode) (WorkflowNodeResult, error) {
	if n.tracker == nil {
		return WorkflowNodeResult{}, nil
	}
	n.tracker.mu.Lock()
	n.tracker.started++
	if n.tracker.started >= 2 {
		n.tracker.concurrent = true
		n.tracker.once.Do(func() {
			close(n.tracker.release)
		})
	}
	n.tracker.mu.Unlock()
	<-n.tracker.release
	return WorkflowNodeResult{Output: map[string]any{"branch": n.label}}, nil
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
