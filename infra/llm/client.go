package llm

import (
	"content-hub/domain"
	"context"
)

type GenerateRequest = domain.LLMGenerateRequest

type GenerateResponse = domain.LLMGenerateResponse

type Client interface {
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)
}

type StaticClient struct {
	Response domain.LLMResponse
	Err      error
}

func (c StaticClient) Generate(_ context.Context, _ GenerateRequest) (*GenerateResponse, error) {
	if c.Err != nil {
		return nil, c.Err
	}

	return &GenerateResponse{
		Response: &domain.LLMResponse{
			Content:          c.Response.Content,
			Model:            c.Response.Model,
			PromptTokens:     c.Response.PromptTokens,
			CompletionTokens: c.Response.CompletionTokens,
			FinishReason:     c.Response.FinishReason,
		},
	}, nil
}
