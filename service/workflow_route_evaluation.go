package service

import (
	"content-hub/domain"
	"fmt"
	"sort"
	"strings"
)

type WorkflowRouteConditionResult string

const (
	WorkflowRouteConditionResultMatch            WorkflowRouteConditionResult = "match"
	WorkflowRouteConditionResultMiss             WorkflowRouteConditionResult = "miss"
	WorkflowRouteConditionResultFallbackMatch    WorkflowRouteConditionResult = "fallback_match"
	WorkflowRouteConditionResultFallbackDeferred WorkflowRouteConditionResult = "fallback_deferred"
	WorkflowRouteConditionResultEvaluationError  WorkflowRouteConditionResult = "evaluation_error"
)

const (
	WorkflowRouteOutcomeSingleMatch    WorkflowRouteOutcome = "single_match"
	WorkflowRouteOutcomeMultiMatch     WorkflowRouteOutcome = "multi_match"
	WorkflowRouteOutcomeFallbackMatch  WorkflowRouteOutcome = "fallback_match"
	WorkflowRouteOutcomeNoMatch        WorkflowRouteOutcome = "no_match"
	WorkflowRouteOutcomeAmbiguousMatch WorkflowRouteOutcome = "ambiguous_match"
	WorkflowRouteOutcomeEvaluationErr  WorkflowRouteOutcome = "evaluation_error"
)

type WorkflowRouteEdgeEvaluation struct {
	Edge    domain.WorkflowEdge
	Matched bool
	Result  WorkflowRouteConditionResult
	Error   error
}

type WorkflowRouteEvaluation struct {
	SourceNodeID   string
	EvaluatedEdges []WorkflowRouteEdgeEvaluation
	SelectedEdges  []domain.WorkflowEdge
	Outcome        WorkflowRouteOutcome
	Error          error
}

func (workflowRouter) EvaluateRoutes(ctx *WorkflowExecutionContext, nodeID string, result WorkflowNodeResult, edges []domain.WorkflowEdge) WorkflowRouteEvaluation {
	evaluator := newWorkflowConditionEvaluator()
	ordered := append([]domain.WorkflowEdge(nil), edges...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Priority < ordered[j].Priority
	})

	evaluation := WorkflowRouteEvaluation{SourceNodeID: nodeID}
	var explicitMatches []domain.WorkflowEdge
	var fallbackMatches []domain.WorkflowEdge
	bestExplicitPriority := 0
	bestFallbackPriority := 0
	hasExplicitPriority := false
	hasFallbackPriority := false

	for i := range ordered {
		edge := ordered[i]
		matched, err := evaluator.Evaluate(ctx, result, edge.Condition)
		if err != nil {
			evaluation.EvaluatedEdges = append(evaluation.EvaluatedEdges, WorkflowRouteEdgeEvaluation{
				Edge:    edge,
				Matched: false,
				Result:  WorkflowRouteConditionResultEvaluationError,
				Error:   err,
			})
			evaluation.Outcome = WorkflowRouteOutcomeEvaluationErr
			evaluation.Error = fmt.Errorf("evaluate route for node %s -> %s: %w", nodeID, edge.ToNodeID, err)
			return evaluation
		}

		isFallback := isWorkflowFallbackRoute(edge)
		edgeEvaluation := WorkflowRouteEdgeEvaluation{Edge: edge, Matched: matched}
		switch {
		case !matched:
			edgeEvaluation.Result = WorkflowRouteConditionResultMiss
		case isFallback:
			if !hasFallbackPriority || edge.Priority < bestFallbackPriority {
				bestFallbackPriority = edge.Priority
				hasFallbackPriority = true
				fallbackMatches = []domain.WorkflowEdge{edge}
			} else if edge.Priority == bestFallbackPriority {
				fallbackMatches = append(fallbackMatches, edge)
			}
			edgeEvaluation.Result = WorkflowRouteConditionResultFallbackDeferred
		default:
			if !hasExplicitPriority || edge.Priority < bestExplicitPriority {
				bestExplicitPriority = edge.Priority
				hasExplicitPriority = true
				explicitMatches = []domain.WorkflowEdge{edge}
			} else if edge.Priority == bestExplicitPriority {
				explicitMatches = append(explicitMatches, edge)
			}
			edgeEvaluation.Result = WorkflowRouteConditionResultMatch
		}
		evaluation.EvaluatedEdges = append(evaluation.EvaluatedEdges, edgeEvaluation)
	}

	if len(explicitMatches) > 0 && (!hasFallbackPriority || bestExplicitPriority <= bestFallbackPriority) {
		evaluation.SelectedEdges = append(evaluation.SelectedEdges, explicitMatches...)
		evaluation.Outcome = WorkflowRouteOutcomeSingleMatch
		if len(explicitMatches) > 1 {
			evaluation.Outcome = WorkflowRouteOutcomeMultiMatch
		}
		return evaluation
	}

	if len(fallbackMatches) > 0 && (!hasExplicitPriority || bestFallbackPriority < bestExplicitPriority) {
		evaluation.SelectedEdges = append(evaluation.SelectedEdges, fallbackMatches...)
		for i := range evaluation.EvaluatedEdges {
			if evaluation.EvaluatedEdges[i].Matched && isWorkflowFallbackRoute(evaluation.EvaluatedEdges[i].Edge) && evaluation.EvaluatedEdges[i].Edge.Priority == bestFallbackPriority {
				evaluation.EvaluatedEdges[i].Result = WorkflowRouteConditionResultFallbackMatch
			}
		}
		evaluation.Outcome = WorkflowRouteOutcomeFallbackMatch
		return evaluation
	}

	evaluation.Outcome = WorkflowRouteOutcomeNoMatch
	if result.RouteRequired {
		evaluation.Error = fmt.Errorf("no matching route for node %s", nodeID)
	}
	return evaluation
}

func isWorkflowFallbackRoute(edge domain.WorkflowEdge) bool {
	condition := strings.TrimSpace(edge.Condition)
	return condition == "" || condition == "always"
}
