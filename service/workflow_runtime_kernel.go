package service

import (
	"content-hub/domain"
	"context"
	"fmt"
	"strings"
	"time"
)

type workflowRuntimeKernel struct {
	nodes  map[string]WorkflowNode
	router workflowRouter
}

func newWorkflowRuntimeKernel(nodes map[string]WorkflowNode) workflowRuntimeKernel {
	return workflowRuntimeKernel{nodes: nodes, router: newWorkflowRouter()}
}

func (k workflowRuntimeKernel) Execute(ctx context.Context, wf *domain.WorkflowDefinition, wc *domain.WorkflowContext) error {
	runtimeCtx := &WorkflowExecutionContext{
		Workflow:      wf,
		Context:       wc,
		CurrentNodeID: wf.EntryNodeID,
	}
	return k.executeFrom(ctx, runtimeCtx, wf.EntryNodeID)
}

func (k workflowRuntimeKernel) Resume(ctx context.Context, runtimeCtx *WorkflowExecutionContext) error {
	checkpoint, err := latestResumableCheckpoint(runtimeCtx.Checkpoints)
	if err != nil {
		return err
	}
	runtimeCtx.ActiveTokens = workflowActiveTokensFromCheckpoint(checkpoint)
	if len(runtimeCtx.ActiveTokens) > 0 {
		runtimeCtx.CurrentToken = runtimeCtx.ActiveTokens[len(runtimeCtx.ActiveTokens)-1]
	} else {
		runtimeCtx.CurrentToken = workflowTokenFromCheckpoint(checkpoint)
		if runtimeCtx.CurrentToken != nil {
			runtimeCtx.ActiveTokens = []*WorkflowToken{runtimeCtx.CurrentToken}
		}
	}
	runtimeCtx.CurrentNodeID = checkpoint.NodeID
	return k.executeFrom(ctx, runtimeCtx, checkpoint.NodeID)
}

func (k workflowRuntimeKernel) executeFrom(ctx context.Context, runtimeCtx *WorkflowExecutionContext, startNodeID string) error {
	if runtimeCtx == nil || runtimeCtx.Workflow == nil {
		return fmt.Errorf("workflow definition is required")
	}
	if err := validateWorkflowRuntimeGraph(runtimeCtx.Workflow); err != nil {
		return err
	}
	consumeActiveCheckpoints(runtimeCtx, time.Now().UTC())

	nodeIndex := indexWorkflowNodes(runtimeCtx.Workflow)
	initializeWorkflowTokens(runtimeCtx, startNodeID)
	for len(runtimeCtx.ActiveTokens) > 0 {
		token := popNextActiveWorkflowToken(runtimeCtx)
		if token == nil || strings.TrimSpace(token.NodeID) == "" {
			continue
		}
		current := token.NodeID
		bindWorkflowTokenBranch(runtimeCtx, token)
		runtimeCtx.CurrentToken = token
		runtimeCtx.CurrentNodeID = current
		node, ok := k.nodes[current]
		if !ok {
			return fmt.Errorf("workflow node not found: %s", current)
		}
		result := WorkflowNodeResult{}
		nextEdges := outgoingEdges(runtimeCtx.Workflow.Edges, current)
		graphRouteRequired := routeRequired(nextEdges)
		if runtimeNode, ok := node.(workflowRuntimeNode); ok {
			nodeDef, found := nodeIndex[current]
			if !found {
				return fmt.Errorf("workflow node not found in definition: %s", current)
			}
			execResult, err := runtimeNode.ExecuteWorkflow(ctx, runtimeCtx, nodeDef)
			if err != nil {
				return err
			}
			result = execResult
			result.RouteRequired = graphRouteRequired && !result.AllowNaturalTermination
		} else {
			if err := node.Execute(ctx, runtimeCtx.Context); err != nil {
				return err
			}
			result.RouteRequired = graphRouteRequired
		}
		storeWorkflowNodeResult(token, result)
		rebindWorkflowNodeResult(runtimeCtx, token)

		evaluation := k.router.EvaluateRoutes(runtimeCtx, current, result, nextEdges)
		recordLatestRouteOutcome(runtimeCtx, workflowRouteOutcomeSummary(evaluation))
		switch evaluation.Outcome {
		case WorkflowRouteOutcomeSingleMatch, WorkflowRouteOutcomeMultiMatch, WorkflowRouteOutcomeFallbackMatch:
			type checkpointChild struct {
				token   *WorkflowToken
				summary WorkflowRouteOutcomeSummary
			}
			children := make([]checkpointChild, 0, len(evaluation.SelectedEdges))
			for i := len(evaluation.SelectedEdges) - 1; i >= 0; i-- {
				nextEdge := evaluation.SelectedEdges[i]
				if _, ok := nodeIndex[nextEdge.ToNodeID]; !ok {
					return fmt.Errorf("workflow entry path references unknown node: %s", nextEdge.ToNodeID)
				}
				routeSummary := workflowRouteOutcomeSummaryForEdge(evaluation, nextEdge)
				child := token.Child(nextEdge.ToNodeID, WorkflowTokenRouteLineage{
					SourceNodeID:   current,
					SelectedEdgeID: workflowEdgeSummaryID(nextEdge),
					SelectedNodeID: nextEdge.ToNodeID,
				})
				children = append(children, checkpointChild{token: child, summary: routeSummary})
				pushActiveWorkflowToken(runtimeCtx, child)
			}
			for _, child := range children {
				routeSummary := child.summary
				appendCheckpointWithSnapshot(runtimeCtx, "", child.token.NodeID, workflowCheckpointSnapshot{RouteSummary: &routeSummary, Token: child.token})
			}
		case WorkflowRouteOutcomeNoMatch:
			if result.RouteRequired {
				return fmt.Errorf("no matching route for node %s", current)
			}
		case WorkflowRouteOutcomeAmbiguousMatch, WorkflowRouteOutcomeEvaluationErr:
			return evaluation.Error
		}
		runtimeCtx.CompletedTokens = append(runtimeCtx.CompletedTokens, token)
	}
	consumeActiveCheckpoints(runtimeCtx, time.Now().UTC())
	clearWorkflowBranchBindings(runtimeCtx)
	return nil
}

func initializeWorkflowTokens(runtimeCtx *WorkflowExecutionContext, startNodeID string) {
	if runtimeCtx == nil || len(runtimeCtx.ActiveTokens) > 0 {
		return
	}
	if runtimeCtx.CurrentToken != nil {
		runtimeCtx.CurrentToken.NodeID = strings.TrimSpace(startNodeID)
		ensureWorkflowTokenBranch(runtimeCtx, runtimeCtx.CurrentToken)
		runtimeCtx.ActiveTokens = []*WorkflowToken{runtimeCtx.CurrentToken}
		if runtimeCtx.RootToken == nil && runtimeCtx.CurrentToken.ParentTokenID == "" {
			runtimeCtx.RootToken = runtimeCtx.CurrentToken
		}
		return
	}
	if runtimeCtx.RootToken == nil {
		runtimeCtx.RootToken = newWorkflowRootToken(startNodeID)
	}
	runtimeCtx.RootToken.NodeID = strings.TrimSpace(startNodeID)
	ensureWorkflowTokenBranch(runtimeCtx, runtimeCtx.RootToken)
	runtimeCtx.ActiveTokens = []*WorkflowToken{runtimeCtx.RootToken}
}

func ensureWorkflowTokenBranch(runtimeCtx *WorkflowExecutionContext, token *WorkflowToken) {
	if runtimeCtx == nil || token == nil || token.Branch != nil {
		return
	}
	token.Branch = newWorkflowBranchContext(workflowRuntimeSharedInput(runtimeCtx), runtimeCtx.Metadata)
}

func bindWorkflowTokenBranch(runtimeCtx *WorkflowExecutionContext, token *WorkflowToken) {
	if runtimeCtx == nil || token == nil {
		return
	}
	ensureWorkflowTokenBranch(runtimeCtx, token)
	runtimeCtx.Input = cloneWorkflowPayload(workflowRuntimeSharedInput(runtimeCtx))
	runtimeCtx.Variables = token.Branch.Variables
	runtimeCtx.Result = token.Branch.Result
	runtimeCtx.Metadata = cloneWorkflowPayload(workflowRuntimeSharedMetadata(runtimeCtx))
	runtimeCtx.Artifacts = token.Branch.Artifacts
	bindWorkflowPayloadBridge(runtimeCtx)
}

func storeWorkflowNodeResult(token *WorkflowToken, result WorkflowNodeResult) {
	if token == nil || token.Branch == nil {
		return
	}
	if len(result.Output) == 0 {
		return
	}
	token.Branch.Result = cloneWorkflowPayload(result.Output)
}

func rebindWorkflowNodeResult(runtimeCtx *WorkflowExecutionContext, token *WorkflowToken) {
	if runtimeCtx == nil || token == nil || token.Branch == nil {
		return
	}
	runtimeCtx.Result = token.Branch.Result
	bindWorkflowPayloadBridge(runtimeCtx)
}

func clearWorkflowBranchBindings(runtimeCtx *WorkflowExecutionContext) {
	if runtimeCtx == nil {
		return
	}
	runtimeCtx.Input = workflowRuntimeSharedInput(runtimeCtx)
	runtimeCtx.Variables = nil
	runtimeCtx.Result = nil
	runtimeCtx.Metadata = workflowRuntimeSharedMetadata(runtimeCtx)
	runtimeCtx.Artifacts = nil
	if runtimeCtx.Context != nil {
		runtimeCtx.Context.Payload = cloneWorkflowPayload(runtimeCtx.Input)
	}
}

func workflowRuntimeInput(ctx *domain.WorkflowContext) map[string]any {
	if ctx == nil {
		return map[string]any{}
	}
	if ctx.Payload == nil {
		ctx.Payload = map[string]any{}
	}
	return ctx.Payload
}

func workflowRuntimeSharedInput(runtimeCtx *WorkflowExecutionContext) map[string]any {
	if runtimeCtx == nil {
		return map[string]any{}
	}
	if runtimeCtx.sharedInput != nil {
		return runtimeCtx.sharedInput
	}
	if runtimeCtx.Context != nil {
		runtimeCtx.sharedInput = cloneWorkflowPayload(workflowRuntimeInput(runtimeCtx.Context))
		if runtimeCtx.Input == nil {
			runtimeCtx.Input = cloneWorkflowPayload(runtimeCtx.sharedInput)
		}
		return runtimeCtx.sharedInput
	}
	runtimeCtx.sharedInput = map[string]any{}
	if runtimeCtx.Input == nil {
		runtimeCtx.Input = map[string]any{}
	}
	return runtimeCtx.sharedInput
}

func workflowRuntimeSharedMetadata(runtimeCtx *WorkflowExecutionContext) map[string]any {
	if runtimeCtx == nil {
		return map[string]any{}
	}
	if runtimeCtx.sharedMetadata != nil {
		return runtimeCtx.sharedMetadata
	}
	if runtimeCtx.Metadata != nil {
		runtimeCtx.sharedMetadata = runtimeCtx.Metadata
		return runtimeCtx.sharedMetadata
	}
	runtimeCtx.sharedMetadata = map[string]any{}
	runtimeCtx.Metadata = runtimeCtx.sharedMetadata
	return runtimeCtx.sharedMetadata
}

func bindWorkflowPayloadBridge(runtimeCtx *WorkflowExecutionContext) {
	if runtimeCtx == nil {
		return
	}
	if runtimeCtx.Context == nil {
		runtimeCtx.Context = &domain.WorkflowContext{}
	}
	runtimeCtx.Context.Payload = workflowRuntimePayload(runtimeCtx)
}

func workflowRuntimePayload(runtimeCtx *WorkflowExecutionContext) map[string]any {
	payload := cloneWorkflowPayload(workflowRuntimeSharedInput(runtimeCtx))
	for key, value := range runtimeCtx.Variables {
		payload[key] = value
	}
	for key, value := range runtimeCtx.Result {
		payload[key] = value
	}
	for key, value := range runtimeCtx.Artifacts {
		payload[key] = value
	}
	return payload
}

func popNextActiveWorkflowToken(runtimeCtx *WorkflowExecutionContext) *WorkflowToken {
	if runtimeCtx == nil || len(runtimeCtx.ActiveTokens) == 0 {
		return nil
	}
	last := len(runtimeCtx.ActiveTokens) - 1
	token := runtimeCtx.ActiveTokens[last]
	runtimeCtx.ActiveTokens = runtimeCtx.ActiveTokens[:last]
	return token
}

func pushActiveWorkflowToken(runtimeCtx *WorkflowExecutionContext, token *WorkflowToken) {
	if runtimeCtx == nil || token == nil {
		return
	}
	runtimeCtx.ActiveTokens = append(runtimeCtx.ActiveTokens, token)
}

func workflowRouteOutcomeSummaryForEdge(evaluation WorkflowRouteEvaluation, selected domain.WorkflowEdge) WorkflowRouteOutcomeSummary {
	summary := workflowRouteOutcomeSummary(evaluation)
	selectedIndex := indexOfWorkflowSelectedEdge(evaluation.EvaluatedEdges, selected)
	summary.SelectedNodeID = strings.TrimSpace(selected.ToNodeID)
	summary.SelectedEdgeID = workflowEvaluatedEdgeSummaryID(selectedIndex, selected)
	return summary
}

func validateWorkflowRuntimeGraph(wf *domain.WorkflowDefinition) error {
	if wf == nil {
		return fmt.Errorf("workflow definition is required")
	}

	nodeIDs := make(map[string]struct{}, len(wf.Nodes))
	for _, node := range wf.Nodes {
		nodeIDs[node.ID] = struct{}{}
	}

	outgoing := make(map[string][]domain.WorkflowEdge, len(wf.Edges))
	for _, edge := range wf.Edges {
		outgoing[edge.FromNodeID] = append(outgoing[edge.FromNodeID], edge)
	}

	visited := make(map[string]struct{}, len(wf.Nodes))
	stack := make(map[string]struct{}, len(wf.Nodes))
	var walk func(string) error
	walk = func(nodeID string) error {
		if _, ok := nodeIDs[nodeID]; !ok {
			return fmt.Errorf("workflow entry path references unknown node: %s", nodeID)
		}
		if _, ok := stack[nodeID]; ok {
			return fmt.Errorf("unsupported cycle in workflow graph at node %s", nodeID)
		}
		if _, ok := visited[nodeID]; ok {
			return nil
		}
		visited[nodeID] = struct{}{}
		stack[nodeID] = struct{}{}

		for _, edge := range outgoing[nodeID] {
			if err := walk(edge.ToNodeID); err != nil {
				return err
			}
		}
		delete(stack, nodeID)
		return nil
	}

	if err := walk(wf.EntryNodeID); err != nil {
		return err
	}
	if len(visited) != len(nodeIDs) {
		return fmt.Errorf("unsupported disconnected workflow graph: only %d of %d nodes reachable from entry node %s", len(visited), len(nodeIDs), wf.EntryNodeID)
	}
	return nil
}

func outgoingEdges(edges []domain.WorkflowEdge, fromNodeID string) []domain.WorkflowEdge {
	result := make([]domain.WorkflowEdge, 0, len(edges))
	for _, edge := range edges {
		if edge.FromNodeID == fromNodeID {
			result = append(result, edge)
		}
	}
	return result
}

func indexWorkflowNodes(wf *domain.WorkflowDefinition) map[string]domain.WorkflowNode {
	result := make(map[string]domain.WorkflowNode, len(wf.Nodes))
	for _, node := range wf.Nodes {
		result[node.ID] = node
	}
	return result
}
