package service

import (
	collectorscheduler "content-hub/collector/scheduler"
	collectorservice "content-hub/collector/service"
	"content-hub/infra/config"
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

// BuildCollectorRuntime 负责把平台注册、同步、任务服务和调度器组装成统一运行时。
//
// 当前重构重点：
// 1. 平台元数据改由外部配置驱动，确保 22 个目标平台全部可见；
// 2. 对尚未开发完成的平台，通过 placeholder plugin 落入统一 registry；
// 3. 为后续把 detail fetch / bridge 暴露成正式运维入口保留稳定装配点。
func BuildCollectorRuntime(ctx context.Context, repos *RuntimeRepos, interval time.Duration) (*CollectorRuntime, error) {
	collectorCfg := config.DefaultConfig().Collector
	registry, err := collectorservice.NewRegistryFromCollectorConfig(collectorCfg)
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
