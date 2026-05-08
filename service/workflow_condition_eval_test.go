package service

import (
	"testing"

	"content-hub/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowConditionEvaluatorSupportsExplicitNamespaces(t *testing.T) {
	evaluator := newWorkflowConditionEvaluator()
	runtimeCtx := &WorkflowExecutionContext{
		Context: &domain.WorkflowContext{
			Payload: map[string]any{
				"status": "approved",
				"count":  3,
			},
		},
		Variables: map[string]any{
			"flag": true,
		},
		Metadata: map[string]any{
			"source": "upload",
		},
		Artifacts: map[string]any{
			"draft": map[string]any{"type": "html"},
		},
	}
	result := WorkflowNodeResult{
		Output: map[string]any{
			"decision": "go",
		},
	}

	tests := []struct {
		name      string
		condition string
	}{
		{name: "input namespace", condition: "input.status == approved"},
		{name: "vars namespace", condition: "vars.flag == true"},
		{name: "metadata namespace", condition: "metadata.source == upload"},
		{name: "result namespace", condition: "result.decision == go"},
		{name: "artifacts namespace", condition: "artifacts.draft.type == html"},
		{name: "numeric comparison", condition: "input.count >= 3"},
		{name: "boolean and", condition: "input.status == approved and vars.flag == true"},
		{name: "boolean or", condition: "input.status == denied or metadata.source == upload"},
		{name: "boolean not", condition: "not input.status == denied"},
		{name: "inequality", condition: "result.decision != stop"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, err := evaluator.Evaluate(runtimeCtx, result, tt.condition)

			require.NoError(t, err)
			assert.True(t, matched)
		})
	}
}

func TestWorkflowConditionEvaluatorReturnsEvaluationErrorForUnknownNamespace(t *testing.T) {
	evaluator := newWorkflowConditionEvaluator()

	matched, err := evaluator.Evaluate(&WorkflowExecutionContext{}, WorkflowNodeResult{}, "unknown.route == approved")

	assert.False(t, matched)
	require.Error(t, err)
	assert.EqualError(t, err, "unsupported workflow condition namespace \"unknown\"")
}

func TestWorkflowConditionEvaluatorTreatsEmptyConditionAsFallbackCandidate(t *testing.T) {
	evaluator := newWorkflowConditionEvaluator()

	for _, condition := range []string{"", "   ", "always"} {
		matched, err := evaluator.Evaluate(&WorkflowExecutionContext{}, WorkflowNodeResult{}, condition)

		require.NoError(t, err)
		assert.True(t, matched)
	}
}

func TestWorkflowConditionEvaluatorSupportsPayloadAlias(t *testing.T) {
	evaluator := newWorkflowConditionEvaluator()
	runtimeCtx := &WorkflowExecutionContext{
		Context: &domain.WorkflowContext{
			Payload: map[string]any{"route": "approved"},
		},
	}

	matched, err := evaluator.Evaluate(runtimeCtx, WorkflowNodeResult{}, "payload.route == approved")

	require.NoError(t, err)
	assert.True(t, matched)
}
