package service

import (
	"content-hub/domain"
	"encoding/json"
	"sort"
	"fmt"
	"strings"
)

const workflowLoopFrameSetMetadataKey = "workflow_loop_frame_set"

type workflowLoopDecision string

const (
	workflowLoopDecisionRepeat workflowLoopDecision = "repeat"
	workflowLoopDecisionExit   workflowLoopDecision = "exit"
)

type workflowLoopFrame struct {
	NodeID         string
	RunID          string
	Iteration      int
	MaxIterations  int
	LatestDecision workflowLoopDecision
	PausedByLimit  bool
	Paused         bool
	ResumeMode     WorkflowResumeMode
}

type workflowLoopStateStore struct {
	frames map[string]*workflowLoopFrame
}

type workflowLoopNodeConfig struct {
	BodyToNodeID string `json:"body_to_node_id"`
	ExitToNodeID string `json:"exit_to_node_id"`
}

func newWorkflowLoopStateStore() *workflowLoopStateStore {
	return &workflowLoopStateStore{frames: map[string]*workflowLoopFrame{}}
}

func (s *workflowLoopStateStore) Frame(runID, nodeID string) *workflowLoopFrame {
	if s == nil {
		return nil
	}
	key := strings.TrimSpace(runID) + ":" + strings.TrimSpace(nodeID)
	if frame, ok := s.frames[key]; ok {
		return frame
	}
	frame := &workflowLoopFrame{RunID: strings.TrimSpace(runID), NodeID: strings.TrimSpace(nodeID)}
	s.frames[key] = frame
	return frame
}

func ensureWorkflowLoopFrame(runtimeCtx *WorkflowExecutionContext, nodeID, runID string, maxIterations int) *workflowLoopFrame {
	if runtimeCtx == nil {
		return nil
	}
	sharedMetadata := workflowRuntimeSharedMetadata(runtimeCtx)
	store, _ := sharedMetadata["workflow_loop_state_store"].(*workflowLoopStateStore)
	if store == nil {
		store = newWorkflowLoopStateStore()
		sharedMetadata["workflow_loop_state_store"] = store
	}
	frame := store.Frame(runID, nodeID)
	frame.MaxIterations = maxIterations
	syncWorkflowLoopFrameMetadata(runtimeCtx, frame)
	return frame
}

func workflowLoopPauseState(frame *workflowLoopFrame) *WorkflowPauseState {
	if frame == nil {
		return nil
	}
	return &WorkflowPauseState{
		Source: WorkflowPauseSourcePolicy,
		Scope:  WorkflowPauseScopeToken,
		Reason: "loop iteration limit reached",
		Payload: map[string]any{
			"paused_by_limit": true,
			"iteration":       frame.Iteration,
			"node_id":         frame.NodeID,
			"run_id":          frame.RunID,
		},
		AllowedResumeModes: []WorkflowResumeMode{WorkflowResumeModeContinueToken, WorkflowResumeModeContinueActiveTokens},
	}
}

func workflowLoopResumeContinuesCurrentIteration(mode WorkflowResumeMode) bool {
	return mode == WorkflowResumeModeContinueToken || mode == WorkflowResumeModeContinueActiveTokens
}

func workflowLoopShouldPause(frame *workflowLoopFrame) bool {
	return frame != nil && frame.MaxIterations > 0 && frame.Iteration >= frame.MaxIterations
}

func workflowLoopApplyDecision(runtimeCtx *WorkflowExecutionContext, frame *workflowLoopFrame, decision workflowLoopDecision) WorkflowNodeResult {
	if frame == nil {
		return WorkflowNodeResult{}
	}
	frame.LatestDecision = decision
	if decision == workflowLoopDecisionRepeat {
		if frame.ResumeMode == WorkflowResumeModeContinueToken || frame.ResumeMode == WorkflowResumeModeContinueActiveTokens {
			frame.ResumeMode = ""
			frame.PausedByLimit = false
			frame.Paused = false
			syncWorkflowLoopFrameMetadata(runtimeCtx, frame)
		} else {
			frame.Iteration++
			if workflowLoopShouldPause(frame) {
			frame.PausedByLimit = true
			frame.Paused = true
			frame.ResumeMode = WorkflowResumeModeContinueToken
			syncWorkflowLoopFrameMetadata(runtimeCtx, frame)
			return WorkflowNodeResult{Paused: true, PauseState: workflowLoopPauseState(frame), Output: map[string]any{"loop_decision": string(decision)}}
 			}
			syncWorkflowLoopFrameMetadata(runtimeCtx, frame)
		}
	}
	frame.PausedByLimit = false
	frame.Paused = false
	if decision != workflowLoopDecisionRepeat {
		frame.ResumeMode = ""
	}
	syncWorkflowLoopFrameMetadata(runtimeCtx, frame)
	if runtimeCtx != nil {
		if runtimeCtx.Result == nil {
			runtimeCtx.Result = map[string]any{}
		}
		runtimeCtx.Result["loop_decision"] = string(decision)
	}
	return WorkflowNodeResult{Output: map[string]any{"loop_decision": string(decision)}}
}

func workflowLoopResumeModeFromCheckpoint(checkpoint *domain.WorkflowCheckpoint) WorkflowResumeMode {
	if pauseState := workflowPauseStateFromCheckpoint(checkpoint); pauseState != nil && workflowLoopResumeContinuesCurrentIteration(defaultWorkflowResumeMode(checkpoint)) {
		return defaultWorkflowResumeMode(checkpoint)
	}
	return defaultWorkflowResumeMode(checkpoint)
}

func workflowLoopFrameMetadata(frame *workflowLoopFrame) map[string]any {
	if frame == nil {
		return nil
	}
	return map[string]any{
		"node_id":                 strings.TrimSpace(frame.NodeID),
		"run_id":                  strings.TrimSpace(frame.RunID),
		"iteration":               frame.Iteration,
		"max_iterations":          frame.MaxIterations,
		"latest_decision":         strings.TrimSpace(string(frame.LatestDecision)),
		"paused_by_limit":         frame.PausedByLimit,
		"paused":                  frame.Paused,
		"resume_mode":             strings.TrimSpace(string(frame.ResumeMode)),
		"workflow_loop_iteration": frame.Iteration,
		"loop_paused_by_limit":    frame.PausedByLimit,
	}
}

func workflowLoopFrameSetMetadata(ctx *WorkflowExecutionContext) map[string]any {
	if ctx == nil {
		return nil
	}
	store, _ := workflowRuntimeSharedMetadata(ctx)["workflow_loop_state_store"].(*workflowLoopStateStore)
	if store == nil || len(store.frames) == 0 {
		return nil
	}
	frames := make([]map[string]any, 0, len(store.frames))
	for _, frame := range store.frames {
		if entry := workflowLoopFrameMetadata(frame); len(entry) > 0 {
			frames = append(frames, entry)
		}
	}
	if len(frames) == 0 {
		return nil
	}
	return map[string]any{workflowLoopFrameSetMetadataKey: frames}
}

func workflowLoopFrameFromCheckpoint(checkpoint *domain.WorkflowCheckpoint) *workflowLoopFrame {
	frames := workflowLoopFramesFromCheckpoint(checkpoint)
	if len(frames) == 0 {
		return nil
	}
	if checkpoint != nil {
		nodeID := strings.TrimSpace(checkpoint.NodeID)
		for _, frame := range frames {
			if strings.TrimSpace(frame.NodeID) == nodeID {
				return frame
			}
		}
	}
	return nil
}

func workflowLoopFramesFromCheckpoint(checkpoint *domain.WorkflowCheckpoint) []*workflowLoopFrame {
	if checkpoint == nil || len(checkpoint.Metadata) == 0 {
		return nil
	}
	rawEntries, ok := checkpoint.Metadata[workflowLoopFrameSetMetadataKey]
	if !ok {
		if frame := workflowLoopFrameFromMetadata(checkpoint.Metadata); frame != nil {
			return []*workflowLoopFrame{frame}
		}
		return nil
	}
	entries := workflowActiveTokenSetEntries(rawEntries)
	frames := make([]*workflowLoopFrame, 0, len(entries))
	for _, entry := range entries {
		frame := workflowLoopFrameFromMetadata(entry)
		if frame != nil {
			frames = append(frames, frame)
		}
	}
	if len(frames) == 0 {
		if frame := workflowLoopFrameFromMetadata(checkpoint.Metadata); frame != nil {
			return []*workflowLoopFrame{frame}
		}
	}
	return frames
}

func workflowLoopFrameFromMetadata(metadata map[string]any) *workflowLoopFrame {
	if len(metadata) == 0 {
		return nil
	}
	nodeID := strings.TrimSpace(domain.DraftString(metadata["node_id"]))
	runID := strings.TrimSpace(domain.DraftString(metadata["run_id"]))
	decision := workflowLoopDecision(strings.TrimSpace(domain.DraftString(metadata["latest_decision"])))
	iteration := workflowIntValue(metadata["iteration"])
	if iteration == 0 {
		iteration = workflowIntValue(metadata["workflow_loop_iteration"])
	}
	maxIterations := workflowIntValue(metadata["max_iterations"])
	pausedByLimit, _ := metadata["paused_by_limit"].(bool)
	if !pausedByLimit {
		pausedByLimit, _ = metadata["loop_paused_by_limit"].(bool)
	}
	paused, _ := metadata["paused"].(bool)
	if nodeID == "" && runID == "" && decision == "" && iteration == 0 && maxIterations == 0 && !pausedByLimit && !paused {
		return nil
	}
	return &workflowLoopFrame{
		NodeID:         nodeID,
		RunID:          runID,
		Iteration:      iteration,
		MaxIterations:  maxIterations,
		LatestDecision: decision,
		PausedByLimit:  pausedByLimit,
		Paused:         paused,
		ResumeMode:     WorkflowResumeMode(strings.TrimSpace(domain.DraftString(metadata["resume_mode"]))),
	}
}

func applyWorkflowLoopFrames(runtimeCtx *WorkflowExecutionContext, checkpoint *domain.WorkflowCheckpoint) {
	if runtimeCtx == nil {
		return
	}
	frames := workflowLoopFramesFromCheckpoint(checkpoint)
	if len(frames) == 0 {
		return
	}
	sharedMetadata := workflowRuntimeSharedMetadata(runtimeCtx)
	store := newWorkflowLoopStateStore()
	for _, frame := range frames {
		if frame == nil {
			continue
		}
		store.frames[strings.TrimSpace(frame.RunID)+":"+strings.TrimSpace(frame.NodeID)] = frame
		for key, value := range workflowLoopFrameMetadata(frame) {
			sharedMetadata[key] = value
		}
	}
	sharedMetadata["workflow_loop_state_store"] = store
}

func workflowLoopFrameForNode(runtimeCtx *WorkflowExecutionContext, nodeID string) *workflowLoopFrame {
	if runtimeCtx == nil {
		return nil
	}
	store, _ := workflowRuntimeSharedMetadata(runtimeCtx)["workflow_loop_state_store"].(*workflowLoopStateStore)
	if store == nil {
		return nil
	}
	for _, frame := range store.frames {
		if frame != nil && strings.TrimSpace(frame.NodeID) == strings.TrimSpace(nodeID) {
			return frame
		}
	}
	return nil
}

func workflowLoopDecisionFromResult(result WorkflowNodeResult) workflowLoopDecision {
	if len(result.Output) == 0 {
		return ""
	}
	return workflowLoopDecision(strings.TrimSpace(domain.DraftString(result.Output["loop_decision"])))
}

func workflowLoopSelectedEdges(node domain.WorkflowNode, edges []domain.WorkflowEdge, decision workflowLoopDecision) ([]domain.WorkflowEdge, error) {
	if len(edges) == 0 {
		return nil, nil
	}
	config, err := parseWorkflowLoopNodeConfig(node.ConfigJSON)
	if err != nil {
		return nil, fmt.Errorf("parse loop workflow node config: %w", err)
	}
	if config.BodyToNodeID != "" || config.ExitToNodeID != "" {
		selected, ambiguous := workflowLoopSelectEdgesByTarget(edges, config, decision)
		if ambiguous {
			return nil, fmt.Errorf("loop node %s has ambiguous explicit %s edge", strings.TrimSpace(node.ID), workflowLoopDecisionLabel(decision))
		}
		if len(selected) == 0 {
			return nil, fmt.Errorf("loop node %s missing explicit %s edge", strings.TrimSpace(node.ID), workflowLoopDecisionLabel(decision))
		}
		return selected, nil
	}
	ordered := append([]domain.WorkflowEdge(nil), edges...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Priority < ordered[j].Priority
	})
	if len(ordered) == 1 {
		return ordered[:1], nil
	}
	switch decision {
	case workflowLoopDecisionRepeat:
		return ordered[:1], nil
	case workflowLoopDecisionExit:
		return ordered[1:2], nil
	default:
		return ordered, nil
	}
}

func workflowLoopResumeCurrentIteration(runtimeCtx *WorkflowExecutionContext, frame *workflowLoopFrame) WorkflowNodeResult {
	if frame == nil {
		return WorkflowNodeResult{}
	}
	frame.Paused = false
	frame.PausedByLimit = true
	frame.ResumeMode = WorkflowResumeModeContinueToken
	syncWorkflowLoopFrameMetadata(runtimeCtx, frame)
	return WorkflowNodeResult{Output: map[string]any{"loop_decision": string(workflowLoopDecisionRepeat)}}
}

func syncWorkflowLoopFrameMetadata(runtimeCtx *WorkflowExecutionContext, frame *workflowLoopFrame) {
	if runtimeCtx == nil || frame == nil {
		return
	}
	sharedMetadata := workflowRuntimeSharedMetadata(runtimeCtx)
	for key, value := range workflowLoopFrameMetadata(frame) {
		sharedMetadata[key] = value
	}
}

func parseWorkflowLoopNodeConfig(configJSON string) (workflowLoopNodeConfig, error) {
	configJSON = strings.TrimSpace(configJSON)
	if configJSON == "" {
		return workflowLoopNodeConfig{}, nil
	}
	var cfg workflowLoopNodeConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return workflowLoopNodeConfig{}, fmt.Errorf("decode workflow loop node config json: %w", err)
	}
	cfg.BodyToNodeID = strings.TrimSpace(cfg.BodyToNodeID)
	cfg.ExitToNodeID = strings.TrimSpace(cfg.ExitToNodeID)
	return cfg, nil
}

func workflowLoopSelectEdgesByTarget(edges []domain.WorkflowEdge, config workflowLoopNodeConfig, decision workflowLoopDecision) ([]domain.WorkflowEdge, bool) {
	targetNodeID := ""
	switch decision {
	case workflowLoopDecisionRepeat:
		targetNodeID = config.BodyToNodeID
	case workflowLoopDecisionExit:
		targetNodeID = config.ExitToNodeID
	}
	if targetNodeID == "" {
		return nil, false
	}
	selected := make([]domain.WorkflowEdge, 0, 1)
	for _, edge := range edges {
		if strings.TrimSpace(edge.ToNodeID) == targetNodeID {
			selected = append(selected, edge)
		}
	}
	if len(selected) > 1 {
		return nil, true
	}
	return selected, false
}

func workflowLoopDecisionLabel(decision workflowLoopDecision) string {
	if decision == workflowLoopDecisionExit {
		return "exit"
	}
	return "body"
}
