package service

import (
	"content-hub/domain"
	"fmt"
	"sort"
	"strings"
)

type workflowRouter struct{}

func newWorkflowRouter() workflowRouter {
	return workflowRouter{}
}

func (workflowRouter) SelectNextEdge(ctx *WorkflowExecutionContext, nodeID string, result WorkflowNodeResult, edges []domain.WorkflowEdge) (*domain.WorkflowEdge, error) {
	ordered := append([]domain.WorkflowEdge(nil), edges...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Priority < ordered[j].Priority
	})

	for i := range ordered {
		matched, err := routeConditionMatches(ctx, ordered[i].Condition)
		if err != nil {
			return nil, err
		}
		if matched {
			return &ordered[i], nil
		}
	}

	if result.RouteRequired {
		return nil, fmt.Errorf("no matching route for node %s", nodeID)
	}
	return nil, nil
}

func routeConditionMatches(ctx *WorkflowExecutionContext, condition string) (bool, error) {
	trimmed := strings.TrimSpace(condition)
	if trimmed == "" || trimmed == "always" {
		return true, nil
	}
	left, right, ok := strings.Cut(trimmed, "==")
	if !ok {
		return false, fmt.Errorf("unsupported workflow route condition: %s", condition)
	}
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if !strings.HasPrefix(left, "payload.") {
		return false, fmt.Errorf("unsupported workflow route condition: %s", condition)
	}
	key := strings.TrimPrefix(left, "payload.")
	if ctx == nil || ctx.Context == nil || ctx.Context.Payload == nil {
		return false, nil
	}
	value, ok := ctx.Context.Payload[key]
	if !ok {
		return false, nil
	}
	return fmt.Sprint(value) == right, nil
}

func routeRequired(edges []domain.WorkflowEdge) bool {
	for _, edge := range edges {
		condition := strings.TrimSpace(edge.Condition)
		if condition != "" && condition != "always" {
			return true
		}
	}
	return false
}
