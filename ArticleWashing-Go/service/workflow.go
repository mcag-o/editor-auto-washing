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
	for _, nodeName := range wf.Nodes {
		node, ok := e.nodes[nodeName]
		if !ok {
			return fmt.Errorf("workflow node not found: %s", nodeName)
		}
		if err := node.Execute(ctx, wc); err != nil {
			return err
		}
	}
	return nil
}
