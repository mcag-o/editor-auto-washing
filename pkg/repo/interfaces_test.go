package repo

import (
	"context"
	"testing"

	"content-hub/domain"
)

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
