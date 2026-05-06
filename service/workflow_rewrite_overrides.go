package service

import (
	"content-hub/domain"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	workflowStageOverridesMetadataKey = "workflow_stage_overrides"
	workflowPromptRefMetadataKey      = "workflow_prompt_ref"
)

type workflowStageOverride struct {
	NodeID    string         `json:"node_id"`
	PromptRef string         `json:"prompt_ref"`
	Vars      map[string]any `json:"vars"`
}

type workflowNodeConfig struct {
	StageName string         `json:"stage_name"`
	PromptRef string         `json:"prompt_ref"`
	Vars      map[string]any `json:"vars"`
}

func deriveWorkflowStageOverrides(workflow *domain.WorkflowDefinition) (map[string]workflowStageOverride, error) {
	path, err := linearExecutionPath(workflow)
	if err != nil {
		return nil, err
	}
	if len(path) == 0 {
		return nil, nil
	}
	nodesByID := make(map[string]domain.WorkflowNode, len(workflow.Nodes))
	for _, node := range workflow.Nodes {
		nodesByID[node.ID] = node
	}
	overrides := map[string]workflowStageOverride{}
	for _, nodeID := range path {
		node := nodesByID[nodeID]
		cfg, err := parseWorkflowNodeConfig(node.ConfigJSON)
		if err != nil {
			return nil, fmt.Errorf("node %s config: %w", node.ID, err)
		}
		stageName := strings.TrimSpace(cfg.StageName)
		if stageName == "" {
			stageName = strings.TrimSpace(node.Name)
		}
		if stageName == "" {
			stageName = strings.TrimSpace(node.ID)
		}
		if stageName == "" {
			continue
		}
		overrides[stageName] = workflowStageOverride{
			NodeID:    node.ID,
			PromptRef: strings.TrimSpace(cfg.PromptRef),
			Vars:      cfg.Vars,
		}
	}
	if len(overrides) == 0 {
		return nil, nil
	}
	return overrides, nil
}

func parseWorkflowNodeConfig(configJSON string) (workflowNodeConfig, error) {
	configJSON = strings.TrimSpace(configJSON)
	if configJSON == "" {
		return workflowNodeConfig{}, nil
	}
	var cfg workflowNodeConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return workflowNodeConfig{}, fmt.Errorf("decode workflow node config json: %w", err)
	}
	if cfg.Vars == nil {
		cfg.Vars = map[string]any{}
	}
	return cfg, nil
}

func workflowStageOverridesFromMetadata(metadata map[string]any) map[string]workflowStageOverride {
	if len(metadata) == 0 {
		return nil
	}
	raw, ok := metadata[workflowStageOverridesMetadataKey]
	if !ok || raw == nil {
		return nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var overrides map[string]workflowStageOverride
	if err := json.Unmarshal(encoded, &overrides); err != nil {
		return nil
	}
	if len(overrides) == 0 {
		return nil
	}
	return overrides
}

func applyWorkflowOverride(stage domain.RewriteStageDefinition, runMetadata map[string]any, inputVars map[string]any) domain.RewriteStageDefinition {
	overrides := workflowStageOverridesFromMetadata(runMetadata)
	if len(overrides) == 0 {
		return stage
	}
	override, ok := overrides[strings.TrimSpace(stage.Name)]
	if !ok {
		return stage
	}
	if promptRef := strings.TrimSpace(override.PromptRef); promptRef != "" {
		stage.PromptRef = promptRef
	}
	if len(override.Vars) > 0 {
		for key, value := range override.Vars {
			inputVars[key] = value
		}
	}
	if strings.TrimSpace(override.NodeID) != "" {
		inputVars["workflow_node_"+stage.Name] = override.NodeID
	}
	return stage
}
