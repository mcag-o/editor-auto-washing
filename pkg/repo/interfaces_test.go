package repo

import (
	"context"
	"testing"

	"content-hub/domain"
)

type staticLLMProvider struct{}

func (staticLLMProvider) Generate(_ context.Context, _ domain.LLMGenerateRequest) (*domain.LLMGenerateResponse, error) {
	return &domain.LLMGenerateResponse{}, nil
}

func (staticLLMProvider) Models(_ context.Context) ([]string, error) {
	return []string{"mock-1"}, nil
}

func (staticLLMProvider) Name() string {
	return "static"
}

func TestLLMProviderUsesLLMClientContract(t *testing.T) {
	var _ LLMProvider = staticLLMProvider{}
}
