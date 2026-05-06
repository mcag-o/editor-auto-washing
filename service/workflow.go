package service

import (
	"content-hub/domain"
	"context"
	"fmt"
)

type WorkflowNode interface {
	Name() string
	Execute(ctx context.Context, wc *domain.WorkflowContext) error
}

type WorkflowEngine struct {
	nodes map[string]WorkflowNode
}

func NewWorkflowEngine() *WorkflowEngine {
	return &WorkflowEngine{nodes: make(map[string]WorkflowNode)}
}

func (e *WorkflowEngine) Register(name string, node WorkflowNode) {
	e.nodes[name] = node
}

func (e *WorkflowEngine) RegisteredNames() []string {
	result := make([]string, 0, len(e.nodes))
	for name := range e.nodes {
		result = append(result, name)
	}
	return result
}

func (e *WorkflowEngine) Execute(ctx context.Context, wf *domain.WorkflowDefinition, wc *domain.WorkflowContext) error {
	path, err := linearExecutionPath(wf)
	if err != nil {
		return err
	}

	for _, nodeID := range path {
		node, ok := e.nodes[nodeID]
		if !ok {
			return fmt.Errorf("workflow node not found: %s", nodeID)
		}
		if err := node.Execute(ctx, wc); err != nil {
			return err
		}
	}
	return nil
}

func linearExecutionPath(wf *domain.WorkflowDefinition) ([]string, error) {
	if wf == nil {
		return nil, fmt.Errorf("workflow definition is required")
	}

	nodeIDs := make(map[string]struct{}, len(wf.Nodes))
	for _, node := range wf.Nodes {
		nodeIDs[node.ID] = struct{}{}
	}

	outgoing := make(map[string][]string, len(wf.Edges))
	for _, edge := range wf.Edges {
		outgoing[edge.FromNodeID] = append(outgoing[edge.FromNodeID], edge.ToNodeID)
	}

	visited := make(map[string]struct{}, len(wf.Nodes))
	path := make([]string, 0, len(wf.Nodes))
	current := wf.EntryNodeID
	for current != "" {
		if _, seen := visited[current]; seen {
			return nil, fmt.Errorf("unsupported cycle in workflow graph at node %s", current)
		}
		if _, ok := nodeIDs[current]; !ok {
			return nil, fmt.Errorf("workflow entry path references unknown node: %s", current)
		}

		visited[current] = struct{}{}
		path = append(path, current)

		next := outgoing[current]
		if len(next) > 1 {
			return nil, fmt.Errorf("unsupported branching in workflow graph at node %s", current)
		}
		if len(next) == 0 {
			current = ""
			continue
		}
		current = next[0]
	}

	if len(visited) != len(nodeIDs) {
		return nil, fmt.Errorf("unsupported disconnected workflow graph: only %d of %d nodes reachable from entry node %s", len(visited), len(nodeIDs), wf.EntryNodeID)
	}

	return path, nil
}
