package service

import (
	"content-hub/domain"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowRouterSelectsFirstMatchingEdgeByPriority(t *testing.T) {
	router := newWorkflowRouter()
	ctx := &WorkflowExecutionContext{
		Workflow: &domain.WorkflowDefinition{},
		Context:  &domain.WorkflowContext{Payload: map[string]any{"route": "approved"}},
	}
	edges := []domain.WorkflowEdge{
		{FromNodeID: "start", ToNodeID: "fallback", Condition: "always", Priority: 50},
		{FromNodeID: "start", ToNodeID: "wrong", Condition: "payload.route == denied", Priority: 1},
		{FromNodeID: "start", ToNodeID: "approved", Condition: "payload.route == approved", Priority: 2},
	}

	selected, err := router.SelectNextEdge(ctx, "start", WorkflowNodeResult{RouteRequired: true}, edges)

	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, "approved", selected.ToNodeID)
}

func TestWorkflowRouterSelectsFirstWinningEdgeWhenEvaluationReturnsMultiEdgeGroup(t *testing.T) {
	router := newWorkflowRouter()
	ctx := &WorkflowExecutionContext{
		Workflow: &domain.WorkflowDefinition{},
		Context:  &domain.WorkflowContext{Payload: map[string]any{"route": "approved"}},
	}
	edges := []domain.WorkflowEdge{
		{FromNodeID: "start", ToNodeID: "approved", Condition: "payload.route == approved", Priority: 1},
		{FromNodeID: "start", ToNodeID: "approved", Condition: " payload.route == approved ", Priority: 1},
	}

	selected, err := router.SelectNextEdge(ctx, "start", WorkflowNodeResult{RouteRequired: true}, edges)

	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, "approved", selected.ToNodeID)
	routeSummary, ok := ctx.Metadata[workflowLatestRouteMetadataKey].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "start", routeSummary["node_id"])
	assert.Equal(t, string(WorkflowRouteOutcomeMultiMatch), routeSummary["outcome"])
	assert.Equal(t, "1:start->approved@1[payload.route == approved]", routeSummary["selected_edge_id"])
	assert.Equal(t, "approved", routeSummary["selected_node_id"])
	assert.Equal(t, []string{
		"1:start->approved@1[payload.route == approved]=match",
		"2:start->approved@1[payload.route == approved]=match",
	}, routeSummary["evaluation_trace"])
}

func TestWorkflowRouterMatchesResultNamespaceFromNodeResult(t *testing.T) {
	router := newWorkflowRouter()
	ctx := &WorkflowExecutionContext{Workflow: &domain.WorkflowDefinition{}}
	edges := []domain.WorkflowEdge{
		{FromNodeID: "start", ToNodeID: "fallback", Condition: "always", Priority: 50},
		{FromNodeID: "start", ToNodeID: "go", Condition: "result.decision == go", Priority: 1},
	}

	selected, err := router.SelectNextEdge(ctx, "start", WorkflowNodeResult{
		RouteRequired: true,
		Output:        map[string]any{"decision": "go"},
	}, edges)

	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, "go", selected.ToNodeID)
}

func TestWorkflowRouterReturnsNilAndRecordsNoMatchSummaryWhenRouteIsOptional(t *testing.T) {
	router := newWorkflowRouter()
	ctx := &WorkflowExecutionContext{Workflow: &domain.WorkflowDefinition{}}
	edges := []domain.WorkflowEdge{{FromNodeID: "start", ToNodeID: "approved", Condition: "payload.route == approved", Priority: 1}}

	selected, err := router.SelectNextEdge(ctx, "start", WorkflowNodeResult{}, edges)

	require.NoError(t, err)
	assert.Nil(t, selected)
	routeSummary, ok := ctx.Metadata[workflowLatestRouteMetadataKey].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "start", routeSummary["node_id"])
	assert.Equal(t, string(WorkflowRouteOutcomeNoMatch), routeSummary["outcome"])
	assert.Equal(t, []string{"1:start->approved@1[payload.route == approved]=miss"}, routeSummary["evaluation_trace"])
	assert.Empty(t, routeSummary["selected_edge_id"])
	assert.Empty(t, routeSummary["selected_node_id"])
}
