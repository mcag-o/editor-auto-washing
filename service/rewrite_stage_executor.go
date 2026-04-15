package service

import (
	"content-hub/domain"
	llminfra "content-hub/infra/llm"
	"context"
	"fmt"
	"strings"
)

type rewriteStagePromptRegistry interface {
	Get(ctx context.Context, key, version string) (*domain.PromptTemplate, error)
}

type StageExecutionInput struct {
	Vars      map[string]any
	MinLength int
	MaxLength int
}

type StagePromptSnapshot struct {
	Key     string
	Version string
	System  string
	User    string
}

type StageResponseInfo struct {
	Content          string
	Model            string
	PromptTokens     int
	CompletionTokens int
	FinishReason     string
}

type StageExecutionResult struct {
	StructuredOutput map[string]any
	PromptSnapshot   StagePromptSnapshot
	Response         *StageResponseInfo
	Quality          QualityResult
}

type RewriteStageExecutor struct {
	prompts rewriteStagePromptRegistry
	client  llminfra.Client
	quality *QualityGateEngine
}

func NewRewriteStageExecutor(prompts rewriteStagePromptRegistry, client llminfra.Client, quality *QualityGateEngine) *RewriteStageExecutor {
	return &RewriteStageExecutor{
		prompts: prompts,
		client:  client,
		quality: quality,
	}
}

func (e *RewriteStageExecutor) Execute(ctx context.Context, stage domain.RewriteStageDefinition, input StageExecutionInput) (*StageExecutionResult, error) {
	promptKey, promptVersion, err := parsePromptRef(stage.PromptRef)
	if err != nil {
		return nil, err
	}

	prompt, err := e.prompts.Get(ctx, promptKey, promptVersion)
	if err != nil {
		return nil, fmt.Errorf("load prompt template: %w", err)
	}
	if prompt == nil {
		return nil, domain.NewNotFoundErr("prompt template", stage.PromptRef)
	}

	systemPrompt, err := llminfra.RenderPrompt(prompt.SystemTemplate, input.Vars)
	if err != nil {
		return nil, domain.NewValidationErr("render system prompt", err)
	}

	userPrompt, err := llminfra.RenderPrompt(prompt.UserTemplate, input.Vars)
	if err != nil {
		return nil, domain.NewValidationErr("render user prompt", err)
	}

	resp, err := e.client.Generate(ctx, llminfra.GenerateRequest{
		Messages: []domain.ChatMessage{{Role: "system", Content: systemPrompt}, {Role: "user", Content: userPrompt}},
		Options:  domain.LLMOptions{Model: stage.ModelProfileRef},
		Metadata: map[string]any{
			"stage_name": stage.Name,
			"prompt_ref": stage.PromptRef,
		},
	})
	if err != nil {
		return nil, domain.NewExternalErr("generate rewrite stage output", err)
	}
	if resp == nil || resp.Response == nil {
		return nil, domain.NewExternalErr("generate rewrite stage output", fmt.Errorf("missing llm response"))
	}

	structuredOutput, err := llminfra.DecodeJSONMap([]byte(resp.Response.Content))
	if err != nil {
		return nil, domain.NewValidationErr("decode rewrite stage output", err)
	}

	quality := e.quality.Evaluate(QualityInput{
		StructuredOutput: structuredOutput,
		MinLength:        input.MinLength,
		MaxLength:        input.MaxLength,
	})

	return &StageExecutionResult{
		StructuredOutput: structuredOutput,
		PromptSnapshot: StagePromptSnapshot{
			Key:     prompt.Key,
			Version: prompt.Version,
			System:  systemPrompt,
			User:    userPrompt,
		},
		Response: &StageResponseInfo{
			Content:          resp.Response.Content,
			Model:            resp.Response.Model,
			PromptTokens:     resp.Response.PromptTokens,
			CompletionTokens: resp.Response.CompletionTokens,
			FinishReason:     resp.Response.FinishReason,
		},
		Quality: quality,
	}, nil
}

func parsePromptRef(promptRef string) (string, string, error) {
	key, version, ok := strings.Cut(strings.TrimSpace(promptRef), "@")
	if !ok || strings.TrimSpace(key) == "" || strings.TrimSpace(version) == "" || strings.Contains(version, "@") {
		return "", "", domain.NewValidationErr("prompt ref must use key@version format", nil)
	}

	return strings.TrimSpace(key), strings.TrimSpace(version), nil
}
