package main

import (
	"bytes"
	"content-hub/domain"
	workspaceinfra "content-hub/infra/workspace"
	"content-hub/pkg/repo"
	"content-hub/service"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

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

type rssCLIService interface {
	CreateSubscription(ctx context.Context, sub *domain.RSSSubscription) (*domain.RSSSubscription, error)
	GetSubscription(ctx context.Context, id string) (*domain.RSSSubscription, error)
	ListSubscriptions(ctx context.Context) ([]domain.RSSSubscription, error)
	UpdateSubscription(ctx context.Context, sub *domain.RSSSubscription) (*domain.RSSSubscription, error)
	DeleteSubscription(ctx context.Context, id string) error
	RunByID(ctx context.Context, subscriptionID string) (*service.RSSPullResult, error)
	RunAll(ctx context.Context) ([]service.RSSScheduledRunResult, error)
	ListRuns(ctx context.Context, limit int) ([]domain.RSSPullRun, error)
	ListItems(ctx context.Context, limit int) ([]domain.RSSItemRecord, error)
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

type runtimeRSSCLIService struct {
	subscriptions *service.RSSSubscriptionService
	scheduler     *service.RSSScheduler
	runs          repo.RSSPullRunRepo
	items         repo.RSSItemRepo
}

func (s *runtimeRSSCLIService) CreateSubscription(ctx context.Context, sub *domain.RSSSubscription) (*domain.RSSSubscription, error) {
	return s.subscriptions.Create(ctx, sub)
}

func (s *runtimeRSSCLIService) GetSubscription(ctx context.Context, id string) (*domain.RSSSubscription, error) {
	return s.subscriptions.Get(ctx, id)
}

func (s *runtimeRSSCLIService) ListSubscriptions(ctx context.Context) ([]domain.RSSSubscription, error) {
	return s.subscriptions.List(ctx)
}

func (s *runtimeRSSCLIService) UpdateSubscription(ctx context.Context, sub *domain.RSSSubscription) (*domain.RSSSubscription, error) {
	return s.subscriptions.Update(ctx, sub)
}

func (s *runtimeRSSCLIService) DeleteSubscription(ctx context.Context, id string) error {
	return s.subscriptions.Delete(ctx, id)
}

func (s *runtimeRSSCLIService) RunByID(ctx context.Context, subscriptionID string) (*service.RSSPullResult, error) {
	return s.scheduler.RunByID(ctx, subscriptionID)
}

func (s *runtimeRSSCLIService) RunAll(ctx context.Context) ([]service.RSSScheduledRunResult, error) {
	return s.scheduler.RunAll(ctx)
}

func (s *runtimeRSSCLIService) ListRuns(ctx context.Context, limit int) ([]domain.RSSPullRun, error) {
	return s.runs.List(ctx, limit)
}

func (s *runtimeRSSCLIService) ListItems(ctx context.Context, limit int) ([]domain.RSSItemRecord, error) {
	return s.items.List(ctx, limit)
}

var runtimeRSSServiceFactory = func(root string) (rssCLIService, func() error, error) {
	repos, cleanup, err := service.BuildRuntimeRepos(root)
	if err != nil {
		return nil, nil, err
	}
	rssRuntime, err := service.BuildRSSRuntime(repos)
	if err != nil {
		_ = cleanup()
		return nil, nil, err
	}
	return &runtimeRSSCLIService{subscriptions: rssRuntime.SubscriptionService, scheduler: rssRuntime.Scheduler, runs: rssRuntime.PullRunReader, items: rssRuntime.ItemReader}, cleanup, nil
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: cli workspace <...> | formatting <render|validate> | rewrite <run> | automation <run-once|daemon|retry-failed|status|health|stop> | rss <subscriptions|run|run-all|runs|items> [--root PATH]")
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
		switch args[1] {
		case "run":
			req, err := parseRewriteRunRequest(args[2:])
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 2
			}
			svc, cleanup, err := runtimeRewriteServiceFactory(root)
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			defer cleanup()
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
	case "rss":
		svc, cleanup, err := runtimeRSSServiceFactory(root)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		defer cleanup()
		switch args[1] {
		case "subscriptions":
			if len(args) < 3 {
				fmt.Fprintln(stderr, "missing rss subscriptions subcommand")
				return 2
			}
			switch args[2] {
			case "list":
				items, err := svc.ListSubscriptions(context.Background())
				if err != nil {
					fmt.Fprintln(stderr, err.Error())
					return 1
				}
				fmt.Fprint(stdout, formatResolvedConfig(items))
				return 0
			case "add":
				sub, err := parseRSSSubscriptionArgs(args[3:])
				if err != nil {
					fmt.Fprintln(stderr, err.Error())
					return 2
				}
				created, err := svc.CreateSubscription(context.Background(), sub)
				if err != nil {
					fmt.Fprintln(stderr, err.Error())
					return 1
				}
				fmt.Fprint(stdout, formatResolvedConfig(created))
				return 0
			case "update":
				if len(args) < 4 {
					fmt.Fprintln(stderr, "missing rss subscription id")
					return 2
				}
				existing, err := svc.GetSubscription(context.Background(), args[3])
				if err != nil {
					fmt.Fprintln(stderr, err.Error())
					return 1
				}
				updated, err := parseRSSSubscriptionUpdateArgs(existing, args[4:])
				if err != nil {
					fmt.Fprintln(stderr, err.Error())
					return 2
				}
				result, err := svc.UpdateSubscription(context.Background(), updated)
				if err != nil {
					fmt.Fprintln(stderr, err.Error())
					return 1
				}
				fmt.Fprint(stdout, formatResolvedConfig(result))
				return 0
			case "remove":
				if len(args) < 4 {
					fmt.Fprintln(stderr, "missing rss subscription id")
					return 2
				}
				if err := svc.DeleteSubscription(context.Background(), args[3]); err != nil {
					fmt.Fprintln(stderr, err.Error())
					return 1
				}
				fmt.Fprint(stdout, formatResolvedConfig(map[string]bool{"deleted": true}))
				return 0
			default:
				fmt.Fprintf(stderr, "unknown rss subscriptions subcommand: %s\n", args[2])
				return 2
			}
		case "run":
			if len(args) < 3 {
				fmt.Fprintln(stderr, "missing rss subscription id")
				return 2
			}
			result, err := svc.RunByID(context.Background(), args[2])
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			fmt.Fprint(stdout, formatResolvedConfig(result))
			return 0
		case "run-all":
			items, err := svc.RunAll(context.Background())
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			fmt.Fprint(stdout, formatResolvedConfig(items))
			return 0
		case "runs":
			if len(args) < 3 || args[2] != "list" {
				fmt.Fprintf(stderr, "unknown rss runs subcommand: %s\n", safeArg(args, 2))
				return 2
			}
			limit, err := parseRSSListLimit(args[3:])
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 2
			}
			items, err := svc.ListRuns(context.Background(), limit)
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			fmt.Fprint(stdout, formatResolvedConfig(items))
			return 0
		case "items":
			if len(args) < 3 || args[2] != "list" {
				fmt.Fprintf(stderr, "unknown rss items subcommand: %s\n", safeArg(args, 2))
				return 2
			}
			limit, err := parseRSSListLimit(args[3:])
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 2
			}
			items, err := svc.ListItems(context.Background(), limit)
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			fmt.Fprint(stdout, formatResolvedConfig(items))
			return 0
		default:
			fmt.Fprintf(stderr, "unknown rss subcommand: %s\n", args[1])
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
	if len(args) == 0 || strings.HasPrefix(strings.TrimSpace(args[0]), "--") {
		return service.RewriteRunRequest{}, fmt.Errorf("missing positional workspace article id")
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
		return service.RewriteRunRequest{}, fmt.Errorf("missing positional workspace article id")
	}
	if req.TargetType == "" {
		return service.RewriteRunRequest{}, fmt.Errorf("missing --target")
	}
	if req.SourceProfile == "" {
		return service.RewriteRunRequest{}, fmt.Errorf("missing --source")
	}
	return req, nil
}

func parseRSSSubscriptionArgs(args []string) (*domain.RSSSubscription, error) {
	sub := domain.NewRSSSubscription("", "", "", "")
	if err := applyRSSSubscriptionFlags(sub, args); err != nil {
		return nil, err
	}
	if err := sub.Validate(); err != nil {
		return nil, err
	}
	return sub, nil
}

func parseRSSSubscriptionUpdateArgs(existing *domain.RSSSubscription, args []string) (*domain.RSSSubscription, error) {
	if existing == nil {
		return nil, fmt.Errorf("rss subscription is required")
	}
	copySub := *existing
	copySub.Metadata = cloneMap(existing.Metadata)
	if err := applyRSSSubscriptionFlags(&copySub, args); err != nil {
		return nil, err
	}
	if err := copySub.Validate(); err != nil {
		return nil, err
	}
	return &copySub, nil
}

func applyRSSSubscriptionFlags(sub *domain.RSSSubscription, args []string) error {
	for idx := 0; idx < len(args); idx++ {
		switch args[idx] {
		case "--name":
			value, next, err := requireFlagValue(args, idx, "--name")
			if err != nil {
				return err
			}
			sub.Name = strings.TrimSpace(value)
			idx = next
		case "--feed-url":
			value, next, err := requireFlagValue(args, idx, "--feed-url")
			if err != nil {
				return err
			}
			sub.FeedURL = strings.TrimSpace(value)
			idx = next
		case "--target":
			value, next, err := requireFlagValue(args, idx, "--target")
			if err != nil {
				return err
			}
			sub.TargetType = strings.TrimSpace(value)
			idx = next
		case "--source":
			value, next, err := requireFlagValue(args, idx, "--source")
			if err != nil {
				return err
			}
			sub.SourceProfile = strings.TrimSpace(value)
			idx = next
		case "--version":
			value, next, err := requireFlagValue(args, idx, "--version")
			if err != nil {
				return err
			}
			sub.RewriteProfileVersion = strings.TrimSpace(value)
			idx = next
		case "--poll-interval":
			value, next, err := requireFlagValue(args, idx, "--poll-interval")
			if err != nil {
				return err
			}
			parsed, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return fmt.Errorf("invalid value for --poll-interval")
			}
			sub.PollIntervalSec = parsed
			idx = next
		case "--enabled":
			value, next, err := requireFlagValue(args, idx, "--enabled")
			if err != nil {
				return err
			}
			parsed, err := strconv.ParseBool(strings.TrimSpace(value))
			if err != nil {
				return fmt.Errorf("invalid value for --enabled")
			}
			sub.Enabled = parsed
			idx = next
		case "--root":
			_, next, err := requireFlagValue(args, idx, "--root")
			if err != nil {
				return err
			}
			idx = next
		default:
			return fmt.Errorf("unknown rss subscription flag: %s", args[idx])
		}
	}
	return nil
}

func parseRSSListLimit(args []string) (int, error) {
	limit := 20
	for idx := 0; idx < len(args); idx++ {
		switch args[idx] {
		case "--limit":
			value, next, err := requireFlagValue(args, idx, "--limit")
			if err != nil {
				return 0, err
			}
			parsed, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil || parsed <= 0 {
				return 0, fmt.Errorf("invalid value for --limit")
			}
			limit = parsed
			idx = next
		case "--root":
			_, next, err := requireFlagValue(args, idx, "--root")
			if err != nil {
				return 0, err
			}
			idx = next
		default:
			return 0, fmt.Errorf("unknown rss list flag: %s", args[idx])
		}
	}
	return limit, nil
}

func requireFlagValue(args []string, idx int, flag string) (string, int, error) {
	if idx+1 >= len(args) || strings.HasPrefix(args[idx+1], "--") {
		return "", idx, fmt.Errorf("missing value for %s", flag)
	}
	return args[idx+1], idx + 1, nil
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	clone := make(map[string]any, len(src))
	for k, v := range src {
		clone[k] = v
	}
	return clone
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
