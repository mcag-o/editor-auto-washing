package service

import (
	"content-hub/domain"
	"context"
	"errors"
	"fmt"
	"strings"
)

type rssSchedulerSubscriptionReader interface {
	Get(ctx context.Context, id string) (*domain.RSSSubscription, error)
	List(ctx context.Context) ([]domain.RSSSubscription, error)
}

type rssSchedulerPullRunner interface {
	RunOnce(ctx context.Context, sub domain.RSSSubscription) (*RSSPullResult, error)
}

type RSSScheduler struct {
	subscriptions rssSchedulerSubscriptionReader
	puller        rssSchedulerPullRunner
}

type RSSScheduledRunResult struct {
	SubscriptionID string
	Result         *RSSPullResult
	Err            error
}

func NewRSSScheduler(subscriptions rssSchedulerSubscriptionReader, puller rssSchedulerPullRunner) *RSSScheduler {
	return &RSSScheduler{subscriptions: subscriptions, puller: puller}
}

func (s *RSSScheduler) RunByID(ctx context.Context, subscriptionID string) (*RSSPullResult, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}

	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" {
		return nil, domain.NewValidationErr("subscription id is required", nil)
	}

	sub, err := s.subscriptions.Get(ctx, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("get rss subscription: %w", err)
	}

	result, err := s.puller.RunOnce(ctx, *sub)
	if err != nil {
		return result, fmt.Errorf("run rss subscription %s: %w", subscriptionID, err)
	}
	return result, nil
}

func (s *RSSScheduler) RunAll(ctx context.Context) ([]RSSScheduledRunResult, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}

	subscriptions, err := s.subscriptions.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list rss subscriptions: %w", err)
	}

	results := make([]RSSScheduledRunResult, 0, len(subscriptions))
	var runErrs []error
	for _, sub := range subscriptions {
		if !sub.Enabled {
			continue
		}

		pullResult, runErr := s.puller.RunOnce(ctx, sub)
		results = append(results, RSSScheduledRunResult{
			SubscriptionID: sub.ID,
			Result:         pullResult,
			Err:            runErr,
		})
		if runErr != nil {
			runErrs = append(runErrs, fmt.Errorf("run rss subscription %s: %w", sub.ID, runErr))
		}
	}

	if len(runErrs) > 0 {
		return results, errors.Join(runErrs...)
	}
	return results, nil
}

func (s *RSSScheduler) validate() error {
	if s.subscriptions == nil || s.puller == nil {
		return domain.NewInternalErr("rss scheduler is not configured", nil)
	}
	return nil
}
