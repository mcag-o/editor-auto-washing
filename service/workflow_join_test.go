package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowJoinBarrierWaitsForAllIncomingBranches(t *testing.T) {
	barrier := newWorkflowJoinBarrier("join-review", []string{"token-left", "token-right"})

	require.False(t, barrier.Ready())
	barrier.Arrive("token-left")
	require.False(t, barrier.Ready())
	barrier.Arrive("token-right")
	require.True(t, barrier.Ready())
}

func TestWorkflowJoinBarrierBlocksOnRequiredBranchFailure(t *testing.T) {
	barrier := newWorkflowJoinBarrier("join-review", []string{"token-left", "token-right"})
	barrier.Fail("token-left")

	require.Equal(t, workflowJoinBarrierStateBlocked, barrier.State)
}

func TestWorkflowJoinBarrierAppliesExplicitMergePolicy(t *testing.T) {
	left := &WorkflowToken{Branch: &WorkflowBranchContext{
		Variables: map[string]any{"owner": "left", "shared": "left"},
		Result:    map[string]any{"winner": "left"},
		Artifacts: map[string]any{"path": "left"},
	}}
	right := &WorkflowToken{Branch: &WorkflowBranchContext{
		Variables: map[string]any{"owner": "right", "shared": "right"},
		Result:    map[string]any{"winner": "right"},
		Artifacts: map[string]any{"path": "right"},
	}}

	merged := mergeWorkflowJoinBranches([]*WorkflowToken{left, right}, workflowJoinMergePolicy{
		Variables: workflowJoinMergeStrategyFirstWriterWins,
		Result:    workflowJoinMergeStrategyLastWriterWins,
		Artifacts: workflowJoinMergeStrategyLastWriterWins,
	})

	require.NotNil(t, merged)
	assert.Equal(t, "left", merged.Variables["shared"])
	assert.Equal(t, "right", merged.Result["winner"])
	assert.Equal(t, "right", merged.Artifacts["path"])
}
