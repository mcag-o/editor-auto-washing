package llm

import (
	"testing"
	"time"
)

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

	_, err := p.Generate(nil, nil, LLMOptions{})
	if err == nil {
		t.Fatal("expected error for empty API key")
	}
	if got := err.Error(); got != "LLM API key not configured" {
		t.Errorf("expected error %q, got %q", "LLM API key not configured", got)
	}
}

func TestProviderEmptyAPIKeyStream(t *testing.T) {
	p := NewProvider("https://api.openai.com/v1", "", "gpt-4", 30*time.Second)

	err := p.GenerateStream(nil, nil, LLMOptions{}, func(s string) error { return nil })
	if err == nil {
		t.Fatal("expected error for empty API key in stream mode")
	}
	if got := err.Error(); got != "LLM API key not configured" {
		t.Errorf("expected error %q, got %q", "LLM API key not configured", got)
	}
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
