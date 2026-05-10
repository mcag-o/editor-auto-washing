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
