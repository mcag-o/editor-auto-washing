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
	NodeID         string                    `json:"node_id"`
	Payload        map[string]any            `json:"payload"`
	TokenID        string                    `json:"token_id,omitempty"`
	ParentTokenID  string                    `json:"token_parent_id,omitempty"`
	OriginTokenID  string                    `json:"token_origin_id,omitempty"`
	OriginRoute    WorkflowTokenRouteLineage `json:"origin_route,omitempty"`
	Variables      map[string]any            `json:"variables,omitempty"`
	Result         map[string]any            `json:"result,omitempty"`
	Artifacts      map[string]any            `json:"artifacts,omitempty"`
	ActiveTokenSet []map[string]any          `json:"active_token_set,omitempty"`
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
		Metadata:    run.Metadata,
	}
	if resume {
		err = kernel.Resume(ctx, runtimeCtx)
	} else {
		err = kernel.executeFrom(ctx, runtimeCtx, workflow.EntryNodeID)
	}
	if err != nil {
		return "", err
	}
	if summary := latestRouteOutcomeSummary(runtimeCtx); summary != nil {
		run.Metadata[workflowLatestRouteMetadataKey] = workflowRouteSummaryMetadata(*summary)
	}
	stageOutput, ok := workflowRuntimePayload(runtimeCtx)["rewrite_stage_output"].(map[string]any)
	if (!ok || len(stageOutput) == 0) && len(runtimeCtx.CompletedTokens) > 0 {
		for i := len(runtimeCtx.CompletedTokens) - 1; i >= 0; i-- {
			token := runtimeCtx.CompletedTokens[i]
			if token == nil || token.Branch == nil {
				continue
			}
			output, outputOK := token.Branch.Variables["rewrite_stage_output"].(map[string]any)
			if outputOK && len(output) > 0 {
				stageOutput = cloneWorkflowPayload(output)
				ok = true
				break
			}
		}
	}
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
	_, err := n.ExecuteWorkflow(ctx, &WorkflowExecutionContext{Context: wc}, n.def)
	return err
}

func (n *rewriteKernelWorkflowNode) ExecuteWorkflow(ctx context.Context, runtimeCtx *WorkflowExecutionContext, node domain.WorkflowNode) (WorkflowNodeResult, error) {
	if n == nil || n.run == nil || n.runs == nil || n.delegate == nil {
		return WorkflowNodeResult{}, domain.NewInternalErr("rewrite kernel workflow node is not configured", nil)
	}
	if runtimeCtx == nil || runtimeCtx.Context == nil {
		return WorkflowNodeResult{}, domain.NewValidationErr("workflow context is required", nil)
	}
	config, err := normalizeRewriteWorkflowNodeConfig(node, workflowRuntimePayload(runtimeCtx))
	if err != nil {
		return WorkflowNodeResult{}, err
	}
	execNode, err := marshalRewriteWorkflowNodeConfig(node, config)
	if err != nil {
		return WorkflowNodeResult{}, err
	}
	inputVars := cloneWorkflowPayload(workflowRuntimePayload(runtimeCtx))
	inputVars = applyRewriteWorkflowInputOverride(config, inputVars)
	runtimeCtx.Context.Payload = cloneWorkflowPayload(inputVars)
	syncWorkflowPayloadBridge(runtimeCtx)
	currentStage := strings.TrimSpace(config.Stage.Name)
	if currentStage == "" {
		currentStage = node.ID
	}
	n.run.CurrentStage = currentStage
	storeRewriteWorkflowCheckpoint(n.run.Metadata, execNode.ID, inputVars, runtimeCtx.CurrentToken, runtimeCtx.ActiveTokens...)
	if err := n.runs.Update(ctx, n.run); err != nil {
		return WorkflowNodeResult{}, wrapRewriteKernelNodeError(execNode, inputVars, err)
	}
	result, execErr := n.delegate.Execute(ctx, runtimeCtx, execNode)
	var outcomeErr *rewriteStageOutcomeError
	if execErr != nil && !errors.As(execErr, &outcomeErr) {
		return WorkflowNodeResult{}, wrapRewriteKernelNodeError(execNode, inputVars, execErr)
	}
	if execNode.Type == rewriteWorkflowNodeTypeMaterialize {
		if execErr != nil {
			return WorkflowNodeResult{}, wrapRewriteKernelNodeError(execNode, inputVars, execErr)
		}
		return result, nil
	}
	stageRun, err := buildRewriteStageRunFromWorkflowNode(n.run.ID, execNode, inputVars, workflowRuntimePayload(runtimeCtx))
	if err != nil {
		return WorkflowNodeResult{}, wrapRewriteKernelNodeError(execNode, inputVars, err)
	}
	if err := n.stageRuns.Create(ctx, stageRun); err != nil {
		return WorkflowNodeResult{}, wrapRewriteKernelNodeError(execNode, inputVars, err)
	}
	if execErr != nil {
		return WorkflowNodeResult{}, wrapRewriteKernelNodeError(execNode, inputVars, execErr)
	}
	if err := validateRewriteWorkflowNodeOutcome(execNode, workflowRuntimePayload(runtimeCtx)); err != nil {
		return WorkflowNodeResult{}, wrapRewriteKernelNodeError(execNode, inputVars, err)
	}
	return result, nil
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
	checkpointMetadata := checkpoint.Metadata()
	return payload, []domain.WorkflowCheckpoint{{
		WorkflowRunID: workflowRunID,
		NodeID:        strings.TrimSpace(checkpoint.NodeID),
		State:         domain.WorkflowCheckpointStateActive,
		Resumable:     true,
		Metadata:      checkpointMetadata,
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
		checkpoint.ActiveTokenSet = cloneWorkflowActiveTokenSet(checkpoint.ActiveTokenSet)
		return &checkpoint, nil
	}
	if checkpoint, ok := raw.(*rewriteWorkflowCheckpointState); ok {
		return &rewriteWorkflowCheckpointState{
			NodeID:         checkpoint.NodeID,
			Payload:        cloneWorkflowPayload(checkpoint.Payload),
			TokenID:        checkpoint.TokenID,
			ParentTokenID:  checkpoint.ParentTokenID,
			OriginTokenID:  checkpoint.OriginTokenID,
			OriginRoute:    checkpoint.OriginRoute,
			Variables:      cloneWorkflowPayload(checkpoint.Variables),
			Result:         cloneWorkflowPayload(checkpoint.Result),
			Artifacts:      cloneWorkflowPayload(checkpoint.Artifacts),
			ActiveTokenSet: cloneWorkflowActiveTokenSet(checkpoint.ActiveTokenSet),
		}, nil
	}
	rawMap, ok := raw.(map[string]any)
	if !ok {
		return nil, domain.NewValidationErr("rewrite workflow checkpoint metadata is invalid", nil)
	}
	nodeID, _ := rawMap["node_id"].(string)
	payload, _ := rawMap["payload"].(map[string]any)
	originRoute := workflowTokenRouteLineageFromRaw(rawMap["origin_route"])
	return &rewriteWorkflowCheckpointState{
		NodeID:         strings.TrimSpace(nodeID),
		Payload:        cloneWorkflowPayload(payload),
		TokenID:        strings.TrimSpace(domain.DraftString(rawMap["token_id"])),
		ParentTokenID:  strings.TrimSpace(domain.DraftString(rawMap["token_parent_id"])),
		OriginTokenID:  strings.TrimSpace(domain.DraftString(rawMap["token_origin_id"])),
		OriginRoute:    originRoute,
		Variables:      workflowCheckpointPayload(firstRewriteCheckpointPayload(rawMap, "variables", "token_branch_vars")),
		Result:         workflowCheckpointPayload(firstRewriteCheckpointPayload(rawMap, "result", "token_branch_result")),
		Artifacts:      workflowCheckpointPayload(firstRewriteCheckpointPayload(rawMap, "artifacts", "token_branch_artifacts")),
		ActiveTokenSet: workflowActiveTokenSetFromRaw(rawMap[workflowActiveTokenSetMetadataKey]),
	}, nil
}

func storeRewriteWorkflowCheckpoint(metadata map[string]any, nodeID string, payload map[string]any, token *WorkflowToken, activeTokens ...*WorkflowToken) {
	if metadata == nil {
		return
	}
	checkpoint := rewriteWorkflowCheckpointState{
		NodeID:  strings.TrimSpace(nodeID),
		Payload: cloneWorkflowPayload(payload),
	}
	if token != nil {
		checkpoint.TokenID = strings.TrimSpace(token.ID)
		checkpoint.ParentTokenID = strings.TrimSpace(token.ParentTokenID)
		checkpoint.OriginTokenID = strings.TrimSpace(token.OriginTokenID)
		checkpoint.OriginRoute = WorkflowTokenRouteLineage{
			SourceNodeID:   strings.TrimSpace(token.OriginRoute.SourceNodeID),
			SelectedEdgeID: strings.TrimSpace(token.OriginRoute.SelectedEdgeID),
			SelectedNodeID: strings.TrimSpace(token.OriginRoute.SelectedNodeID),
		}
		if token.Branch != nil {
			checkpoint.Variables = cloneWorkflowPayload(token.Branch.Variables)
			checkpoint.Result = cloneWorkflowPayload(token.Branch.Result)
			checkpoint.Artifacts = cloneWorkflowPayload(token.Branch.Artifacts)
		}
	}
	checkpoint.ActiveTokenSet = workflowResumableTokenSet(token, activeTokens)
	metadata[rewriteWorkflowCheckpointMetadataKey] = checkpoint
	syncRewriteWorkflowRuntimeVisibility(metadata, checkpoint)
}

func (c *rewriteWorkflowCheckpointState) Token() *WorkflowToken {
	if c == nil || strings.TrimSpace(c.TokenID) == "" {
		return nil
	}
	originID := strings.TrimSpace(c.OriginTokenID)
	if originID == "" {
		originID = strings.TrimSpace(c.TokenID)
	}
	return &WorkflowToken{
		ID:            strings.TrimSpace(c.TokenID),
		NodeID:        strings.TrimSpace(c.NodeID),
		ParentTokenID: strings.TrimSpace(c.ParentTokenID),
		OriginTokenID: originID,
		OriginRoute: WorkflowTokenRouteLineage{
			SourceNodeID:   strings.TrimSpace(c.OriginRoute.SourceNodeID),
			SelectedEdgeID: strings.TrimSpace(c.OriginRoute.SelectedEdgeID),
			SelectedNodeID: strings.TrimSpace(c.OriginRoute.SelectedNodeID),
		},
		Branch: &WorkflowBranchContext{
			Variables: cloneWorkflowPayload(c.Variables),
			Result:    cloneWorkflowPayload(c.Result),
			Artifacts: cloneWorkflowPayload(c.Artifacts),
		},
	}
}

func (c *rewriteWorkflowCheckpointState) Metadata() map[string]any {
	if c == nil {
		return nil
	}
	metadata := map[string]any{}
	if token := c.Token(); token != nil {
		metadata = workflowTokenMetadata(*token)
	}
	if len(c.ActiveTokenSet) > 0 {
		metadata = mergeCheckpointMetadata(metadata, map[string]any{
			workflowActiveTokenSetMetadataKey: cloneWorkflowActiveTokenSet(c.ActiveTokenSet),
		})
	}
	return metadata
}

func workflowActiveTokenSetFromTokens(tokens []*WorkflowToken) []map[string]any {
	if len(tokens) == 0 {
		return nil
	}
	activeSet := make([]map[string]any, 0, len(tokens))
	for _, token := range tokens {
		if token == nil {
			continue
		}
		activeSet = append(activeSet, workflowTokenMetadata(*token))
	}
	if len(activeSet) == 0 {
		return nil
	}
	return activeSet
}

func workflowResumableTokenSet(current *WorkflowToken, queued []*WorkflowToken) []map[string]any {
	tokens := make([]*WorkflowToken, 0, len(queued)+1)
	for _, token := range queued {
		if token == nil {
			continue
		}
		if current != nil && strings.TrimSpace(token.ID) != "" && token.ID == current.ID {
			continue
		}
		tokens = append(tokens, token)
	}
	if current != nil {
		tokens = append(tokens, current)
	}
	return workflowActiveTokenSetFromTokens(tokens)
}

func workflowActiveTokenSetFromRaw(raw any) []map[string]any {
	if raw == nil {
		return nil
	}
	if typed, ok := raw.([]map[string]any); ok {
		return cloneWorkflowActiveTokenSet(typed)
	}
	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		return nil
	}
	activeSet := make([]map[string]any, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok || len(entry) == 0 {
			continue
		}
		activeSet = append(activeSet, cloneWorkflowPayload(entry))
	}
	if len(activeSet) == 0 {
		return nil
	}
	return activeSet
}

func cloneWorkflowActiveTokenSet(activeSet []map[string]any) []map[string]any {
	if len(activeSet) == 0 {
		return nil
	}
	cloned := make([]map[string]any, 0, len(activeSet))
	for _, entry := range activeSet {
		if len(entry) == 0 {
			cloned = append(cloned, map[string]any{})
			continue
		}
		cloned = append(cloned, cloneWorkflowPayload(entry))
	}
	return cloned
}

func firstRewriteCheckpointPayload(values map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := values[key]; ok && value != nil {
			return value
		}
	}
	return nil
}

func workflowTokenRouteLineageFromRaw(raw any) WorkflowTokenRouteLineage {
	rawMap, ok := raw.(map[string]any)
	if !ok || len(rawMap) == 0 {
		return WorkflowTokenRouteLineage{}
	}
	return WorkflowTokenRouteLineage{
		SourceNodeID:   strings.TrimSpace(firstNonEmptyDraftString(rawMap, "source_node_id", "SourceNodeID")),
		SelectedEdgeID: strings.TrimSpace(firstNonEmptyDraftString(rawMap, "selected_edge_id", "SelectedEdgeID")),
		SelectedNodeID: strings.TrimSpace(firstNonEmptyDraftString(rawMap, "selected_node_id", "SelectedNodeID")),
	}
}

func firstNonEmptyDraftString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		value := strings.TrimSpace(domain.DraftString(values[key]))
		if value != "" {
			return value
		}
	}
	return ""
}

func clearRewriteWorkflowCheckpoint(metadata map[string]any) {
	if metadata == nil {
		return
	}
	delete(metadata, rewriteWorkflowCheckpointMetadataKey)
	delete(metadata, workflowActiveTokenSetMetadataKey)
}

func syncRewriteWorkflowRuntimeVisibility(metadata map[string]any, checkpoint rewriteWorkflowCheckpointState) {
	if metadata == nil {
		return
	}
	activeSet := cloneWorkflowActiveTokenSet(checkpoint.ActiveTokenSet)
	if len(activeSet) == 0 {
		delete(metadata, workflowActiveTokenSetMetadataKey)
		return
	}
	metadata[workflowActiveTokenSetMetadataKey] = activeSet
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
	config.Stage = stageCopy
	if promptRef := strings.TrimSpace(stageCopy.PromptRef); promptRef != "" {
		inputVars[workflowPromptRefMetadataKey] = promptRef
	}
	return inputVars
}
