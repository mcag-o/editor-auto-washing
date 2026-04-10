package service

import (
	collectorscheduler "content-hub/collector/scheduler"
	collectorservice "content-hub/collector/service"
	"context"
	"time"
)

type CollectorRuntime struct {
	RegistryService     *collectorservice.SourceRegistryService
	RunService          *collectorservice.RunService
	ArticleFetchService *collectorservice.ArticleFetchService
	BridgeService       *collectorservice.BridgeService
	SchedulerService    *collectorscheduler.Service
}

func BuildCollectorRuntime(ctx context.Context, repos *RuntimeRepos, interval time.Duration) (*CollectorRuntime, error) {
	registry, err := collectorservice.NewDefaultRegistry()
	if err != nil {
		return nil, err
	}
	registrySvc := collectorservice.NewSourceRegistryService(repos.CollectorSourceRepo, registry)
	if err := registrySvc.Sync(ctx); err != nil {
		return nil, err
	}
	runSvc := collectorservice.NewRunService(repos.CollectorSourceRepo, repos.CollectorRunRepo, repos.CollectorEntryRepo, registry)
	articleFetchSvc := collectorservice.NewArticleFetchService(repos.CollectorEntryRepo, repos.CollectorArticleRepo, repos.CollectorAttemptRepo, repos.CollectorRunRepo, registry)
	bridgeSvc := collectorservice.NewBridgeService(repos.CollectorArticleRepo, repos.WorkspaceRepo)
	schedulerSvc := collectorscheduler.NewService(repos.CollectorSchedulerRepo, runSvc, interval)
	return &CollectorRuntime{RegistryService: registrySvc, RunService: runSvc, ArticleFetchService: articleFetchSvc, BridgeService: bridgeSvc, SchedulerService: schedulerSvc}, nil
}
