package llm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStaticClientReturnsConfiguredResponse(t *testing.T) {
	client := StaticClient{Response: []byte(`{"headline":"ok"}`), Model: "mock-1"}

	resp, err := client.Generate(context.Background(), GenerateRequest{SystemPrompt: "sys", UserPrompt: "user"})
	require.NoError(t, err)
	require.Equal(t, "mock-1", resp.Model)
	require.JSONEq(t, `{"headline":"ok"}`, string(resp.Raw))
}

func TestStaticClientReturnsConfiguredError(t *testing.T) {
	client := StaticClient{Err: context.DeadlineExceeded}

	resp, err := client.Generate(context.Background(), GenerateRequest{SystemPrompt: "sys", UserPrompt: "user"})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, resp)
}
