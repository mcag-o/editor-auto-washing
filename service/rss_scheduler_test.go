package service

import (
	"content-hub/domain"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubRSSSchedulerSubscriptionReader struct {
	getByID   map[string]*domain.RSSSubscription
	list      []domain.RSSSubscription
	getErr    error
	listErr   error
	requested []string
}

func (r *stubRSSSchedulerSubscriptionReader) Get(ctx context.Context, id string) (*domain.RSSSubscription, error) {
	r.requested = append(r.requested, id)
	if r.getErr != nil {
		return nil, r.getErr
	}
	if sub, ok := r.getByID[id]; ok {
		copyValue := *sub
		return &copyValue, nil
	}
	return nil, domain.NewNotFoundErr("rss subscription", id)
}

func (r *stubRSSSchedulerSubscriptionReader) List(context.Context) ([]domain.RSSSubscription, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	result := make([]domain.RSSSubscription, len(r.list))
	copy(result, r.list)
	return result, nil
}

type stubRSSSchedulerPullRunner struct {
	runs   []string
	err    error
	byID   map[string]*RSSPullResult
	byErr  map[string]error
	result *RSSPullResult
}

func (r *stubRSSSchedulerPullRunner) RunOnce(ctx context.Context, sub domain.RSSSubscription) (*RSSPullResult, error) {
	r.runs = append(r.runs, sub.ID)
	if err, ok := r.byErr[sub.ID]; ok {
		return nil, err
	}
	if result, ok := r.byID[sub.ID]; ok {
		return result, nil
	}
	if r.err != nil {
		return nil, r.err
	}
	if r.result != nil {
		return r.result, nil
	}
	return &RSSPullResult{Run: domain.NewRSSPullRun(sub.ID)}, nil
}

func TestRSSSchedulerRunOneByIDRunsRequestedSubscription(t *testing.T) {
	enabled := domain.NewRSSSubscription("Tech", "https://example.com/feed.xml", "wechat-longform", "sspai")
	reader := &stubRSSSchedulerSubscriptionReader{getByID: map[string]*domain.RSSSubscription{enabled.ID: enabled}}
	puller := &stubRSSSchedulerPullRunner{}
	scheduler := NewRSSScheduler(reader, puller)

	result, err := scheduler.RunByID(t.Context(), enabled.ID)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, []string{enabled.ID}, reader.requested)
	require.Equal(t, []string{enabled.ID}, puller.runs)
}

func TestRSSSchedulerRunAllOnlyProcessesEnabledSubscriptions(t *testing.T) {
	enabled := domain.NewRSSSubscription("Enabled", "https://example.com/enabled.xml", "wechat-longform", "sspai")
	disabled := domain.NewRSSSubscription("Disabled", "https://example.com/disabled.xml", "wechat-longform", "sspai")
	disabled.Enabled = false
	reader := &stubRSSSchedulerSubscriptionReader{list: []domain.RSSSubscription{*enabled, *disabled}}
	puller := &stubRSSSchedulerPullRunner{}
	scheduler := NewRSSScheduler(reader, puller)

	results, err := scheduler.RunAll(t.Context())

	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, enabled.ID, results[0].SubscriptionID)
	require.Equal(t, []string{enabled.ID}, puller.runs)
}

func TestRSSSchedulerRunAllReturnsPullErrorForEnabledSubscription(t *testing.T) {
	enabled := domain.NewRSSSubscription("Enabled", "https://example.com/enabled.xml", "wechat-longform", "sspai")
	reader := &stubRSSSchedulerSubscriptionReader{list: []domain.RSSSubscription{*enabled}}
	puller := &stubRSSSchedulerPullRunner{byErr: map[string]error{enabled.ID: errors.New("pull failed")}}
	scheduler := NewRSSScheduler(reader, puller)

	results, err := scheduler.RunAll(t.Context())

	require.Error(t, err)
	require.ErrorContains(t, err, "pull failed")
	require.Len(t, results, 1)
	require.Equal(t, enabled.ID, results[0].SubscriptionID)
	require.ErrorContains(t, results[0].Err, "pull failed")
}
