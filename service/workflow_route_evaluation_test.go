package service

import (
	"content-hub/domain"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowRouteEvaluationReportsSingleMatchWithPerEdgeDiagnostics(t *testing.T) {
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

	evaluation := router.EvaluateRoutes(ctx, "start", WorkflowNodeResult{RouteRequired: true}, edges)

	require.Equal(t, WorkflowRouteOutcomeSingleMatch, evaluation.Outcome)
	require.Len(t, evaluation.EvaluatedEdges, 3)
	require.Len(t, evaluation.SelectedEdges, 1)
	assert.Equal(t, "start", evaluation.SourceNodeID)
	assert.Equal(t, "wrong", evaluation.EvaluatedEdges[0].Edge.ToNodeID)
	assert.False(t, evaluation.EvaluatedEdges[0].Matched)
	assert.Equal(t, WorkflowRouteConditionResultMiss, evaluation.EvaluatedEdges[0].Result)
	assert.Equal(t, "approved", evaluation.EvaluatedEdges[1].Edge.ToNodeID)
	assert.True(t, evaluation.EvaluatedEdges[1].Matched)
	assert.Equal(t, WorkflowRouteConditionResultMatch, evaluation.EvaluatedEdges[1].Result)
	assert.Equal(t, "fallback", evaluation.EvaluatedEdges[2].Edge.ToNodeID)
	assert.True(t, evaluation.EvaluatedEdges[2].Matched)
	assert.Equal(t, WorkflowRouteConditionResultFallbackDeferred, evaluation.EvaluatedEdges[2].Result)
	assert.Equal(t, "approved", evaluation.SelectedEdges[0].ToNodeID)
}

func TestWorkflowRouteEvaluationSelectsAllMatchesInWinningPriorityGroup(t *testing.T) {
	router := newWorkflowRouter()
	ctx := &WorkflowExecutionContext{
		Workflow: &domain.WorkflowDefinition{},
		Context:  &domain.WorkflowContext{Payload: map[string]any{"route": "approved"}},
	}
	edges := []domain.WorkflowEdge{
		{FromNodeID: "start", ToNodeID: "approved-a", Condition: "payload.route == approved", Priority: 1},
		{FromNodeID: "start", ToNodeID: "approved-b", Condition: "payload.route == approved", Priority: 1},
		{FromNodeID: "start", ToNodeID: "fallback", Condition: "always", Priority: 50},
	}

	evaluation := router.EvaluateRoutes(ctx, "start", WorkflowNodeResult{RouteRequired: true}, edges)

	require.Equal(t, WorkflowRouteOutcomeMultiMatch, evaluation.Outcome)
	require.Len(t, evaluation.SelectedEdges, 2)
	assert.Equal(t, "approved-a", evaluation.SelectedEdges[0].ToNodeID)
	assert.Equal(t, "approved-b", evaluation.SelectedEdges[1].ToNodeID)
	assert.Equal(t, WorkflowRouteConditionResultMatch, evaluation.EvaluatedEdges[0].Result)
	assert.Equal(t, WorkflowRouteConditionResultMatch, evaluation.EvaluatedEdges[1].Result)
	assert.Equal(t, WorkflowRouteConditionResultFallbackDeferred, evaluation.EvaluatedEdges[2].Result)
	assert.NoError(t, evaluation.Error)
	assert.Equal(t, "fallback", evaluation.EvaluatedEdges[2].Edge.ToNodeID)
}

func TestWorkflowRouteEvaluationSuppressesLowerPriorityMatchesWhenHigherPriorityGroupWins(t *testing.T) {
	router := newWorkflowRouter()
	ctx := &WorkflowExecutionContext{
		Workflow: &domain.WorkflowDefinition{},
		Context:  &domain.WorkflowContext{Payload: map[string]any{"route": "approved"}},
	}
	edges := []domain.WorkflowEdge{
		{FromNodeID: "start", ToNodeID: "approved-a", Condition: "payload.route == approved", Priority: 1},
		{FromNodeID: "start", ToNodeID: "approved-b", Condition: "payload.route == approved", Priority: 1},
		{FromNodeID: "start", ToNodeID: "approved-lower", Condition: "payload.route == approved", Priority: 2},
	}

	evaluation := router.EvaluateRoutes(ctx, "start", WorkflowNodeResult{RouteRequired: true}, edges)

	require.Equal(t, WorkflowRouteOutcomeMultiMatch, evaluation.Outcome)
	require.Len(t, evaluation.SelectedEdges, 2)
	assert.Equal(t, "approved-a", evaluation.SelectedEdges[0].ToNodeID)
	assert.Equal(t, "approved-b", evaluation.SelectedEdges[1].ToNodeID)
	assert.Equal(t, WorkflowRouteConditionResultMatch, evaluation.EvaluatedEdges[0].Result)
	assert.Equal(t, WorkflowRouteConditionResultMatch, evaluation.EvaluatedEdges[1].Result)
	assert.Equal(t, "approved-lower", evaluation.EvaluatedEdges[2].Edge.ToNodeID)
	assert.Equal(t, WorkflowRouteConditionResultMatch, evaluation.EvaluatedEdges[2].Result)
	assert.NotContains(t, []string{evaluation.SelectedEdges[0].ToNodeID, evaluation.SelectedEdges[1].ToNodeID}, "approved-lower")
	assert.NoError(t, evaluation.Error)
}

func TestWorkflowRouteEvaluationExcludesFallbackWhenHigherPriorityExplicitGroupWins(t *testing.T) {
	router := newWorkflowRouter()
	ctx := &WorkflowExecutionContext{
		Workflow: &domain.WorkflowDefinition{},
		Context:  &domain.WorkflowContext{Payload: map[string]any{"route": "approved"}},
	}
	edges := []domain.WorkflowEdge{
		{FromNodeID: "start", ToNodeID: "approved-a", Condition: "payload.route == approved", Priority: 1},
		{FromNodeID: "start", ToNodeID: "approved-b", Condition: "payload.route == approved", Priority: 1},
		{FromNodeID: "start", ToNodeID: "fallback", Condition: "always", Priority: 50},
	}

	evaluation := router.EvaluateRoutes(ctx, "start", WorkflowNodeResult{RouteRequired: true}, edges)

	require.Equal(t, WorkflowRouteOutcomeMultiMatch, evaluation.Outcome)
	require.Len(t, evaluation.SelectedEdges, 2)
	assert.Equal(t, "approved-a", evaluation.SelectedEdges[0].ToNodeID)
	assert.Equal(t, "approved-b", evaluation.SelectedEdges[1].ToNodeID)
	assert.Equal(t, "fallback", evaluation.EvaluatedEdges[2].Edge.ToNodeID)
	assert.Equal(t, WorkflowRouteConditionResultFallbackDeferred, evaluation.EvaluatedEdges[2].Result)
	assert.NotContains(t, []string{evaluation.SelectedEdges[0].ToNodeID, evaluation.SelectedEdges[1].ToNodeID}, "fallback")
	assert.NoError(t, evaluation.Error)
}

func TestWorkflowRouteEvaluationUsesFallbackOnlyAfterHigherPriorityExplicitMisses(t *testing.T) {
	router := newWorkflowRouter()
	ctx := &WorkflowExecutionContext{
		Workflow: &domain.WorkflowDefinition{},
		Context:  &domain.WorkflowContext{Payload: map[string]any{"route": "review"}},
	}
	edges := []domain.WorkflowEdge{
		{FromNodeID: "start", ToNodeID: "approved", Condition: "payload.route == approved", Priority: 1},
		{FromNodeID: "start", ToNodeID: "fallback", Condition: "always", Priority: 50},
	}

	evaluation := router.EvaluateRoutes(ctx, "start", WorkflowNodeResult{RouteRequired: true}, edges)

	require.Equal(t, WorkflowRouteOutcomeFallbackMatch, evaluation.Outcome)
	require.Len(t, evaluation.SelectedEdges, 1)
	assert.Equal(t, "approved", evaluation.EvaluatedEdges[0].Edge.ToNodeID)
	assert.Equal(t, WorkflowRouteConditionResultMiss, evaluation.EvaluatedEdges[0].Result)
	assert.Equal(t, "fallback", evaluation.EvaluatedEdges[1].Edge.ToNodeID)
	assert.Equal(t, WorkflowRouteConditionResultFallbackMatch, evaluation.EvaluatedEdges[1].Result)
	assert.Equal(t, "fallback", evaluation.SelectedEdges[0].ToNodeID)
}

func TestWorkflowRouteEvaluationFallbackWinsOverLowerPriorityExplicitEdge(t *testing.T) {
	router := newWorkflowRouter()
	ctx := &WorkflowExecutionContext{
		Workflow: &domain.WorkflowDefinition{},
		Context:  &domain.WorkflowContext{Payload: map[string]any{"route": "approved"}},
	}
	edges := []domain.WorkflowEdge{
		{FromNodeID: "start", ToNodeID: "fallback", Condition: "always", Priority: 2},
		{FromNodeID: "start", ToNodeID: "approved", Condition: "payload.route == approved", Priority: 10},
	}

	evaluation := router.EvaluateRoutes(ctx, "start", WorkflowNodeResult{RouteRequired: true}, edges)

	require.Equal(t, WorkflowRouteOutcomeFallbackMatch, evaluation.Outcome)
	require.Len(t, evaluation.SelectedEdges, 1)
	assert.Equal(t, "fallback", evaluation.SelectedEdges[0].ToNodeID)
	assert.Equal(t, "fallback", evaluation.EvaluatedEdges[0].Edge.ToNodeID)
	assert.Equal(t, WorkflowRouteConditionResultFallbackMatch, evaluation.EvaluatedEdges[0].Result)
	assert.Equal(t, "approved", evaluation.EvaluatedEdges[1].Edge.ToNodeID)
	assert.Equal(t, WorkflowRouteConditionResultMatch, evaluation.EvaluatedEdges[1].Result)
}

func TestWorkflowRouteEvaluationFallbackLosesToHigherPriorityExplicitEdge(t *testing.T) {
	router := newWorkflowRouter()
	ctx := &WorkflowExecutionContext{
		Workflow: &domain.WorkflowDefinition{},
		Context:  &domain.WorkflowContext{Payload: map[string]any{"route": "approved"}},
	}
	edges := []domain.WorkflowEdge{
		{FromNodeID: "start", ToNodeID: "approved", Condition: "payload.route == approved", Priority: 1},
		{FromNodeID: "start", ToNodeID: "fallback", Condition: "always", Priority: 2},
	}

	evaluation := router.EvaluateRoutes(ctx, "start", WorkflowNodeResult{RouteRequired: true}, edges)

	require.Equal(t, WorkflowRouteOutcomeSingleMatch, evaluation.Outcome)
	require.Len(t, evaluation.SelectedEdges, 1)
	assert.Equal(t, "approved", evaluation.SelectedEdges[0].ToNodeID)
	assert.Equal(t, "approved", evaluation.EvaluatedEdges[0].Edge.ToNodeID)
	assert.Equal(t, WorkflowRouteConditionResultMatch, evaluation.EvaluatedEdges[0].Result)
	assert.Equal(t, "fallback", evaluation.EvaluatedEdges[1].Edge.ToNodeID)
	assert.Equal(t, WorkflowRouteConditionResultFallbackDeferred, evaluation.EvaluatedEdges[1].Result)
}

func TestWorkflowRouteEvaluationFallbackDoesNotBeatSamePriorityExplicitEdge(t *testing.T) {
	router := newWorkflowRouter()
	ctx := &WorkflowExecutionContext{
		Workflow: &domain.WorkflowDefinition{},
		Context:  &domain.WorkflowContext{Payload: map[string]any{"route": "approved"}},
	}
	edges := []domain.WorkflowEdge{
		{FromNodeID: "start", ToNodeID: "fallback", Condition: "always", Priority: 2},
		{FromNodeID: "start", ToNodeID: "approved", Condition: "payload.route == approved", Priority: 2},
	}

	evaluation := router.EvaluateRoutes(ctx, "start", WorkflowNodeResult{RouteRequired: true}, edges)

	require.Equal(t, WorkflowRouteOutcomeSingleMatch, evaluation.Outcome)
	require.Len(t, evaluation.SelectedEdges, 1)
	assert.Equal(t, "approved", evaluation.SelectedEdges[0].ToNodeID)
	assert.Equal(t, "fallback", evaluation.EvaluatedEdges[0].Edge.ToNodeID)
	assert.Equal(t, WorkflowRouteConditionResultFallbackDeferred, evaluation.EvaluatedEdges[0].Result)
	assert.Equal(t, "approved", evaluation.EvaluatedEdges[1].Edge.ToNodeID)
	assert.Equal(t, WorkflowRouteConditionResultMatch, evaluation.EvaluatedEdges[1].Result)
}
