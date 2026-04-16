package service

import (
	"content-hub/domain"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubRSSSubscriptionRepo struct {
	created []*domain.RSSSubscription
	updated []*domain.RSSSubscription
	items   map[string]*domain.RSSSubscription
	listErr error
	getErr  error
	err     error
	deleted []string
}

func (r *stubRSSSubscriptionRepo) Create(_ context.Context, subscription *domain.RSSSubscription) error {
	if r.err != nil {
		return r.err
	}
	if r.items == nil {
		r.items = map[string]*domain.RSSSubscription{}
	}
	r.created = append(r.created, subscription)
	r.items[subscription.ID] = subscription
	return nil
}

func (r *stubRSSSubscriptionRepo) Update(_ context.Context, subscription *domain.RSSSubscription) error {
	if r.err != nil {
		return r.err
	}
	if r.items == nil {
		r.items = map[string]*domain.RSSSubscription{}
	}
	r.updated = append(r.updated, subscription)
	r.items[subscription.ID] = subscription
	return nil
}

func (r *stubRSSSubscriptionRepo) Delete(_ context.Context, id string) error {
	if r.err != nil {
		return r.err
	}
	r.deleted = append(r.deleted, id)
	delete(r.items, id)
	return nil
}

func (r *stubRSSSubscriptionRepo) GetByID(_ context.Context, id string) (*domain.RSSSubscription, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	return r.items[id], nil
}

func (r *stubRSSSubscriptionRepo) List(_ context.Context) ([]domain.RSSSubscription, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	items := make([]domain.RSSSubscription, 0, len(r.items))
	for _, item := range r.items {
		items = append(items, *item)
	}
	return items, nil
}

func TestRSSSubscriptionServiceCreatePersistsSubscription(t *testing.T) {
	repo := &stubRSSSubscriptionRepo{}
	svc := NewRSSSubscriptionService(repo)

	sub, err := svc.Create(context.Background(), domain.NewRSSSubscription("Tech", "https://example.com/feed.xml", "wechat-longform", "sspai"))

	require.NoError(t, err)
	require.NotNil(t, sub)
	assert.Equal(t, "Tech", sub.Name)
	require.Len(t, repo.created, 1)
	assert.Same(t, sub, repo.created[0])
	require.Contains(t, repo.items, sub.ID)
	assert.Equal(t, "https://example.com/feed.xml", repo.items[sub.ID].FeedURL)
}

func TestRSSSubscriptionServiceListReturnsStoredValues(t *testing.T) {
	first := domain.NewRSSSubscription("Tech", "https://example.com/feed.xml", "wechat-longform", "sspai")
	second := domain.NewRSSSubscription("News", "https://example.com/news.xml", "wechat-longform", "infoq")
	repo := &stubRSSSubscriptionRepo{items: map[string]*domain.RSSSubscription{first.ID: first, second.ID: second}}
	svc := NewRSSSubscriptionService(repo)

	items, err := svc.List(context.Background())

	require.NoError(t, err)
	assert.Len(t, items, 2)
	assert.ElementsMatch(t, []string{"Tech", "News"}, []string{items[0].Name, items[1].Name})
}
