package service

import (
	"content-hub/domain"
	"content-hub/pkg/id"
	"strings"
)

type workflowJoinBarrierState string

const (
	workflowJoinBarrierStateWaiting workflowJoinBarrierState = "waiting"
	workflowJoinBarrierStateReady   workflowJoinBarrierState = "ready"
	workflowJoinBarrierStateBlocked workflowJoinBarrierState = "blocked"
)

type workflowJoinMergeStrategy string

const (
	workflowJoinMergeStrategyFirstWriterWins workflowJoinMergeStrategy = "first_writer_wins"
	workflowJoinMergeStrategyLastWriterWins  workflowJoinMergeStrategy = "last_writer_wins"
)

type workflowJoinMergePolicy struct {
	Variables workflowJoinMergeStrategy
	Result    workflowJoinMergeStrategy
	Artifacts workflowJoinMergeStrategy
}

type workflowJoinBarrier struct {
	NodeID           string
	ExpectedTokenIDs []string
	ExpectedCount    int
	ArrivedTokenIDs  []string
	FailedTokenIDs   []string
	arrived          map[string]struct{}
	failed           map[string]struct{}
	tokens           map[string]*WorkflowToken
	State            workflowJoinBarrierState
	MergePolicy      workflowJoinMergePolicy
	ParentTokenID    string
	OriginTokenID    string
	OriginRoute      WorkflowTokenRouteLineage
	Frame            *WorkflowExecutionFrame
}

func newWorkflowJoinBarrier(nodeID string, expectedTokenIDs []string) *workflowJoinBarrier {
	expected := make([]string, 0, len(expectedTokenIDs))
	seen := make(map[string]struct{}, len(expectedTokenIDs))
	for _, tokenID := range expectedTokenIDs {
		trimmed := strings.TrimSpace(tokenID)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		expected = append(expected, trimmed)
	}
	return &workflowJoinBarrier{
		NodeID:           strings.TrimSpace(nodeID),
		ExpectedTokenIDs: expected,
		ExpectedCount:    len(expected),
		arrived:          make(map[string]struct{}, len(expected)),
		failed:           make(map[string]struct{}, len(expected)),
		tokens:           make(map[string]*WorkflowToken, len(expected)),
		State:            workflowJoinBarrierStateWaiting,
		MergePolicy: workflowJoinMergePolicy{
			Variables: workflowJoinMergeStrategyFirstWriterWins,
			Result:    workflowJoinMergeStrategyLastWriterWins,
			Artifacts: workflowJoinMergeStrategyLastWriterWins,
		},
	}
}

func newWorkflowJoinBarrierWithExpectedCount(nodeID string, expectedCount int) *workflowJoinBarrier {
	if expectedCount < 0 {
		expectedCount = 0
	}
	barrier := newWorkflowJoinBarrier(nodeID, nil)
	barrier.ExpectedCount = expectedCount
	return barrier
}

func (b *workflowJoinBarrier) Arrive(tokenID string) {
	if b == nil {
		return
	}
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return
	}
	if _, ok := b.arrived[tokenID]; ok {
		return
	}
	b.arrived[tokenID] = struct{}{}
	b.ArrivedTokenIDs = append(b.ArrivedTokenIDs, tokenID)
	if len(b.failed) > 0 {
		b.State = workflowJoinBarrierStateBlocked
		return
	}
	expectedCount := b.ExpectedCount
	if expectedCount == 0 {
		expectedCount = len(b.ExpectedTokenIDs)
	}
	if expectedCount > 0 && len(b.arrived) >= expectedCount {
		b.State = workflowJoinBarrierStateReady
	}
}

func (b *workflowJoinBarrier) Fail(tokenID string) {
	if b == nil {
		return
	}
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return
	}
	if _, ok := b.failed[tokenID]; ok {
		return
	}
	b.failed[tokenID] = struct{}{}
	b.FailedTokenIDs = append(b.FailedTokenIDs, tokenID)
	b.State = workflowJoinBarrierStateBlocked
}

func (b *workflowJoinBarrier) Ready() bool {
	return b != nil && b.State == workflowJoinBarrierStateReady
}

func mergeWorkflowJoinBranches(tokens []*WorkflowToken, policy workflowJoinMergePolicy) *WorkflowBranchContext {
	merged := newWorkflowBranchContext(nil, nil)
	for _, token := range tokens {
		if token == nil || token.Branch == nil {
			continue
		}
		mergeWorkflowJoinField(merged.Variables, token.Branch.Variables, policy.Variables)
		mergeWorkflowJoinField(merged.Result, token.Branch.Result, policy.Result)
		mergeWorkflowJoinField(merged.Artifacts, token.Branch.Artifacts, policy.Artifacts)
	}
	return merged
}

func mergeWorkflowJoinField(dst map[string]any, src map[string]any, strategy workflowJoinMergeStrategy) {
	if dst == nil || len(src) == 0 {
		return
	}
	for key, value := range src {
		_, exists := dst[key]
		switch strategy {
		case workflowJoinMergeStrategyFirstWriterWins:
			if exists {
				continue
			}
			dst[key] = cloneWorkflowValue(value)
		default:
			dst[key] = cloneWorkflowValue(value)
		}
	}
}

func incomingEdges(edges []domain.WorkflowEdge, toNodeID string) []domain.WorkflowEdge {
	result := make([]domain.WorkflowEdge, 0, len(edges))
	for _, edge := range edges {
		if edge.ToNodeID == toNodeID {
			result = append(result, edge)
		}
	}
	return result
}

func workflowJoinTargets(wf *domain.WorkflowDefinition) map[string]int {
	result := map[string]int{}
	if wf == nil {
		return result
	}
	for _, edge := range wf.Edges {
		toNodeID := strings.TrimSpace(edge.ToNodeID)
		if toNodeID == "" {
			continue
		}
		result[toNodeID]++
	}
	for nodeID, count := range result {
		if count < 2 {
			delete(result, nodeID)
		}
	}
	return result
}

func workflowJoinExpectedParentTokenIDs(runtimeCtx *WorkflowExecutionContext, nodeID string) []string {
	if runtimeCtx == nil || runtimeCtx.Workflow == nil {
		return nil
	}
	parents := make([]string, 0)
	for _, token := range runtimeCtx.CompletedTokens {
		if token == nil || token.OriginTokenID == "" {
			continue
		}
		for _, edge := range incomingEdges(runtimeCtx.Workflow.Edges, nodeID) {
			if token.NodeID == edge.FromNodeID {
				parents = append(parents, token.ID)
				break
			}
		}
	}
	return parents
}

func workflowJoinToken(nodeID string, barrier *workflowJoinBarrier) *WorkflowToken {
	if barrier == nil {
		return nil
	}
	return &WorkflowToken{
		ID:            id.New(),
		NodeID:        strings.TrimSpace(nodeID),
		State:         WorkflowTokenStateActive,
		ParentTokenID: barrier.ParentTokenID,
		OriginTokenID: barrier.OriginTokenID,
		OriginRoute:   barrier.OriginRoute,
		Branch:        mergeWorkflowJoinBranches(barrierTokensInArrivalOrder(barrier), barrier.MergePolicy),
		Frame:         cloneWorkflowExecutionFrame(barrier.Frame),
	}
}

func barrierTokensInArrivalOrder(barrier *workflowJoinBarrier) []*WorkflowToken {
	if barrier == nil {
		return nil
	}
	tokens := make([]*WorkflowToken, 0, len(barrier.ArrivedTokenIDs))
	for _, tokenID := range barrier.ArrivedTokenIDs {
		if token := barrier.tokens[tokenID]; token != nil {
			tokens = append(tokens, token)
		}
	}
	return tokens
}
