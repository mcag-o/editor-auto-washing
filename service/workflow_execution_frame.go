package service

type WorkflowExecutionFrame struct {
	Input     map[string]any
	Metadata  map[string]any
}

func newWorkflowTokenExecutionFrame(runtimeCtx *WorkflowExecutionContext, token *WorkflowToken) *WorkflowExecutionFrame {
	if runtimeCtx == nil || token == nil {
		return nil
	}
	ensureWorkflowTokenBranch(runtimeCtx, token)
	frame := cloneWorkflowExecutionFrame(token.Frame)
	if frame == nil {
		frame = &WorkflowExecutionFrame{}
	}
	if frame.Input == nil {
		frame.Input = cloneWorkflowPayload(workflowRuntimeSharedInput(runtimeCtx))
	}
	if frame.Metadata == nil {
		frame.Metadata = cloneWorkflowPayload(workflowRuntimeSharedMetadata(runtimeCtx))
	}
	if frame.Input == nil {
		frame.Input = map[string]any{}
	}
	if frame.Metadata == nil {
		frame.Metadata = map[string]any{}
	}
	return frame
}

func cloneWorkflowExecutionFrame(frame *WorkflowExecutionFrame) *WorkflowExecutionFrame {
	if frame == nil {
		return nil
	}
	clone := &WorkflowExecutionFrame{
		Input:    cloneWorkflowPayload(frame.Input),
		Metadata: cloneWorkflowPayload(frame.Metadata),
	}
	if clone.Input == nil {
		clone.Input = map[string]any{}
	}
	if clone.Metadata == nil {
		clone.Metadata = map[string]any{}
	}
	return clone
}

func bindWorkflowTokenExecutionFrame(runtimeCtx *WorkflowExecutionContext, token *WorkflowToken) *WorkflowExecutionFrame {
	frame := newWorkflowTokenExecutionFrame(runtimeCtx, token)
	if runtimeCtx == nil {
		return frame
	}
	runtimeCtx.CurrentToken = token
	runtimeCtx.CurrentFrame = frame
	token.Frame = frame
	if frame == nil {
		runtimeCtx.Input = nil
		runtimeCtx.Variables = nil
		runtimeCtx.Result = nil
		runtimeCtx.Metadata = nil
		runtimeCtx.Artifacts = nil
		return nil
	}
	runtimeCtx.Input = frame.Input
	runtimeCtx.Metadata = frame.Metadata
	runtimeCtx.Variables = token.Branch.Variables
	runtimeCtx.Result = token.Branch.Result
	runtimeCtx.Artifacts = token.Branch.Artifacts
	bindWorkflowPayloadBridge(runtimeCtx)
	return frame
}

func clearWorkflowTokenExecutionFrame(runtimeCtx *WorkflowExecutionContext) {
	if runtimeCtx == nil {
		return
	}
	finalPayload := workflowRuntimePayload(runtimeCtx)
	runtimeCtx.CurrentFrame = nil
	runtimeCtx.Input = workflowRuntimeSharedInput(runtimeCtx)
	runtimeCtx.Variables = nil
	runtimeCtx.Result = nil
	runtimeCtx.Metadata = workflowRuntimeSharedMetadata(runtimeCtx)
	runtimeCtx.Artifacts = nil
	if runtimeCtx.Context != nil {
		runtimeCtx.Context.Payload = finalPayload
	}
}
