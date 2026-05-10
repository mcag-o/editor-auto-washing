package service

import (
	"content-hub/domain"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowExecutionFrameDoesNotShareMutableStateBetweenTokens(t *testing.T) {
	runtimeCtx := &WorkflowExecutionContext{
		Context:  &domain.WorkflowContext{Payload: map[string]any{"title": "shared"}},
		Metadata: map[string]any{"source": "upload"},
	}
	leftToken := &WorkflowToken{Branch: &WorkflowBranchContext{
		Variables: map[string]any{"branch": "left"},
		Result:    map[string]any{"branch": "left"},
		Artifacts: map[string]any{"branch": "left"},
	}}
	rightToken := &WorkflowToken{Branch: &WorkflowBranchContext{
		Variables: map[string]any{"branch": "right"},
		Result:    map[string]any{"branch": "right"},
		Artifacts: map[string]any{"branch": "right"},
	}}
	left := newWorkflowTokenExecutionFrame(runtimeCtx, leftToken)
	right := newWorkflowTokenExecutionFrame(runtimeCtx, rightToken)

	require.NotNil(t, left)
	require.NotNil(t, right)

	left.Input["title"] = "left"
	left.Metadata["source"] = "left"
	leftToken.Branch.Variables["branch"] = "left-updated"
	leftToken.Branch.Result["branch"] = "left-updated"
	leftToken.Branch.Artifacts["branch"] = "left-updated"

	assert.Equal(t, "shared", right.Input["title"])
	assert.Equal(t, "upload", right.Metadata["source"])
	assert.Equal(t, "right", rightToken.Branch.Variables["branch"])
	assert.Equal(t, "right", rightToken.Branch.Result["branch"])
	assert.Equal(t, "right", rightToken.Branch.Artifacts["branch"])
	assert.Equal(t, "shared", runtimeCtx.Context.Payload["title"])
	assert.Equal(t, "upload", runtimeCtx.Metadata["source"])
}
