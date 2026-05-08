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
	kernel := newWorkflowRuntimeKernel(e.nodes)
	return kernel.Execute(ctx, wf, wc)
}

func linearExecutionPath(wf *domain.WorkflowDefinition) ([]string, error) {
	if err := validateWorkflowRuntimeGraph(wf); err != nil {
		return nil, err
	}
	path := make([]string, 0, len(wf.Nodes))
	current := wf.EntryNodeID
	for current != "" {
		path = append(path, current)
		edges := outgoingEdges(wf.Edges, current)
		if len(edges) == 0 {
			break
		}
		if len(edges) == 1 {
			if !isWorkflowFallbackRoute(edges[0]) {
				return nil, fmt.Errorf("context-dependent route selection at node %s", current)
			}
			current = edges[0].ToNodeID
			continue
		}
		return nil, fmt.Errorf("context-dependent route selection at node %s", current)
	}
	return path, nil
}
