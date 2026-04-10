package main

import (
	"content-hub/domain"
	"content-hub/infra/config"
	workspaceinfra "content-hub/infra/workspace"
	"content-hub/service"
	httpserver "content-hub/transport/http"
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

type serverRunner interface {
	Run() error
}

var buildRuntimeReposFn = buildRuntimeRepos
var newHTTPServer = func(cfg config.Config, provider *httpserver.Provider) serverRunner {
	return httpserver.NewServer(cfg, provider)
}

func main() {
	fmt.Println("content-hub server starting...")
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	loader := config.NewLoader("")
	workspaceConfigSvc := service.NewWorkspaceConfigService(workspaceinfra.NewLoader(), workspaceinfra.NewValidator())
	workspaceRoot := workspaceRootFromEnv()

	var cfg config.Config
	if runtimeCfg, err := workspaceConfigSvc.RuntimeConfig(workspaceRoot); err == nil {
		cfg = runtimeCfg
		loader.SetCurrent(cfg)
	} else {
		// 说明：这里继续保留独立配置文件回退路径，方便在未初始化 workspace 的场景下启动服务。
		// 后续如果所有环境都完成 workspace 化，可以再统一收敛配置入口。
		fallback := config.NewLoader("./config/config.json")
		loadedCfg, loadErr := fallback.Load()
		if loadErr != nil {
			cfg = config.DefaultConfig()
			cfg.ResolveSecrets()
		} else {
			cfg = loadedCfg
		}
		loader.SetCurrent(cfg)
	}

	runtimeRepos, cleanup, err := buildRuntimeReposFn(workspaceRoot)
	if err != nil {
		return err
	}
	defer cleanup()

	contentSvc := service.NewContentService(
		runtimeRepos.ArticleRepo,
		runtimeRepos.PublishRepo,
	)

	templateSvc := service.NewTemplateService(runtimeRepos.TemplateRepo)
	draftSvc := service.NewDraftService(runtimeRepos.DraftRepo)
	formattingSvc := service.NewFormattingPipelineService(runtimeRepos.DraftRepo, runtimeRepos.AssetRepo, runtimeRepos.WorkspaceRepo, runtimeRepos.Formatter).WithRenderedDir(runtimeRepos.RenderedDir)
	ingestionSvc := service.NewIngestionPipelineService(runtimeRepos.IngestionRepo, runtimeRepos.WorkspaceRepo, runtimeRepos.BundleImportTxStarter, workspaceinfra.NewLoader())
	automationSvc := service.NewAutomationService(workspaceConfigSvc, ingestionSvc, nil)
	workspaceSvc := service.NewWorkspaceArticleService(runtimeRepos.WorkspaceRepo)
	reviewSvc := service.NewReviewService(runtimeRepos.ReviewRepo, runtimeRepos.WorkspaceRepo)
	publishSvc := service.NewPublishGateService(runtimeRepos.ReviewRepo, runtimeRepos.AssetRepo, runtimeRepos.DraftRepo, runtimeRepos.PublishRepo, runtimeRepos.WorkspaceRepo, map[string]service.PublisherProvider{"wechat": runtimePublishProvider{}})
	collectorRuntime, err := service.BuildCollectorRuntime(context.Background(), runtimeRepos, 30*time.Minute)
	if err != nil {
		return err
	}

	workflowEngine := service.BuildDefaultWorkflowEngine(workspaceRoot, automationSvc)
	jobSvc := service.NewJobService(
		runtimeRepos.JobRepo,
		runtimeRepos.JobEventRepo,
		&jobExecutor{engine: workflowEngine},
	)
	automationSvc.SetJobService(jobSvc)

	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()
	var workerWG sync.WaitGroup
	workerWG.Add(1)
	go func() {
		defer workerWG.Done()
		jobSvc.RunWorker(workerCtx)
	}()

	serverProvider := &httpserver.Provider{
		ContentSvc:         contentSvc,
		TemplateSvc:        templateSvc,
		DraftSvc:           draftSvc,
		FormattingSvc:      formattingSvc,
		IngestionSvc:       ingestionSvc,
		AutomationSvc:      automationSvc,
		WorkspaceSvc:       workspaceSvc,
		JobSvc:             jobSvc,
		ReviewSvc:          reviewSvc,
		PublishSvc:         publishSvc,
		CollectorSourceSvc: collectorRuntime.RegistryService,
		CollectorRunSvc:    collectorRuntime.RunService,
		CollectorScheduler: collectorRuntime.SchedulerService,
		WorkflowEngine:     workflowEngine,
		ConfigLoader:       loader,
		WorkspaceRoot:      workspaceRoot,
	}

	server := newHTTPServer(cfg, serverProvider)

	err = server.Run()
	cancelWorker()
	shutdownErr := jobSvc.CloseQueueWithContext(context.Background())
	workerWG.Wait()
	if err == nil {
		return shutdownErr
	}
	return err
}

func workspaceRootFromEnv() string {
	if envRoot := os.Getenv("CONTENT_HUB_WORKSPACE_ROOT"); envRoot != "" {
		return envRoot
	}
	return "."
}

func buildRuntimeRepos(root string) (*service.RuntimeRepos, func() error, error) {
	return service.BuildRuntimeRepos(root)
}

type jobExecutor struct {
	engine *service.WorkflowEngine
}

type runtimePublishProvider struct{}

func (e *jobExecutor) Execute(ctx context.Context, wf *domain.WorkflowDefinition, wc *domain.WorkflowContext) error {
	return e.engine.Execute(ctx, wf, wc)
}

func (runtimePublishProvider) Publish(_ context.Context, req domain.PublishRequest) (*domain.PublishResult, error) {
	return &domain.PublishResult{Success: true, Platform: req.Platform, Message: "published", Metadata: map[string]any{"provider": "runtime"}}, nil
}

func (runtimePublishProvider) Platforms() []string {
	return []string{"wechat"}
}
