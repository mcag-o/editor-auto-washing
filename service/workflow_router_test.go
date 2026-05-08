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
