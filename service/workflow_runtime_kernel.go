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
	current := startNodeID
	for current != "" {
		runtimeCtx.CurrentNodeID = current
		node, ok := k.nodes[current]
		if !ok {
			return fmt.Errorf("workflow node not found: %s", current)
		}
		if err := node.Execute(ctx, runtimeCtx.Context); err != nil {
			return err
		}

		nextEdges := outgoingEdges(runtimeCtx.Workflow.Edges, current)
		nextEdge, err := k.router.SelectNextEdge(runtimeCtx, current, WorkflowNodeResult{RouteRequired: routeRequired(nextEdges)}, nextEdges)
		if err != nil {
			return err
		}
		if nextEdge == nil {
			return nil
		}
		if _, ok := nodeIndex[nextEdge.ToNodeID]; !ok {
			return fmt.Errorf("workflow entry path references unknown node: %s", nextEdge.ToNodeID)
		}
		appendCheckpoint(runtimeCtx, "", nextEdge.ToNodeID)
		consumeActiveCheckpoints(runtimeCtx, time.Now().UTC())
		current = nextEdge.ToNodeID
	}
	return nil
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

		edges := outgoing[nodeID]
		if hasAmbiguousRoutes(edges) {
			return fmt.Errorf("unsupported branching in workflow graph at node %s", nodeID)
		}
		for _, edge := range edges {
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

func hasAmbiguousRoutes(edges []domain.WorkflowEdge) bool {
	if len(edges) <= 1 {
		return false
	}
	fallbackCount := 0
	lowestFallbackPriority := 0
	hasFallback := false
	for _, edge := range edges {
		condition := strings.TrimSpace(edge.Condition)
		if condition != "" && condition != "always" {
			continue
		}
		fallbackCount++
		if !hasFallback || edge.Priority > lowestFallbackPriority {
			lowestFallbackPriority = edge.Priority
			hasFallback = true
		}
	}
	if fallbackCount > 1 {
		return true
	}
	if fallbackCount == 0 {
		return false
	}
	for _, edge := range edges {
		condition := strings.TrimSpace(edge.Condition)
		if condition == "" || condition == "always" {
			continue
		}
		if edge.Priority >= lowestFallbackPriority {
			return true
		}
	}
	return false
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
