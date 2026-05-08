package service

import (
	"content-hub/domain"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowHumanNodePausesCurrentTokenWithActionAndFormSchema(t *testing.T) {
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"review": &humanWorkflowNode{actionSchema: map[string]any{"type": "object", "properties": map[string]any{"decision": map[string]any{"type": "string"}}}, formSchema: map[string]any{"type": "object", "properties": map[string]any{"notes": map[string]any{"type": "string"}}}},
	})

	wf := &domain.WorkflowDefinition{
		Name:        "human-node",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "review",
		Nodes: []domain.WorkflowNode{{
			ID:   "review",
			Type: "human",
			Name: "Review",
		}},
	}
	runtimeCtx := &WorkflowExecutionContext{Workflow: wf, Context: &domain.WorkflowContext{}}

	err := kernel.executeFrom(context.Background(), runtimeCtx, wf.EntryNodeID)

	require.NoError(t, err)
	require.NotNil(t, runtimeCtx.CurrentToken)
	assert.Equal(t, WorkflowTokenStatePaused, runtimeCtx.CurrentToken.State)
	require.NotNil(t, runtimeCtx.CurrentToken.Branch)
	pauseState, ok := runtimeCtx.CurrentToken.Branch.Result[workflowPauseStateResultKey].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "review", pauseState["node_id"])
	assert.Equal(t, runtimeCtx.CurrentToken.ID, pauseState["token_id"])
	assert.Equal(t, []any{"continue_token"}, pauseState["allowed_resume_modes"])
	actionSchema, err := json.Marshal(pauseState["action_schema"])
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"object","properties":{"decision":{"type":"string"}}}`, string(actionSchema))
	formSchema, err := json.Marshal(pauseState["form_schema"])
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"object","properties":{"notes":{"type":"string"}}}`, string(formSchema))
	require.Len(t, runtimeCtx.Checkpoints, 1)
	assert.Equal(t, workflowHumanResumeInputMetadata(nil, false), map[string]any{workflowHumanResumeInputMetadataKey: runtimeCtx.Checkpoints[0].Metadata[workflowHumanResumeInputMetadataKey]})
}

func TestWorkflowHumanResumeMapsActionAndFormIntoTokenLocalResult(t *testing.T) {
	token := &WorkflowToken{Branch: newWorkflowBranchContext(map[string]any{}, map[string]any{})}
	runtimeCtx := &WorkflowExecutionContext{CurrentToken: token}

	applyWorkflowHumanResumeInput(runtimeCtx, map[string]any{"action": map[string]any{"decision": "approve"}, "form": map[string]any{"notes": "looks good"}})

	require.NotNil(t, token.Branch)
	result := token.Branch.Result
	require.NotNil(t, result)
	human, ok := result["human"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{"decision": "approve"}, human["action"])
	assert.Equal(t, map[string]any{"notes": "looks good"}, human["form"])
}

func TestWorkflowHumanResumeAppliesInputDuringKernelResume(t *testing.T) {
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"review": &humanWorkflowNode{actionSchema: map[string]any{"type": "object"}, formSchema: map[string]any{"type": "object"}},
	})

	wf := &domain.WorkflowDefinition{
		Name:        "human-node-resume",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "review",
		Nodes: []domain.WorkflowNode{{
			ID:   "review",
			Type: "human",
			Name: "Review",
		}},
	}
	runtimeCtx := &WorkflowExecutionContext{Workflow: wf, Context: &domain.WorkflowContext{}}

	err := kernel.executeFrom(context.Background(), runtimeCtx, wf.EntryNodeID)
	require.NoError(t, err)
	require.Len(t, runtimeCtx.Checkpoints, 1)
	runtimeCtx.Checkpoints[0].Metadata[workflowHumanResumeInputMetadataKey] = map[string]any{
		workflowHumanResumeSubmittedKey: true,
		"action": map[string]any{"decision": "approve"},
		"form":   map[string]any{"notes": "looks good"},
	}

	err = kernel.Resume(context.Background(), runtimeCtx)

	require.NoError(t, err)
	require.Len(t, runtimeCtx.CompletedTokens, 1)
	require.NotNil(t, runtimeCtx.CompletedTokens[0].Branch)
	human, ok := runtimeCtx.CompletedTokens[0].Branch.Result["human"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{"decision": "approve"}, human["action"])
	assert.Equal(t, map[string]any{"notes": "looks good"}, human["form"])
	assert.Equal(t, true, human[workflowHumanResumeSubmittedKey])
}

func TestWorkflowHumanResumeAllowsEmptyExplicitSubmission(t *testing.T) {
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"review": &humanWorkflowNode{actionSchema: map[string]any{"type": "object"}, formSchema: map[string]any{"type": "object"}},
	})

	wf := &domain.WorkflowDefinition{
		Name:        "human-node-empty-resume",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "review",
		Nodes: []domain.WorkflowNode{{
			ID:   "review",
			Type: "human",
			Name: "Review",
		}},
	}
	runtimeCtx := &WorkflowExecutionContext{Workflow: wf, Context: &domain.WorkflowContext{}}

	err := kernel.executeFrom(context.Background(), runtimeCtx, wf.EntryNodeID)
	require.NoError(t, err)
	require.Len(t, runtimeCtx.Checkpoints, 1)
	runtimeCtx.Checkpoints[0].Metadata[workflowHumanResumeInputMetadataKey] = workflowCheckpointPayload(workflowHumanResumeInputMetadata(map[string]any{}, true)[workflowHumanResumeInputMetadataKey])

	err = kernel.Resume(context.Background(), runtimeCtx)

	require.NoError(t, err)
	require.Len(t, runtimeCtx.CompletedTokens, 1)
	require.NotNil(t, runtimeCtx.CompletedTokens[0].Branch)
	human, ok := runtimeCtx.CompletedTokens[0].Branch.Result["human"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{}, human["action"])
	assert.Equal(t, map[string]any{}, human["form"])
	assert.Equal(t, true, human[workflowHumanResumeSubmittedKey])
}
