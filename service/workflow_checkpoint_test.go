package service

import (
	"content-hub/domain"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLatestResumableCheckpointReturnsMostRecentActiveCheckpoint(t *testing.T) {
	checkpoints := []domain.WorkflowCheckpoint{
		{NodeID: "start", State: domain.WorkflowCheckpointStateActive, Resumable: true},
		{NodeID: "middle", State: domain.WorkflowCheckpointStateTerminal, Resumable: true},
		{NodeID: "end", State: domain.WorkflowCheckpointStateActive, Resumable: true},
	}

	checkpoint, err := latestResumableCheckpoint(checkpoints)

	require.NoError(t, err)
	assert.Equal(t, "end", checkpoint.NodeID)
}

func TestAppendCheckpointAppendsResumableCheckpoint(t *testing.T) {
	runtimeCtx := &WorkflowExecutionContext{}

	appendCheckpoint(runtimeCtx, "run-1", "next")

	require.Len(t, runtimeCtx.Checkpoints, 1)
	assert.Equal(t, "run-1", runtimeCtx.Checkpoints[0].WorkflowRunID)
	assert.Equal(t, "next", runtimeCtx.Checkpoints[0].NodeID)
	assert.Equal(t, domain.WorkflowCheckpointStateActive, runtimeCtx.Checkpoints[0].State)
	assert.True(t, runtimeCtx.Checkpoints[0].Resumable)
}

func TestConsumeActiveCheckpointsTerminalizesResumePoints(t *testing.T) {
	checkpointTime := time.Date(2026, time.May, 8, 2, 0, 0, 0, time.UTC)
	runtimeCtx := &WorkflowExecutionContext{Checkpoints: []domain.WorkflowCheckpoint{
		{NodeID: "middle", State: domain.WorkflowCheckpointStateActive, Resumable: true},
		{NodeID: "end", State: domain.WorkflowCheckpointStateActive, Resumable: true},
	}}

	consumeActiveCheckpoints(runtimeCtx, checkpointTime)

	assert.Equal(t, domain.WorkflowCheckpointStateTerminal, runtimeCtx.Checkpoints[0].State)
	assert.False(t, runtimeCtx.Checkpoints[0].Resumable)
	require.NotNil(t, runtimeCtx.Checkpoints[0].ConsumedAt)
	assert.Equal(t, checkpointTime, *runtimeCtx.Checkpoints[0].ConsumedAt)
	assert.Equal(t, domain.WorkflowCheckpointStateTerminal, runtimeCtx.Checkpoints[1].State)
	assert.False(t, runtimeCtx.Checkpoints[1].Resumable)
	require.NotNil(t, runtimeCtx.Checkpoints[1].ConsumedAt)
	assert.Equal(t, checkpointTime, *runtimeCtx.Checkpoints[1].ConsumedAt)
}

func TestWorkflowCheckpointCapturesLatestRouteOutcomeSummary(t *testing.T) {
	runtimeCtx := &WorkflowExecutionContext{}
	summary := WorkflowRouteOutcomeSummary{
		NodeID:          "router-1",
		SelectedEdgeID:  "edge-approved",
		SelectedNodeID:  "approved",
		Outcome:         WorkflowRouteOutcome("matched"),
		EvaluationTrace: []string{"edge-approved=true", "edge-rejected=false"},
	}

	recordLatestRouteOutcome(runtimeCtx, summary)
	appendCheckpoint(runtimeCtx, "run-1", "approved")

	require.Len(t, runtimeCtx.Checkpoints, 1)
	assert.Equal(t, summary.NodeID, runtimeCtx.Checkpoints[0].Metadata["route_node_id"])
	assert.Equal(t, summary.SelectedEdgeID, runtimeCtx.Checkpoints[0].Metadata["route_selected_edge_id"])
	assert.Equal(t, summary.SelectedNodeID, runtimeCtx.Checkpoints[0].Metadata["route_selected_node_id"])
	assert.Equal(t, string(summary.Outcome), runtimeCtx.Checkpoints[0].Metadata["route_outcome"])
	assert.Equal(t, []string{"edge-approved=true", "edge-rejected=false"}, runtimeCtx.Checkpoints[0].Metadata["route_evaluation_trace"])
}

func TestWorkflowCheckpointRetainsPriorRouteSummaryAcrossLaterTransitions(t *testing.T) {
	runtimeCtx := &WorkflowExecutionContext{}
	firstSummary := WorkflowRouteOutcomeSummary{
		NodeID:          "router-1",
		SelectedEdgeID:  "router-1->approved",
		SelectedNodeID:  "approved",
		Outcome:         WorkflowRouteOutcomeSingleMatch,
		EvaluationTrace: []string{"router-1->approved=match"},
	}
	secondSummary := WorkflowRouteOutcomeSummary{
		NodeID:          "router-2",
		SelectedEdgeID:  "router-2->rendered",
		SelectedNodeID:  "rendered",
		Outcome:         WorkflowRouteOutcomeSingleMatch,
		EvaluationTrace: []string{"router-2->rendered=match"},
	}

	recordLatestRouteOutcome(runtimeCtx, firstSummary)
	appendCheckpoint(runtimeCtx, "run-1", "approved")
	recordLatestRouteOutcome(runtimeCtx, secondSummary)
	appendCheckpoint(runtimeCtx, "run-1", "rendered")

	require.Len(t, runtimeCtx.Checkpoints, 2)
	assert.Equal(t, firstSummary.NodeID, runtimeCtx.Checkpoints[0].Metadata["route_node_id"])
	assert.Equal(t, firstSummary.SelectedEdgeID, runtimeCtx.Checkpoints[0].Metadata["route_selected_edge_id"])
	assert.Equal(t, firstSummary.SelectedNodeID, runtimeCtx.Checkpoints[0].Metadata["route_selected_node_id"])
	assert.Equal(t, string(firstSummary.Outcome), runtimeCtx.Checkpoints[0].Metadata["route_outcome"])
	assert.Equal(t, []string{"router-1->approved=match"}, runtimeCtx.Checkpoints[0].Metadata["route_evaluation_trace"])
	assert.Equal(t, secondSummary.NodeID, runtimeCtx.Checkpoints[1].Metadata["route_node_id"])
	assert.Equal(t, secondSummary.SelectedEdgeID, runtimeCtx.Checkpoints[1].Metadata["route_selected_edge_id"])
	assert.Equal(t, secondSummary.SelectedNodeID, runtimeCtx.Checkpoints[1].Metadata["route_selected_node_id"])
	assert.Equal(t, string(secondSummary.Outcome), runtimeCtx.Checkpoints[1].Metadata["route_outcome"])
	assert.Equal(t, []string{"router-2->rendered=match"}, runtimeCtx.Checkpoints[1].Metadata["route_evaluation_trace"])
}

func TestAppendCheckpointWithSnapshotCapturesTokenMetadata(t *testing.T) {
	runtimeCtx := &WorkflowExecutionContext{}
	summary := WorkflowRouteOutcomeSummary{
		NodeID:          "start",
		SelectedEdgeID:  "1:start->left@1[payload.route == approved]",
		SelectedNodeID:  "left",
		Outcome:         WorkflowRouteOutcomeSingleMatch,
		EvaluationTrace: []string{"1:start->left@1[payload.route == approved]=match"},
	}
	token := &WorkflowToken{
		ID:            "token-left",
		NodeID:        "left",
		ParentTokenID: "token-root",
		OriginTokenID: "token-root",
		OriginRoute: WorkflowTokenRouteLineage{
			SourceNodeID:   "start",
			SelectedEdgeID: "start->left@1[payload.route == approved]",
			SelectedNodeID: "left",
		},
	}

	appendCheckpointWithSnapshot(runtimeCtx, "run-1", "left", workflowCheckpointSnapshot{RouteSummary: &summary, Token: token})

	require.Len(t, runtimeCtx.Checkpoints, 1)
	checkpoint := runtimeCtx.Checkpoints[0]
	assert.Equal(t, "token-left", checkpoint.Metadata["token_id"])
	assert.Equal(t, "token-root", checkpoint.Metadata["token_parent_id"])
	assert.Equal(t, "token-root", checkpoint.Metadata["token_origin_id"])
	assert.Equal(t, "start", checkpoint.Metadata["token_origin_route_node_id"])
	assert.Equal(t, "start->left@1[payload.route == approved]", checkpoint.Metadata["token_origin_route_edge_id"])
	assert.Equal(t, "left", checkpoint.Metadata["token_origin_route_selected_node_id"])
	assert.Equal(t, "left", checkpoint.Metadata["route_selected_node_id"])
}

func TestWorkflowCheckpointCapturesActiveTokenSetAfterFanOut(t *testing.T) {
	runtimeCtx := &WorkflowExecutionContext{}
	left := &WorkflowToken{
		ID:            "token-left",
		NodeID:        "left",
		ParentTokenID: "token-root",
		OriginTokenID: "token-root",
		OriginRoute: WorkflowTokenRouteLineage{
			SourceNodeID:   "start",
			SelectedEdgeID: "start->left@1[payload.route == approved]",
			SelectedNodeID: "left",
		},
	}
	right := &WorkflowToken{
		ID:            "token-right",
		NodeID:        "right",
		ParentTokenID: "token-root",
		OriginTokenID: "token-root",
		OriginRoute: WorkflowTokenRouteLineage{
			SourceNodeID:   "start",
			SelectedEdgeID: "start->right@1[payload.route == approved]",
			SelectedNodeID: "right",
		},
	}
	runtimeCtx.ActiveTokens = []*WorkflowToken{left, right}

	appendCheckpointWithSnapshot(runtimeCtx, "run-1", "left", workflowCheckpointSnapshot{Token: left})
	appendCheckpointWithSnapshot(runtimeCtx, "run-1", "right", workflowCheckpointSnapshot{Token: right})

	require.Len(t, runtimeCtx.Checkpoints, 2)
	for _, checkpoint := range runtimeCtx.Checkpoints {
		activeSet, ok := checkpoint.Metadata["active_token_set"].([]map[string]any)
		require.True(t, ok)
		require.Len(t, activeSet, 2)
		assert.Equal(t, []map[string]any{
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
		}, activeSet)
	}
}

func TestAppendCheckpointWithSnapshotCapturesJoinBarrierState(t *testing.T) {
	left := &WorkflowToken{
		ID:            "token-left",
		NodeID:        "join",
		ParentTokenID: "token-root",
		OriginTokenID: "token-root",
		Branch: &WorkflowBranchContext{
			Variables: map[string]any{"shared": "left"},
			Result:    map[string]any{"from": "left"},
			Artifacts: map[string]any{"artifact": "left"},
		},
		Frame: &WorkflowExecutionFrame{
			Input:    map[string]any{"title": "shared"},
			Metadata: map[string]any{"source": "upload"},
		},
	}
	barrier := newWorkflowJoinBarrierWithExpectedCount("join", 2)
	barrier.ParentTokenID = "token-root"
	barrier.OriginTokenID = "token-root"
	barrier.OriginRoute = WorkflowTokenRouteLineage{
		SourceNodeID:   "start",
		SelectedEdgeID: "start->left@1[result.route == approved]",
		SelectedNodeID: "left",
	}
	barrier.Frame = &WorkflowExecutionFrame{
		Input:    map[string]any{"title": "shared"},
		Metadata: map[string]any{"source": "upload"},
	}
	barrier.tokens[left.ID] = left
	barrier.Arrive(left.ID)

	runtimeCtx := &WorkflowExecutionContext{
		JoinBarriers: map[string]*workflowJoinBarrier{"join": barrier},
		ActiveTokens: []*WorkflowToken{left},
	}

	appendCheckpointWithSnapshot(runtimeCtx, "run-1", "join", workflowCheckpointSnapshot{Token: left})

	require.Len(t, runtimeCtx.Checkpoints, 1)
	check := runtimeCtx.Checkpoints[0]
	barriers := workflowJoinBarriersFromCheckpoint(&check)
	require.Len(t, barriers, 1)
	restored := barriers["join"]
	require.NotNil(t, restored)
	assert.Equal(t, 2, restored.ExpectedCount)
	assert.Equal(t, []string{"token-left"}, restored.ArrivedTokenIDs)
	assert.Equal(t, workflowJoinBarrierStateWaiting, restored.State)
	require.Contains(t, restored.tokens, "token-left")
	assert.Equal(t, map[string]any{"shared": "left"}, restored.tokens["token-left"].Branch.Variables)
	assert.Equal(t, map[string]any{"title": "shared"}, restored.tokens["token-left"].Frame.Input)
}
