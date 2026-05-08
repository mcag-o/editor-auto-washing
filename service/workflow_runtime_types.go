package service

import "content-hub/domain"

type WorkflowExecutionContext struct {
	Workflow      *domain.WorkflowDefinition
	Context       *domain.WorkflowContext
	CurrentNodeID string
	Checkpoints   []domain.WorkflowCheckpoint
}

type WorkflowNodeResult struct {
	RouteRequired bool
}
