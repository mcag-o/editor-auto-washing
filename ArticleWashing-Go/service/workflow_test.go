package service

import (
	"content-hub/domain"
	"content-hub/infra/memory"
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
	provider := memory.NewProvider()
	ingestionSvc := NewIngestionPipelineService(provider.IngestionRepo(), provider.WorkspaceRepo(), provider, workspaceinfra.NewLoader())
	jobSvc := NewJobService(provider.JobRepo(), provider.JobEventRepo(), NewWorkflowEngine())
	automationSvc := NewAutomationService(NewWorkspaceConfigService(workspaceinfra.NewLoader(), workspaceinfra.NewValidator()), ingestionSvc, jobSvc)

	engine := BuildDefaultWorkflowEngine(root, automationSvc)

	assert.Contains(t, engine.RegisteredNames(), "automation_dispatch")
	assert.Contains(t, engine.RegisteredNames(), "automation_snapshot")
	require.NoError(t, engine.Execute(t.Context(), domain.DefaultWorkflowDefinition(), &domain.WorkflowContext{Payload: map[string]any{"automation_command": "run-once"}}))
}
