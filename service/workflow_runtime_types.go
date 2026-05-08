package service

import (
	"content-hub/domain"
	"context"
)

type WorkflowRouteOutcome string

type WorkflowRouteOutcomeSummary struct {
	NodeID          string
	SelectedEdgeID  string
	SelectedNodeID  string
	Outcome         WorkflowRouteOutcome
	EvaluationTrace []string
}

type WorkflowExecutionContext struct {
	Workflow        *domain.WorkflowDefinition
	Context         *domain.WorkflowContext
	Input           map[string]any
	sharedInput     map[string]any
	LatestRoute     *WorkflowRouteOutcomeSummary
	CurrentNodeID   string
	CurrentToken    *WorkflowToken
	RootToken       *WorkflowToken
	ActiveTokens    []*WorkflowToken
	CompletedTokens []*WorkflowToken
	Checkpoints     []domain.WorkflowCheckpoint
	Variables       map[string]any
	Result          map[string]any
	Metadata        map[string]any
	sharedMetadata  map[string]any
	Artifacts       map[string]any
}

type WorkflowNodeResult struct {
	RouteRequired           bool
	AllowNaturalTermination bool
	Output                  map[string]any
}

type workflowRuntimeNode interface {
	WorkflowNode
	ExecuteWorkflow(ctx context.Context, runtimeCtx *WorkflowExecutionContext, node domain.WorkflowNode) (WorkflowNodeResult, error)
}
