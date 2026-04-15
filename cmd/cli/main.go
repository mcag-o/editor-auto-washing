package main

import (
	"bytes"
	collectorscheduler "content-hub/collector/scheduler"
	collectorservice "content-hub/collector/service"
	"content-hub/domain"
	workspaceinfra "content-hub/infra/workspace"
	"content-hub/service"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

var runtimeIngestionServiceFactory = func(root string) (*service.IngestionPipelineService, func() error, error) {
	repos, cleanup, err := service.BuildRuntimeRepos(root)
	if err != nil {
		return nil, nil, err
	}
	return service.NewIngestionPipelineService(repos.IngestionRepo, repos.WorkspaceRepo, repos.BundleImportTxStarter, workspaceinfra.NewLoader()), cleanup, nil
}

type formattingCLIService interface {
	Render(ctx context.Context, draftID, platform, templateName string) (*domain.RenderedAssetRecord, error)
	Validate(ctx context.Context, draftID, platform, templateName string) (domain.DraftValidationResult, error)
	GetAsset(ctx context.Context, assetID string) (*domain.RenderedAssetRecord, error)
}

var runtimeFormattingServiceFactory = func(root string) (formattingCLIService, func() error, error) {
	return service.NewRuntimeFormattingPipelineService(root)
}

type reviewPublishCLIService interface {
	ApproveReview(ctx context.Context, id, reviewer, notes string) (*domain.ReviewTask, error)
	RejectReview(ctx context.Context, id, reviewer, notes string) (*domain.ReviewTask, error)
	PublishReview(ctx context.Context, reviewID string) (*domain.PublishOutcome, error)
	History(ctx context.Context, articleID string) ([]domain.PublishRecord, error)
}

type rewriteCLIService interface {
	Run(ctx context.Context, req service.RewriteRunRequest) (*domain.RewritePipelineRun, error)
}

type rewriteRunner interface {
	Run(ctx context.Context, req service.RewriteRunRequest) (*domain.RewritePipelineRun, error)
}

type automationCLIService interface {
	RunOnce(ctx context.Context) (*domain.AutomationRunResult, error)
	RunDaemon(ctx context.Context) (*domain.AutomationRunResult, error)
	RetryFailed(ctx context.Context) (*domain.AutomationRunResult, error)
	Status(ctx context.Context) (*domain.AutomationStatusSnapshot, error)
	Health(ctx context.Context) (*domain.AutomationHealthReport, error)
	Stop(ctx context.Context) (*domain.AutomationStopResult, error)
}

type collectorCLIService interface {
	ListSources(ctx context.Context) ([]domain.CollectorSource, error)
	HealthSources(ctx context.Context) ([]domain.CollectorSourceHealthStatus, error)
	ListRuns(ctx context.Context, limit int) ([]domain.CollectorRun, error)
	RunOnce(ctx context.Context) (*domain.CollectorRunSummary, error)
	RunDaemon(ctx context.Context) (*domain.CollectorSchedulerControlResult, error)
	SchedulerStatus(ctx context.Context) (*domain.CollectorSchedulerStatus, error)
	SchedulerHealth(ctx context.Context) (*domain.CollectorSchedulerHealthReport, error)
	StopDaemon(ctx context.Context) (*domain.CollectorSchedulerControlResult, error)
}

type runtimeReviewPublishService struct {
	review  *service.ReviewService
	publish *service.PublishGateService
}

func (s *runtimeReviewPublishService) ApproveReview(ctx context.Context, id, reviewer, notes string) (*domain.ReviewTask, error) {
	return s.review.ApproveReview(ctx, id, reviewer, notes)
}

func (s *runtimeReviewPublishService) RejectReview(ctx context.Context, id, reviewer, notes string) (*domain.ReviewTask, error) {
	return s.review.RejectReview(ctx, id, reviewer, notes)
}

func (s *runtimeReviewPublishService) PublishReview(ctx context.Context, reviewID string) (*domain.PublishOutcome, error) {
	return s.publish.PublishReview(ctx, reviewID)
}

func (s *runtimeReviewPublishService) History(ctx context.Context, articleID string) ([]domain.PublishRecord, error) {
	return s.publish.History(ctx, articleID)
}

var runtimeReviewPublishServiceFactory = func(root string) (reviewPublishCLIService, func() error, error) {
	repos, cleanup, err := service.BuildRuntimeRepos(root)
	if err != nil {
		return nil, nil, err
	}
	reviewSvc := service.NewReviewService(repos.ReviewRepo, repos.WorkspaceRepo)
	publishSvc := service.NewPublishGateService(repos.ReviewRepo, repos.AssetRepo, repos.DraftRepo, repos.PublishRepo, repos.WorkspaceRepo, map[string]service.PublisherProvider{"wechat": cliPublishProvider{}})
	return &runtimeReviewPublishService{review: reviewSvc, publish: publishSvc}, cleanup, nil
}

type runtimeRewriteCLIService struct {
	workspaceRepo repoWorkspaceReader
	runner        rewriteRunner
}

type repoWorkspaceReader interface {
	GetByID(ctx context.Context, id string) (*domain.ArticleWorkspaceRecord, error)
}

func (s *runtimeRewriteCLIService) Run(ctx context.Context, req service.RewriteRunRequest) (*domain.RewritePipelineRun, error) {
	workspace, err := s.workspaceRepo.GetByID(ctx, req.WorkspaceArticleID)
	if err != nil {
		return nil, err
	}
	if workspace == nil {
		return nil, domain.NewNotFoundErr("workspace article", req.WorkspaceArticleID)
	}
	if strings.TrimSpace(workspace.Title) == "" {
		return nil, domain.NewValidationErr("workspace article title is required for rewrite", nil)
	}
	collectorArticleID, _ := workspace.Metadata["collector_article_id"].(string)
	if strings.TrimSpace(collectorArticleID) == "" {
		return nil, domain.NewValidationErr("workspace article collector_article_id is required for rewrite", nil)
	}
	request := req
	request.Title = workspace.Title
	request.CollectorArticleID = collectorArticleID
	return s.runner.Run(ctx, request)
}

var runtimeRewriteServiceFactory = func(root string) (rewriteCLIService, func() error, error) {
	repos, cleanup, err := service.BuildRuntimeRepos(root)
	if err != nil {
		return nil, nil, err
	}
	rewriteRuntime, err := service.BuildRewriteRuntime(repos)
	if err != nil {
		_ = cleanup()
		return nil, nil, err
	}
	return &runtimeRewriteCLIService{workspaceRepo: repos.WorkspaceRepo, runner: rewriteRuntime.Orchestrator}, cleanup, nil
}

type runtimeAutomationCLIService struct {
	root string
	svc  *service.AutomationService
}

func (s *runtimeAutomationCLIService) RunOnce(ctx context.Context) (*domain.AutomationRunResult, error) {
	return s.svc.RunOnce(ctx, s.root)
}

func (s *runtimeAutomationCLIService) RunDaemon(ctx context.Context) (*domain.AutomationRunResult, error) {
	return s.svc.RunDaemon(ctx, s.root, 0)
}

func (s *runtimeAutomationCLIService) RetryFailed(ctx context.Context) (*domain.AutomationRunResult, error) {
	return s.svc.RetryFailed(ctx, s.root)
}

func (s *runtimeAutomationCLIService) Status(ctx context.Context) (*domain.AutomationStatusSnapshot, error) {
	return s.svc.Status(ctx, s.root)
}

func (s *runtimeAutomationCLIService) Health(ctx context.Context) (*domain.AutomationHealthReport, error) {
	return s.svc.Health(ctx, s.root)
}

func (s *runtimeAutomationCLIService) Stop(ctx context.Context) (*domain.AutomationStopResult, error) {
	return s.svc.Stop(ctx, s.root)
}

var runtimeAutomationServiceFactory = func(root string) (automationCLIService, func() error, error) {
	automationSvc, cleanup, err := service.NewRuntimeAutomationService(root)
	if err != nil {
		return nil, nil, err
	}
	return &runtimeAutomationCLIService{root: root, svc: automationSvc}, cleanup, nil
}

type runtimeCollectorCLIService struct {
	registry          *collectorservice.SourceRegistryService
	runs              *collectorservice.RunService
	scheduler         *collectorscheduler.Service
	daemonStopTimeout time.Duration
}

func (s *runtimeCollectorCLIService) ListSources(ctx context.Context) ([]domain.CollectorSource, error) {
	return s.registry.ListSources(ctx)
}

func (s *runtimeCollectorCLIService) HealthSources(ctx context.Context) ([]domain.CollectorSourceHealthStatus, error) {
	return s.registry.Health(ctx)
}

func (s *runtimeCollectorCLIService) ListRuns(ctx context.Context, limit int) ([]domain.CollectorRun, error) {
	return s.runs.ListRuns(ctx, limit)
}

func (s *runtimeCollectorCLIService) RunOnce(ctx context.Context) (*domain.CollectorRunSummary, error) {
	return s.scheduler.RunOnce(ctx)
}

func (s *runtimeCollectorCLIService) RunDaemon(ctx context.Context) (*domain.CollectorSchedulerControlResult, error) {
	_, err := s.scheduler.StartDaemon(ctx)
	if err != nil {
		return nil, err
	}
	<-ctx.Done()
	stopTimeout := s.daemonStopTimeout
	if stopTimeout <= 0 {
		stopTimeout = 5 * time.Second
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), stopTimeout)
	defer cancel()
	return s.scheduler.Stop(stopCtx)
}

func (s *runtimeCollectorCLIService) SchedulerStatus(ctx context.Context) (*domain.CollectorSchedulerStatus, error) {
	return s.scheduler.Status(ctx)
}

func (s *runtimeCollectorCLIService) SchedulerHealth(ctx context.Context) (*domain.CollectorSchedulerHealthReport, error) {
	return s.scheduler.Health(ctx)
}

func (s *runtimeCollectorCLIService) StopDaemon(ctx context.Context) (*domain.CollectorSchedulerControlResult, error) {
	return s.scheduler.Stop(ctx)
}

var runtimeCollectorServiceFactory = func(root string) (collectorCLIService, func() error, error) {
	repos, cleanup, err := service.BuildRuntimeRepos(root)
	if err != nil {
		return nil, nil, err
	}
	collectorRuntime, err := service.BuildCollectorRuntime(context.Background(), repos, 30*time.Minute)
	if err != nil {
		_ = cleanup()
		return nil, nil, err
	}
	return &runtimeCollectorCLIService{registry: collectorRuntime.RegistryService, runs: collectorRuntime.RunService, scheduler: collectorRuntime.SchedulerService, daemonStopTimeout: 5 * time.Second}, cleanup, nil
}

var collectorDaemonContextFactory = func() (context.Context, context.CancelFunc) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	return ctx, cancel
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: cli workspace <...> | ingestion <import|retry-failed> | formatting <render|validate> | rewrite <run> | automation <run-once|daemon|retry-failed|status|health|stop> | collector <sources|runs|scheduler> [--root PATH]")
		return 2
	}
	if len(args) < 2 {
		fmt.Fprintf(stderr, "missing %s subcommand\n", args[0])
		return 2
	}

	root := "."
	for idx := 2; idx < len(args); idx++ {
		if args[idx] == "--root" && idx+1 < len(args) {
			root = args[idx+1]
			idx++
		}
	}

	workspaceSvc := service.NewWorkspaceConfigService(workspaceinfra.NewLoader(), workspaceinfra.NewValidator())
	switch args[0] {
	case "workspace":
		switch args[1] {
		case "init":
			resolved, err := workspaceSvc.Init(root)
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			fmt.Fprintf(stdout, "workspace initialized at %s\n", resolved.Paths.Root)
			return 0
		case "show-config":
			output, err := workspaceSvc.ShowConfig(root)
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			fmt.Fprint(stdout, output)
			return 0
		case "resolve-config":
			resolved, err := workspaceSvc.Resolve(root)
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			fmt.Fprint(stdout, formatResolvedConfig(resolved))
			return 0
		case "doctor":
			report, err := workspaceSvc.Doctor(root)
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			if len(report.Errors()) == 0 {
				fmt.Fprintln(stdout, "workspace diagnostics: ok")
				return 0
			}
			for _, item := range report.Errors() {
				fmt.Fprintln(stdout, item)
			}
			return 1
		default:
			fmt.Fprintf(stderr, "unknown workspace subcommand: %s\n", args[1])
			return 2
		}
	case "ingestion":
		pipeline, cleanup, err := runtimeIngestionServiceFactory(root)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		defer cleanup()
		var output any
		switch args[1] {
		case "import":
			output, err = pipeline.ImportIncoming(context.Background(), root)
		case "retry-failed":
			output, err = pipeline.RetryFailed(context.Background(), root)
		default:
			fmt.Fprintf(stderr, "unknown ingestion subcommand: %s\n", args[1])
			return 2
		}
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		fmt.Fprint(stdout, formatResolvedConfig(output))
		return 0
	case "formatting":
		if len(args) < 3 {
			fmt.Fprintf(stderr, "missing formatting target\n")
			return 2
		}
		pipeline, cleanup, err := runtimeFormattingServiceFactory(root)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		defer cleanup()
		platform := "wechat"
		templateName := ""
		for idx := 3; idx < len(args); idx++ {
			switch args[idx] {
			case "--platform":
				if idx+1 < len(args) {
					platform = args[idx+1]
					idx++
				}
			case "--template":
				if idx+1 < len(args) {
					templateName = args[idx+1]
					idx++
				}
			}
		}
		switch args[1] {
		case "render":
			asset, err := pipeline.Render(context.Background(), args[2], platform, templateName)
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			fmt.Fprint(stdout, formatResolvedConfig(asset))
			return 0
		case "validate":
			result, err := pipeline.Validate(context.Background(), args[2], platform, templateName)
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			if len(result.Errors) > 0 {
				fmt.Fprint(stdout, formatResolvedConfig(result))
				return 1
			}
			fmt.Fprint(stdout, formatResolvedConfig(result))
			return 0
		default:
			fmt.Fprintf(stderr, "unknown formatting subcommand: %s\n", args[1])
			return 2
		}
	case "review":
		svc, cleanup, err := runtimeReviewPublishServiceFactory(root)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		defer cleanup()
		reviewer, notes := parseReviewerNotesFlags(args[3:])
		switch args[1] {
		case "approve":
			review, err := svc.ApproveReview(context.Background(), args[2], reviewer, notes)
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			fmt.Fprint(stdout, formatResolvedConfig(review))
			return 0
		case "reject":
			review, err := svc.RejectReview(context.Background(), args[2], reviewer, notes)
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			fmt.Fprint(stdout, formatResolvedConfig(review))
			return 0
		default:
			fmt.Fprintf(stderr, "unknown review subcommand: %s\n", args[1])
			return 2
		}
	case "publish":
		svc, cleanup, err := runtimeReviewPublishServiceFactory(root)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		defer cleanup()
		switch args[1] {
		case "run":
			records, err := svc.PublishReview(context.Background(), args[2])
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			fmt.Fprint(stdout, formatResolvedConfig(records))
			return 0
		case "history":
			records, err := svc.History(context.Background(), args[2])
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			fmt.Fprint(stdout, formatResolvedConfig(records))
			return 0
		default:
			fmt.Fprintf(stderr, "unknown publish subcommand: %s\n", args[1])
			return 2
		}
	case "rewrite":
		if len(args) < 3 {
			fmt.Fprintln(stderr, "missing rewrite target")
			return 2
		}
		svc, cleanup, err := runtimeRewriteServiceFactory(root)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		defer cleanup()
		switch args[1] {
		case "run":
			req, err := parseRewriteRunRequest(args[2:])
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 2
			}
			result, err := svc.Run(context.Background(), req)
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			fmt.Fprint(stdout, formatResolvedConfig(result))
			return 0
		default:
			fmt.Fprintf(stderr, "unknown rewrite subcommand: %s\n", args[1])
			return 2
		}
	case "automation":
		svc, cleanup, err := runtimeAutomationServiceFactory(root)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		defer cleanup()
		switch args[1] {
		case "run-once":
			result, err := svc.RunOnce(context.Background())
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			fmt.Fprint(stdout, formatResolvedConfig(result))
			return 0
		case "daemon":
			result, err := svc.RunDaemon(context.Background())
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			fmt.Fprint(stdout, formatResolvedConfig(result))
			return 0
		case "retry-failed":
			result, err := svc.RetryFailed(context.Background())
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			fmt.Fprint(stdout, formatResolvedConfig(result))
			return 0
		case "status":
			result, err := svc.Status(context.Background())
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			fmt.Fprint(stdout, formatResolvedConfig(result))
			return 0
		case "health":
			result, err := svc.Health(context.Background())
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			fmt.Fprint(stdout, formatResolvedConfig(result))
			return 0
		case "stop":
			result, err := svc.Stop(context.Background())
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			fmt.Fprint(stdout, formatResolvedConfig(result))
			return 0
		default:
			fmt.Fprintf(stderr, "unknown automation subcommand: %s\n", args[1])
			return 2
		}
	case "collector":
		svc, cleanup, err := runtimeCollectorServiceFactory(root)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		defer cleanup()
		switch args[1] {
		case "sources":
			if len(args) < 3 {
				fmt.Fprintln(stderr, "missing collector sources subcommand")
				return 2
			}
			switch args[2] {
			case "list":
				items, err := svc.ListSources(context.Background())
				if err != nil {
					fmt.Fprintln(stderr, err.Error())
					return 1
				}
				fmt.Fprint(stdout, formatResolvedConfig(items))
				return 0
			case "health":
				items, err := svc.HealthSources(context.Background())
				if err != nil {
					fmt.Fprintln(stderr, err.Error())
					return 1
				}
				fmt.Fprint(stdout, formatResolvedConfig(items))
				return 0
			default:
				fmt.Fprintf(stderr, "unknown collector sources subcommand: %s\n", args[2])
				return 2
			}
		case "runs":
			if len(args) < 3 || args[2] != "list" {
				fmt.Fprintf(stderr, "unknown collector runs subcommand: %s\n", safeArg(args, 2))
				return 2
			}
			items, err := svc.ListRuns(context.Background(), 20)
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			fmt.Fprint(stdout, formatResolvedConfig(items))
			return 0
		case "scheduler":
			if len(args) < 3 {
				fmt.Fprintln(stderr, "missing collector scheduler subcommand")
				return 2
			}
			switch args[2] {
			case "run-once":
				result, err := svc.RunOnce(context.Background())
				if err != nil {
					fmt.Fprintln(stderr, err.Error())
					return 1
				}
				fmt.Fprint(stdout, formatResolvedConfig(result))
				return 0
			case "daemon":
				daemonCtx, cancel := collectorDaemonContextFactory()
				defer cancel()
				result, err := svc.RunDaemon(daemonCtx)
				if err != nil {
					fmt.Fprintln(stderr, err.Error())
					return 1
				}
				fmt.Fprint(stdout, formatResolvedConfig(result))
				return 0
			case "status":
				result, err := svc.SchedulerStatus(context.Background())
				if err != nil {
					fmt.Fprintln(stderr, err.Error())
					return 1
				}
				fmt.Fprint(stdout, formatResolvedConfig(result))
				return 0
			case "health":
				result, err := svc.SchedulerHealth(context.Background())
				if err != nil {
					fmt.Fprintln(stderr, err.Error())
					return 1
				}
				fmt.Fprint(stdout, formatResolvedConfig(result))
				return 0
			case "stop":
				result, err := svc.StopDaemon(context.Background())
				if err != nil {
					fmt.Fprintln(stderr, err.Error())
					return 1
				}
				fmt.Fprint(stdout, formatResolvedConfig(result))
				return 0
			default:
				fmt.Fprintf(stderr, "unknown collector scheduler subcommand: %s\n", args[2])
				return 2
			}
		default:
			fmt.Fprintf(stderr, "unknown collector subcommand: %s\n", args[1])
			return 2
		}
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		return 2
	}
}

var _ = domain.DraftValidationResult{}

func formatResolvedConfig(resolved any) string {
	data, err := yaml.Marshal(resolved)
	if err == nil {
		return string(data)
	}
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("%+v", resolved))
	buffer.WriteString("\n")
	return strings.ReplaceAll(buffer.String(), "  ", " ")
}

func parseReviewerNotesFlags(args []string) (string, string) {
	reviewer := ""
	notes := ""
	for idx := 0; idx < len(args); idx++ {
		switch args[idx] {
		case "--reviewer":
			if idx+1 < len(args) {
				reviewer = args[idx+1]
				idx++
			}
		case "--notes":
			if idx+1 < len(args) {
				notes = args[idx+1]
				idx++
			}
		}
	}
	return reviewer, notes
}

func parseRewriteRunRequest(args []string) (service.RewriteRunRequest, error) {
	if len(args) == 0 {
		return service.RewriteRunRequest{}, fmt.Errorf("missing workspace article id")
	}
	req := service.RewriteRunRequest{
		WorkspaceArticleID: strings.TrimSpace(args[0]),
		Version:            "latest",
	}
	for idx := 1; idx < len(args); idx++ {
		switch args[idx] {
		case "--target":
			if idx+1 >= len(args) || strings.HasPrefix(args[idx+1], "--") {
				return service.RewriteRunRequest{}, fmt.Errorf("missing value for --target")
			}
			req.TargetType = strings.TrimSpace(args[idx+1])
			idx++
		case "--source":
			if idx+1 >= len(args) || strings.HasPrefix(args[idx+1], "--") {
				return service.RewriteRunRequest{}, fmt.Errorf("missing value for --source")
			}
			req.SourceProfile = strings.TrimSpace(args[idx+1])
			idx++
		case "--root":
			if idx+1 >= len(args) || strings.HasPrefix(args[idx+1], "--") {
				return service.RewriteRunRequest{}, fmt.Errorf("missing value for --root")
			}
			idx++
		default:
			return service.RewriteRunRequest{}, fmt.Errorf("unknown rewrite flag: %s", args[idx])
		}
	}
	if req.WorkspaceArticleID == "" {
		return service.RewriteRunRequest{}, fmt.Errorf("missing workspace article id")
	}
	if req.TargetType == "" {
		return service.RewriteRunRequest{}, fmt.Errorf("missing --target")
	}
	if req.SourceProfile == "" {
		return service.RewriteRunRequest{}, fmt.Errorf("missing --source")
	}
	return req, nil
}

func safeArg(args []string, idx int) string {
	if idx >= 0 && idx < len(args) {
		return args[idx]
	}
	return ""
}

type cliPublishProvider struct{}

func (cliPublishProvider) Publish(_ context.Context, req domain.PublishRequest) (*domain.PublishResult, error) {
	return &domain.PublishResult{Success: true, Platform: req.Platform, Message: "published", Metadata: map[string]any{"provider": "cli"}}, nil
}

func (cliPublishProvider) Platforms() []string { return []string{"wechat"} }
