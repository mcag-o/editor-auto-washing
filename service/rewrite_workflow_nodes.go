package service

import (
	"content-hub/domain"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type RewriteWorkflowNodeExecutor interface {
	Execute(ctx context.Context, runtimeCtx *WorkflowExecutionContext, node domain.WorkflowNode) (WorkflowNodeResult, error)
}

type rewriteGenerateNodeExecutor struct {
	stageExecutor *RewriteStageExecutor
}

type rewriteReviewNodeExecutor struct {
	stageExecutor *RewriteStageExecutor
}

type rewriteRepairNodeExecutor struct {
	stageExecutor *RewriteStageExecutor
}

type rewriteMaterializeNodeExecutor struct{}

type rewriteStageOutcomeError struct {
	message string
}

func (e *rewriteStageOutcomeError) Error() string {
	if e == nil {
		return ""
	}
	return e.message
}

func NewRewriteGenerateNodeExecutor(stageExecutor *RewriteStageExecutor) RewriteWorkflowNodeExecutor {
	return &rewriteGenerateNodeExecutor{stageExecutor: stageExecutor}
}

func NewRewriteReviewNodeExecutor(stageExecutor *RewriteStageExecutor) RewriteWorkflowNodeExecutor {
	return &rewriteReviewNodeExecutor{stageExecutor: stageExecutor}
}

func NewRewriteRepairNodeExecutor(stageExecutor *RewriteStageExecutor) RewriteWorkflowNodeExecutor {
	return &rewriteRepairNodeExecutor{stageExecutor: stageExecutor}
}

func NewRewriteMaterializeNodeExecutor() RewriteWorkflowNodeExecutor {
	return &rewriteMaterializeNodeExecutor{}
}

func (e *rewriteGenerateNodeExecutor) Execute(ctx context.Context, runtimeCtx *WorkflowExecutionContext, node domain.WorkflowNode) (WorkflowNodeResult, error) {
	return executeRewriteWorkflowStageNode(ctx, runtimeCtx, node, e.stageExecutor)
}

func (e *rewriteReviewNodeExecutor) Execute(ctx context.Context, runtimeCtx *WorkflowExecutionContext, node domain.WorkflowNode) (WorkflowNodeResult, error) {
	result, err := executeRewriteWorkflowStageNode(ctx, runtimeCtx, node, e.stageExecutor)
	if err != nil {
		return WorkflowNodeResult{}, err
	}
	if runtimeCtx == nil || runtimeCtx.Context == nil {
		return result, nil
	}
	payload := workflowRuntimePayload(runtimeCtx)
	qualityDecision, _ := payload["quality_decision"].(string)
	routeDecision, _ := payload["quality_route_decision"].(string)
	if strings.TrimSpace(qualityDecision) == QualityDecisionPass {
		return result, nil
	}
	if strings.TrimSpace(routeDecision) == QualityDecisionRepair && nodeHasExplicitRepairPolicy(node) {
		return result, nil
	}
	message, _ := payload["quality_message"].(string)
	if strings.TrimSpace(message) == "" {
		message = "review quality did not pass"
	}
	return WorkflowNodeResult{}, &rewriteStageOutcomeError{message: message}
}

func nodeHasExplicitRepairPolicy(node domain.WorkflowNode) bool {
	config, err := parseRewriteWorkflowNodeConfig(node.ConfigJSON)
	if err != nil {
		return false
	}
	return strings.TrimSpace(config.RouteOnQualityAction) == QualityDecisionRepair
}

func (e *rewriteRepairNodeExecutor) Execute(ctx context.Context, runtimeCtx *WorkflowExecutionContext, node domain.WorkflowNode) (WorkflowNodeResult, error) {
	result, err := executeRewriteWorkflowStageNode(ctx, runtimeCtx, node, e.stageExecutor)
	if err != nil {
		return WorkflowNodeResult{}, err
	}
	if runtimeCtx != nil && runtimeCtx.Context != nil {
		payload := workflowRuntimePayload(runtimeCtx)
		if payload["quality_route_decision"] == QualityDecisionPass {
			return result, nil
		}
		message, _ := payload["quality_message"].(string)
		if strings.TrimSpace(message) == "" {
			message = "repair quality did not pass"
		}
		return WorkflowNodeResult{}, &rewriteStageOutcomeError{message: message}
	}
	return result, nil
}

func (e *rewriteMaterializeNodeExecutor) Execute(_ context.Context, runtimeCtx *WorkflowExecutionContext, node domain.WorkflowNode) (WorkflowNodeResult, error) {
	if runtimeCtx == nil || runtimeCtx.Context == nil {
		return WorkflowNodeResult{}, domain.NewValidationErr("workflow execution context is required", nil)
	}
	ensureWorkflowPayload(runtimeCtx)
	runtimeCtx.Context.Payload["materialize_node_id"] = node.ID
	runtimeCtx.Context.Payload["materialization_requested"] = true
	syncWorkflowPayloadBridge(runtimeCtx)
	return WorkflowNodeResult{}, nil
}

func executeRewriteWorkflowStageNode(ctx context.Context, runtimeCtx *WorkflowExecutionContext, node domain.WorkflowNode, stageExecutor *RewriteStageExecutor) (WorkflowNodeResult, error) {
	if stageExecutor == nil {
		return WorkflowNodeResult{}, domain.NewInternalErr("rewrite stage executor is not configured", nil)
	}
	if runtimeCtx == nil || runtimeCtx.Context == nil {
		return WorkflowNodeResult{}, domain.NewValidationErr("workflow execution context is required", nil)
	}

	config, err := parseRewriteWorkflowNodeConfig(node.ConfigJSON)
	if err != nil {
		return WorkflowNodeResult{}, fmt.Errorf("parse rewrite workflow node config: %w", err)
	}
	if strings.TrimSpace(config.Stage.Name) == "" {
		config.Stage.Name = strings.TrimSpace(node.Name)
	}
	if strings.TrimSpace(config.Stage.Type) == "" {
		config.Stage.Type = strings.TrimSpace(node.Type)
	}
	if strings.TrimSpace(config.Stage.ModelProfileRef) == "" {
		config.Stage.ModelProfileRef = strings.TrimSpace(config.DefaultLLMProfile)
	}

	ensureWorkflowPayload(runtimeCtx)
	result, err := stageExecutor.Execute(ctx, config.Stage, StageExecutionInput{Vars: cloneWorkflowPayload(runtimeCtx.Context.Payload)})
	if err != nil {
		return WorkflowNodeResult{}, err
	}
	mergeRewriteStageResult(runtimeCtx.Context.Payload, config.Stage, result)
	mergeRewriteRouteDecision(runtimeCtx.Context.Payload, config, result)
	syncWorkflowPayloadBridge(runtimeCtx)
	return WorkflowNodeResult{RouteRequired: strings.TrimSpace(config.RouteOnQualityAction) != ""}, nil
}

func normalizeRewriteWorkflowNodeConfig(node domain.WorkflowNode, payload map[string]any) (rewriteWorkflowNodeConfig, error) {
	config, err := parseRewriteWorkflowNodeConfig(node.ConfigJSON)
	if err != nil {
		return rewriteWorkflowNodeConfig{}, fmt.Errorf("parse rewrite workflow node config: %w", err)
	}
	if strings.TrimSpace(config.Stage.Name) == "" {
		config.Stage.Name = strings.TrimSpace(node.Name)
	}
	if strings.TrimSpace(config.Stage.Type) == "" {
		config.Stage.Type = strings.TrimSpace(node.Type)
	}
	if strings.TrimSpace(config.Stage.ModelProfileRef) == "" {
		config.Stage.ModelProfileRef = strings.TrimSpace(config.DefaultLLMProfile)
	}
	config.Stage = applyWorkflowOverride(config.Stage, payload, payload)
	return config, nil
}

func marshalRewriteWorkflowNodeConfig(node domain.WorkflowNode, config rewriteWorkflowNodeConfig) (domain.WorkflowNode, error) {
	encoded, err := json.Marshal(config)
	if err != nil {
		return domain.WorkflowNode{}, fmt.Errorf("marshal rewrite workflow node config: %w", err)
	}
	node.ConfigJSON = string(encoded)
	return node, nil
}

func validateRewriteWorkflowNodeOutcome(node domain.WorkflowNode, payload map[string]any) error {
	if node.Type == rewriteWorkflowNodeTypeMaterialize {
		return nil
	}
	routeDecision, _ := payload["quality_route_decision"].(string)
	if strings.TrimSpace(routeDecision) == QualityDecisionPass {
		return nil
	}
	config, err := parseRewriteWorkflowNodeConfig(node.ConfigJSON)
	if err == nil && strings.TrimSpace(config.RouteOnQualityAction) == QualityDecisionRepair && strings.TrimSpace(routeDecision) == QualityDecisionRepair {
		return nil
	}
	message, _ := payload["quality_message"].(string)
	if strings.TrimSpace(message) == "" {
		message = "rewrite quality did not pass"
	}
	return &rewriteStageOutcomeError{message: message}
}

func parseRewriteWorkflowNodeConfig(configJSON string) (rewriteWorkflowNodeConfig, error) {
	configJSON = strings.TrimSpace(configJSON)
	if configJSON == "" {
		return rewriteWorkflowNodeConfig{}, nil
	}
	var cfg rewriteWorkflowNodeConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return rewriteWorkflowNodeConfig{}, fmt.Errorf("decode rewrite workflow node config json: %w", err)
	}
	return cfg, nil
}

func ensureWorkflowPayload(runtimeCtx *WorkflowExecutionContext) {
	if runtimeCtx == nil {
		return
	}
	bindWorkflowPayloadBridge(runtimeCtx)
}

func syncWorkflowPayloadBridge(runtimeCtx *WorkflowExecutionContext) {
	if runtimeCtx == nil || runtimeCtx.Context == nil {
		return
	}
	sharedInput := workflowRuntimeSharedInput(runtimeCtx)
	variables := map[string]any{}
	for key, value := range runtimeCtx.Context.Payload {
		if sharedValue, shared := sharedInput[key]; shared && fmt.Sprint(sharedValue) == fmt.Sprint(value) {
			continue
		}
		variables[key] = value
	}
	runtimeCtx.Variables = variables
	if runtimeCtx.CurrentToken != nil && runtimeCtx.CurrentToken.Branch != nil {
		runtimeCtx.CurrentToken.Branch.Variables = cloneWorkflowPayload(variables)
	}
	bindWorkflowPayloadBridge(runtimeCtx)
}

func cloneWorkflowPayload(payload map[string]any) map[string]any {
	if payload == nil {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(payload))
	for key, value := range payload {
		cloned[key] = cloneWorkflowValue(value)
	}
	return cloned
}

func cloneWorkflowValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return cloneWorkflowPayload(v)
	case []any:
		cloned := make([]any, len(v))
		for i := range v {
			cloned[i] = cloneWorkflowValue(v[i])
		}
		return cloned
	default:
		return v
	}
}

func mergeRewriteStageResult(payload map[string]any, stage domain.RewriteStageDefinition, result *StageExecutionResult) {
	for key, value := range result.StructuredOutput {
		payload[key] = value
	}
	payload["rewrite_stage_name"] = stage.Name
	payload["rewrite_stage_output"] = cloneWorkflowPayload(result.StructuredOutput)
	payload["rewrite_prompt_snapshot"] = map[string]any{
		"key":     result.PromptSnapshot.Key,
		"version": result.PromptSnapshot.Version,
		"system":  result.PromptSnapshot.System,
		"user":    result.PromptSnapshot.User,
	}
	if result.Response != nil {
		payload["rewrite_response"] = map[string]any{
			"content":           result.Response.Content,
			"model":             result.Response.Model,
			"prompt_tokens":     result.Response.PromptTokens,
			"completion_tokens": result.Response.CompletionTokens,
			"finish_reason":     result.Response.FinishReason,
		}
	}
	payload["quality_decision"] = result.Quality.Action
	payload["quality_message"] = result.Quality.Message
}

func mergeRewriteRouteDecision(payload map[string]any, config rewriteWorkflowNodeConfig, result *StageExecutionResult) {
	routeDecision := result.Quality.Action
	if strings.TrimSpace(config.RouteOnQualityAction) != "" && result.Quality.Action != QualityDecisionPass {
		routeDecision = strings.TrimSpace(config.RouteOnQualityAction)
	}
	payload["quality_route_decision"] = routeDecision
}
