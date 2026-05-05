package service

import (
	"content-hub/domain"
	workspaceinfra "content-hub/infra/workspace"
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
