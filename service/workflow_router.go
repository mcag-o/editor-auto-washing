package service

import (
	"content-hub/domain"
	"fmt"
	"strings"
)

type workflowRouter struct{}

func newWorkflowRouter() workflowRouter {
	return workflowRouter{}
}

func (workflowRouter) SelectNextEdge(ctx *WorkflowExecutionContext, nodeID string, result WorkflowNodeResult, edges []domain.WorkflowEdge) (*domain.WorkflowEdge, error) {
	evaluation := newWorkflowRouter().EvaluateRoutes(ctx, nodeID, result, edges)
	summary := workflowRouteOutcomeSummary(evaluation)
	recordLatestRouteOutcome(ctx, summary)
	if ctx != nil {
		if ctx.Metadata == nil {
			ctx.Metadata = map[string]any{}
		}
		ctx.Metadata[workflowLatestRouteMetadataKey] = workflowRouteSummaryMetadata(summary)
	}
	switch evaluation.Outcome {
	case WorkflowRouteOutcomeSingleMatch, WorkflowRouteOutcomeMultiMatch, WorkflowRouteOutcomeFallbackMatch:
		selected, ok := firstSelectedWorkflowEdge(evaluation)
		if !ok {
			return nil, fmt.Errorf("route outcome %s for node %s produced no selected edge", evaluation.Outcome, nodeID)
		}
		return &selected, nil
	case WorkflowRouteOutcomeNoMatch:
		if result.RouteRequired {
			return nil, fmt.Errorf("no matching route for node %s", nodeID)
		}
		return nil, nil
	case WorkflowRouteOutcomeAmbiguousMatch, WorkflowRouteOutcomeEvaluationErr:
		return nil, evaluation.Error
	}
	return nil, nil
}

func workflowRouteOutcomeSummary(evaluation WorkflowRouteEvaluation) WorkflowRouteOutcomeSummary {
	summary := WorkflowRouteOutcomeSummary{
		NodeID:          strings.TrimSpace(evaluation.SourceNodeID),
		Outcome:         evaluation.Outcome,
		EvaluationTrace: make([]string, 0, len(evaluation.EvaluatedEdges)),
	}
	if selected, ok := firstSelectedWorkflowEdge(evaluation); ok {
		selectedIndex := indexOfWorkflowSelectedEdge(evaluation.EvaluatedEdges, selected)
		summary.SelectedNodeID = strings.TrimSpace(selected.ToNodeID)
		summary.SelectedEdgeID = workflowEvaluatedEdgeSummaryID(selectedIndex, selected)
	}
	for i, edge := range evaluation.EvaluatedEdges {
		summary.EvaluationTrace = append(summary.EvaluationTrace, workflowEvaluatedEdgeSummaryID(i+1, edge.Edge)+"="+string(edge.Result))
	}
	return summary
}

func firstSelectedWorkflowEdge(evaluation WorkflowRouteEvaluation) (domain.WorkflowEdge, bool) {
	if len(evaluation.SelectedEdges) == 0 {
		return domain.WorkflowEdge{}, false
	}
	return evaluation.SelectedEdges[0], true
}

func workflowEvaluatedEdgeSummaryID(index int, edge domain.WorkflowEdge) string {
	return fmt.Sprintf("%d:%s", index, workflowEdgeSummaryID(edge))
}

func workflowEdgeSummaryID(edge domain.WorkflowEdge) string {
	fromNodeID := strings.TrimSpace(edge.FromNodeID)
	toNodeID := strings.TrimSpace(edge.ToNodeID)
	condition := strings.Join(strings.Fields(edge.Condition), " ")
	if fromNodeID == "" && toNodeID == "" {
		return fmt.Sprintf("edge@%d[%s]", edge.Priority, condition)
	}
	return fmt.Sprintf("%s->%s@%d[%s]", fromNodeID, toNodeID, edge.Priority, condition)
}

func indexOfWorkflowSelectedEdge(evaluatedEdges []WorkflowRouteEdgeEvaluation, selected domain.WorkflowEdge) int {
	for i, edge := range evaluatedEdges {
		if edge.Edge == selected {
			return i + 1
		}
	}
	return 0
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
