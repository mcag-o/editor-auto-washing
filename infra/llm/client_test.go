package llm

import (
	"content-hub/domain"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStaticClientReturnsConfiguredResponse(t *testing.T) {
	client := StaticClient{
		Response: domain.LLMResponse{
			Content:          `{"headline":"ok"}`,
			Model:            "mock-1",
			PromptTokens:     11,
			CompletionTokens: 7,
			FinishReason:     "stop",
		},
	}

	resp, err := client.Generate(context.Background(), GenerateRequest{
		Messages: []domain.ChatMessage{{Role: "system", Content: "sys"}, {Role: "user", Content: "user"}},
		Options:  domain.LLMOptions{Model: "mock-1", Temperature: 0.2, MaxTokens: 256},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Response)
	require.Equal(t, "mock-1", resp.Response.Model)
	require.JSONEq(t, `{"headline":"ok"}`, resp.Response.Content)
	require.Equal(t, 11, resp.Response.PromptTokens)
	require.Equal(t, 7, resp.Response.CompletionTokens)
	require.Equal(t, "stop", resp.Response.FinishReason)
}

func TestStaticClientReturnsConfiguredError(t *testing.T) {
	client := StaticClient{Err: context.DeadlineExceeded}

	resp, err := client.Generate(context.Background(), GenerateRequest{
		Messages: []domain.ChatMessage{{Role: "user", Content: "user"}},
		Options:  domain.LLMOptions{Model: "mock-1"},
	})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, resp)
}
