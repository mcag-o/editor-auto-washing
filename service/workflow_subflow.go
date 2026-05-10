package service

import "strings"

type workflowSubflowState string

const (
	workflowSubflowStateRunning workflowSubflowState = "running"
	workflowSubflowStateFailed  workflowSubflowState = "failed"
	workflowSubflowStateDone    workflowSubflowState = "done"
)

type workflowSubflowFailureStrategy string

const (
	workflowSubflowFailureStrategyFailParent     workflowSubflowFailureStrategy = "fail_parent"
	workflowSubflowFailureStrategyPauseParent    workflowSubflowFailureStrategy = "pause_parent"
	workflowSubflowFailureStrategyContinueParent workflowSubflowFailureStrategy = "continue_parent"
)

type workflowSubflowFrame struct {
	ParentTokenID    string
	ParentNodeID     string
	ChildWorkflowID  string
	EntryNodeID      string
	ReturnNodeID     string
	ReturnMapping    map[string]string
	ParentBranch     *WorkflowBranchContext
	State            workflowSubflowState
	FailureStrategy  workflowSubflowFailureStrategy
}

func applyWorkflowSubflowReturnMapping(parent *WorkflowToken, child *WorkflowToken, mapping map[string]string) {
	if parent == nil || parent.Branch == nil || child == nil || child.Branch == nil || len(mapping) == 0 {
		return
	}
	for source, target := range mapping {
		source = strings.TrimSpace(source)
		target = strings.TrimSpace(target)
		if source == "" || target == "" {
			continue
		}
		if value, ok := child.Branch.Variables[source]; ok {
			parent.Branch.Variables[target] = cloneWorkflowValue(value)
		}
		if value, ok := child.Branch.Result[source]; ok {
			parent.Branch.Result[target] = cloneWorkflowValue(value)
		}
	}
}
