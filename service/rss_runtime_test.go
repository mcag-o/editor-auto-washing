package service

import "testing"

func TestBuildRSSRuntimeReturnsReadyServices(t *testing.T) {
	repos, cleanup, err := BuildRuntimeRepos(t.TempDir())
	if cleanup != nil {
		defer func() {
			if closeErr := cleanup(); closeErr != nil {
				t.Fatalf("cleanup returned error: %v", closeErr)
			}
		}()
	}
	if err != nil {
		t.Fatalf("BuildRuntimeRepos error: %v", err)
	}

	runtime, err := BuildRSSRuntime(repos)
	if err != nil {
		t.Fatalf("BuildRSSRuntime error: %v", err)
	}
	if runtime == nil {
		t.Fatal("expected rss runtime to be configured")
	}
	if runtime.PullService == nil || runtime.SubscriptionService == nil {
		t.Fatal("expected RSS runtime services to be configured")
	}
	if runtime.ArticleIntakeService == nil || runtime.Scheduler == nil {
		t.Fatal("expected RSS runtime assembly to expose intake and scheduler")
	}
	if runtime.PullRunReader == nil || runtime.ItemReader == nil {
		t.Fatal("expected RSS runtime assembly to expose run and item readers")
	}
	if runtime.Scheduler.subscriptions != runtime.SubscriptionService {
		t.Fatal("expected scheduler to use runtime subscription service")
	}
	if runtime.Scheduler.puller != runtime.PullService {
		t.Fatal("expected scheduler to use runtime pull service")
	}
	if runtime.PullService.intake != runtime.ArticleIntakeService {
		t.Fatal("expected pull service to use runtime article intake service")
	}
	if runtime.PullRunReader != repos.RSSPullRunRepo {
		t.Fatal("expected runtime pull run reader to come from runtime repos")
	}
	if runtime.ItemReader != repos.RSSItemRepo {
		t.Fatal("expected runtime item reader to come from runtime repos")
	}
}
