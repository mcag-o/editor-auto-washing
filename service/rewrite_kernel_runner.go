package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"errors"
	"fmt"
	"strings"
)

const rewriteWorkflowCheckpointMetadataKey = "rewrite_workflow_checkpoint"

type RewriteKernelRunner struct {
	compiler      *RewriteWorkflowCompiler
	runs          repo.RewritePipelineRunRepo
	stageRuns     repo.RewriteStageRunRepo
	materialize   *DraftMaterializer
	stageExecutor *RewriteStageExecutor
}

type rewriteWorkflowCheckpointState struct {
	NodeID  string         `json:"node_id"`
	Payload map[string]any `json:"payload"`
}

type rewriteKernelStageFailure struct {
	Stage     domain.RewriteStageDefinition
	InputVars map[string]any
	Cause     error
}

func (e *rewriteKernelStageFailure) Error() string {
	if e == nil || e.Cause == nil {
		return ""
	}
	return e.Cause.Error()
}

func (e *rewriteKernelStageFailure) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

type rewriteKernelWorkflowNode struct {
	run       *domain.RewritePipelineRun
	runs      repo.RewritePipelineRunRepo
	stageRuns repo.RewriteStageRunRepo
	delegate  RewriteWorkflowNodeExecutor
	def       domain.WorkflowNode
}

func NewRewriteKernelRunner(runs repo.RewritePipelineRunRepo, stageRuns repo.RewriteStageRunRepo, stageExecutor *RewriteStageExecutor, materialize *DraftMaterializer) *RewriteKernelRunner {
	return &RewriteKernelRunner{
		compiler:      NewRewriteWorkflowCompiler(),
		runs:          runs,
		stageRuns:     stageRuns,
		materialize:   materialize,
		stageExecutor: stageExecutor,
	}
}

func (r *RewriteKernelRunner) Run(ctx context.Context, run *domain.RewritePipelineRun, profile *domain.RewritePipelineProfile, workspaceArticleID string, title string) (string, error) {
	return r.execute(ctx, run, profile, workspaceArticleID, title, false)
}

func (r *RewriteKernelRunner) Resume(ctx context.Context, run *domain.RewritePipelineRun, profile *domain.RewritePipelineProfile, workspaceArticleID string, title string) (string, error) {
	return r.execute(ctx, run, profile, workspaceArticleID, title, true)
}

func (r *RewriteKernelRunner) execute(ctx context.Context, run *domain.RewritePipelineRun, profile *domain.RewritePipelineProfile, workspaceArticleID string, title string, resume bool) (string, error) {
	if r == nil || r.compiler == nil || r.runs == nil || r.stageRuns == nil || r.materialize == nil || r.stageExecutor == nil {
		return "", domain.NewInternalErr("rewrite kernel runner is not configured", nil)
	}
	if run == nil {
		return "", domain.NewValidationErr("rewrite run is required", nil)
	}
	workflow, err := r.compiler.Compile(profile)
	if err != nil {
		return "", normalizeRewriteKernelRunError(err)
	}
	payload, checkpoints, err := buildRewriteWorkflowExecutionState(run.Metadata, title, run.ID, resume)
	if err != nil {
		return "", err
	}
	nodes, err := r.buildNodes(run, workflow)
	if err != nil {
		return "", err
	}
	kernel := newWorkflowRuntimeKernel(nodes)
	runtimeCtx := &WorkflowExecutionContext{
		Workflow:    workflow,
		Context:     &domain.WorkflowContext{Payload: payload},
		Checkpoints: checkpoints,
	}
	if resume {
		err = kernel.Resume(ctx, runtimeCtx)
	} else {
		err = kernel.executeFrom(ctx, runtimeCtx, workflow.EntryNodeID)
	}
	if err != nil {
		return "", err
	}
	stageOutput, ok := runtimeCtx.Context.Payload["rewrite_stage_output"].(map[string]any)
	if !ok || len(stageOutput) == 0 {
		return "", domain.NewInternalErr("rewrite run has no output to materialize", nil)
	}
	draft, err := r.materialize.Materialize(ctx, workspaceArticleID, stageOutput)
	if err != nil {
		return "", err
	}
	clearRewriteWorkflowCheckpoint(run.Metadata)
	return draft.ID, nil
}

func (r *RewriteKernelRunner) buildNodes(run *domain.RewritePipelineRun, workflow *domain.WorkflowDefinition) (map[string]WorkflowNode, error) {
	delegates := map[string]RewriteWorkflowNodeExecutor{
		rewriteWorkflowNodeTypeGenerate:    NewRewriteGenerateNodeExecutor(r.stageExecutor),
		rewriteWorkflowNodeTypeReview:      NewRewriteReviewNodeExecutor(r.stageExecutor),
		rewriteWorkflowNodeTypeRepair:      NewRewriteRepairNodeExecutor(r.stageExecutor),
		rewriteWorkflowNodeTypeMaterialize: NewRewriteMaterializeNodeExecutor(),
	}
	nodes := make(map[string]WorkflowNode, len(workflow.Nodes))
	for _, node := range workflow.Nodes {
		delegate, ok := delegates[node.Type]
		if !ok {
			return nil, fmt.Errorf("unsupported rewrite workflow node type: %s", node.Type)
		}
		nodes[node.ID] = &rewriteKernelWorkflowNode{
			run:       run,
			runs:      r.runs,
			stageRuns: r.stageRuns,
			delegate:  delegate,
			def:       node,
		}
	}
	return nodes, nil
}

func (n *rewriteKernelWorkflowNode) Name() string {
	return n.def.ID
}

func (n *rewriteKernelWorkflowNode) Execute(ctx context.Context, wc *domain.WorkflowContext) error {
	if n == nil || n.run == nil || n.runs == nil || n.delegate == nil {
		return domain.NewInternalErr("rewrite kernel workflow node is not configured", nil)
	}
	if wc == nil {
		return domain.NewValidationErr("workflow context is required", nil)
	}
	config, err := normalizeRewriteWorkflowNodeConfig(n.def, wc.Payload)
	if err != nil {
		return err
	}
	execNode, err := marshalRewriteWorkflowNodeConfig(n.def, config)
	if err != nil {
		return err
	}
	inputVars := cloneWorkflowPayload(wc.Payload)
	inputVars = applyRewriteWorkflowInputOverride(config, inputVars)
	currentStage := strings.TrimSpace(config.Stage.Name)
	if currentStage == "" {
		currentStage = n.def.ID
	}
	n.run.CurrentStage = currentStage
	storeRewriteWorkflowCheckpoint(n.run.Metadata, execNode.ID, inputVars)
	if err := n.runs.Update(ctx, n.run); err != nil {
		return wrapRewriteKernelNodeError(execNode, inputVars, err)
	}
	_, execErr := n.delegate.Execute(ctx, &WorkflowExecutionContext{Context: wc}, execNode)
	var outcomeErr *rewriteStageOutcomeError
	if execErr != nil && !errors.As(execErr, &outcomeErr) {
		return wrapRewriteKernelNodeError(execNode, inputVars, execErr)
	}
	if execNode.Type == rewriteWorkflowNodeTypeMaterialize {
		if execErr != nil {
			return wrapRewriteKernelNodeError(execNode, inputVars, execErr)
		}
		return nil
	}
	stageRun, err := buildRewriteStageRunFromWorkflowNode(n.run.ID, execNode, inputVars, wc.Payload)
	if err != nil {
		return wrapRewriteKernelNodeError(execNode, inputVars, err)
	}
	if err := n.stageRuns.Create(ctx, stageRun); err != nil {
		return wrapRewriteKernelNodeError(execNode, inputVars, err)
	}
	if execErr != nil {
		return wrapRewriteKernelNodeError(execNode, inputVars, execErr)
	}
	if err := validateRewriteWorkflowNodeOutcome(execNode, wc.Payload); err != nil {
		return wrapRewriteKernelNodeError(execNode, inputVars, err)
	}
	return nil
}

func buildRewriteWorkflowExecutionState(metadata map[string]any, title string, workflowRunID string, resume bool) (map[string]any, []domain.WorkflowCheckpoint, error) {
	payload := rewriteWorkflowPayloadFromMetadata(metadata, title)
	if !resume {
		return payload, nil, nil
	}
	checkpoint, err := loadRewriteWorkflowCheckpoint(metadata)
	if err != nil {
		return nil, nil, err
	}
	if checkpoint == nil || strings.TrimSpace(checkpoint.NodeID) == "" {
		return nil, nil, domain.NewValidationErr("rewrite run does not contain a resumable checkpoint", nil)
	}
	for key, value := range checkpoint.Payload {
		payload[key] = value
	}
	if strings.TrimSpace(title) != "" {
		payload["title"] = strings.TrimSpace(title)
	}
	return payload, []domain.WorkflowCheckpoint{{
		WorkflowRunID: workflowRunID,
		NodeID:        strings.TrimSpace(checkpoint.NodeID),
		State:         domain.WorkflowCheckpointStateActive,
		Resumable:     true,
	}}, nil
}

func rewriteWorkflowPayloadFromMetadata(metadata map[string]any, title string) map[string]any {
	payload := map[string]any{}
	for key, value := range metadata {
		if key == rewriteWorkflowCheckpointMetadataKey {
			continue
		}
		payload[key] = value
	}
	if strings.TrimSpace(title) != "" {
		payload["title"] = strings.TrimSpace(title)
	}
	return payload
}

func loadRewriteWorkflowCheckpoint(metadata map[string]any) (*rewriteWorkflowCheckpointState, error) {
	if len(metadata) == 0 {
		return nil, nil
	}
	raw, ok := metadata[rewriteWorkflowCheckpointMetadataKey]
	if !ok || raw == nil {
		return nil, nil
	}
	if checkpoint, ok := raw.(rewriteWorkflowCheckpointState); ok {
		checkpoint.Payload = cloneWorkflowPayload(checkpoint.Payload)
		return &checkpoint, nil
	}
	if checkpoint, ok := raw.(*rewriteWorkflowCheckpointState); ok {
		return &rewriteWorkflowCheckpointState{NodeID: checkpoint.NodeID, Payload: cloneWorkflowPayload(checkpoint.Payload)}, nil
	}
	rawMap, ok := raw.(map[string]any)
	if !ok {
		return nil, domain.NewValidationErr("rewrite workflow checkpoint metadata is invalid", nil)
	}
	nodeID, _ := rawMap["node_id"].(string)
	payload, _ := rawMap["payload"].(map[string]any)
	return &rewriteWorkflowCheckpointState{NodeID: strings.TrimSpace(nodeID), Payload: cloneWorkflowPayload(payload)}, nil
}

func storeRewriteWorkflowCheckpoint(metadata map[string]any, nodeID string, payload map[string]any) {
	if metadata == nil {
		return
	}
	metadata[rewriteWorkflowCheckpointMetadataKey] = rewriteWorkflowCheckpointState{
		NodeID:  strings.TrimSpace(nodeID),
		Payload: cloneWorkflowPayload(payload),
	}
}

func clearRewriteWorkflowCheckpoint(metadata map[string]any) {
	if metadata == nil {
		return
	}
	delete(metadata, rewriteWorkflowCheckpointMetadataKey)
}

func wrapRewriteKernelNodeError(node domain.WorkflowNode, inputVars map[string]any, cause error) error {
	if cause == nil {
		return nil
	}
	stage, ok := stageDefinitionFromWorkflowNode(node)
	if !ok {
		return cause
	}
	return &rewriteKernelStageFailure{
		Stage:     stage,
		InputVars: cloneWorkflowPayload(inputVars),
		Cause:     cause,
	}
}

func normalizeRewriteKernelRunError(err error) error {
	if err == nil {
		return nil
	}
	var stageErr *rewriteKernelStageFailure
	if !errors.As(err, &stageErr) {
		return err
	}
	stageErr.InputVars = cloneWorkflowPayload(stageErr.InputVars)
	return stageErr
}

func stageDefinitionFromWorkflowNode(node domain.WorkflowNode) (domain.RewriteStageDefinition, bool) {
	config, err := parseRewriteWorkflowNodeConfig(node.ConfigJSON)
	if err != nil {
		return domain.RewriteStageDefinition{}, false
	}
	stage := config.Stage
	if strings.TrimSpace(stage.Name) == "" {
		stage.Name = strings.TrimSpace(node.Name)
	}
	if strings.TrimSpace(stage.Type) == "" {
		stage.Type = strings.TrimSpace(node.Type)
	}
	if strings.TrimSpace(stage.ModelProfileRef) == "" {
		stage.ModelProfileRef = strings.TrimSpace(config.DefaultLLMProfile)
	}
	return stage, strings.TrimSpace(stage.Name) != ""
}

func buildRewriteStageRunFromWorkflowNode(pipelineRunID string, node domain.WorkflowNode, inputVars map[string]any, payload map[string]any) (*domain.RewriteStageRun, error) {
	config, err := parseRewriteWorkflowNodeConfig(node.ConfigJSON)
	if err != nil {
		return nil, err
	}
	stage := config.Stage
	if strings.TrimSpace(stage.Name) == "" {
		stage.Name = strings.TrimSpace(node.Name)
	}
	if strings.TrimSpace(stage.Type) == "" {
		stage.Type = strings.TrimSpace(node.Type)
	}
	if strings.TrimSpace(stage.ModelProfileRef) == "" {
		stage.ModelProfileRef = strings.TrimSpace(config.DefaultLLMProfile)
	}
	promptSnapshot, _ := payload["rewrite_prompt_snapshot"].(map[string]any)
	promptKey, _ := promptSnapshot["key"].(string)
	promptVersion, _ := promptSnapshot["version"].(string)
	if strings.TrimSpace(promptKey) != "" && strings.TrimSpace(promptVersion) != "" {
		stage.PromptRef = strings.TrimSpace(promptKey) + "@" + strings.TrimSpace(promptVersion)
	}
	output, _ := payload["rewrite_stage_output"].(map[string]any)
	if len(output) == 0 {
		return nil, domain.NewInternalErr("rewrite workflow stage produced no structured output", nil)
	}
	return buildRewriteStageRun(pipelineRunID, stage, inputVars, &StageExecutionResult{StructuredOutput: cloneWorkflowPayload(output)})
}

func applyRewriteWorkflowInputOverride(config rewriteWorkflowNodeConfig, inputVars map[string]any) map[string]any {
	if len(inputVars) == 0 {
		inputVars = map[string]any{}
	}
	stageCopy := config.Stage
	_ = applyWorkflowOverride(stageCopy, inputVars, inputVars)
	if promptRef := strings.TrimSpace(stageCopy.PromptRef); promptRef != "" {
		inputVars[workflowPromptRefMetadataKey] = promptRef
	}
	return inputVars
}
