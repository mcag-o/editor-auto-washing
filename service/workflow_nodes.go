package service

import (
	"content-hub/domain"
	"context"
	"fmt"
)

type automationDispatchNode struct {
	root string
	svc  *AutomationService
}

type automationSnapshotNode struct {
	root string
	svc  *AutomationService
}

func NewAutomationDispatchNode(root string, svc *AutomationService) WorkflowNode {
	return &automationDispatchNode{root: root, svc: svc}
}

func NewAutomationSnapshotNode(root string, svc *AutomationService) WorkflowNode {
	return &automationSnapshotNode{root: root, svc: svc}
}

func BuildDefaultWorkflowEngine(root string, automationSvc *AutomationService) *WorkflowEngine {
	engine := NewWorkflowEngine()
	if automationSvc != nil {
		engine.Register("automation_dispatch", NewAutomationDispatchNode(root, automationSvc))
		engine.Register("automation_snapshot", NewAutomationSnapshotNode(root, automationSvc))
	}
	return engine
}

func (n *automationDispatchNode) Name() string { return "automation_dispatch" }

func (n *automationDispatchNode) Execute(ctx context.Context, wc *domain.WorkflowContext) error {
	command := workflowCommand(wc)
	if command == "" {
		return fmt.Errorf("automation workflow command is required")
	}
	var (
		result *domain.AutomationRunResult
		err    error
	)
	switch command {
	case "run-once":
		result, err = n.svc.RunOnce(ctx, n.root)
	case "retry-failed":
		result, err = n.svc.RetryFailed(ctx, n.root)
	case "daemon":
		result, err = n.svc.StartDaemon(ctx, n.root, 0)
	default:
		return fmt.Errorf("unsupported automation workflow command: %s", command)
	}
	if err != nil {
		return err
	}
	if wc.Payload == nil {
		wc.Payload = map[string]any{}
	}
	wc.Payload["automation_result"] = result
	return nil
}

func (n *automationSnapshotNode) Name() string { return "automation_snapshot" }

func (n *automationSnapshotNode) Execute(ctx context.Context, wc *domain.WorkflowContext) error {
	status, err := n.svc.Status(ctx, n.root)
	if err != nil {
		return err
	}
	if wc.Payload == nil {
		wc.Payload = map[string]any{}
	}
	wc.Payload["automation_status"] = status
	return nil
}

func workflowCommand(wc *domain.WorkflowContext) string {
	if wc == nil {
		return ""
	}
	if wc.Command != "" {
		return wc.Command
	}
	if wc.Payload == nil {
		return ""
	}
	if command, ok := wc.Payload["automation_command"].(string); ok {
		return command
	}
	return ""
}
