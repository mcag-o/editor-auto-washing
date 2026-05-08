package service

import (
	"content-hub/domain"
	"context"
)

type WorkflowRouteOutcome string

type WorkflowPauseSource string

const (
	WorkflowPauseSourceHumanNode WorkflowPauseSource = "human_node"
	WorkflowPauseSourceManual    WorkflowPauseSource = "manual"
	WorkflowPauseSourcePolicy    WorkflowPauseSource = "policy"
)

type WorkflowResumeMode string

const (
	WorkflowResumeModeContinueToken        WorkflowResumeMode = "continue_token"
	WorkflowResumeModeContinueActiveTokens WorkflowResumeMode = "continue_active_tokens"
	WorkflowResumeModeReplayFromCheckpoint WorkflowResumeMode = "replay_from_checkpoint"
)

type WorkflowTokenState string

const (
	WorkflowTokenStateActive WorkflowTokenState = "active"
	WorkflowTokenStatePaused WorkflowTokenState = "paused"
)

type WorkflowPauseState struct {
	Source             WorkflowPauseSource
	Scope              WorkflowPauseScope
	Reason             string
	Payload            map[string]any
	AllowedResumeModes []WorkflowResumeMode
}

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
	Paused                  bool
	PauseState              *WorkflowPauseState
	Output                  map[string]any
}

type workflowRuntimeNode interface {
	WorkflowNode
	ExecuteWorkflow(ctx context.Context, runtimeCtx *WorkflowExecutionContext, node domain.WorkflowNode) (WorkflowNodeResult, error)
}
