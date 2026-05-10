package service

import (
	"content-hub/domain"
	"strings"
)

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
	if runtimeCtx.Metadata == nil {
		runtimeCtx.Metadata = map[string]any{}
	}
	store, _ := runtimeCtx.Metadata["workflow_loop_state_store"].(*workflowLoopStateStore)
	if store == nil {
		store = newWorkflowLoopStateStore()
		runtimeCtx.Metadata["workflow_loop_state_store"] = store
	}
	frame := store.Frame(runID, nodeID)
	frame.MaxIterations = maxIterations
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
		frame.Iteration++
		if workflowLoopShouldPause(frame) {
			frame.PausedByLimit = true
			frame.Paused = true
			return WorkflowNodeResult{Paused: true, PauseState: workflowLoopPauseState(frame), Output: map[string]any{"loop_decision": string(decision)}}
		}
	}
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
