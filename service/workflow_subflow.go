package service

import (
	"content-hub/domain"
	"fmt"
	"strings"
	"time"
)

type workflowSubflowState string

const (
	workflowSubflowStateRunning workflowSubflowState = "running"
	workflowSubflowStateFailed  workflowSubflowState = "failed"
	workflowSubflowStateDone    workflowSubflowState = "done"
)

type workflowSubflowFailureStrategy string

const (
	workflowSubflowFailureStrategyFailParent     workflowSubflowFailureStrategy = "fail_parent"
	workflowSubflowFailureStrategyPauseParent    workflowSubflowFailureStrategy = "pause_parent"
	workflowSubflowFailureStrategyContinueParent workflowSubflowFailureStrategy = "continue_parent"
)

type workflowSubflowFrame struct {
	ParentTokenID    string
	ParentNodeID     string
	ChildWorkflowID  string
	ChildWorkflow    *domain.WorkflowDefinition
	EntryNodeID      string
	ReturnNodeID     string
	ReturnMapping    map[string]string
	ParentBranch     *WorkflowBranchContext
	State            workflowSubflowState
	FailureStrategy  workflowSubflowFailureStrategy
}

func workflowSubflowMetadata(frame *workflowSubflowFrame) map[string]any {
	if frame == nil {
		return nil
	}
	metadata := map[string]any{
		"token_subflow_frame": workflowSubflowFrameSnapshot(frame),
		"subflow_parent_token_id":  strings.TrimSpace(frame.ParentTokenID),
		"subflow_parent_node_id":   strings.TrimSpace(frame.ParentNodeID),
		"subflow_child_workflow_id": strings.TrimSpace(frame.ChildWorkflowID),
		"subflow_entry_node_id":    strings.TrimSpace(frame.EntryNodeID),
		"subflow_return_node_id":   strings.TrimSpace(frame.ReturnNodeID),
		"subflow_state":            strings.TrimSpace(string(frame.State)),
		"subflow_failure_strategy": strings.TrimSpace(string(frame.FailureStrategy)),
	}
	if frame.ChildWorkflow != nil {
		metadata["subflow_child_workflow"] = workflowDefinitionMetadata(frame.ChildWorkflow)
	}
	if len(frame.ReturnMapping) > 0 {
		mapping := make(map[string]any, len(frame.ReturnMapping))
		for source, target := range frame.ReturnMapping {
			mapping[source] = target
		}
		metadata["subflow_return_mapping"] = mapping
	}
	if frame.ParentBranch != nil {
		metadata["subflow_parent_branch_vars"] = cloneWorkflowPayload(frame.ParentBranch.Variables)
		metadata["subflow_parent_branch_result"] = cloneWorkflowPayload(frame.ParentBranch.Result)
		metadata["subflow_parent_branch_artifacts"] = cloneWorkflowPayload(frame.ParentBranch.Artifacts)
	}
	return metadata
}

func workflowSubflowFrameSnapshot(frame *workflowSubflowFrame) map[string]any {
	if frame == nil {
		return nil
	}
	snapshot := map[string]any{
		"parent_token_id":  strings.TrimSpace(frame.ParentTokenID),
		"parent_node_id":   strings.TrimSpace(frame.ParentNodeID),
		"child_workflow_id": strings.TrimSpace(frame.ChildWorkflowID),
		"entry_node_id":    strings.TrimSpace(frame.EntryNodeID),
		"return_node_id":   strings.TrimSpace(frame.ReturnNodeID),
		"state":            strings.TrimSpace(string(frame.State)),
		"failure_strategy": strings.TrimSpace(string(frame.FailureStrategy)),
	}
	if frame.ChildWorkflow != nil {
		snapshot["child_workflow"] = workflowDefinitionMetadata(frame.ChildWorkflow)
	}
	if len(frame.ReturnMapping) > 0 {
		mapping := make(map[string]any, len(frame.ReturnMapping))
		for source, target := range frame.ReturnMapping {
			mapping[source] = target
		}
		snapshot["return_mapping"] = mapping
	}
	if frame.ParentBranch != nil {
		snapshot["parent_branch_vars"] = cloneWorkflowPayload(frame.ParentBranch.Variables)
		snapshot["parent_branch_result"] = cloneWorkflowPayload(frame.ParentBranch.Result)
		snapshot["parent_branch_artifacts"] = cloneWorkflowPayload(frame.ParentBranch.Artifacts)
	}
	return snapshot
}

func workflowSubflowFromMetadata(metadata map[string]any) *workflowSubflowFrame {
	if len(metadata) == 0 {
		return nil
	}
	if rawSnapshot, ok := metadata["token_subflow_frame"].(map[string]any); ok && len(rawSnapshot) > 0 {
		if frame := workflowSubflowFrameFromSnapshot(rawSnapshot); frame != nil {
			return frame
		}
	}
	childWorkflowID := strings.TrimSpace(domain.DraftString(metadata["subflow_child_workflow_id"]))
	entryNodeID := strings.TrimSpace(domain.DraftString(metadata["subflow_entry_node_id"]))
	returnNodeID := strings.TrimSpace(domain.DraftString(metadata["subflow_return_node_id"]))
	if childWorkflowID == "" && entryNodeID == "" && returnNodeID == "" {
		return nil
	}
	frame := &workflowSubflowFrame{
		ParentTokenID:   strings.TrimSpace(domain.DraftString(metadata["subflow_parent_token_id"])),
		ParentNodeID:    strings.TrimSpace(domain.DraftString(metadata["subflow_parent_node_id"])),
		ChildWorkflowID: childWorkflowID,
		ChildWorkflow:   workflowDefinitionFromCheckpoint(metadata["subflow_child_workflow"]),
		EntryNodeID:     entryNodeID,
		ReturnNodeID:    returnNodeID,
		State:           workflowSubflowState(strings.TrimSpace(domain.DraftString(metadata["subflow_state"]))),
		FailureStrategy: workflowSubflowFailureStrategy(strings.TrimSpace(domain.DraftString(metadata["subflow_failure_strategy"]))),
		ParentBranch: &WorkflowBranchContext{
			Variables: workflowCheckpointPayload(metadata["subflow_parent_branch_vars"]),
			Result:    workflowCheckpointPayload(metadata["subflow_parent_branch_result"]),
			Artifacts: workflowCheckpointPayload(metadata["subflow_parent_branch_artifacts"]),
		},
	}
	if frame.State == "" {
		frame.State = workflowSubflowStateRunning
	}
	if frame.ChildWorkflow == nil {
		frame.ChildWorkflow = workflowDefinitionFromCheckpoint(metadata["subflow_child_workflow"])
	}
	if frame.ChildWorkflow != nil && strings.TrimSpace(frame.ChildWorkflowID) == "" {
		frame.ChildWorkflowID = strings.TrimSpace(frame.ChildWorkflow.ID)
	}
	if rawMapping, ok := metadata["subflow_return_mapping"].(map[string]any); ok {
		frame.ReturnMapping = make(map[string]string, len(rawMapping))
		for source, target := range rawMapping {
			trimmedSource := strings.TrimSpace(source)
			trimmedTarget := strings.TrimSpace(domain.DraftString(target))
			if trimmedSource == "" || trimmedTarget == "" {
				continue
			}
			frame.ReturnMapping[trimmedSource] = trimmedTarget
		}
	}
	if frame.ParentBranch == nil {
		frame.ParentBranch = newWorkflowBranchContext(nil, nil)
	}
	return frame
}

func workflowSubflowFrameFromSnapshot(snapshot map[string]any) *workflowSubflowFrame {
	if len(snapshot) == 0 {
		return nil
	}
	childWorkflowID := strings.TrimSpace(domain.DraftString(snapshot["child_workflow_id"]))
	entryNodeID := strings.TrimSpace(domain.DraftString(snapshot["entry_node_id"]))
	returnNodeID := strings.TrimSpace(domain.DraftString(snapshot["return_node_id"]))
	if childWorkflowID == "" && entryNodeID == "" && returnNodeID == "" {
		return nil
	}
	frame := &workflowSubflowFrame{
		ParentTokenID:   strings.TrimSpace(domain.DraftString(snapshot["parent_token_id"])),
		ParentNodeID:    strings.TrimSpace(domain.DraftString(snapshot["parent_node_id"])),
		ChildWorkflowID: childWorkflowID,
		ChildWorkflow:   workflowDefinitionFromCheckpoint(snapshot["child_workflow"]),
		EntryNodeID:     entryNodeID,
		ReturnNodeID:    returnNodeID,
		State:           workflowSubflowState(strings.TrimSpace(domain.DraftString(snapshot["state"]))),
		FailureStrategy: workflowSubflowFailureStrategy(strings.TrimSpace(domain.DraftString(snapshot["failure_strategy"]))),
		ParentBranch: &WorkflowBranchContext{
			Variables: workflowCheckpointPayload(snapshot["parent_branch_vars"]),
			Result:    workflowCheckpointPayload(snapshot["parent_branch_result"]),
			Artifacts: workflowCheckpointPayload(snapshot["parent_branch_artifacts"]),
		},
	}
	if frame.State == "" {
		frame.State = workflowSubflowStateRunning
	}
	if frame.ChildWorkflow != nil && strings.TrimSpace(frame.ChildWorkflowID) == "" {
		frame.ChildWorkflowID = strings.TrimSpace(frame.ChildWorkflow.ID)
	}
	if rawMapping, ok := snapshot["return_mapping"].(map[string]any); ok {
		frame.ReturnMapping = make(map[string]string, len(rawMapping))
		for source, target := range rawMapping {
			trimmedSource := strings.TrimSpace(source)
			trimmedTarget := strings.TrimSpace(domain.DraftString(target))
			if trimmedSource == "" || trimmedTarget == "" {
				continue
			}
			frame.ReturnMapping[trimmedSource] = trimmedTarget
		}
	}
	if frame.ParentBranch == nil {
		frame.ParentBranch = newWorkflowBranchContext(nil, nil)
	}
	return frame
}

func cloneWorkflowDefinition(wf *domain.WorkflowDefinition) *domain.WorkflowDefinition {
	if wf == nil {
		return nil
	}
	clone := &domain.WorkflowDefinition{
		ID:          strings.TrimSpace(wf.ID),
		Name:        wf.Name,
		Description: wf.Description,
		Version:     wf.Version,
		Enabled:     wf.Enabled,
		EntryNodeID: strings.TrimSpace(wf.EntryNodeID),
		UpdatedBy:   wf.UpdatedBy,
		UpdatedAt:   wf.UpdatedAt,
	}
	if len(wf.Nodes) > 0 {
		clone.Nodes = append([]domain.WorkflowNode(nil), wf.Nodes...)
	}
	if len(wf.Edges) > 0 {
		clone.Edges = append([]domain.WorkflowEdge(nil), wf.Edges...)
	}
	return clone
}

func workflowDefinitionFromCheckpoint(raw any) *domain.WorkflowDefinition {
	if wf, ok := raw.(*domain.WorkflowDefinition); ok && wf != nil {
		return cloneWorkflowDefinition(wf)
	}
	rawMap, ok := raw.(map[string]any)
	if !ok || len(rawMap) == 0 {
		return nil
	}
	wf := &domain.WorkflowDefinition{
		ID:          strings.TrimSpace(domain.DraftString(rawMap["id"])),
		Name:        strings.TrimSpace(domain.DraftString(rawMap["name"])),
		Description: strings.TrimSpace(domain.DraftString(rawMap["description"])),
		Version:     strings.TrimSpace(domain.DraftString(rawMap["version"])),
		Enabled:     rawMap["enabled"] == true,
		EntryNodeID: strings.TrimSpace(domain.DraftString(rawMap["entry_node_id"])),
		UpdatedBy:   strings.TrimSpace(domain.DraftString(rawMap["updated_by"])),
	}
	if updatedAt, ok := rawMap["updated_at"].(string); ok && strings.TrimSpace(updatedAt) != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, updatedAt); err == nil {
			wf.UpdatedAt = parsed
		}
	}
	for _, nodeMap := range workflowDefinitionMapEntries(rawMap["nodes"]) {
		wf.Nodes = append(wf.Nodes, domain.WorkflowNode{
			ID:         strings.TrimSpace(domain.DraftString(nodeMap["id"])),
			Type:       strings.TrimSpace(domain.DraftString(nodeMap["type"])),
			Name:       strings.TrimSpace(domain.DraftString(nodeMap["name"])),
			ConfigJSON: strings.TrimSpace(domain.DraftString(nodeMap["config_json"])),
		})
	}
	for _, edgeMap := range workflowDefinitionMapEntries(rawMap["edges"]) {
		wf.Edges = append(wf.Edges, domain.WorkflowEdge{
			FromNodeID: strings.TrimSpace(domain.DraftString(edgeMap["from_node_id"])),
			ToNodeID:   strings.TrimSpace(domain.DraftString(edgeMap["to_node_id"])),
			Condition:  strings.TrimSpace(domain.DraftString(edgeMap["condition"])),
			Priority:   workflowIntValue(edgeMap["priority"]),
		})
	}
	return wf
}

func workflowDefinitionMapEntries(raw any) []map[string]any {
	if typed, ok := raw.([]map[string]any); ok {
		return typed
	}
	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		return nil
	}
	entries := make([]map[string]any, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok || len(entry) == 0 {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

func workflowDefinitionMetadata(wf *domain.WorkflowDefinition) map[string]any {
	if wf == nil {
		return nil
	}
	nodes := make([]map[string]any, 0, len(wf.Nodes))
	for _, node := range wf.Nodes {
		nodes = append(nodes, map[string]any{
			"id":          strings.TrimSpace(node.ID),
			"type":        strings.TrimSpace(node.Type),
			"name":        strings.TrimSpace(node.Name),
			"config_json":  strings.TrimSpace(node.ConfigJSON),
		})
	}
	edges := make([]map[string]any, 0, len(wf.Edges))
	for _, edge := range wf.Edges {
		edges = append(edges, map[string]any{
			"from_node_id": strings.TrimSpace(edge.FromNodeID),
			"to_node_id":   strings.TrimSpace(edge.ToNodeID),
			"condition":    strings.TrimSpace(edge.Condition),
			"priority":     edge.Priority,
		})
	}
	return map[string]any{
		"id":           strings.TrimSpace(wf.ID),
		"name":         strings.TrimSpace(wf.Name),
		"description":  strings.TrimSpace(wf.Description),
		"version":      strings.TrimSpace(wf.Version),
		"enabled":      wf.Enabled,
		"entry_node_id": strings.TrimSpace(wf.EntryNodeID),
		"updated_by":    strings.TrimSpace(wf.UpdatedBy),
		"updated_at":    wf.UpdatedAt.UTC().Format(time.RFC3339Nano),
		"nodes":         nodes,
		"edges":         edges,
	}
}

func workflowDefinitionLookup(runtimeCtx *WorkflowExecutionContext) map[string]*domain.WorkflowDefinition {
	if runtimeCtx == nil || len(runtimeCtx.Metadata) == 0 {
		return nil
	}
	lookup, _ := runtimeCtx.Metadata["workflow_definitions"].(map[string]*domain.WorkflowDefinition)
	return lookup
}

func workflowResolveSubflowDefinition(runtimeCtx *WorkflowExecutionContext, frame *workflowSubflowFrame) (*domain.WorkflowDefinition, error) {
	if frame == nil {
		return nil, fmt.Errorf("subflow frame is required")
	}
	workflowID := strings.TrimSpace(frame.ChildWorkflowID)
	if workflowID == "" {
		return nil, fmt.Errorf("subflow child workflow id is required")
	}
	if strings.TrimSpace(string(frame.FailureStrategy)) == "" {
		return nil, fmt.Errorf("subflow failure strategy is required")
	}
	if !isSupportedWorkflowSubflowFailureStrategy(frame.FailureStrategy) {
		return nil, fmt.Errorf("unsupported subflow failure strategy: %s", strings.TrimSpace(string(frame.FailureStrategy)))
	}
	wf := frame.ChildWorkflow
	if wf == nil {
		lookup := workflowDefinitionLookup(runtimeCtx)
		if lookup != nil {
			wf = lookup[workflowID]
		}
	}
	if wf == nil {
		return nil, fmt.Errorf("child workflow %s not found", workflowID)
	}
	return wf, nil
}

func cloneWorkflowSubflowFrame(frame *workflowSubflowFrame) *workflowSubflowFrame {
	if frame == nil {
		return nil
	}
	clone := &workflowSubflowFrame{
		ParentTokenID:   strings.TrimSpace(frame.ParentTokenID),
		ParentNodeID:    strings.TrimSpace(frame.ParentNodeID),
		ChildWorkflowID: strings.TrimSpace(frame.ChildWorkflowID),
		ChildWorkflow:   cloneWorkflowDefinition(frame.ChildWorkflow),
		EntryNodeID:     strings.TrimSpace(frame.EntryNodeID),
		ReturnNodeID:    strings.TrimSpace(frame.ReturnNodeID),
		ParentBranch:    cloneWorkflowBranchContext(frame.ParentBranch),
		State:           frame.State,
		FailureStrategy: frame.FailureStrategy,
	}
	if len(frame.ReturnMapping) > 0 {
		clone.ReturnMapping = make(map[string]string, len(frame.ReturnMapping))
		for source, target := range frame.ReturnMapping {
			clone.ReturnMapping[source] = target
		}
	}
	return clone
}

func isSupportedWorkflowSubflowFailureStrategy(strategy workflowSubflowFailureStrategy) bool {
	switch strategy {
	case workflowSubflowFailureStrategyFailParent, workflowSubflowFailureStrategyPauseParent, workflowSubflowFailureStrategyContinueParent:
		return true
	default:
		return false
	}
}

func applyWorkflowSubflowReturnMapping(parent *WorkflowToken, child *WorkflowToken, mapping map[string]string) {
	if parent == nil || parent.Branch == nil || child == nil || child.Branch == nil || len(mapping) == 0 {
		return
	}
	for source, target := range mapping {
		source = strings.TrimSpace(source)
		target = strings.TrimSpace(target)
		if source == "" || target == "" {
			continue
		}
		if value, ok := child.Branch.Variables[source]; ok {
			parent.Branch.Variables[target] = cloneWorkflowValue(value)
		}
		if value, ok := child.Branch.Result[source]; ok {
			parent.Branch.Result[target] = cloneWorkflowValue(value)
		}
	}
}
