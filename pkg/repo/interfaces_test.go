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

type rssItemRepoCompileStub struct{}

func (rssItemRepoCompileStub) Create(context.Context, *domain.RSSItemRecord) error { return nil }
func (rssItemRepoCompileStub) Update(context.Context, *domain.RSSItemRecord) error { return nil }
func (rssItemRepoCompileStub) FindDuplicate(context.Context, domain.RSSDuplicateKey) (*domain.RSSItemRecord, error) {
	return nil, nil
}
func (rssItemRepoCompileStub) GetByID(context.Context, string) (*domain.RSSItemRecord, error) {
	return nil, nil
}
func (rssItemRepoCompileStub) List(context.Context, int) ([]domain.RSSItemRecord, error) {
	return nil, nil
}

func TestRSSItemRepoFindDuplicateUsesStructuredKey(t *testing.T) {
	var repo RSSItemRepo = rssItemRepoCompileStub{}
	_, _ = repo.FindDuplicate(t.Context(), domain.RSSDuplicateKey{
		SubscriptionID: "sub-1",
		GUID:           "guid-1",
		Link:           "https://example.com/item",
		ContentHash:    "hash-1",
	})
}

func TestLLMProviderUsesLLMClientContract(t *testing.T) {
	var _ LLMProvider = staticLLMProvider{}
}
