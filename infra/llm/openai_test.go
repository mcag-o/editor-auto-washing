package llm

import (
	"content-hub/domain"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProviderImplementsClient(t *testing.T) {
	var _ Client = NewProvider("https://api.openai.com/v1", "sk-test", "gpt-4", 30*time.Second)
}

func TestNewProvider(t *testing.T) {
	p := NewProvider("https://api.openai.com/v1", "sk-test", "gpt-4", 30*time.Second)

	if p == nil {
		t.Fatal("NewProvider returned nil")
	}
	if p.BaseURL() != "https://api.openai.com/v1" {
		t.Errorf("expected baseURL %q, got %q", "https://api.openai.com/v1", p.BaseURL())
	}
	if p.Model() != "gpt-4" {
		t.Errorf("expected model %q, got %q", "gpt-4", p.Model())
	}
	if p.Timeout() != 30*time.Second {
		t.Errorf("expected timeout %v, got %v", 30*time.Second, p.Timeout())
	}
}

func TestProviderName(t *testing.T) {
	p := NewProvider("https://api.example.com/v1", "sk-test", "gpt-3.5", 10*time.Second)
	if got := p.Name(); got != "openai-compatible" {
		t.Errorf("expected Name() %q, got %q", "openai-compatible", got)
	}
}

func TestProviderEmptyAPIKey(t *testing.T) {
	p := NewProvider("https://api.openai.com/v1", "", "gpt-4", 30*time.Second)

	_, err := p.Generate(nil, GenerateRequest{Options: domain.LLMOptions{Model: "gpt-4"}})
	require.EqualError(t, err, "LLM API key not configured")
}

func TestProviderEmptyAPIKeyStream(t *testing.T) {
	p := NewProvider("https://api.openai.com/v1", "", "gpt-4", 30*time.Second)

	err := p.GenerateStream(nil, GenerateRequest{Options: domain.LLMOptions{Model: "gpt-4"}}, func(s string) error { return nil })
	require.EqualError(t, err, "LLM API key not configured")
}

func TestProviderGenerateMapsFullSharedResponse(t *testing.T) {
	p := NewProvider("https://api.openai.com/v1", "sk-test", "gpt-4", 30*time.Second)
	p.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "Bearer sk-test", req.Header.Get("Authorization"))
		require.Equal(t, "application/json", req.Header.Get("Content-Type"))

		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		require.JSONEq(t, `{"model":"gpt-4.1-mini","messages":[{"role":"user","content":"hello"}],"temperature":0.2,"max_tokens":64,"stream":false}`, string(body))

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"choices":[{"message":{"content":"hi"},"finish_reason":"stop"}],
				"usage":{"prompt_tokens":12,"completion_tokens":5}
			}`)),
		}, nil
	})}

	resp, err := p.Generate(t.Context(), GenerateRequest{
		Messages: []domain.ChatMessage{{Role: "user", Content: "hello"}},
		Options:  domain.LLMOptions{Model: "gpt-4.1-mini", Temperature: 0.2, MaxTokens: 64},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Response)
	require.Equal(t, "hi", resp.Response.Content)
	require.Equal(t, "gpt-4.1-mini", resp.Response.Model)
	require.Equal(t, 12, resp.Response.PromptTokens)
	require.Equal(t, 5, resp.Response.CompletionTokens)
	require.Equal(t, "stop", resp.Response.FinishReason)
}

func TestProviderDefaults(t *testing.T) {
	p := NewProvider("http://localhost:8080", "sk-123", "llama-3", 60*time.Second)

	if p.Name() != "openai-compatible" {
		t.Errorf("unexpected name: %s", p.Name())
	}
	if p.BaseURL() != "http://localhost:8080" {
		t.Errorf("unexpected baseURL: %s", p.BaseURL())
	}
	if p.Model() != "llama-3" {
		t.Errorf("unexpected model: %s", p.Model())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
