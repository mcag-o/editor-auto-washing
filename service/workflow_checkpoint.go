package service

import (
	"content-hub/domain"
	"fmt"
	"strings"
	"time"
)

const workflowLatestRouteMetadataKey = "workflow_route_latest"
const workflowActiveTokenSetMetadataKey = "active_token_set"

type workflowCheckpointSnapshot struct {
	RouteSummary *WorkflowRouteOutcomeSummary
	Token        *WorkflowToken
}

func latestResumableCheckpoint(checkpoints []domain.WorkflowCheckpoint) (*domain.WorkflowCheckpoint, error) {
	for i := len(checkpoints) - 1; i >= 0; i-- {
		checkpoint := checkpoints[i]
		if checkpoint.Resumable && checkpoint.State == domain.WorkflowCheckpointStateActive {
			return &checkpoint, nil
		}
	}
	return nil, fmt.Errorf("no resumable checkpoint available")
}

func appendCheckpoint(ctx *WorkflowExecutionContext, workflowRunID, nodeID string) {
	appendCheckpointWithSnapshot(ctx, workflowRunID, nodeID, workflowCheckpointSnapshot{})
}

func appendCheckpointWithSnapshot(ctx *WorkflowExecutionContext, workflowRunID, nodeID string, snapshot workflowCheckpointSnapshot) {
	if ctx == nil {
		return
	}
	checkpoint := domain.WorkflowCheckpoint{
		WorkflowRunID: workflowRunID,
		NodeID:        nodeID,
		State:         domain.WorkflowCheckpointStateActive,
		Resumable:     true,
		CreatedAt:     time.Now().UTC(),
	}
	if summary := snapshot.RouteSummary; summary != nil {
		checkpoint.Metadata = workflowRouteSummaryMetadata(*summary)
	} else if summary := latestRouteOutcomeSummary(ctx); summary != nil {
		checkpoint.Metadata = workflowRouteSummaryMetadata(*summary)
	}
	if snapshot.Token != nil {
		checkpoint.Metadata = mergeCheckpointMetadata(checkpoint.Metadata, workflowTokenMetadata(*snapshot.Token))
	}
	if activeSet := workflowActiveTokenSetMetadata(ctx); activeSet != nil {
		checkpoint.Metadata = mergeCheckpointMetadata(checkpoint.Metadata, activeSet)
	}
	ctx.Checkpoints = append(ctx.Checkpoints, checkpoint)
}

func consumeActiveCheckpoints(ctx *WorkflowExecutionContext, consumedAt time.Time) {
	if ctx == nil {
		return
	}
	for i := range ctx.Checkpoints {
		if ctx.Checkpoints[i].State != domain.WorkflowCheckpointStateActive || !ctx.Checkpoints[i].Resumable {
			continue
		}
		ctx.Checkpoints[i].State = domain.WorkflowCheckpointStateTerminal
		ctx.Checkpoints[i].Resumable = false
		consumedAtCopy := consumedAt
		ctx.Checkpoints[i].ConsumedAt = &consumedAtCopy
	}
}

func recordLatestRouteOutcome(ctx *WorkflowExecutionContext, summary WorkflowRouteOutcomeSummary) {
	if ctx == nil {
		return
	}
	copySummary := summary
	copySummary.EvaluationTrace = append([]string(nil), summary.EvaluationTrace...)
	ctx.LatestRoute = &copySummary
}

func latestRouteOutcomeSummary(ctx *WorkflowExecutionContext) *WorkflowRouteOutcomeSummary {
	if ctx == nil {
		return nil
	}
	if ctx.LatestRoute != nil {
		copySummary := *ctx.LatestRoute
		copySummary.EvaluationTrace = append([]string(nil), ctx.LatestRoute.EvaluationTrace...)
		return &copySummary
	}
	if len(ctx.Metadata) == 0 {
		return nil
	}
	raw, ok := ctx.Metadata[workflowLatestRouteMetadataKey].(map[string]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	summary := WorkflowRouteOutcomeSummary{
		NodeID:         strings.TrimSpace(domain.DraftString(raw["node_id"])),
		SelectedEdgeID: strings.TrimSpace(domain.DraftString(raw["selected_edge_id"])),
		SelectedNodeID: strings.TrimSpace(domain.DraftString(raw["selected_node_id"])),
		Outcome:        WorkflowRouteOutcome(strings.TrimSpace(domain.DraftString(raw["outcome"]))),
	}
	if trace, ok := raw["evaluation_trace"].([]string); ok {
		summary.EvaluationTrace = append([]string(nil), trace...)
		return &summary
	}
	if trace, ok := raw["evaluation_trace"].([]any); ok {
		summary.EvaluationTrace = make([]string, 0, len(trace))
		for _, item := range trace {
			value := strings.TrimSpace(domain.DraftString(item))
			if value != "" {
				summary.EvaluationTrace = append(summary.EvaluationTrace, value)
			}
		}
	}
	return &summary
}

func workflowRouteSummaryMetadata(summary WorkflowRouteOutcomeSummary) map[string]any {
	trace := make([]string, 0, len(summary.EvaluationTrace))
	for _, entry := range summary.EvaluationTrace {
		if trimmed := strings.TrimSpace(entry); trimmed != "" {
			trace = append(trace, trimmed)
		}
	}
	return map[string]any{
		"node_id":                strings.TrimSpace(summary.NodeID),
		"selected_edge_id":       strings.TrimSpace(summary.SelectedEdgeID),
		"selected_node_id":       strings.TrimSpace(summary.SelectedNodeID),
		"outcome":                strings.TrimSpace(string(summary.Outcome)),
		"evaluation_trace":       trace,
		"route_node_id":          strings.TrimSpace(summary.NodeID),
		"route_selected_edge_id": strings.TrimSpace(summary.SelectedEdgeID),
		"route_selected_node_id": strings.TrimSpace(summary.SelectedNodeID),
		"route_outcome":          strings.TrimSpace(string(summary.Outcome)),
		"route_evaluation_trace": trace,
	}
}

func workflowTokenMetadata(token WorkflowToken) map[string]any {
	metadata := map[string]any{
		"node_id":                             strings.TrimSpace(token.NodeID),
		"token_id":                            strings.TrimSpace(token.ID),
		"token_parent_id":                     strings.TrimSpace(token.ParentTokenID),
		"token_origin_id":                     strings.TrimSpace(token.OriginTokenID),
		"token_origin_route_node_id":          strings.TrimSpace(token.OriginRoute.SourceNodeID),
		"token_origin_route_edge_id":          strings.TrimSpace(token.OriginRoute.SelectedEdgeID),
		"token_origin_route_selected_node_id": strings.TrimSpace(token.OriginRoute.SelectedNodeID),
	}
	if token.Branch != nil {
		metadata["token_branch_vars"] = cloneWorkflowPayload(token.Branch.Variables)
		metadata["token_branch_result"] = cloneWorkflowPayload(token.Branch.Result)
		metadata["token_branch_artifacts"] = cloneWorkflowPayload(token.Branch.Artifacts)
	}
	return metadata
}

func workflowActiveTokenSetMetadata(ctx *WorkflowExecutionContext) map[string]any {
	if ctx == nil || len(ctx.ActiveTokens) == 0 {
		return nil
	}
	activeSet := make([]map[string]any, 0, len(ctx.ActiveTokens))
	for _, token := range ctx.ActiveTokens {
		if token == nil {
			continue
		}
		activeSet = append(activeSet, workflowTokenMetadata(*token))
	}
	if len(activeSet) == 0 {
		return nil
	}
	return map[string]any{workflowActiveTokenSetMetadataKey: activeSet}
}

func workflowTokenFromCheckpoint(checkpoint *domain.WorkflowCheckpoint) *WorkflowToken {
	return workflowTokenFromMetadata(checkpoint.NodeID, checkpoint.Metadata)
}

func workflowTokenFromMetadata(defaultNodeID string, metadata map[string]any) *WorkflowToken {
	if len(metadata) == 0 {
		return nil
	}
	tokenID := strings.TrimSpace(domain.DraftString(metadata["token_id"]))
	if tokenID == "" {
		return nil
	}
	originID := strings.TrimSpace(domain.DraftString(metadata["token_origin_id"]))
	if originID == "" {
		originID = tokenID
	}
	nodeID := strings.TrimSpace(domain.DraftString(metadata["node_id"]))
	if nodeID == "" {
		nodeID = strings.TrimSpace(defaultNodeID)
	}
	return &WorkflowToken{
		ID:            tokenID,
		NodeID:        nodeID,
		ParentTokenID: strings.TrimSpace(domain.DraftString(metadata["token_parent_id"])),
		OriginTokenID: originID,
		OriginRoute: WorkflowTokenRouteLineage{
			SourceNodeID:   strings.TrimSpace(domain.DraftString(metadata["token_origin_route_node_id"])),
			SelectedEdgeID: strings.TrimSpace(domain.DraftString(metadata["token_origin_route_edge_id"])),
			SelectedNodeID: strings.TrimSpace(domain.DraftString(metadata["token_origin_route_selected_node_id"])),
		},
		Branch: &WorkflowBranchContext{
			Variables: workflowCheckpointPayload(metadata["token_branch_vars"]),
			Result:    workflowCheckpointPayload(metadata["token_branch_result"]),
			Artifacts: workflowCheckpointPayload(metadata["token_branch_artifacts"]),
		},
	}
}

func workflowActiveTokensFromCheckpoint(checkpoint *domain.WorkflowCheckpoint) []*WorkflowToken {
	if checkpoint == nil || len(checkpoint.Metadata) == 0 {
		return nil
	}
	rawSet := workflowActiveTokenSetEntries(checkpoint.Metadata[workflowActiveTokenSetMetadataKey])
	if len(rawSet) == 0 {
		if token := workflowTokenFromCheckpoint(checkpoint); token != nil {
			return []*WorkflowToken{token}
		}
		return nil
	}
	tokens := make([]*WorkflowToken, 0, len(rawSet))
	for _, raw := range rawSet {
		token := workflowTokenFromMetadata("", raw)
		if token == nil || strings.TrimSpace(token.NodeID) == "" {
			continue
		}
		tokens = append(tokens, token)
	}
	if len(tokens) == 0 {
		if token := workflowTokenFromCheckpoint(checkpoint); token != nil {
			return []*WorkflowToken{token}
		}
	}
	return tokens
}

func workflowActiveTokenSetEntries(raw any) []map[string]any {
	if raw == nil {
		return nil
	}
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
	if len(entries) == 0 {
		return nil
	}
	return entries
}

func workflowCheckpointPayload(value any) map[string]any {
	payload, ok := value.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return cloneWorkflowPayload(payload)
}

func mergeCheckpointMetadata(parts ...map[string]any) map[string]any {
	result := map[string]any{}
	for _, part := range parts {
		for key, value := range part {
			result[key] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
