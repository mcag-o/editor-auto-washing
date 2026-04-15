package service

import (
	"content-hub/domain"
	llminfra "content-hub/infra/llm"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubRewritePromptRegistry struct {
	prompt     *domain.PromptTemplate
	err        error
	gotKey     string
	gotVersion string
}

func (s *stubRewritePromptRegistry) Get(_ context.Context, key, version string) (*domain.PromptTemplate, error) {
	s.gotKey = key
	s.gotVersion = version
	if s.err != nil {
		return nil, s.err
	}
	return s.prompt, nil
}

type recordingLLMClient struct {
	response *llminfra.GenerateResponse
	err      error
	gotReq   llminfra.GenerateRequest
}

type stubLLMProfileResolver struct {
	profile *domain.LLMProfile
	err     error
	gotName string
}

func (s *stubLLMProfileResolver) GetByName(_ context.Context, name string) (*domain.LLMProfile, error) {
	s.gotName = name
	if s.err != nil {
		return nil, s.err
	}
	return s.profile, nil
}

func (c *recordingLLMClient) Generate(_ context.Context, req llminfra.GenerateRequest) (*llminfra.GenerateResponse, error) {
	c.gotReq = req
	if c.err != nil {
		return nil, c.err
	}
	return c.response, nil
}

func TestRewriteStageExecutorExecutesPromptAndDecodesOutput(t *testing.T) {
	prompts := &stubRewritePromptRegistry{prompt: &domain.PromptTemplate{
		Key:            "extract_facts",
		Version:        "v1",
		SystemTemplate: "sys {{title}}",
		UserTemplate:   "Title: {{title}}",
	}}
	client := &recordingLLMClient{response: &llminfra.GenerateResponse{Response: &domain.LLMResponse{
		Content:          `{"body":"rewritten"}`,
		Model:            "mock-1",
		PromptTokens:     11,
		CompletionTokens: 7,
		FinishReason:     "stop",
	}}}

	executor := NewRewriteStageExecutor(prompts, client, NewQualityGateEngine())
	stage := domain.RewriteStageDefinition{Name: "generate_draft", PromptRef: "extract_facts@v1", ModelProfileRef: "gpt-4.1-mini"}

	result, err := executor.Execute(t.Context(), stage, StageExecutionInput{Vars: map[string]any{"title": "Hello"}})
	require.NoError(t, err)
	require.Equal(t, "extract_facts", prompts.gotKey)
	require.Equal(t, "v1", prompts.gotVersion)
	require.Equal(t, "rewritten", result.StructuredOutput["body"])
	require.Equal(t, "sys Hello", result.PromptSnapshot.System)
	require.Equal(t, "Title: Hello", result.PromptSnapshot.User)
	require.Equal(t, QualityDecisionPass, result.Quality.Action)
	require.NotNil(t, result.Response)
	require.Equal(t, `{"body":"rewritten"}`, result.Response.Content)
	require.Len(t, client.gotReq.Messages, 2)
	require.Equal(t, "system", client.gotReq.Messages[0].Role)
	require.Equal(t, "sys Hello", client.gotReq.Messages[0].Content)
	require.Equal(t, "user", client.gotReq.Messages[1].Role)
	require.Equal(t, "Title: Hello", client.gotReq.Messages[1].Content)
	require.Equal(t, "gpt-4.1-mini", client.gotReq.Options.Model)
	require.Equal(t, "generate_draft", client.gotReq.Metadata["stage_name"])
	require.Equal(t, "extract_facts@v1", client.gotReq.Metadata["prompt_ref"])
}

func TestRewriteStageExecutorReturnsValidationErrorForInvalidPromptRef(t *testing.T) {
	executor := NewRewriteStageExecutor(&stubRewritePromptRegistry{}, &recordingLLMClient{}, NewQualityGateEngine())

	result, err := executor.Execute(t.Context(), domain.RewriteStageDefinition{Name: "generate_draft", PromptRef: "extract_facts"}, StageExecutionInput{})
	require.Nil(t, result)
	require.Error(t, err)

	appErr, ok := err.(*domain.AppError)
	require.True(t, ok)
	require.Equal(t, domain.ErrValidation, appErr.Code)
	require.Contains(t, appErr.Message, "prompt ref")
}

func TestRewriteStageExecutorReturnsQualityDecisionFromGate(t *testing.T) {
	prompts := &stubRewritePromptRegistry{prompt: &domain.PromptTemplate{
		Key:            "extract_facts",
		Version:        "v1",
		SystemTemplate: "sys",
		UserTemplate:   "Title: {{title}}",
	}}
	client := &recordingLLMClient{response: &llminfra.GenerateResponse{Response: &domain.LLMResponse{
		Content: `{"body":"no"}`,
		Model:   "mock-1",
	}}}

	executor := NewRewriteStageExecutor(prompts, client, NewQualityGateEngine())
	stage := domain.RewriteStageDefinition{Name: "generate_draft", PromptRef: "extract_facts@v1"}

	result, err := executor.Execute(t.Context(), stage, StageExecutionInput{
		Vars:      map[string]any{"title": "Hello"},
		MinLength: 5,
	})
	require.NoError(t, err)
	require.Equal(t, QualityDecisionRepair, result.Quality.Action)
	require.Contains(t, result.Quality.Message, "too short")
}

func TestRewriteStageExecutorResolvesModelProfileRefToLLMOptions(t *testing.T) {
	prompts := &stubRewritePromptRegistry{prompt: &domain.PromptTemplate{
		Key:            "extract_facts",
		Version:        "v1",
		SystemTemplate: "sys",
		UserTemplate:   "Title: {{title}}",
	}}
	profiles := &stubLLMProfileResolver{profile: &domain.LLMProfile{
		Name:        "rewrite-default",
		Model:       "gpt-4.1-mini-real",
		Temperature: 0.4,
		MaxTokens:   512,
	}}
	client := &recordingLLMClient{response: &llminfra.GenerateResponse{Response: &domain.LLMResponse{
		Content: `{"body":"rewritten"}`,
		Model:   "gpt-4.1-mini-real",
	}}}

	executor := NewRewriteStageExecutorWithProfileResolver(prompts, profiles, client, NewQualityGateEngine())
	stage := domain.RewriteStageDefinition{Name: "generate_draft", PromptRef: "extract_facts@v1", ModelProfileRef: "rewrite-default"}

	result, err := executor.Execute(t.Context(), stage, StageExecutionInput{Vars: map[string]any{"title": "Hello"}})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "rewrite-default", profiles.gotName)
	require.Equal(t, "gpt-4.1-mini-real", client.gotReq.Options.Model)
	require.Equal(t, 0.4, client.gotReq.Options.Temperature)
	require.Equal(t, 512, client.gotReq.Options.MaxTokens)
}
