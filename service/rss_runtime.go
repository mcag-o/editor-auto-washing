package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type RSSRuntime struct {
	SubscriptionService  *RSSSubscriptionService
	PullService          *RSSPullService
	ArticleIntakeService *ArticleIntakeService
	Scheduler            *RSSScheduler
	PullRunReader        repo.RSSPullRunRepo
	ItemReader           repo.RSSItemRepo
}

func BuildRSSRuntime(repos *RuntimeRepos) (*RSSRuntime, error) {
	if repos == nil {
		return nil, domain.NewInternalErr("rss runtime repos are required", nil)
	}

	subscriptionService := NewRSSSubscriptionService(repos.RSSSubscriptionRepo)
	rewriteAssembly := buildRewriteAssembly(repos)
	articleIntakeService := NewArticleIntakeService(repos.WorkspaceRepo, rewriteAssembly.orchestrator)
	pullService := NewRSSPullService(newRSSHTTPFeedFetcher(30*time.Second), repos.RSSPullRunRepo, repos.RSSItemRepo, articleIntakeService)
	scheduler := NewRSSScheduler(subscriptionService, pullService)

	return &RSSRuntime{
		SubscriptionService:  subscriptionService,
		PullService:          pullService,
		ArticleIntakeService: articleIntakeService,
		Scheduler:            scheduler,
		PullRunReader:        repos.RSSPullRunRepo,
		ItemReader:           repos.RSSItemRepo,
	}, nil
}

type rssHTTPFeedFetcher struct {
	client *http.Client
}

func newRSSHTTPFeedFetcher(timeout time.Duration) *rssHTTPFeedFetcher {
	return &rssHTTPFeedFetcher{client: &http.Client{Timeout: timeout}}
}

func (f *rssHTTPFeedFetcher) Fetch(ctx context.Context, feedURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build rss request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request rss feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("request rss feed: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read rss feed response: %w", err)
	}
	return body, nil
}
