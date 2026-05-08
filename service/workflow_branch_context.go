package service

type WorkflowBranchContext struct {
	Variables map[string]any
	Result    map[string]any
	Artifacts map[string]any
}

func newWorkflowBranchContext(input map[string]any, metadata map[string]any) *WorkflowBranchContext {
	return &WorkflowBranchContext{
		Variables: map[string]any{},
		Result:    map[string]any{},
		Artifacts: map[string]any{},
	}
}

func workflowBranchInput(branch *WorkflowBranchContext) map[string]any {
	_ = branch
	return map[string]any{}
}

func workflowBranchMetadata(branch *WorkflowBranchContext) map[string]any {
	_ = branch
	return map[string]any{}
}

func cloneWorkflowBranchContext(branch *WorkflowBranchContext) *WorkflowBranchContext {
	if branch == nil {
		return &WorkflowBranchContext{
			Variables: map[string]any{},
			Result:    map[string]any{},
			Artifacts: map[string]any{},
		}
	}
	return &WorkflowBranchContext{
		Variables: cloneWorkflowPayload(branch.Variables),
		Result:    cloneWorkflowPayload(branch.Result),
		Artifacts: cloneWorkflowPayload(branch.Artifacts),
	}
}
