package main

import (
	"content-hub/domain"
	"content-hub/infra/config"
	"content-hub/infra/memory"
	"content-hub/service"
	httpserver "content-hub/transport/http"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	fmt.Println("content-hub server starting...")
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.DefaultConfig()

	loader := config.NewLoader("./config.json")
	if loadedCfg, err := loader.Load(); err == nil {
		cfg = loadedCfg
		cfg.ResolveSecrets()
	} else {
		cfg.ResolveSecrets()
	}

	memProvider := memory.NewProvider()

	contentSvc := service.NewContentService(
		memProvider.ArticleRepo(),
		memProvider.PublishRepo(),
	)

	templateSvc := service.NewTemplateService(memProvider.TemplateRepo())
	draftSvc := service.NewDraftService(memProvider.DraftRepo())

	workflowEngine := service.NewWorkflowEngine()

	jobSvc := service.NewJobService(
		memProvider.JobRepo(),
		memProvider.JobEventRepo(),
		&jobExecutor{engine: workflowEngine},
	)

	go jobSvc.RunWorker(context.Background())

	serverProvider := &httpserver.Provider{
		ContentSvc:     contentSvc,
		TemplateSvc:    templateSvc,
		DraftSvc:       draftSvc,
		JobSvc:         jobSvc,
		WorkflowEngine: workflowEngine,
		ConfigLoader:   loader,
	}

	server := httpserver.NewServer(cfg, serverProvider)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		jobSvc.CloseQueue()
		os.Exit(0)
	}()

	return server.Run()
}

type jobExecutor struct {
	engine *service.WorkflowEngine
}

func (e *jobExecutor) Execute(ctx context.Context, wf *domain.WorkflowDefinition, wc *domain.WorkflowContext) error {
	return e.engine.Execute(ctx, wf, wc)
}
