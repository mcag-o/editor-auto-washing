package service

import (
	"content-hub/domain"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLatestResumableCheckpointReturnsMostRecentActiveCheckpoint(t *testing.T) {
	checkpoints := []domain.WorkflowCheckpoint{
		{NodeID: "start", State: domain.WorkflowCheckpointStateActive, Resumable: true},
		{NodeID: "middle", State: domain.WorkflowCheckpointStateTerminal, Resumable: true},
		{NodeID: "end", State: domain.WorkflowCheckpointStateActive, Resumable: true},
	}

	checkpoint, err := latestResumableCheckpoint(checkpoints)

	require.NoError(t, err)
	assert.Equal(t, "end", checkpoint.NodeID)
}

func TestAppendCheckpointAppendsResumableCheckpoint(t *testing.T) {
	runtimeCtx := &WorkflowExecutionContext{}

	appendCheckpoint(runtimeCtx, "run-1", "next")

	require.Len(t, runtimeCtx.Checkpoints, 1)
	assert.Equal(t, "run-1", runtimeCtx.Checkpoints[0].WorkflowRunID)
	assert.Equal(t, "next", runtimeCtx.Checkpoints[0].NodeID)
	assert.Equal(t, domain.WorkflowCheckpointStateActive, runtimeCtx.Checkpoints[0].State)
	assert.True(t, runtimeCtx.Checkpoints[0].Resumable)
}

func TestConsumeActiveCheckpointsTerminalizesResumePoints(t *testing.T) {
	checkpointTime := time.Date(2026, time.May, 8, 2, 0, 0, 0, time.UTC)
	runtimeCtx := &WorkflowExecutionContext{Checkpoints: []domain.WorkflowCheckpoint{
		{NodeID: "middle", State: domain.WorkflowCheckpointStateActive, Resumable: true},
		{NodeID: "end", State: domain.WorkflowCheckpointStateActive, Resumable: true},
	}}

	consumeActiveCheckpoints(runtimeCtx, checkpointTime)

	assert.Equal(t, domain.WorkflowCheckpointStateTerminal, runtimeCtx.Checkpoints[0].State)
	assert.False(t, runtimeCtx.Checkpoints[0].Resumable)
	require.NotNil(t, runtimeCtx.Checkpoints[0].ConsumedAt)
	assert.Equal(t, checkpointTime, *runtimeCtx.Checkpoints[0].ConsumedAt)
	assert.Equal(t, domain.WorkflowCheckpointStateTerminal, runtimeCtx.Checkpoints[1].State)
	assert.False(t, runtimeCtx.Checkpoints[1].Resumable)
	require.NotNil(t, runtimeCtx.Checkpoints[1].ConsumedAt)
	assert.Equal(t, checkpointTime, *runtimeCtx.Checkpoints[1].ConsumedAt)
}
