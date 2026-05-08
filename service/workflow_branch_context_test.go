package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowBranchContextCopiesMutableStatePerChildToken(t *testing.T) {
	input := map[string]any{"title": "shared-input"}
	metadata := map[string]any{"source": "upload"}
	root := newWorkflowRootToken("start")
	root.Branch = newWorkflowBranchContext(input, metadata)
	root.Branch.Variables = map[string]any{"shared": "root", "nested": map[string]any{"flag": "root"}}
	root.Branch.Result = map[string]any{"decision": "root", "nested": map[string]any{"items": []any{"root"}}}
	root.Branch.Artifacts = map[string]any{"draft": "root", "nested": map[string]any{"kind": "root"}}
	runtimeCtx := &WorkflowExecutionContext{Metadata: metadata}
	assert.Equal(t, map[string]any{"source": "upload"}, workflowRuntimeSharedMetadata(runtimeCtx))

	left := root.Child("left", WorkflowTokenRouteLineage{SelectedNodeID: "left"})
	right := root.Child("right", WorkflowTokenRouteLineage{SelectedNodeID: "right"})

	require.NotNil(t, left.Branch)
	require.NotNil(t, right.Branch)
	assert.NotSame(t, root.Branch, left.Branch)
	assert.NotSame(t, root.Branch, right.Branch)
	assert.NotSame(t, left.Branch, right.Branch)
	assert.Equal(t, map[string]any{"title": "shared-input"}, input)
	assert.Equal(t, map[string]any{"source": "upload"}, metadata)
	left.Branch.Variables["shared"] = "left"
	left.Branch.Result["decision"] = "left"
	left.Branch.Artifacts["draft"] = "left"
	left.Branch.Variables["nested"].(map[string]any)["flag"] = "left"
	left.Branch.Result["nested"].(map[string]any)["items"].([]any)[0] = "left"
	left.Branch.Artifacts["nested"].(map[string]any)["kind"] = "left"

	assert.Equal(t, "root", root.Branch.Variables["shared"])
	assert.Equal(t, "root", root.Branch.Result["decision"])
	assert.Equal(t, "root", root.Branch.Artifacts["draft"])
	assert.Equal(t, "root", right.Branch.Variables["shared"])
	assert.Equal(t, "root", right.Branch.Result["decision"])
	assert.Equal(t, "root", right.Branch.Artifacts["draft"])
	assert.Equal(t, "root", root.Branch.Variables["nested"].(map[string]any)["flag"])
	assert.Equal(t, "root", right.Branch.Variables["nested"].(map[string]any)["flag"])
	assert.Equal(t, "root", root.Branch.Result["nested"].(map[string]any)["items"].([]any)[0])
	assert.Equal(t, "root", right.Branch.Result["nested"].(map[string]any)["items"].([]any)[0])
	assert.Equal(t, "root", root.Branch.Artifacts["nested"].(map[string]any)["kind"])
	assert.Equal(t, "root", right.Branch.Artifacts["nested"].(map[string]any)["kind"])
	assert.Equal(t, map[string]any{"title": "shared-input"}, input)
	assert.Equal(t, map[string]any{"source": "upload"}, metadata)

	right.Branch.Variables["new"] = "right"
	right.Branch.Result["outcome"] = "right"
	right.Branch.Artifacts["asset"] = "right"

	assert.NotContains(t, left.Branch.Variables, "new")
	assert.NotContains(t, left.Branch.Result, "outcome")
	assert.NotContains(t, left.Branch.Artifacts, "asset")
	assert.NotContains(t, root.Branch.Variables, "new")
	assert.NotContains(t, root.Branch.Result, "outcome")
	assert.NotContains(t, root.Branch.Artifacts, "asset")
}
