package service

import (
	"content-hub/domain"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubPromptTemplateRepo struct {
	prompt    *domain.PromptTemplate
	getErr    error
	gotKey    string
	gotVer    string
	listErr   error
	upsertErr error
}

func (s *stubPromptTemplateRepo) Upsert(ctx context.Context, prompt *domain.PromptTemplate) error {
	return s.upsertErr
}

func (s *stubPromptTemplateRepo) Get(ctx context.Context, key, version string) (*domain.PromptTemplate, error) {
	s.gotKey = key
	s.gotVer = version
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.prompt, nil
}

func (s *stubPromptTemplateRepo) List(ctx context.Context) ([]domain.PromptTemplate, error) {
	return nil, s.listErr
}

func TestPromptRegistryGetReturnsStoredPrompt(t *testing.T) {
	repo := &stubPromptTemplateRepo{prompt: &domain.PromptTemplate{Key: "generate_draft", Version: "v1", SystemTemplate: "sys"}}
	registry := NewPromptRegistry(repo)

	prompt, err := registry.Get(t.Context(), "generate_draft", "v1")
	require.NoError(t, err)
	require.Equal(t, "sys", prompt.SystemTemplate)
	require.Equal(t, "generate_draft", repo.gotKey)
	require.Equal(t, "v1", repo.gotVer)
}

func TestPromptRegistryGetPropagatesRepoError(t *testing.T) {
	repo := &stubPromptTemplateRepo{getErr: errors.New("boom")}
	registry := NewPromptRegistry(repo)

	prompt, err := registry.Get(t.Context(), "generate_draft", "v1")
	require.Error(t, err)
	require.Nil(t, prompt)
	require.EqualError(t, err, "boom")
}
