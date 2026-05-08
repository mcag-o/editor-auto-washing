package service

import (
	"content-hub/domain"
	llminfra "content-hub/infra/llm"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRewriteGenerateNodeExecutorMergesStructuredOutputIntoExecutionContext(t *testing.T) {
	prompts := &stubRewritePromptRegistry{prompt: &domain.PromptTemplate{
		Key:            "generate_draft",
		Version:        "v1",
		SystemTemplate: "sys {{title}}",
		UserTemplate:   "Title: {{title}}",
	}}
	client := &recordingLLMClient{response: &llminfra.GenerateResponse{Response: &domain.LLMResponse{
		Content:          `{"title":"Final Title","body":"Paragraph 1"}`,
		Model:            "mock-1",
		PromptTokens:     11,
		CompletionTokens: 7,
		FinishReason:     "stop",
	}}}
	executor := NewRewriteStageExecutor(prompts, client, NewQualityGateEngine())
	node := NewRewriteGenerateNodeExecutor(executor)
	runtimeCtx := &WorkflowExecutionContext{
		Workflow: &domain.WorkflowDefinition{},
		Context: &domain.WorkflowContext{Payload: map[string]any{
			"title": "Source Title",
		}},
		CurrentNodeID: "generate_draft",
	}
	nodeDef := domain.WorkflowNode{
		ID:   "generate_draft",
		Type: rewriteWorkflowNodeTypeGenerate,
		Name: "generate_draft",
		ConfigJSON: `{"stage":{"name":"generate_draft","type":"generate_draft","prompt_ref":"generate_draft@v1","enabled":true}}`,
	}

	result, err := node.Execute(t.Context(), runtimeCtx, nodeDef)

	require.NoError(t, err)
	require.False(t, result.RouteRequired)
	require.Equal(t, "Final Title", runtimeCtx.Context.Payload["title"])
	require.Equal(t, "Paragraph 1", runtimeCtx.Context.Payload["body"])
	require.Equal(t, QualityDecisionPass, runtimeCtx.Context.Payload["quality_decision"])
	require.Equal(t, "generate_draft", runtimeCtx.Context.Payload["rewrite_stage_name"])
	require.NotNil(t, runtimeCtx.Context.Payload["rewrite_stage_output"])
	require.NotNil(t, runtimeCtx.Context.Payload["rewrite_prompt_snapshot"])
	require.NotNil(t, runtimeCtx.Context.Payload["rewrite_response"])
	stageOutput, ok := runtimeCtx.Context.Payload["rewrite_stage_output"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Paragraph 1", stageOutput["body"])
}

func TestRewriteGenerateNodeExecutorAppliesDefaultLLMProfileFromCompiledStageConfig(t *testing.T) {
	compiler := NewRewriteWorkflowCompiler()
	prompts := &stubRewritePromptRegistry{prompt: &domain.PromptTemplate{
		Key:            "generate_draft",
		Version:        "v1",
		SystemTemplate: "sys {{title}}",
		UserTemplate:   "Title: {{title}}",
	}}
	profiles := &stubLLMProfileResolver{profile: &domain.LLMProfile{
		Name:        "rewrite-default",
		Model:       "gpt-4.1-mini-real",
		Temperature: 0.4,
		MaxTokens:   512,
	}}
	client := &recordingLLMClient{response: &llminfra.GenerateResponse{Response: &domain.LLMResponse{
		Content: `{"title":"Final Title","body":"Paragraph 1"}`,
		Model:   "gpt-4.1-mini-real",
	}}}
	executor := NewRewriteStageExecutorWithProfileResolver(prompts, profiles, client, NewQualityGateEngine())
	node := NewRewriteGenerateNodeExecutor(executor)
	workflow, err := compiler.Compile(&domain.RewritePipelineProfile{
		ID:                "profile-default-llm",
		Name:              "Default LLM Workflow",
		Version:           "v1",
		Enabled:           true,
		DefaultLLMProfile: "rewrite-default",
		Stages: []domain.RewriteStageDefinition{{
			Name:      "generate_draft",
			Type:      "generate_draft",
			PromptRef: "generate_draft@v1",
			Enabled:   true,
		}},
	})
	require.NoError(t, err)
	runtimeCtx := &WorkflowExecutionContext{
		Workflow: workflow,
		Context: &domain.WorkflowContext{Payload: map[string]any{
			"title": "Source Title",
		}},
		CurrentNodeID: "generate_draft",
	}
	nodeDef := workflow.Nodes[0]

	_, err = node.Execute(t.Context(), runtimeCtx, nodeDef)

	require.NoError(t, err)
	require.Equal(t, "rewrite-default", profiles.gotName)
	require.Equal(t, "gpt-4.1-mini-real", client.gotReq.Options.Model)
	require.Equal(t, 0.4, client.gotReq.Options.Temperature)
	require.Equal(t, 512, client.gotReq.Options.MaxTokens)
}

func TestRewriteReviewNodeExecutorEmitsFailRunQualityDecisionWithoutPassFallback(t *testing.T) {
	prompts := &stubRewritePromptRegistry{prompt: &domain.PromptTemplate{
		Key:            "review_draft",
		Version:        "v1",
		SystemTemplate: "sys {{title}}",
		UserTemplate:   "Body: {{body}}",
	}}
	client := &recordingLLMClient{response: &llminfra.GenerateResponse{Response: &domain.LLMResponse{
		Content: `{"title":"Reviewed"}`,
		Model:   "mock-1",
	}}}
	executor := NewRewriteStageExecutor(prompts, client, NewQualityGateEngine())
	node := NewRewriteReviewNodeExecutor(executor)
	runtimeCtx := &WorkflowExecutionContext{
		Workflow: &domain.WorkflowDefinition{},
		Context: &domain.WorkflowContext{Payload: map[string]any{
			"title": "Source Title",
			"body":  "seed",
		}},
		CurrentNodeID: "review_draft",
	}
	nodeDef := domain.WorkflowNode{
		ID:   "review_draft",
		Type: rewriteWorkflowNodeTypeReview,
		Name: "review_draft",
		ConfigJSON: `{"stage":{"name":"review_draft","type":"review","prompt_ref":"review_draft@v1","enabled":true},"route_on_quality_action":"fail_run"}`,
	}

	result, err := node.Execute(t.Context(), runtimeCtx, nodeDef)

	require.Error(t, err)
	require.Equal(t, WorkflowNodeResult{}, result)
	require.ErrorContains(t, err, "body is missing")
	require.Equal(t, QualityDecisionRepair, runtimeCtx.Context.Payload["quality_decision"])
	require.Equal(t, QualityDecisionFail, runtimeCtx.Context.Payload["quality_route_decision"])
}

func TestRewriteReviewNodeExecutorFailsFastForRetryStageQualityFailure(t *testing.T) {
	prompts := &stubRewritePromptRegistry{prompt: &domain.PromptTemplate{
		Key:            "review_draft",
		Version:        "v1",
		SystemTemplate: "sys {{title}}",
		UserTemplate:   "Body: {{body}}",
	}}
	client := &recordingLLMClient{response: &llminfra.GenerateResponse{Response: &domain.LLMResponse{
		Content: `{"title":"Reviewed"}`,
		Model:   "mock-1",
	}}}
	executor := NewRewriteStageExecutor(prompts, client, NewQualityGateEngine())
	node := NewRewriteReviewNodeExecutor(executor)
	runtimeCtx := &WorkflowExecutionContext{
		Workflow: &domain.WorkflowDefinition{},
		Context: &domain.WorkflowContext{Payload: map[string]any{
			"title": "Source Title",
			"body":  "seed",
		}},
		CurrentNodeID: "review_draft",
	}
	nodeDef := domain.WorkflowNode{
		ID:   "review_draft",
		Type: rewriteWorkflowNodeTypeReview,
		Name: "review_draft",
		ConfigJSON: `{"stage":{"name":"review_draft","type":"review","prompt_ref":"review_draft@v1","enabled":true},"route_on_quality_action":"retry_stage"}`,
	}

	result, err := node.Execute(t.Context(), runtimeCtx, nodeDef)

	require.Error(t, err)
	require.Equal(t, WorkflowNodeResult{}, result)
	require.ErrorContains(t, err, "body is missing")
	require.Equal(t, QualityDecisionRepair, runtimeCtx.Context.Payload["quality_decision"])
	require.Equal(t, QualityDecisionRetry, runtimeCtx.Context.Payload["quality_route_decision"])
}

func TestRewriteReviewNodeExecutorFailsFastWithoutExplicitRepairPolicy(t *testing.T) {
	prompts := &stubRewritePromptRegistry{prompt: &domain.PromptTemplate{
		Key:            "review_draft",
		Version:        "v1",
		SystemTemplate: "sys {{title}}",
		UserTemplate:   "Body: {{body}}",
	}}
	client := &recordingLLMClient{response: &llminfra.GenerateResponse{Response: &domain.LLMResponse{
		Content: `{"title":"Reviewed"}`,
		Model:   "mock-1",
	}}}
	executor := NewRewriteStageExecutor(prompts, client, NewQualityGateEngine())
	node := NewRewriteReviewNodeExecutor(executor)
	runtimeCtx := &WorkflowExecutionContext{
		Workflow: &domain.WorkflowDefinition{},
		Context: &domain.WorkflowContext{Payload: map[string]any{
			"title": "Source Title",
			"body":  "seed",
		}},
		CurrentNodeID: "review_draft",
	}
	nodeDef := domain.WorkflowNode{
		ID:   "review_draft",
		Type: rewriteWorkflowNodeTypeReview,
		Name: "review_draft",
		ConfigJSON: `{"stage":{"name":"review_draft","type":"review","prompt_ref":"review_draft@v1","enabled":true}}`,
	}

	result, err := node.Execute(t.Context(), runtimeCtx, nodeDef)

	require.Error(t, err)
	require.Equal(t, WorkflowNodeResult{}, result)
	require.ErrorContains(t, err, "body is missing")
	require.Equal(t, QualityDecisionRepair, runtimeCtx.Context.Payload["quality_decision"])
	require.Equal(t, QualityDecisionRepair, runtimeCtx.Context.Payload["quality_route_decision"])
}

func TestRewriteRepairNodeExecutorFailsWhenRepairQualityDoesNotPass(t *testing.T) {
	prompts := &stubRewritePromptRegistry{prompt: &domain.PromptTemplate{
		Key:            "repair_draft",
		Version:        "v1",
		SystemTemplate: "sys {{title}}",
		UserTemplate:   "Body: {{body}}",
	}}
	client := &recordingLLMClient{response: &llminfra.GenerateResponse{Response: &domain.LLMResponse{
		Content: `{"title":"Still Broken"}`,
		Model:   "mock-1",
	}}}
	executor := NewRewriteStageExecutor(prompts, client, NewQualityGateEngine())
	node := NewRewriteRepairNodeExecutor(executor)
	runtimeCtx := &WorkflowExecutionContext{
		Workflow: &domain.WorkflowDefinition{},
		Context: &domain.WorkflowContext{Payload: map[string]any{
			"title": "Source Title",
			"body":  "seed",
		}},
		CurrentNodeID: "repair_draft",
	}
	nodeDef := domain.WorkflowNode{
		ID:   "repair_draft",
		Type: rewriteWorkflowNodeTypeRepair,
		Name: "repair_draft",
		ConfigJSON: `{"stage":{"name":"repair_draft","type":"repair","prompt_ref":"repair_draft@v1","enabled":true}}`,
	}

	result, err := node.Execute(t.Context(), runtimeCtx, nodeDef)

	require.Error(t, err)
	require.Equal(t, WorkflowNodeResult{}, result)
	require.Contains(t, err.Error(), "body is missing")
}
