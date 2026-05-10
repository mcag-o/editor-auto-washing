package service

import (
	"content-hub/domain"
	"context"
	"testing"
	"time"

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

func TestPauseWorkflowTokenPreservesScope(t *testing.T) {
	token := &WorkflowToken{ID: "token-1", NodeID: "subflow"}
	pauseWorkflowToken(token, &WorkflowPauseState{
		Source:             WorkflowPauseSourcePolicy,
		Scope:              WorkflowPauseScopeToken,
		Reason:             "child failed",
		AllowedResumeModes: []WorkflowResumeMode{WorkflowResumeModeContinueToken},
	})

	require.NotNil(t, token.PauseState)
	assert.Equal(t, WorkflowPauseScopeToken, token.PauseState.Scope)
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

type inlineChildWorkflowNode struct {
	childWorkflow *domain.WorkflowDefinition
}

type childInlineEntryWorkflowNode struct {
	order *[]string
}

type inlineFailingChildWorkflowNode struct {
	strategy      workflowSubflowFailureStrategy
	childWorkflow *domain.WorkflowDefinition
}

type childContextProbeWorkflowNode struct {
	ctxValue *string
	ctxErr   *error
	hasDeadline *bool
}

func (n *inlineChildWorkflowNode) Name() string {
	return "subflow"
}

func (n *inlineChildWorkflowNode) Execute(_ context.Context, _ *domain.WorkflowContext) error {
	return nil
}

func (n *inlineChildWorkflowNode) ExecuteWorkflow(_ context.Context, runtimeCtx *WorkflowExecutionContext, _ domain.WorkflowNode) (WorkflowNodeResult, error) {
	runtimeCtx.CurrentToken.Subflow = &workflowSubflowFrame{
		ParentTokenID:   runtimeCtx.CurrentToken.ID,
		ParentNodeID:    "subflow",
		ChildWorkflowID: "child-flow",
		ChildWorkflow:   cloneWorkflowDefinition(n.childWorkflow),
		EntryNodeID:     "child-start",
		ReturnNodeID:    "after-subflow",
		ReturnMapping:   map[string]string{"headline": "title"},
		ParentBranch:    cloneWorkflowBranchContext(runtimeCtx.CurrentToken.Branch),
		State:           workflowSubflowStateRunning,
			FailureStrategy:  workflowSubflowFailureStrategyContinueParent,
		}
	return WorkflowNodeResult{}, nil
}

func (n *childInlineEntryWorkflowNode) Name() string {
	return "child-start"
}

func (n *childInlineEntryWorkflowNode) Execute(_ context.Context, _ *domain.WorkflowContext) error {
	return nil
}

func (n *childInlineEntryWorkflowNode) ExecuteWorkflow(_ context.Context, runtimeCtx *WorkflowExecutionContext, _ domain.WorkflowNode) (WorkflowNodeResult, error) {
	if n.order != nil {
		*n.order = append(*n.order, "child-start")
	}
	runtimeCtx.Variables["headline"] = "Child Title"
	runtimeCtx.Variables["ignored"] = "drop"
	return WorkflowNodeResult{}, nil
}

func (n *inlineFailingChildWorkflowNode) Name() string {
	return "subflow"
}

func (n *inlineFailingChildWorkflowNode) Execute(_ context.Context, _ *domain.WorkflowContext) error {
	return nil
}

func (n *inlineFailingChildWorkflowNode) ExecuteWorkflow(_ context.Context, runtimeCtx *WorkflowExecutionContext, _ domain.WorkflowNode) (WorkflowNodeResult, error) {
	runtimeCtx.CurrentToken.Subflow = &workflowSubflowFrame{
		ParentTokenID:   runtimeCtx.CurrentToken.ID,
		ParentNodeID:    "subflow",
		ChildWorkflowID: "child-flow",
		ChildWorkflow:   cloneWorkflowDefinition(n.childWorkflow),
		EntryNodeID:     "child-start",
		ReturnNodeID:    "after-subflow",
		ParentBranch:    cloneWorkflowBranchContext(runtimeCtx.CurrentToken.Branch),
		State:           workflowSubflowStateRunning,
		FailureStrategy: n.strategy,
		}
	return WorkflowNodeResult{}, nil
}

func (n *childContextProbeWorkflowNode) Name() string {
	return "child-start"
}

func (n *childContextProbeWorkflowNode) Execute(_ context.Context, _ *domain.WorkflowContext) error {
	return nil
}

func (n *childContextProbeWorkflowNode) ExecuteWorkflow(ctx context.Context, _ *WorkflowExecutionContext, _ domain.WorkflowNode) (WorkflowNodeResult, error) {
	if n.ctxValue != nil {
		if value, ok := ctx.Value("subflow-trace").(string); ok {
			*n.ctxValue = value
		}
	}
	if n.ctxErr != nil {
		*n.ctxErr = ctx.Err()
	}
	if n.hasDeadline != nil {
		_, ok := ctx.Deadline()
		*n.hasDeadline = ok
	}
	return WorkflowNodeResult{AllowNaturalTermination: true}, nil
}

func TestWorkflowSubflowFrameMetadataRoundTripsThroughCheckpointMetadata(t *testing.T) {
	childWorkflow := &domain.WorkflowDefinition{
		ID:          "child-flow",
		Name:        "child-flow",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "child-start",
		Nodes: []domain.WorkflowNode{
			{ID: "child-start", Type: "action", Name: "ChildStart"},
			{ID: "child-end", Type: "action", Name: "ChildEnd"},
		},
		Edges: []domain.WorkflowEdge{{FromNodeID: "child-start", ToNodeID: "child-end", Priority: 1}},
	}
	token := &WorkflowToken{
		ID:            "token-parent",
		NodeID:        "subflow",
		ParentTokenID: "token-root",
		OriginTokenID: "token-root",
		Branch: &WorkflowBranchContext{
			Variables: map[string]any{"title": "Parent Title"},
			Result:    map[string]any{"decision": "approved"},
			Artifacts: map[string]any{"asset": "asset-1"},
		},
		Subflow: &workflowSubflowFrame{
			ParentTokenID:   "token-parent",
			ParentNodeID:    "subflow",
			ChildWorkflowID: "child-flow",
			EntryNodeID:     "child-start",
			ReturnNodeID:    "after-subflow",
			ReturnMapping:   map[string]string{"headline": "title"},
			ParentBranch: &WorkflowBranchContext{
				Variables: map[string]any{"title": "Parent Title"},
				Result:    map[string]any{"decision": "approved"},
				Artifacts: map[string]any{"asset": "asset-1"},
			},
			ChildWorkflow:   childWorkflow,
			State:           workflowSubflowStateRunning,
			FailureStrategy: workflowSubflowFailureStrategyPauseParent,
		},
	}

	metadata := workflowTokenMetadata(*token)
	rawFrame, ok := metadata["token_subflow_frame"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "child-flow", rawFrame["child_workflow_id"])
	assert.Equal(t, "child-start", rawFrame["entry_node_id"])
	rawChild, ok := rawFrame["child_workflow"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "child-flow", rawChild["id"])
	assert.Equal(t, "child-start", rawChild["entry_node_id"])
	restored := workflowTokenFromMetadata("subflow", metadata)

	require.NotNil(t, restored)
	require.NotNil(t, restored.Subflow)
	assert.Equal(t, token.Subflow.ParentTokenID, restored.Subflow.ParentTokenID)
	assert.Equal(t, token.Subflow.ParentNodeID, restored.Subflow.ParentNodeID)
	assert.Equal(t, token.Subflow.ChildWorkflowID, restored.Subflow.ChildWorkflowID)
	assert.Equal(t, token.Subflow.EntryNodeID, restored.Subflow.EntryNodeID)
	assert.Equal(t, token.Subflow.ReturnNodeID, restored.Subflow.ReturnNodeID)
	assert.Equal(t, token.Subflow.ReturnMapping, restored.Subflow.ReturnMapping)
	assert.Equal(t, token.Subflow.State, restored.Subflow.State)
	assert.Equal(t, token.Subflow.FailureStrategy, restored.Subflow.FailureStrategy)
	require.NotNil(t, restored.Subflow.ParentBranch)
	assert.Equal(t, token.Subflow.ParentBranch.Variables, restored.Subflow.ParentBranch.Variables)
	assert.Equal(t, token.Subflow.ParentBranch.Result, restored.Subflow.ParentBranch.Result)
	assert.Equal(t, token.Subflow.ParentBranch.Artifacts, restored.Subflow.ParentBranch.Artifacts)
	require.NotNil(t, restored.Subflow.ChildWorkflow)
	assert.Equal(t, childWorkflow.ID, restored.Subflow.ChildWorkflow.ID)
	assert.Equal(t, childWorkflow.EntryNodeID, restored.Subflow.ChildWorkflow.EntryNodeID)
	assert.Len(t, restored.Subflow.ChildWorkflow.Nodes, 2)
	assert.Len(t, restored.Subflow.ChildWorkflow.Edges, 1)
}

func TestWorkflowSubflowRestorePrefersDedicatedFrameSnapshot(t *testing.T) {
	metadata := map[string]any{
		"token_id":   "token-parent",
		"node_id":    "subflow",
		"token_origin_id": "token-root",
		"subflow_child_workflow_id": "wrong-child",
		"subflow_entry_node_id":     "wrong-entry",
		"subflow_return_node_id":    "wrong-return",
		"token_subflow_frame": map[string]any{
			"parent_token_id":   "token-parent",
			"parent_node_id":    "subflow",
			"child_workflow_id": "child-flow",
			"entry_node_id":     "child-start",
			"return_node_id":    "after-subflow",
			"state":             string(workflowSubflowStateRunning),
			"failure_strategy":  string(workflowSubflowFailureStrategyContinueParent),
			"child_workflow": map[string]any{
				"id":            "child-flow",
				"entry_node_id": "child-start",
				"enabled":       true,
				"nodes": []map[string]any{{"id": "child-start", "type": "action", "name": "ChildStart"}},
			},
		},
	}

	restored := workflowTokenFromMetadata("subflow", metadata)

	require.NotNil(t, restored)
	require.NotNil(t, restored.Subflow)
	assert.Equal(t, "child-flow", restored.Subflow.ChildWorkflowID)
	assert.Equal(t, "child-start", restored.Subflow.EntryNodeID)
	assert.Equal(t, "after-subflow", restored.Subflow.ReturnNodeID)
	assert.Equal(t, workflowSubflowFailureStrategyContinueParent, restored.Subflow.FailureStrategy)
	require.NotNil(t, restored.Subflow.ChildWorkflow)
	assert.Equal(t, "child-flow", restored.Subflow.ChildWorkflow.ID)
}

func TestWorkflowSubflowRestoreFallsBackFromPartialDedicatedSnapshotToLegacyFields(t *testing.T) {
	metadata := map[string]any{
		"token_id":                    "token-parent",
		"node_id":                     "subflow",
		"token_origin_id":             "token-root",
		"subflow_parent_token_id":     "token-parent",
		"subflow_parent_node_id":      "subflow",
		"subflow_child_workflow_id":   "child-flow",
		"subflow_entry_node_id":       "child-start",
		"subflow_return_node_id":      "after-subflow",
		"subflow_state":               string(workflowSubflowStateRunning),
		"subflow_failure_strategy":    string(workflowSubflowFailureStrategyContinueParent),
		"subflow_child_workflow":      map[string]any{"id": "child-flow", "entry_node_id": "child-start", "enabled": true, "nodes": []map[string]any{{"id": "child-start", "type": "action", "name": "ChildStart"}}},
		"token_subflow_frame":         map[string]any{"parent_token_id": "token-parent"},
	}

	restored := workflowTokenFromMetadata("subflow", metadata)

	require.NotNil(t, restored)
	require.NotNil(t, restored.Subflow)
	assert.Equal(t, "child-flow", restored.Subflow.ChildWorkflowID)
	assert.Equal(t, "child-start", restored.Subflow.EntryNodeID)
	assert.Equal(t, "after-subflow", restored.Subflow.ReturnNodeID)
	assert.Equal(t, workflowSubflowFailureStrategyContinueParent, restored.Subflow.FailureStrategy)
	require.NotNil(t, restored.Subflow.ChildWorkflow)
	assert.Equal(t, "child-flow", restored.Subflow.ChildWorkflow.ID)
}

func TestWorkflowSubflowCheckpointSnapshotRestoresSubflowFrameState(t *testing.T) {
	runtimeCtx := &WorkflowExecutionContext{}
	childWorkflow := &domain.WorkflowDefinition{
		ID:          "child-flow",
		Name:        "child-flow",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "child-start",
		Nodes: []domain.WorkflowNode{{ID: "child-start", Type: "action", Name: "ChildStart"}},
	}
	token := &WorkflowToken{
		ID:            "token-parent",
		NodeID:        "subflow",
		ParentTokenID: "token-root",
		OriginTokenID: "token-root",
		Subflow: &workflowSubflowFrame{
			ParentTokenID:   "token-parent",
			ParentNodeID:    "subflow",
			ChildWorkflowID: "child-flow",
			EntryNodeID:     "child-start",
			ReturnNodeID:    "after-subflow",
			ReturnMapping:   map[string]string{"headline": "title"},
			ChildWorkflow:   childWorkflow,
			State:           workflowSubflowStateRunning,
			FailureStrategy: workflowSubflowFailureStrategyContinueParent,
		},
	}

	appendCheckpointWithSnapshot(runtimeCtx, "run-1", "subflow", workflowCheckpointSnapshot{Token: token})

	require.Len(t, runtimeCtx.Checkpoints, 1)
	rawChild, ok := runtimeCtx.Checkpoints[0].Metadata["token_subflow_frame"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "child-flow", rawChild["child_workflow_id"])
	restored := workflowTokenFromCheckpoint(&runtimeCtx.Checkpoints[0])
	require.NotNil(t, restored)
	require.NotNil(t, restored.Subflow)
	assert.Equal(t, token.Subflow.ChildWorkflowID, restored.Subflow.ChildWorkflowID)
	assert.Equal(t, token.Subflow.EntryNodeID, restored.Subflow.EntryNodeID)
	assert.Equal(t, token.Subflow.ReturnNodeID, restored.Subflow.ReturnNodeID)
	assert.Equal(t, token.Subflow.FailureStrategy, restored.Subflow.FailureStrategy)
	require.NotNil(t, restored.Subflow.ChildWorkflow)
	assert.Equal(t, childWorkflow.ID, restored.Subflow.ChildWorkflow.ID)
}

func TestWorkflowSubflowResolveDefinitionUsesSnapshotBeforeRuntimeLookup(t *testing.T) {
	childWorkflow := &domain.WorkflowDefinition{
		ID:          "child-flow",
		Name:        "child-flow",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "child-start",
		Nodes: []domain.WorkflowNode{{ID: "child-start", Type: "action", Name: "ChildStart"}},
	}
	frame := &workflowSubflowFrame{
		ChildWorkflowID: "child-flow",
		EntryNodeID:     "child-start",
		ReturnNodeID:    "after-subflow",
		ChildWorkflow:   childWorkflow,
		FailureStrategy: workflowSubflowFailureStrategyContinueParent,
	}

	resolved, err := workflowResolveSubflowDefinition(&WorkflowExecutionContext{}, frame)

	require.NoError(t, err)
	require.NotNil(t, resolved)
	assert.Equal(t, childWorkflow.ID, resolved.ID)
	assert.Equal(t, childWorkflow.EntryNodeID, resolved.EntryNodeID)
}

func TestWorkflowSubflowInlineExecutionUsesEmbeddedChildWorkflowSnapshotWithoutRuntimeLookup(t *testing.T) {
	var order []string
	childWorkflow := &domain.WorkflowDefinition{
		ID:          "child-flow",
		Name:        "child-flow",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "child-start",
		Nodes: []domain.WorkflowNode{
			{ID: "child-start", Type: "action", Name: "ChildStart"},
			{ID: "child-end", Type: "action", Name: "ChildEnd"},
		},
		Edges: []domain.WorkflowEdge{{FromNodeID: "child-start", ToNodeID: "child-end", Priority: 1}},
	}
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"start":         &recordingWorkflowNode{label: "start", order: &order},
		"subflow":       &inlineChildWorkflowNode{childWorkflow: childWorkflow},
		"after-subflow": &recordingWorkflowNode{label: "after-subflow", order: &order},
		"child-start":   &childInlineEntryWorkflowNode{order: &order},
		"child-end":     &recordingWorkflowNode{label: "child-end", order: &order},
	})
	parentWorkflow := &domain.WorkflowDefinition{
		ID:          "parent-flow",
		Name:        "parent-flow",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "start",
		Nodes: []domain.WorkflowNode{{ID: "start", Type: "action", Name: "Start"}, {ID: "subflow", Type: "action", Name: "Subflow"}, {ID: "after-subflow", Type: "action", Name: "AfterSubflow"}},
		Edges: []domain.WorkflowEdge{{FromNodeID: "start", ToNodeID: "subflow", Priority: 1}, {FromNodeID: "subflow", ToNodeID: "after-subflow", Priority: 1}},
	}
	runtimeCtx := &WorkflowExecutionContext{
		Workflow: parentWorkflow,
		Context:  &domain.WorkflowContext{Payload: map[string]any{"title": "Parent Title"}},
	}

	err := kernel.executeFrom(context.Background(), runtimeCtx, parentWorkflow.EntryNodeID)

	require.NoError(t, err)
	assert.Equal(t, []string{"start", "child-start", "child-end", "after-subflow"}, order)
}

func TestWorkflowSubflowExecuteInlineUsesCallerContext(t *testing.T) {
	var observed string
	var observedErr error
	var hasDeadline bool
	childWorkflow := &domain.WorkflowDefinition{
		ID:          "child-flow",
		Name:        "child-flow",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "child-start",
		Nodes: []domain.WorkflowNode{
			{ID: "child-start", Type: "action", Name: "ChildStart"},
			{ID: "child-end", Type: "action", Name: "ChildEnd"},
		},
		Edges: []domain.WorkflowEdge{{FromNodeID: "child-start", ToNodeID: "child-end", Priority: 1}},
	}
	var order []string
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"start":         &recordingWorkflowNode{label: "start", order: &order},
		"subflow":       &inlineChildWorkflowNode{childWorkflow: childWorkflow},
		"after-subflow": &recordingWorkflowNode{label: "after-subflow", order: &order},
		"child-start":   &childContextProbeWorkflowNode{ctxValue: &observed, ctxErr: &observedErr, hasDeadline: &hasDeadline},
		"child-end":     &recordingWorkflowNode{label: "child-end", order: &order},
	})
	runtimeCtx := &WorkflowExecutionContext{
		Workflow: &domain.WorkflowDefinition{ID: "parent-flow", EntryNodeID: "start", Nodes: []domain.WorkflowNode{{ID: "start", Type: "action", Name: "Start"}, {ID: "subflow", Type: "action", Name: "Subflow"}, {ID: "after-subflow", Type: "action", Name: "AfterSubflow"}}, Edges: []domain.WorkflowEdge{{FromNodeID: "start", ToNodeID: "subflow", Priority: 1}, {FromNodeID: "subflow", ToNodeID: "after-subflow", Priority: 1}}},
		Context:  &domain.WorkflowContext{},
	}
	ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), "subflow-trace", "trace-123"), time.Second)
	defer cancel()

	err := kernel.executeFrom(ctx, runtimeCtx, runtimeCtx.Workflow.EntryNodeID)

	require.NoError(t, err)
	assert.Equal(t, "trace-123", observed)
	assert.NoError(t, observedErr)
	assert.True(t, hasDeadline)
}

func TestWorkflowSubflowExecuteInlineRejectsMissingFailureStrategyBeforeExecution(t *testing.T) {
	childWorkflow := &domain.WorkflowDefinition{
		ID:          "child-flow",
		Name:        "child-flow",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "child-start",
		Nodes: []domain.WorkflowNode{{ID: "child-start", Type: "action", Name: "ChildStart"}},
	}
	parentToken := &WorkflowToken{
		ID:            "token-parent",
		NodeID:        "subflow",
		OriginTokenID: "token-parent",
		Branch:        newWorkflowBranchContext(nil, nil),
		Subflow: &workflowSubflowFrame{
			ParentTokenID:   "token-parent",
			ParentNodeID:    "subflow",
			ChildWorkflowID: "child-flow",
			ChildWorkflow:   childWorkflow,
			EntryNodeID:     "child-start",
			ReturnNodeID:    "after-subflow",
			State:           workflowSubflowStateRunning,
		},
	}
	runtimeCtx := &WorkflowExecutionContext{
		Workflow: &domain.WorkflowDefinition{ID: "parent-flow", EntryNodeID: "subflow", Nodes: []domain.WorkflowNode{{ID: "subflow", Type: "action", Name: "Subflow"}, {ID: "after-subflow", Type: "action", Name: "AfterSubflow"}}, Edges: []domain.WorkflowEdge{{FromNodeID: "subflow", ToNodeID: "after-subflow", Priority: 1}}},
		Context:  &domain.WorkflowContext{},
	}
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{"child-start": &failOnceWorkflowNode{label: "child-start", failed: false}})

	err := kernel.executeInlineSubflow(context.Background(), runtimeCtx, parentToken, runtimeCtx.Workflow)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "subflow failure strategy is required")
}

func TestWorkflowSubflowReturnTokenPreservesFailureSignalForContinueParent(t *testing.T) {
	parentToken := &WorkflowToken{
		ID:            "token-parent",
		NodeID:        "subflow",
		OriginTokenID: "token-parent",
		Branch:        newWorkflowBranchContext(nil, nil),
		Subflow: &workflowSubflowFrame{
			ParentTokenID:   "token-parent",
			ParentNodeID:    "subflow",
			ChildWorkflowID: "child-flow",
			EntryNodeID:     "child-start",
			ReturnNodeID:    "after-subflow",
			ParentBranch:    cloneWorkflowBranchContext(newWorkflowBranchContext(nil, nil)),
			State:           workflowSubflowStateFailed,
			FailureStrategy: workflowSubflowFailureStrategyContinueParent,
		},
	}
	markWorkflowSubflowFailure(parentToken, assert.AnError)

	continuation := workflowSubflowReturnToken(parentToken, nil)

	require.NotNil(t, continuation)
	require.NotNil(t, continuation.Branch)
	assert.Equal(t, "failed", continuation.Branch.Result["subflow_status"])
	assert.Equal(t, "child-flow", continuation.Branch.Result["subflow_child_workflow_id"])
	assert.Equal(t, string(workflowSubflowFailureStrategyContinueParent), continuation.Branch.Result["subflow_failure_strategy"])
}

func TestWorkflowSubflowContinueParentWritesCheckpointBeforeReturnTokenRuns(t *testing.T) {
	childWorkflow := &domain.WorkflowDefinition{
		ID:          "child-flow",
		Name:        "child-flow",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "child-start",
		Nodes:       []domain.WorkflowNode{{ID: "child-start", Type: "action", Name: "ChildStart"}},
	}
	parentToken := &WorkflowToken{
		ID:            "token-parent",
		NodeID:        "subflow",
		OriginTokenID: "token-parent",
		Branch:        newWorkflowBranchContext(nil, nil),
		Subflow: &workflowSubflowFrame{
			ParentTokenID:   "token-parent",
			ParentNodeID:    "subflow",
			ChildWorkflowID: "child-flow",
			ChildWorkflow:   childWorkflow,
			EntryNodeID:     "child-start",
			ReturnNodeID:    "after-subflow",
			ParentBranch:    cloneWorkflowBranchContext(newWorkflowBranchContext(nil, nil)),
			State:           workflowSubflowStateRunning,
			FailureStrategy: workflowSubflowFailureStrategyContinueParent,
		},
	}
	runtimeCtx := &WorkflowExecutionContext{
		Workflow: &domain.WorkflowDefinition{ID: "parent-flow", EntryNodeID: "subflow", Nodes: []domain.WorkflowNode{{ID: "subflow", Type: "action", Name: "Subflow"}}},
		Context:  &domain.WorkflowContext{},
	}
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{"child-start": &failOnceWorkflowNode{label: "child-start"}})

	err := kernel.executeInlineSubflow(context.Background(), runtimeCtx, parentToken, runtimeCtx.Workflow)

	require.NoError(t, err)
	require.NotEmpty(t, runtimeCtx.Checkpoints)
	last := runtimeCtx.Checkpoints[len(runtimeCtx.Checkpoints)-1]
	assert.Equal(t, "after-subflow", last.NodeID)
	assert.Equal(t, "failed", last.Metadata["token_branch_result"].(map[string]any)["subflow_status"])
}

func TestWorkflowSubflowContinueParentFailureSignalSurvivesCheckpointRestore(t *testing.T) {
	childWorkflow := &domain.WorkflowDefinition{
		ID:          "child-flow",
		Name:        "child-flow",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "child-start",
		Nodes:       []domain.WorkflowNode{{ID: "child-start", Type: "action", Name: "ChildStart"}},
	}
	parentToken := &WorkflowToken{
		ID:            "token-parent",
		NodeID:        "subflow",
		OriginTokenID: "token-parent",
		Branch:        newWorkflowBranchContext(nil, nil),
		Subflow: &workflowSubflowFrame{
			ParentTokenID:   "token-parent",
			ParentNodeID:    "subflow",
			ChildWorkflowID: "child-flow",
			ChildWorkflow:   childWorkflow,
			EntryNodeID:     "child-start",
			ReturnNodeID:    "after-subflow",
			ParentBranch:    cloneWorkflowBranchContext(newWorkflowBranchContext(nil, nil)),
			State:           workflowSubflowStateRunning,
			FailureStrategy: workflowSubflowFailureStrategyContinueParent,
		},
	}
	runtimeCtx := &WorkflowExecutionContext{
		Workflow: &domain.WorkflowDefinition{ID: "parent-flow", EntryNodeID: "subflow", Nodes: []domain.WorkflowNode{{ID: "subflow", Type: "action", Name: "Subflow"}}},
		Context:  &domain.WorkflowContext{},
	}
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{"child-start": &failOnceWorkflowNode{label: "child-start"}})

	err := kernel.executeInlineSubflow(context.Background(), runtimeCtx, parentToken, runtimeCtx.Workflow)

	require.NoError(t, err)
	require.NotEmpty(t, runtimeCtx.Checkpoints)
	restored := workflowTokenFromCheckpoint(&runtimeCtx.Checkpoints[len(runtimeCtx.Checkpoints)-1])
	require.NotNil(t, restored)
	assert.Equal(t, "after-subflow", restored.NodeID)
	require.NotNil(t, restored.Branch)
	assert.Equal(t, "failed", restored.Branch.Result["subflow_status"])
	assert.Equal(t, "child-flow", restored.Branch.Result["subflow_child_workflow_id"])
	assert.Nil(t, restored.Subflow)
}

func TestWorkflowSubflowContinueParentResumeStartsAtContinuationWithoutRerunningChild(t *testing.T) {
	var initialOrder []string
	childWorkflow := &domain.WorkflowDefinition{
		ID:          "child-flow",
		Name:        "child-flow",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "child-start",
		Nodes:       []domain.WorkflowNode{{ID: "child-start", Type: "action", Name: "ChildStart"}},
	}
	parentToken := &WorkflowToken{
		ID:            "token-parent",
		NodeID:        "subflow",
		OriginTokenID: "token-parent",
		Branch:        newWorkflowBranchContext(nil, nil),
		Subflow: &workflowSubflowFrame{
			ParentTokenID:   "token-parent",
			ParentNodeID:    "subflow",
			ChildWorkflowID: "child-flow",
			ChildWorkflow:   childWorkflow,
			EntryNodeID:     "child-start",
			ReturnNodeID:    "after-subflow",
			ParentBranch:    cloneWorkflowBranchContext(newWorkflowBranchContext(nil, nil)),
			State:           workflowSubflowStateRunning,
			FailureStrategy: workflowSubflowFailureStrategyContinueParent,
		},
	}
	transitionCtx := &WorkflowExecutionContext{
		Workflow: &domain.WorkflowDefinition{ID: "parent-flow", EntryNodeID: "subflow", Nodes: []domain.WorkflowNode{{ID: "subflow", Type: "action", Name: "Subflow"}, {ID: "after-subflow", Type: "action", Name: "AfterSubflow"}}, Edges: []domain.WorkflowEdge{{FromNodeID: "subflow", ToNodeID: "after-subflow", Priority: 1}}},
		Context:  &domain.WorkflowContext{},
	}
	transitionKernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"child-start":   &failOnceWorkflowNode{label: "child-start", order: &initialOrder},
		"after-subflow": &recordingWorkflowNode{label: "after-subflow", order: &initialOrder},
	})

	err := transitionKernel.executeInlineSubflow(context.Background(), transitionCtx, parentToken, transitionCtx.Workflow)

	require.NoError(t, err)
	assert.Equal(t, []string{"child-start"}, initialOrder)
	require.NotEmpty(t, transitionCtx.Checkpoints)

	var resumedOrder []string
	resumeKernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"subflow":       &inlineFailingChildWorkflowNode{strategy: workflowSubflowFailureStrategyContinueParent, childWorkflow: childWorkflow},
		"child-start":   &recordingWorkflowNode{label: "child-start", order: &resumedOrder},
		"after-subflow": &recordingWorkflowNode{label: "after-subflow", order: &resumedOrder},
	})
	resumeCtx := &WorkflowExecutionContext{
		Workflow:    transitionCtx.Workflow,
		Context:     &domain.WorkflowContext{},
		Checkpoints: transitionCtx.Checkpoints,
	}

	err = resumeKernel.Resume(context.Background(), resumeCtx)

	require.NoError(t, err)
	assert.Equal(t, []string{"after-subflow"}, resumedOrder)
}

func TestWorkflowSubflowResolveDefinitionRejectsMissingFailureStrategy(t *testing.T) {
	frame := &workflowSubflowFrame{
		ChildWorkflowID: "child-flow",
		EntryNodeID:     "child-start",
		ReturnNodeID:    "after-subflow",
	}

	_, err := workflowResolveSubflowDefinition(&WorkflowExecutionContext{}, frame)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "subflow failure strategy")
}

func TestWorkflowSubflowResolveDefinitionRejectsInvalidFailureStrategy(t *testing.T) {
	frame := &workflowSubflowFrame{
		ChildWorkflowID:  "child-flow",
		EntryNodeID:      "child-start",
		ReturnNodeID:     "after-subflow",
		FailureStrategy:  workflowSubflowFailureStrategy("skip_parent"),
		ChildWorkflow:    &domain.WorkflowDefinition{ID: "child-flow", EntryNodeID: "child-start", Nodes: []domain.WorkflowNode{{ID: "child-start", Type: "action", Name: "ChildStart"}}},
	}

	_, err := workflowResolveSubflowDefinition(&WorkflowExecutionContext{}, frame)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported subflow failure strategy")
}

func TestWorkflowSubflowInlineExecutionInheritsBranchLocalContextAndRoutesThroughChildWorkflow(t *testing.T) {
	var order []string
	childWorkflow := &domain.WorkflowDefinition{
		ID:          "child-flow",
		Name:        "child-flow",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "child-start",
		Nodes: []domain.WorkflowNode{
			{ID: "child-start", Type: "action", Name: "ChildStart"},
			{ID: "child-end", Type: "action", Name: "ChildEnd"},
		},
		Edges: []domain.WorkflowEdge{{FromNodeID: "child-start", ToNodeID: "child-end", Priority: 1}},
	}
	kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
		"start":         &recordingWorkflowNode{label: "start", order: &order},
		"subflow":       &inlineChildWorkflowNode{childWorkflow: childWorkflow},
		"after-subflow": &recordingWorkflowNode{label: "after-subflow", order: &order},
		"child-start":   &childInlineEntryWorkflowNode{order: &order},
		"child-end":     &recordingWorkflowNode{label: "child-end", order: &order},
	})

	parentWorkflow := &domain.WorkflowDefinition{
		ID:          "parent-flow",
		Name:        "parent-flow",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "start",
		Nodes: []domain.WorkflowNode{
			{ID: "start", Type: "action", Name: "Start"},
			{ID: "subflow", Type: "action", Name: "Subflow"},
			{ID: "after-subflow", Type: "action", Name: "AfterSubflow"},
		},
		Edges: []domain.WorkflowEdge{
			{FromNodeID: "start", ToNodeID: "subflow", Priority: 1},
			{FromNodeID: "subflow", ToNodeID: "after-subflow", Priority: 1},
		},
	}
	runtimeCtx := &WorkflowExecutionContext{
		Workflow: parentWorkflow,
		Context:  &domain.WorkflowContext{Payload: map[string]any{"title": "Parent Title"}},
	}

	err := kernel.executeFrom(context.Background(), runtimeCtx, parentWorkflow.EntryNodeID)

	require.NoError(t, err)
	assert.Equal(t, []string{"start", "child-start", "child-end", "after-subflow"}, order)
	require.Len(t, runtimeCtx.CompletedTokens, 5)
	child := runtimeCtx.CompletedTokens[1]
	assert.Equal(t, "child-start", child.NodeID)
	require.NotNil(t, child.Branch)
	assert.Equal(t, "Parent Title", child.Frame.Input["title"])
	require.NotNil(t, child.Subflow)
	assert.Equal(t, workflowSubflowStateRunning, child.Subflow.State)
	finalToken := runtimeCtx.CompletedTokens[len(runtimeCtx.CompletedTokens)-1]
	require.NotNil(t, finalToken.Branch)
	assert.Equal(t, "Child Title", finalToken.Branch.Variables["title"])
	assert.Nil(t, finalToken.Branch.Variables["headline"])
	assert.Nil(t, finalToken.Branch.Variables["ignored"])
}

func TestWorkflowSubflowFailureStrategyControlsParentOutcome(t *testing.T) {
	tests := []struct {
		name            string
		strategy        workflowSubflowFailureStrategy
		expectsError    bool
		expectsPaused   bool
		expectsContinue bool
	}{
		{name: "fail parent", strategy: workflowSubflowFailureStrategyFailParent, expectsError: true},
		{name: "pause parent", strategy: workflowSubflowFailureStrategyPauseParent, expectsPaused: true},
		{name: "continue parent", strategy: workflowSubflowFailureStrategyContinueParent, expectsContinue: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var order []string
			childWorkflow := &domain.WorkflowDefinition{
				ID:          "child-flow",
				Name:        "child-flow",
				Version:     "v1",
				Enabled:     true,
				EntryNodeID: "child-start",
				Nodes:       []domain.WorkflowNode{{ID: "child-start", Type: "action", Name: "ChildStart"}},
			}
			kernel := newWorkflowRuntimeKernel(map[string]WorkflowNode{
				"start":   &recordingWorkflowNode{label: "start", order: &order},
				"subflow": &inlineFailingChildWorkflowNode{strategy: tt.strategy, childWorkflow: childWorkflow},
				"after-subflow": &recordingWorkflowNode{label: "after-subflow", order: &order},
			})
			parentWorkflow := &domain.WorkflowDefinition{
				ID:          "parent-flow",
				Name:        "parent-flow",
				Version:     "v1",
				Enabled:     true,
				EntryNodeID: "start",
				Nodes: []domain.WorkflowNode{
					{ID: "start", Type: "action", Name: "Start"},
					{ID: "subflow", Type: "action", Name: "Subflow"},
					{ID: "after-subflow", Type: "action", Name: "AfterSubflow"},
				},
				Edges: []domain.WorkflowEdge{{FromNodeID: "start", ToNodeID: "subflow", Priority: 1}, {FromNodeID: "subflow", ToNodeID: "after-subflow", Priority: 1}},
			}
			runtimeCtx := &WorkflowExecutionContext{
				Workflow: parentWorkflow,
				Context:  &domain.WorkflowContext{Payload: map[string]any{"title": "Parent Title"}},
			}

			err := kernel.executeFrom(context.Background(), runtimeCtx, parentWorkflow.EntryNodeID)

			if tt.expectsError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "child workflow")
				return
			}
			require.NoError(t, err)
			if tt.expectsPaused {
				require.NotEmpty(t, runtimeCtx.Checkpoints)
				last := runtimeCtx.Checkpoints[len(runtimeCtx.Checkpoints)-1]
				assert.Equal(t, domain.WorkflowCheckpointStateActive, last.State)
				assert.Equal(t, "subflow", last.NodeID)
				assert.Equal(t, string(workflowSubflowFailureStrategyPauseParent), last.Metadata["subflow_failure_strategy"])
				require.NotNil(t, runtimeCtx.CurrentToken)
				require.NotNil(t, runtimeCtx.CurrentToken.PauseState)
				assert.Equal(t, WorkflowPauseScopeToken, runtimeCtx.CurrentToken.PauseState.Scope)
				return
			}
			assert.Equal(t, []string{"start", "after-subflow"}, order)
			assert.NotEmpty(t, runtimeCtx.CompletedTokens)
			finalToken := runtimeCtx.CompletedTokens[len(runtimeCtx.CompletedTokens)-1]
			require.NotNil(t, finalToken.Branch)
			assert.Equal(t, "failed", finalToken.Branch.Result["subflow_status"])
			assert.Equal(t, "child-flow", finalToken.Branch.Result["subflow_child_workflow_id"])
		})
	}
}
