package service

import (
	"content-hub/domain"
	"fmt"
	"strings"
	"time"
)

const workflowLatestRouteMetadataKey = "workflow_route_latest"
const workflowActiveTokenSetMetadataKey = "active_token_set"
const workflowJoinBarrierSetMetadataKey = "join_barrier_set"

type workflowCheckpointSnapshot struct {
	RouteSummary *WorkflowRouteOutcomeSummary
	Token        *WorkflowToken
	PauseState   *WorkflowPauseState
	Metadata     map[string]any
}

func latestResumableCheckpoint(checkpoints []domain.WorkflowCheckpoint) (*domain.WorkflowCheckpoint, error) {
	for i := len(checkpoints) - 1; i >= 0; i-- {
		if checkpoints[i].Resumable && checkpoints[i].State == domain.WorkflowCheckpointStateActive {
			return &checkpoints[i], nil
		}
	}
	return nil, fmt.Errorf("no resumable checkpoint available")
}

func resumableCheckpointByID(checkpoints []domain.WorkflowCheckpoint, checkpointID string) (*domain.WorkflowCheckpoint, error) {
	checkpointID = strings.TrimSpace(checkpointID)
	if checkpointID == "" {
		return latestResumableCheckpoint(checkpoints)
	}
	for i := range checkpoints {
		if strings.TrimSpace(checkpoints[i].ID) != checkpointID {
			continue
		}
		if checkpoints[i].State != domain.WorkflowCheckpointStateActive || !checkpoints[i].Resumable {
			return nil, fmt.Errorf("checkpoint %s is not resumable", checkpointID)
		}
		return &checkpoints[i], nil
	}
	return nil, fmt.Errorf("checkpoint %s not found", checkpointID)
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
	if snapshot.PauseState != nil {
		if pauseMetadata, err := workflowPauseCheckpointMetadata(*snapshot.PauseState); err == nil {
			checkpoint.Metadata = mergeCheckpointMetadata(checkpoint.Metadata, pauseMetadata)
		}
	}
	if len(snapshot.Metadata) > 0 {
		checkpoint.Metadata = mergeCheckpointMetadata(checkpoint.Metadata, snapshot.Metadata)
	}
	if activeSet := workflowActiveTokenSetMetadata(ctx); activeSet != nil {
		checkpoint.Metadata = mergeCheckpointMetadata(checkpoint.Metadata, activeSet)
	}
	if joinBarriers := workflowJoinBarrierSetMetadata(ctx); joinBarriers != nil {
		checkpoint.Metadata = mergeCheckpointMetadata(checkpoint.Metadata, joinBarriers)
	}
	if loopFrames := workflowLoopFrameSetMetadata(ctx); loopFrames != nil {
		checkpoint.Metadata = mergeCheckpointMetadata(checkpoint.Metadata, loopFrames)
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
	if token.Subflow != nil {
		metadata = mergeCheckpointMetadata(metadata, workflowSubflowMetadata(token.Subflow))
	}
	if token.Frame != nil {
		metadata["token_frame_input"] = cloneWorkflowPayload(token.Frame.Input)
		metadata["token_frame_metadata"] = cloneWorkflowPayload(token.Frame.Metadata)
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

func workflowJoinBarrierSetMetadata(ctx *WorkflowExecutionContext) map[string]any {
	if ctx == nil || len(ctx.JoinBarriers) == 0 {
		return nil
	}
	barriers := make([]map[string]any, 0, len(ctx.JoinBarriers))
	for nodeID, barrier := range ctx.JoinBarriers {
		if entry := workflowJoinBarrierMetadata(nodeID, barrier); len(entry) > 0 {
			barriers = append(barriers, entry)
		}
	}
	if len(barriers) == 0 {
		return nil
	}
	return map[string]any{workflowJoinBarrierSetMetadataKey: barriers}
}

func workflowJoinBarrierMetadata(defaultNodeID string, barrier *workflowJoinBarrier) map[string]any {
	if barrier == nil {
		return nil
	}
	tokens := make([]map[string]any, 0, len(barrier.ArrivedTokenIDs))
	for _, tokenID := range barrier.ArrivedTokenIDs {
		token := barrier.tokens[tokenID]
		if token == nil {
			continue
		}
		tokens = append(tokens, workflowTokenMetadata(*token))
	}
	metadata := map[string]any{
		"node_id":                             strings.TrimSpace(firstNonEmpty(barrier.NodeID, defaultNodeID)),
		"expected_token_ids":                  append([]string(nil), barrier.ExpectedTokenIDs...),
		"expected_count":                      barrier.ExpectedCount,
		"arrived_token_ids":                   append([]string(nil), barrier.ArrivedTokenIDs...),
		"failed_token_ids":                    append([]string(nil), barrier.FailedTokenIDs...),
		"state":                               strings.TrimSpace(string(barrier.State)),
		"parent_token_id":                     strings.TrimSpace(barrier.ParentTokenID),
		"origin_token_id":                     strings.TrimSpace(barrier.OriginTokenID),
		"origin_route_node_id":                strings.TrimSpace(barrier.OriginRoute.SourceNodeID),
		"origin_route_edge_id":                strings.TrimSpace(barrier.OriginRoute.SelectedEdgeID),
		"origin_route_selected_node_id":       strings.TrimSpace(barrier.OriginRoute.SelectedNodeID),
		"merge_policy_variables":              strings.TrimSpace(string(barrier.MergePolicy.Variables)),
		"merge_policy_result":                 strings.TrimSpace(string(barrier.MergePolicy.Result)),
		"merge_policy_artifacts":              strings.TrimSpace(string(barrier.MergePolicy.Artifacts)),
		"arrived_tokens":                      tokens,
	}
	if barrier.Frame != nil {
		metadata["frame_input"] = cloneWorkflowPayload(barrier.Frame.Input)
		metadata["frame_metadata"] = cloneWorkflowPayload(barrier.Frame.Metadata)
	}
	return metadata
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
	var frame *WorkflowExecutionFrame
	if rawInput, hasInput := metadata["token_frame_input"]; hasInput {
		if rawMetadata, hasMetadata := metadata["token_frame_metadata"]; hasMetadata {
			frame = &WorkflowExecutionFrame{
				Input:    workflowCheckpointPayload(rawInput),
				Metadata: workflowCheckpointPayload(rawMetadata),
			}
		} else {
			frame = &WorkflowExecutionFrame{Input: workflowCheckpointPayload(rawInput)}
		}
	} else if rawMetadata, hasMetadata := metadata["token_frame_metadata"]; hasMetadata {
		frame = &WorkflowExecutionFrame{Metadata: workflowCheckpointPayload(rawMetadata)}
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
		Subflow: workflowSubflowFrameFromCheckpointMetadata(metadata),
		Frame: frame,
	}
}

func workflowSubflowFrameFromCheckpointMetadata(metadata map[string]any) *workflowSubflowFrame {
	if len(metadata) == 0 {
		return nil
	}
	if rawSnapshot, ok := metadata["token_subflow_frame"].(map[string]any); ok && len(rawSnapshot) > 0 {
		if frame := workflowSubflowFrameFromSnapshot(rawSnapshot); frame != nil {
			if workflowSubflowFrameComplete(frame) {
				return frame
			}
			if legacy := workflowSubflowFromMetadata(metadata); legacy != nil {
				return legacy
			}
			return frame
		}
	}
	return workflowSubflowFromMetadata(metadata)
}

func workflowSubflowFrameComplete(frame *workflowSubflowFrame) bool {
	if frame == nil {
		return false
	}
	if strings.TrimSpace(frame.ChildWorkflowID) == "" || strings.TrimSpace(frame.EntryNodeID) == "" || strings.TrimSpace(frame.ReturnNodeID) == "" {
		return false
	}
	if strings.TrimSpace(string(frame.FailureStrategy)) == "" {
		return false
	}
	return true
}

func workflowActiveTokensFromCheckpoint(checkpoint *domain.WorkflowCheckpoint) []*WorkflowToken {
	if checkpoint == nil || len(checkpoint.Metadata) == 0 {
		return nil
	}
	rawSet := workflowActiveTokenSetEntries(checkpoint.Metadata[workflowActiveTokenSetMetadataKey])
	if len(rawSet) == 0 {
		if pauseState := workflowPauseStateFromCheckpoint(checkpoint); pauseState != nil && len(pauseState.Payload) > 0 {
			if token := workflowTokenFromMetadata(strings.TrimSpace(checkpoint.NodeID), pauseState.Payload); token != nil {
				return []*WorkflowToken{token}
			}
			if workflowPauseCanSynthesizeToken(pauseState) {
				if token := workflowSyntheticPauseToken(checkpoint, pauseState); token != nil {
					return []*WorkflowToken{token}
				}
			}
		}
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

func workflowJoinBarriersFromCheckpoint(checkpoint *domain.WorkflowCheckpoint) map[string]*workflowJoinBarrier {
	if checkpoint == nil || len(checkpoint.Metadata) == 0 {
		return nil
	}
	rawSet := workflowJoinBarrierSetEntries(checkpoint.Metadata[workflowJoinBarrierSetMetadataKey])
	if len(rawSet) == 0 {
		return nil
	}
	barriers := make(map[string]*workflowJoinBarrier, len(rawSet))
	for _, raw := range rawSet {
		barrier := workflowJoinBarrierFromMetadata(raw)
		if barrier == nil || strings.TrimSpace(barrier.NodeID) == "" {
			continue
		}
		barriers[barrier.NodeID] = barrier
	}
	if len(barriers) == 0 {
		return nil
	}
	return barriers
}

func workflowSyntheticPauseToken(checkpoint *domain.WorkflowCheckpoint, pauseState *WorkflowPauseState) *WorkflowToken {
	if checkpoint == nil || pauseState == nil {
		return nil
	}
	tokenID := strings.TrimSpace(checkpoint.ResumeToken)
	if tokenID == "" {
		tokenID = strings.TrimSpace(checkpoint.ID)
	}
	if tokenID == "" {
		return nil
	}
	nodeID := strings.TrimSpace(checkpoint.NodeID)
	if nodeID == "" {
		nodeID = strings.TrimSpace(domain.DraftString(pauseState.Payload["node_id"]))
	}
	if nodeID == "" {
		return nil
	}
	return &WorkflowToken{ID: tokenID, NodeID: nodeID, OriginTokenID: tokenID}
}

func workflowPauseCanSynthesizeToken(pauseState *WorkflowPauseState) bool {
	if pauseState == nil {
		return false
	}
	if pauseState.Scope == WorkflowPauseScopeToken {
		return true
	}
	return strings.TrimSpace(domain.DraftString(pauseState.Payload["token_id"])) != ""
}

func workflowPauseStateFromCheckpoint(checkpoint *domain.WorkflowCheckpoint) *WorkflowPauseState {
	if checkpoint == nil {
		return nil
	}
	return workflowPauseStateFromMetadata(checkpoint.Metadata)
}

func workflowPauseStateFromMetadata(metadata map[string]any) *WorkflowPauseState {
	if len(metadata) == 0 {
		return nil
	}
	source := WorkflowPauseSource(strings.TrimSpace(domain.DraftString(metadata[workflowPauseSourceMetadataKey])))
	scope := WorkflowPauseScope(strings.TrimSpace(domain.DraftString(metadata[workflowPauseScopeMetadataKey])))
	reason := strings.TrimSpace(domain.DraftString(metadata[workflowPauseReasonMetadataKey]))
	allowedResumeModes := workflowPauseAllowedResumeModes(&domain.WorkflowCheckpoint{Metadata: metadata})
	if source == "" && scope == "" && reason == "" && len(allowedResumeModes) == 0 {
		return nil
	}
	pauseState := &WorkflowPauseState{
		Source:             source,
		Scope:              scope,
		Reason:             reason,
		AllowedResumeModes: allowedResumeModes,
	}
	if payload := workflowCheckpointPayload(metadata[workflowPausePayloadMetadataKey]); len(payload) > 0 {
		pauseState.Payload = payload
	}
	return pauseState
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

func workflowJoinBarrierSetEntries(raw any) []map[string]any {
	return workflowActiveTokenSetEntries(raw)
}

func workflowJoinBarrierFromMetadata(metadata map[string]any) *workflowJoinBarrier {
	if len(metadata) == 0 {
		return nil
	}
	nodeID := strings.TrimSpace(domain.DraftString(metadata["node_id"]))
	if nodeID == "" {
		return nil
	}
	barrier := newWorkflowJoinBarrier(nodeID, workflowStringList(metadata["expected_token_ids"]))
	barrier.ExpectedCount = workflowIntValue(metadata["expected_count"])
	barrier.ArrivedTokenIDs = workflowStringList(metadata["arrived_token_ids"])
	barrier.FailedTokenIDs = workflowStringList(metadata["failed_token_ids"])
	barrier.State = workflowJoinBarrierState(strings.TrimSpace(domain.DraftString(metadata["state"])))
	barrier.ParentTokenID = strings.TrimSpace(domain.DraftString(metadata["parent_token_id"]))
	barrier.OriginTokenID = strings.TrimSpace(domain.DraftString(metadata["origin_token_id"]))
	barrier.OriginRoute = WorkflowTokenRouteLineage{
		SourceNodeID:   strings.TrimSpace(domain.DraftString(metadata["origin_route_node_id"])),
		SelectedEdgeID: strings.TrimSpace(domain.DraftString(metadata["origin_route_edge_id"])),
		SelectedNodeID: strings.TrimSpace(domain.DraftString(metadata["origin_route_selected_node_id"])),
	}
	barrier.MergePolicy = workflowJoinMergePolicy{
		Variables: workflowJoinMergeStrategy(strings.TrimSpace(domain.DraftString(metadata["merge_policy_variables"]))),
		Result:    workflowJoinMergeStrategy(strings.TrimSpace(domain.DraftString(metadata["merge_policy_result"]))),
		Artifacts: workflowJoinMergeStrategy(strings.TrimSpace(domain.DraftString(metadata["merge_policy_artifacts"]))),
	}
	if rawInput, hasInput := metadata["frame_input"]; hasInput {
		barrier.Frame = &WorkflowExecutionFrame{Input: workflowCheckpointPayload(rawInput)}
		if rawMetadata, hasMetadata := metadata["frame_metadata"]; hasMetadata {
			barrier.Frame.Metadata = workflowCheckpointPayload(rawMetadata)
		}
	} else if rawMetadata, hasMetadata := metadata["frame_metadata"]; hasMetadata {
		barrier.Frame = &WorkflowExecutionFrame{Metadata: workflowCheckpointPayload(rawMetadata)}
	}
	for _, tokenID := range barrier.ArrivedTokenIDs {
		barrier.arrived[tokenID] = struct{}{}
	}
	for _, tokenID := range barrier.FailedTokenIDs {
		barrier.failed[tokenID] = struct{}{}
	}
	for _, rawToken := range workflowActiveTokenSetEntries(metadata["arrived_tokens"]) {
		token := workflowTokenFromMetadata(nodeID, rawToken)
		if token == nil {
			continue
		}
		barrier.tokens[token.ID] = token
	}
	if barrier.ExpectedCount == 0 {
		barrier.ExpectedCount = len(barrier.ExpectedTokenIDs)
	}
	if barrier.State == "" {
		barrier.State = workflowJoinBarrierStateWaiting
	}
	if barrier.MergePolicy.Variables == "" {
		barrier.MergePolicy.Variables = workflowJoinMergeStrategyFirstWriterWins
	}
	if barrier.MergePolicy.Result == "" {
		barrier.MergePolicy.Result = workflowJoinMergeStrategyLastWriterWins
	}
	if barrier.MergePolicy.Artifacts == "" {
		barrier.MergePolicy.Artifacts = workflowJoinMergeStrategyLastWriterWins
	}
	return barrier
}

func workflowStringList(raw any) []string {
	if raw == nil {
		return nil
	}
	if typed, ok := raw.([]string); ok {
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if value := strings.TrimSpace(item); value != "" {
				result = append(result, value)
			}
		}
		return result
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if value := strings.TrimSpace(domain.DraftString(item)); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func workflowIntValue(raw any) int {
	switch value := raw.(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float64:
		return int(value)
	case float32:
		return int(value)
	default:
		return 0
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
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
