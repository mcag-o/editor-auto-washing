package service

import (
	"content-hub/domain"
	workspaceinfra "content-hub/infra/workspace"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDefaultWorkflowEngineRegistersConcreteAutomationNodes(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceConfigFileName), []byte("name: workflow\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))
	intake := &stubAutomationFolderIntake{runOnceResult: automationFolderRunSummary{ProcessedPending: 1, CompletedDocuments: 1}}
	automationSvc := newAutomationServiceWithFolderIntakeForTest(root, intake)

	engine := BuildDefaultWorkflowEngine(root, automationSvc)

	assert.Contains(t, engine.RegisteredNames(), "automation_dispatch")
	assert.Contains(t, engine.RegisteredNames(), "automation_snapshot")
	require.NoError(t, engine.Execute(t.Context(), domain.DefaultWorkflowDefinition(), &domain.WorkflowContext{Payload: map[string]any{"automation_command": "run-once"}}))
	assert.Equal(t, 1, intake.runOnceCalls)
}

func TestBuildDefaultWorkflowEngineDispatchesRetryFailedThroughFolderIntakeAutomation(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceConfigFileName), []byte("name: workflow\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))
	intake := &stubAutomationFolderIntake{retryResult: automationFolderRunSummary{ProcessedFailed: 1, CompletedDocuments: 1}}
	automationSvc := newAutomationServiceWithFolderIntakeForTest(root, intake)

	engine := BuildDefaultWorkflowEngine(root, automationSvc)
	wc := &domain.WorkflowContext{Payload: map[string]any{"automation_command": "retry-failed"}}

	require.NoError(t, engine.Execute(t.Context(), domain.DefaultWorkflowDefinition(), wc))
	assert.Equal(t, 1, intake.retryFailedCalls)
	result, ok := wc.Payload["automation_result"].(*domain.AutomationRunResult)
	require.True(t, ok)
	assert.Equal(t, "retry-failed", result.Mode)
}

func TestAutomationDispatchNodeRequiresExplicitCommand(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceConfigFileName), []byte("name: workflow\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))
	intake := &stubAutomationFolderIntake{runOnceResult: automationFolderRunSummary{ProcessedPending: 1, CompletedDocuments: 1}}
	automationSvc := newAutomationServiceWithFolderIntakeForTest(root, intake)

	engine := BuildDefaultWorkflowEngine(root, automationSvc)
	wc := &domain.WorkflowContext{}

	err := engine.Execute(t.Context(), domain.DefaultWorkflowDefinition(), wc)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "automation workflow command is required")
	assert.Equal(t, 0, intake.runOnceCalls)
}

func TestWorkflowEngineExecutesRuntimeNodeByGraphNodeID(t *testing.T) {
	engine := NewWorkflowEngine()
	recorder := &recordingWorkflowNode{label: "runtime"}
	engine.Register("node-id", recorder)

	wf := &domain.WorkflowDefinition{
		Name:        "test",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "node-id",
		Nodes: []domain.WorkflowNode{{
			ID:   "node-id",
			Type: "action",
			Name: "display-name",
		}},
	}

	require.NoError(t, engine.Execute(t.Context(), wf, &domain.WorkflowContext{}))
	assert.Equal(t, []string{"runtime"}, recorder.calls)
}

func TestWorkflowEngineExecutesAccordingToEdgesNotNodeSliceOrder(t *testing.T) {
	engine := NewWorkflowEngine()
	var order []string
	first := &recordingWorkflowNode{label: "first", order: &order}
	second := &recordingWorkflowNode{label: "second", order: &order}
	third := &recordingWorkflowNode{label: "third", order: &order}
	engine.Register("first", first)
	engine.Register("second", second)
	engine.Register("third", third)

	wf := &domain.WorkflowDefinition{
		Name:        "test",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "first",
		Nodes: []domain.WorkflowNode{
			{ID: "third", Type: "action", Name: "Third"},
			{ID: "first", Type: "action", Name: "First"},
			{ID: "second", Type: "action", Name: "Second"},
		},
		Edges: []domain.WorkflowEdge{
			{FromNodeID: "first", ToNodeID: "second", Priority: 1},
			{FromNodeID: "second", ToNodeID: "third", Priority: 1},
		},
	}

	require.NoError(t, engine.Execute(t.Context(), wf, &domain.WorkflowContext{}))
	assert.Equal(t, []string{"first"}, first.calls)
	assert.Equal(t, []string{"second"}, second.calls)
	assert.Equal(t, []string{"third"}, third.calls)
	assert.Equal(t, []string{"first", "second", "third"}, order)
}

func TestWorkflowEngineRequiresARouteWhenAllOutgoingEdgesAreConditional(t *testing.T) {
	engine := NewWorkflowEngine()
	engine.Register("start", &recordingWorkflowNode{label: "start"})

	wf := &domain.WorkflowDefinition{
		Name:        "route-required",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "start",
		Nodes: []domain.WorkflowNode{
			{ID: "start", Type: "action", Name: "Start"},
			{ID: "next", Type: "action", Name: "Next"},
		},
		Edges: []domain.WorkflowEdge{{FromNodeID: "start", ToNodeID: "next", Condition: "payload.route == approved", Priority: 1}},
	}

	err := engine.Execute(t.Context(), wf, &domain.WorkflowContext{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no matching route")
}

func TestLinearExecutionPathRejectsContextDependentRouteSelection(t *testing.T) {
	wf := &domain.WorkflowDefinition{
		Name:        "linear-compatible-routing",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "start",
		Nodes: []domain.WorkflowNode{
			{ID: "start", Type: "action", Name: "Start"},
			{ID: "approved", Type: "action", Name: "Approved"},
			{ID: "fallback", Type: "action", Name: "Fallback"},
		},
		Edges: []domain.WorkflowEdge{
			{FromNodeID: "start", ToNodeID: "approved", Condition: "payload.route == approved", Priority: 1},
			{FromNodeID: "start", ToNodeID: "fallback", Condition: "always", Priority: 99},
		},
	}

	path, err := linearExecutionPath(wf)

	require.Error(t, err)
	assert.Nil(t, path)
	assert.Contains(t, err.Error(), "context-dependent route selection")
}

func TestWorkflowEngineRejectsUnsupportedBranching(t *testing.T) {
	engine := NewWorkflowEngine()
	engine.Register("start", &recordingWorkflowNode{label: "start"})
	engine.Register("left", &recordingWorkflowNode{label: "left"})
	engine.Register("right", &recordingWorkflowNode{label: "right"})

	wf := &domain.WorkflowDefinition{
		Name:        "branching",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "start",
		Nodes: []domain.WorkflowNode{
			{ID: "start", Type: "action", Name: "Start"},
			{ID: "left", Type: "action", Name: "Left"},
			{ID: "right", Type: "action", Name: "Right"},
		},
		Edges: []domain.WorkflowEdge{
			{FromNodeID: "start", ToNodeID: "left", Priority: 1},
			{FromNodeID: "start", ToNodeID: "right", Priority: 2},
		},
	}

	err := engine.Execute(t.Context(), wf, &domain.WorkflowContext{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported branching")
}

func TestWorkflowEngineRejectsUnsupportedCycle(t *testing.T) {
	engine := NewWorkflowEngine()
	engine.Register("start", &recordingWorkflowNode{label: "start"})
	engine.Register("next", &recordingWorkflowNode{label: "next"})

	wf := &domain.WorkflowDefinition{
		Name:        "cycle",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "start",
		Nodes: []domain.WorkflowNode{
			{ID: "start", Type: "action", Name: "Start"},
			{ID: "next", Type: "action", Name: "Next"},
		},
		Edges: []domain.WorkflowEdge{
			{FromNodeID: "start", ToNodeID: "next", Priority: 1},
			{FromNodeID: "next", ToNodeID: "start", Priority: 1},
		},
	}

	err := engine.Execute(t.Context(), wf, &domain.WorkflowContext{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported cycle")
}

type recordingWorkflowNode struct {
	label string
	calls []string
	order *[]string
}

func (n *recordingWorkflowNode) Name() string {
	return n.label
}

func (n *recordingWorkflowNode) Execute(_ context.Context, _ *domain.WorkflowContext) error {
	n.calls = append(n.calls, n.label)
	if n.order != nil {
		*n.order = append(*n.order, n.label)
	}
	return nil
}
