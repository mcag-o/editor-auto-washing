package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowSubflowRunsInlineAndReturnsToParentToken(t *testing.T) {
	frame := &workflowSubflowFrame{
		ParentTokenID:   "token-parent",
		ParentNodeID:    "subflow",
		ChildWorkflowID: "child-flow",
		EntryNodeID:     "child-start",
		ReturnNodeID:    "after-subflow",
		ReturnMapping:   map[string]string{"headline": "title"},
		State:           workflowSubflowStateRunning,
	}

	require.Equal(t, "token-parent", frame.ParentTokenID)
	assert.Equal(t, workflowSubflowStateRunning, frame.State)
	assert.Equal(t, "after-subflow", frame.ReturnNodeID)
}

func TestWorkflowSubflowFailureRespectsConfiguredParentStrategy(t *testing.T) {
	frame := &workflowSubflowFrame{State: workflowSubflowStateFailed, FailureStrategy: workflowSubflowFailureStrategyPauseParent}

	assert.Equal(t, workflowSubflowStateFailed, frame.State)
	assert.Equal(t, workflowSubflowFailureStrategyPauseParent, frame.FailureStrategy)
}

func TestWorkflowSubflowOnlyMapsExplicitOutputFieldsBackToParent(t *testing.T) {
	parent := &WorkflowToken{Branch: &WorkflowBranchContext{
		Variables: map[string]any{"parent": "keep"},
		Result:    map[string]any{"existing": "keep"},
		Artifacts: map[string]any{},
	}}
	child := &WorkflowToken{Branch: &WorkflowBranchContext{
		Variables: map[string]any{"headline": "Child Title", "ignored": "drop"},
		Result:    map[string]any{"summary": "Child Summary", "ignored_result": "drop"},
		Artifacts: map[string]any{"asset": "asset-1"},
	}}

	applyWorkflowSubflowReturnMapping(parent, child, map[string]string{
		"headline": "title",
		"summary":  "summary",
	})

	require.NotNil(t, parent.Branch)
	assert.Equal(t, "Child Title", parent.Branch.Variables["title"])
	assert.Equal(t, "Child Summary", parent.Branch.Result["summary"])
	assert.Equal(t, "keep", parent.Branch.Result["existing"])
	assert.Nil(t, parent.Branch.Variables["ignored"])
	assert.Nil(t, parent.Branch.Result["ignored_result"])
	assert.Nil(t, parent.Branch.Artifacts["asset"])
}
