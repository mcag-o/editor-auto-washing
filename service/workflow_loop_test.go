package service

import (
	"content-hub/domain"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowLoopRepeatsUntilConditionFailsThroughExitEdge(t *testing.T) {
	frame := &workflowLoopFrame{NodeID: "loop", RunID: "run-1", Iteration: 2, MaxIterations: 3, LatestDecision: workflowLoopDecisionRepeat}

	assert.Equal(t, workflowLoopDecisionRepeat, frame.LatestDecision)
	assert.Equal(t, 2, frame.Iteration)
}

func TestWorkflowLoopSharesIterationStateByLoopNodeAndRun(t *testing.T) {
	state := newWorkflowLoopStateStore()
	frameA := state.Frame("run-1", "loop")
	frameA.Iteration = 2
	frameB := state.Frame("run-1", "loop")

	require.Same(t, frameA, frameB)
	assert.Equal(t, 2, frameB.Iteration)
}

func TestWorkflowLoopPausesWhenMaxIterationsReached(t *testing.T) {
	frame := &workflowLoopFrame{NodeID: "loop", RunID: "run-1", Iteration: 3, MaxIterations: 3}
	pauseState := workflowLoopPauseState(frame)

	require.NotNil(t, pauseState)
	assert.Equal(t, WorkflowPauseSourcePolicy, pauseState.Source)
	assert.Equal(t, WorkflowPauseScopeToken, pauseState.Scope)
	assert.Equal(t, true, pauseState.Payload["paused_by_limit"])
	assert.Equal(t, 3, pauseState.Payload["iteration"])
}

func TestWorkflowLoopResumeContinuesCurrentIterationByDefault(t *testing.T) {
	checkpoint := &domain.WorkflowCheckpoint{Metadata: map[string]any{
		workflowPauseAllowedResumeModesMetadataKey: []string{string(WorkflowResumeModeContinueToken)},
	}}

	assert.Equal(t, WorkflowResumeModeContinueToken, defaultWorkflowResumeMode(checkpoint))
	assert.True(t, workflowLoopResumeContinuesCurrentIteration(WorkflowResumeModeContinueToken))
}

func TestWorkflowLoopApplyDecisionPausesAtIterationLimit(t *testing.T) {
	frame := &workflowLoopFrame{NodeID: "loop", RunID: "run-1", MaxIterations: 3}

	result1 := workflowLoopApplyDecision(&WorkflowExecutionContext{}, frame, workflowLoopDecisionRepeat)
	result2 := workflowLoopApplyDecision(&WorkflowExecutionContext{}, frame, workflowLoopDecisionRepeat)
	result3 := workflowLoopApplyDecision(&WorkflowExecutionContext{}, frame, workflowLoopDecisionRepeat)

	assert.False(t, result1.Paused)
	assert.False(t, result2.Paused)
	require.True(t, result3.Paused)
	require.NotNil(t, result3.PauseState)
	assert.Equal(t, 3, frame.Iteration)
}

func TestWorkflowLoopCheckpointMetadataRoundTripsLoopFrame(t *testing.T) {
	runtimeCtx := &WorkflowExecutionContext{
		Metadata: map[string]any{},
	}
	frame := ensureWorkflowLoopFrame(runtimeCtx, "loop", "run-1", 3)
	frame.Iteration = 2
	frame.LatestDecision = workflowLoopDecisionRepeat
	frame.PausedByLimit = true
	frame.Paused = true
	frame.ResumeMode = WorkflowResumeModeContinueToken

	appendCheckpointWithSnapshot(runtimeCtx, "run-1", "loop", workflowCheckpointSnapshot{})
	require.Len(t, runtimeCtx.Checkpoints, 1)

	restored := workflowLoopFrameFromCheckpoint(&runtimeCtx.Checkpoints[0])
	require.NotNil(t, restored)
	assert.Equal(t, frame.NodeID, restored.NodeID)
	assert.Equal(t, frame.RunID, restored.RunID)
	assert.Equal(t, frame.Iteration, restored.Iteration)
	assert.Equal(t, frame.MaxIterations, restored.MaxIterations)
	assert.Equal(t, frame.LatestDecision, restored.LatestDecision)
	assert.Equal(t, frame.PausedByLimit, restored.PausedByLimit)
	assert.Equal(t, frame.Paused, restored.Paused)
	assert.Equal(t, frame.ResumeMode, restored.ResumeMode)
}

func TestWorkflowLoopSelectedEdgesRejectsDuplicateExplicitTargets(t *testing.T) {
	node := domain.WorkflowNode{
		ID:         "loop",
		Type:       "loop",
		Name:       "Loop",
		ConfigJSON: `{"body_to_node_id":"body","exit_to_node_id":"exit"}`,
	}
	edges := []domain.WorkflowEdge{
		{FromNodeID: "loop", ToNodeID: "body", Priority: 1},
		{FromNodeID: "loop", ToNodeID: "body", Priority: 2},
		{FromNodeID: "loop", ToNodeID: "exit", Priority: 3},
	}

	selected, err := workflowLoopSelectedEdges(node, edges, workflowLoopDecisionRepeat)

	require.Error(t, err)
	assert.Nil(t, selected)
	assert.Contains(t, err.Error(), "ambiguous explicit body edge")
}

func TestWorkflowLoopFrameFromCheckpointRequiresMatchingNodeID(t *testing.T) {
	checkpoint := domain.WorkflowCheckpoint{
		NodeID: "loop-b",
		Metadata: map[string]any{
			workflowLoopFrameSetMetadataKey: []any{
				map[string]any{"node_id": "loop-a", "run_id": "run-1", "iteration": 1},
			},
		},
	}

	restored := workflowLoopFrameFromCheckpoint(&checkpoint)

	assert.Nil(t, restored)
}
