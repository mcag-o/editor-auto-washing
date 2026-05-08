package service

import "testing"

func TestWorkflowRouteOutcomeUsesCanonicalRouteEvaluationValues(t *testing.T) {
	var _ WorkflowRouteOutcome = WorkflowRouteOutcomeSingleMatch
	var _ WorkflowRouteOutcome = WorkflowRouteOutcomeMultiMatch
	var _ WorkflowRouteOutcome = WorkflowRouteOutcomeFallbackMatch
	var _ WorkflowRouteOutcome = WorkflowRouteOutcomeNoMatch
	var _ WorkflowRouteOutcome = WorkflowRouteOutcomeAmbiguousMatch
	var _ WorkflowRouteOutcome = WorkflowRouteOutcomeEvaluationErr
}
