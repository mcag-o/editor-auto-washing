package service

import (
	"content-hub/domain"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRewriteWorkflowExecutionStatePreservesTokenLineageFromStoredCheckpoint(t *testing.T) {
	metadata := map[string]any{}
	storeRewriteWorkflowCheckpoint(metadata, "left", map[string]any{"title": "Source"}, &WorkflowToken{
		ID:            "token-left",
		NodeID:        "left",
		ParentTokenID: "token-root",
		OriginTokenID: "token-root",
		Branch: &WorkflowBranchContext{
			Variables: map[string]any{"branch": "left", "nested": map[string]any{"flag": true}},
			Result:    map[string]any{"decision": "go", "nested": map[string]any{"items": []any{"x"}}},
			Artifacts: map[string]any{"draft": "left", "nested": map[string]any{"kind": "html"}},
		},
		OriginRoute: WorkflowTokenRouteLineage{
			SourceNodeID:   "start",
			SelectedEdgeID: "start->left@1[payload.route == approved]",
			SelectedNodeID: "left",
		},
	})

	payload, checkpoints, err := buildRewriteWorkflowExecutionState(metadata, "Source", "run-1", true)

	require.NoError(t, err)
	assert.Equal(t, "Source", payload["title"])
	require.Len(t, checkpoints, 1)
	checkpoint := checkpoints[0]
	assert.Equal(t, "left", checkpoint.NodeID)
	assert.Equal(t, domain.WorkflowCheckpointStateActive, checkpoint.State)
	assert.True(t, checkpoint.Resumable)
	assert.Equal(t, "token-left", checkpoint.Metadata["token_id"])
	assert.Equal(t, "token-root", checkpoint.Metadata["token_parent_id"])
	assert.Equal(t, "token-root", checkpoint.Metadata["token_origin_id"])
	assert.Equal(t, "start", checkpoint.Metadata["token_origin_route_node_id"])
	assert.Equal(t, "start->left@1[payload.route == approved]", checkpoint.Metadata["token_origin_route_edge_id"])
	assert.Equal(t, "left", checkpoint.Metadata["token_origin_route_selected_node_id"])
	assert.Equal(t, map[string]any{"branch": "left", "nested": map[string]any{"flag": true}}, checkpoint.Metadata["token_branch_vars"])
	assert.Equal(t, map[string]any{"decision": "go", "nested": map[string]any{"items": []any{"x"}}}, checkpoint.Metadata["token_branch_result"])
	assert.Equal(t, map[string]any{"draft": "left", "nested": map[string]any{"kind": "html"}}, checkpoint.Metadata["token_branch_artifacts"])
}

func TestBuildRewriteWorkflowExecutionStateRestoresPersistedActiveTokenSet(t *testing.T) {
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
	checkpointState, ok := metadata[rewriteWorkflowCheckpointMetadataKey].(rewriteWorkflowCheckpointState)
	require.True(t, ok)
	checkpointState.ActiveTokenSet = []map[string]any{
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
	}
	metadata[rewriteWorkflowCheckpointMetadataKey] = checkpointState

	payload, checkpoints, err := buildRewriteWorkflowExecutionState(metadata, "Source", "run-1", true)

	require.NoError(t, err)
	assert.Equal(t, "Source", payload["title"])
	require.Len(t, checkpoints, 1)
	activeSet, ok := checkpoints[0].Metadata[workflowActiveTokenSetMetadataKey].([]map[string]any)
	require.True(t, ok)
	require.Len(t, activeSet, 2)
	assert.Equal(t, "token-left", activeSet[0]["token_id"])
	assert.Equal(t, "left", activeSet[0]["node_id"])
	assert.Equal(t, "token-right", activeSet[1]["token_id"])
	assert.Equal(t, "right", activeSet[1]["node_id"])
}

func TestBuildRewriteWorkflowExecutionStateRestoresCurrentAndQueuedBranchesFromPersistedCheckpoint(t *testing.T) {
	metadata := map[string]any{}
	current := &WorkflowToken{
		ID:            "token-left",
		NodeID:        "left",
		ParentTokenID: "token-root",
		OriginTokenID: "token-root",
		Branch: &WorkflowBranchContext{
			Variables: map[string]any{"branch": "left"},
			Result:    map[string]any{"decision": "go-left"},
			Artifacts: map[string]any{"draft": "left"},
		},
		OriginRoute: WorkflowTokenRouteLineage{
			SourceNodeID:   "start",
			SelectedEdgeID: "start->left@1[payload.route == approved]",
			SelectedNodeID: "left",
		},
	}
	queued := &WorkflowToken{
		ID:            "token-right",
		NodeID:        "right",
		ParentTokenID: "token-root",
		OriginTokenID: "token-root",
		Branch: &WorkflowBranchContext{
			Variables: map[string]any{"branch": "right"},
			Result:    map[string]any{"decision": "go-right"},
			Artifacts: map[string]any{"draft": "right"},
		},
		OriginRoute: WorkflowTokenRouteLineage{
			SourceNodeID:   "start",
			SelectedEdgeID: "start->right@1[payload.route == approved]",
			SelectedNodeID: "right",
		},
	}
	storeRewriteWorkflowCheckpoint(metadata, "left", map[string]any{"title": "Source"}, current, queued)

	payload, checkpoints, err := buildRewriteWorkflowExecutionState(metadata, "Source", "run-1", true)

	require.NoError(t, err)
	assert.Equal(t, "Source", payload["title"])
	require.Len(t, checkpoints, 1)
	activeSet, ok := checkpoints[0].Metadata[workflowActiveTokenSetMetadataKey].([]map[string]any)
	require.True(t, ok)
	require.Len(t, activeSet, 2)
	assert.Equal(t, "token-right", activeSet[0]["token_id"])
	assert.Equal(t, "right", activeSet[0]["node_id"])
	assert.Equal(t, map[string]any{"branch": "right"}, activeSet[0]["token_branch_vars"])
	assert.Equal(t, "token-left", activeSet[1]["token_id"])
	assert.Equal(t, "left", activeSet[1]["node_id"])
	assert.Equal(t, map[string]any{"branch": "left"}, activeSet[1]["token_branch_vars"])
	activeTokens := workflowActiveTokensFromCheckpoint(&checkpoints[0])
	require.Len(t, activeTokens, 2)
	assert.Equal(t, "token-right", activeTokens[0].ID)
	assert.Equal(t, "right", activeTokens[0].NodeID)
	assert.Equal(t, "token-left", activeTokens[1].ID)
	assert.Equal(t, "left", activeTokens[1].NodeID)

	var order []string
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"left":  &recordingWorkflowNode{label: "left", order: &order},
		"right": &recordingWorkflowNode{label: "right", order: &order},
	})
	wf := &domain.WorkflowDefinition{
		Name:        "resume-persisted-current-first",
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
		Workflow:    wf,
		Context:     &domain.WorkflowContext{Payload: payload},
		Checkpoints: checkpoints,
	}

	err = kernel.Resume(t.Context(), runtimeCtx)

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"left", "right"}, order)
}

func TestLoadRewriteWorkflowCheckpointPreservesNestedOriginRoute(t *testing.T) {
	metadata := map[string]any{
		rewriteWorkflowCheckpointMetadataKey: map[string]any{
			"node_id":                "left",
			"payload":                map[string]any{"title": "Source"},
			"token_id":               "token-left",
			"token_parent_id":        "token-root",
			"token_origin_id":        "token-root",
			"token_branch_vars":      map[string]any{"branch": "left", "nested": map[string]any{"flag": true}},
			"token_branch_result":    map[string]any{"decision": "go", "nested": map[string]any{"items": []any{"x"}}},
			"token_branch_artifacts": map[string]any{"draft": "left", "nested": map[string]any{"kind": "html"}},
			"origin_route": map[string]any{
				"SourceNodeID":   "start",
				"SelectedEdgeID": "start->left@1[payload.route == approved]",
				"SelectedNodeID": "left",
			},
		},
	}

	checkpoint, err := loadRewriteWorkflowCheckpoint(metadata)

	require.NoError(t, err)
	require.NotNil(t, checkpoint)
	assert.Equal(t, "left", checkpoint.NodeID)
	assert.Equal(t, "token-left", checkpoint.TokenID)
	assert.Equal(t, "token-root", checkpoint.ParentTokenID)
	assert.Equal(t, "token-root", checkpoint.OriginTokenID)
	assert.Equal(t, WorkflowTokenRouteLineage{
		SourceNodeID:   "start",
		SelectedEdgeID: "start->left@1[payload.route == approved]",
		SelectedNodeID: "left",
	}, checkpoint.OriginRoute)
	require.NotNil(t, checkpoint.Token())
	assert.Equal(t, "left", checkpoint.Token().Branch.Variables["branch"])
	assert.Equal(t, true, checkpoint.Token().Branch.Variables["nested"].(map[string]any)["flag"])
	assert.Equal(t, "go", checkpoint.Token().Branch.Result["decision"])
	assert.Equal(t, "x", checkpoint.Token().Branch.Result["nested"].(map[string]any)["items"].([]any)[0])
	assert.Equal(t, "html", checkpoint.Token().Branch.Artifacts["nested"].(map[string]any)["kind"])
}

func TestLoadRewriteWorkflowCheckpointPreservesDecodedActiveTokenSetShape(t *testing.T) {
	metadata := map[string]any{
		rewriteWorkflowCheckpointMetadataKey: map[string]any{
			"node_id":  "left",
			"payload":  map[string]any{"title": "Source"},
			"token_id": "token-left",
			"active_token_set": []any{
				map[string]any{
					"token_id":                            "token-left",
					"token_parent_id":                     "token-root",
					"token_origin_id":                     "token-root",
					"token_origin_route_node_id":          "start",
					"token_origin_route_edge_id":          "start->left@1[payload.route == approved]",
					"token_origin_route_selected_node_id": "left",
					"node_id":                             "left",
				},
				map[string]any{
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
	}

	checkpoint, err := loadRewriteWorkflowCheckpoint(metadata)

	require.NoError(t, err)
	require.NotNil(t, checkpoint)
	require.Len(t, checkpoint.ActiveTokenSet, 2)
	assert.Equal(t, "token-left", checkpoint.ActiveTokenSet[0]["token_id"])
	assert.Equal(t, "left", checkpoint.ActiveTokenSet[0]["node_id"])
	assert.Equal(t, "token-right", checkpoint.ActiveTokenSet[1]["token_id"])
	assert.Equal(t, "right", checkpoint.ActiveTokenSet[1]["node_id"])
	workflowCheckpoint := domain.WorkflowCheckpoint{
		NodeID:   checkpoint.NodeID,
		Metadata: map[string]any{workflowActiveTokenSetMetadataKey: checkpoint.ActiveTokenSet},
	}
	activeTokens := workflowActiveTokensFromCheckpoint(&workflowCheckpoint)
	require.Len(t, activeTokens, 2)
	assert.Equal(t, "token-left", activeTokens[0].ID)
	assert.Equal(t, "left", activeTokens[0].NodeID)
	assert.Equal(t, "token-right", activeTokens[1].ID)
	assert.Equal(t, "right", activeTokens[1].NodeID)
}
