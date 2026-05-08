package service

import (
	"content-hub/domain"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const workflowPauseStateResultKey = "workflow_pause"
const workflowHumanResumeInputMetadataKey = "human_resume_input"
const workflowHumanResumeSubmittedKey = "submitted"

type workflowHumanNodeConfig struct {
	ActionSchema map[string]any `json:"action_schema"`
	FormSchema   map[string]any `json:"form_schema"`
}

type humanWorkflowNode struct {
	actionSchema map[string]any
	formSchema   map[string]any
}

func (n *humanWorkflowNode) Name() string {
	return "human"
}

func (n *humanWorkflowNode) Execute(_ context.Context, _ *domain.WorkflowContext) error {
	return nil
}

func (n *humanWorkflowNode) ExecuteWorkflow(_ context.Context, runtimeCtx *WorkflowExecutionContext, node domain.WorkflowNode) (WorkflowNodeResult, error) {
	if humanResumeResultPresent(runtimeCtx) {
		return WorkflowNodeResult{Output: cloneWorkflowPayload(runtimeCtx.CurrentToken.Branch.Result)}, nil
	}
	config, err := normalizeWorkflowHumanNodeConfig(node, n)
	if err != nil {
		return WorkflowNodeResult{}, err
	}
	pausePayload := workflowHumanPausePayload(node.ID, runtimeCtx.CurrentToken, config)
	return WorkflowNodeResult{
		Paused: true,
		PauseState: &WorkflowPauseState{
			Source:             WorkflowPauseSourceHumanNode,
			Scope:              WorkflowPauseScopeToken,
			Reason:             "awaiting human input",
			Payload:            pausePayload,
			AllowedResumeModes: []WorkflowResumeMode{WorkflowResumeModeContinueToken},
		},
		Output: map[string]any{
			workflowPauseStateResultKey: cloneWorkflowPayload(pausePayload),
		},
	}, nil
}

func normalizeWorkflowHumanNodeConfig(node domain.WorkflowNode, runtimeNode *humanWorkflowNode) (workflowHumanNodeConfig, error) {
	config, err := parseWorkflowHumanNodeConfig(node.ConfigJSON)
	if err != nil {
		return workflowHumanNodeConfig{}, fmt.Errorf("parse human workflow node config: %w", err)
	}
	if len(config.ActionSchema) == 0 && runtimeNode != nil {
		config.ActionSchema = cloneWorkflowPayload(runtimeNode.actionSchema)
	}
	if len(config.FormSchema) == 0 && runtimeNode != nil {
		config.FormSchema = cloneWorkflowPayload(runtimeNode.formSchema)
	}
	if config.ActionSchema == nil {
		config.ActionSchema = map[string]any{}
	}
	if config.FormSchema == nil {
		config.FormSchema = map[string]any{}
	}
	return config, nil
}

func parseWorkflowHumanNodeConfig(configJSON string) (workflowHumanNodeConfig, error) {
	configJSON = strings.TrimSpace(configJSON)
	if configJSON == "" {
		return workflowHumanNodeConfig{}, nil
	}
	var cfg workflowHumanNodeConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return workflowHumanNodeConfig{}, fmt.Errorf("decode workflow human node config json: %w", err)
	}
	if cfg.ActionSchema == nil {
		cfg.ActionSchema = map[string]any{}
	}
	if cfg.FormSchema == nil {
		cfg.FormSchema = map[string]any{}
	}
	return cfg, nil
}

func workflowHumanPausePayload(nodeID string, token *WorkflowToken, config workflowHumanNodeConfig) map[string]any {
	allowedResumeModes := []any{string(WorkflowResumeModeContinueToken)}
	payload := map[string]any{
		"node_id":              strings.TrimSpace(nodeID),
		"token_id":             workflowHumanTokenID(token),
		"action_schema":        cloneWorkflowPayload(config.ActionSchema),
		"form_schema":          cloneWorkflowPayload(config.FormSchema),
		"allowed_resume_modes": allowedResumeModes,
	}
	return payload
}

func workflowHumanResumeInputMetadata(input map[string]any, submitted bool) map[string]any {
	metadata := map[string]any{
		workflowHumanResumeSubmittedKey: submitted,
		"action":                        cloneWorkflowPayload(workflowCheckpointPayload(input["action"])),
		"form":                          cloneWorkflowPayload(workflowCheckpointPayload(input["form"])),
	}
	return map[string]any{workflowHumanResumeInputMetadataKey: metadata}
}

func workflowHumanTokenID(token *WorkflowToken) string {
	if token == nil {
		return ""
	}
	return strings.TrimSpace(token.ID)
}

func applyWorkflowHumanResumeInput(runtimeCtx *WorkflowExecutionContext, input map[string]any) {
	if runtimeCtx == nil || runtimeCtx.CurrentToken == nil {
		return
	}
	ensureWorkflowTokenBranch(runtimeCtx, runtimeCtx.CurrentToken)
	if runtimeCtx.CurrentToken.Branch.Result == nil {
		runtimeCtx.CurrentToken.Branch.Result = map[string]any{}
	}
	human := map[string]any{
		workflowHumanResumeSubmittedKey: true,
		"action":                        cloneWorkflowPayload(workflowCheckpointPayload(input["action"])),
		"form":                          cloneWorkflowPayload(workflowCheckpointPayload(input["form"])),
	}
	runtimeCtx.CurrentToken.Branch.Result["human"] = human
	rebindWorkflowNodeResult(runtimeCtx, runtimeCtx.CurrentToken)
}

func humanResumeResultPresent(runtimeCtx *WorkflowExecutionContext) bool {
	if runtimeCtx == nil || runtimeCtx.CurrentToken == nil || runtimeCtx.CurrentToken.Branch == nil {
		return false
	}
	human, ok := runtimeCtx.CurrentToken.Branch.Result["human"].(map[string]any)
	if !ok {
		return false
	}
	submitted, _ := human[workflowHumanResumeSubmittedKey].(bool)
	return submitted
}

func workflowHumanResumeInputFromCheckpoint(checkpoint *domain.WorkflowCheckpoint) map[string]any {
	if checkpoint == nil || len(checkpoint.Metadata) == 0 {
		return nil
	}
	raw, ok := checkpoint.Metadata[workflowHumanResumeInputMetadataKey].(map[string]any)
	if !ok {
		return nil
	}
	submitted, _ := raw[workflowHumanResumeSubmittedKey].(bool)
	if !submitted {
		return nil
	}
	return cloneWorkflowPayload(raw)
}
