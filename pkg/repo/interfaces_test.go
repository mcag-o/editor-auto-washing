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

type rssSubscriptionRepoCompileStub struct{}

func (rssSubscriptionRepoCompileStub) Create(context.Context, *domain.RSSSubscription) error {
	return nil
}
func (rssSubscriptionRepoCompileStub) Update(context.Context, *domain.RSSSubscription) error {
	return nil
}
func (rssSubscriptionRepoCompileStub) Delete(context.Context, string) error { return nil }
func (rssSubscriptionRepoCompileStub) GetByID(context.Context, string) (*domain.RSSSubscription, error) {
	return nil, nil
}
func (rssSubscriptionRepoCompileStub) List(context.Context) ([]domain.RSSSubscription, error) {
	return nil, nil
}

type rssPullRunRepoCompileStub struct{}

func (rssPullRunRepoCompileStub) Create(context.Context, *domain.RSSPullRun) error { return nil }
func (rssPullRunRepoCompileStub) Update(context.Context, *domain.RSSPullRun) error { return nil }
func (rssPullRunRepoCompileStub) GetByID(context.Context, string) (*domain.RSSPullRun, error) {
	return nil, nil
}
func (rssPullRunRepoCompileStub) List(context.Context, int) ([]domain.RSSPullRun, error) {
	return nil, nil
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

func TestRSSSubscriptionRepoUsesCompileContract(t *testing.T) {
	var _ RSSSubscriptionRepo = rssSubscriptionRepoCompileStub{}
}

func TestRSSPullRunRepoUsesCompileContract(t *testing.T) {
	var _ RSSPullRunRepo = rssPullRunRepoCompileStub{}
}

func TestLLMProviderUsesLLMClientContract(t *testing.T) {
	var _ LLMProvider = staticLLMProvider{}
}
