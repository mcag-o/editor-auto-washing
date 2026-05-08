package service

import (
	"content-hub/domain"
	"fmt"
	"time"
)

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
	if ctx == nil {
		return
	}
	ctx.Checkpoints = append(ctx.Checkpoints, domain.WorkflowCheckpoint{
		WorkflowRunID: workflowRunID,
		NodeID:        nodeID,
		State:         domain.WorkflowCheckpointStateActive,
		Resumable:     true,
		CreatedAt:     time.Now().UTC(),
	})
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
