package main

import (
	"bytes"
	"content-hub/domain"
	workspaceinfra "content-hub/infra/workspace"
	"content-hub/service"
	"context"
	"fmt"
	"io"
	"os"
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

const usageText = "web control plane is the primary operator surface: http://localhost:8123\ncli is retained for development/debug support: workspace <...> | formatting <render|validate> | rewrite <run> | automation <run-once|daemon|retry-failed|status|health|stop> [--root PATH]"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usageText)
		return 2
	}
	root, filteredArgs, err := extractGlobalRoot(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	if len(filteredArgs) == 0 {
		fmt.Fprintln(stderr, usageText)
		return 2
	}
	if len(filteredArgs) < 2 {
		fmt.Fprintf(stderr, "missing %s subcommand\n", filteredArgs[0])
		return 2
	}

	workspaceSvc := service.NewWorkspaceConfigService(workspaceinfra.NewLoader(), workspaceinfra.NewValidator())
	switch filteredArgs[0] {
	case "workspace":
		switch filteredArgs[1] {
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
			fmt.Fprintf(stderr, "unknown workspace subcommand: %s\n", filteredArgs[1])
			return 2
		}
	case "formatting":
		if len(filteredArgs) < 3 {
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
		for idx := 3; idx < len(filteredArgs); idx++ {
			switch filteredArgs[idx] {
			case "--platform":
				if idx+1 < len(filteredArgs) {
					platform = filteredArgs[idx+1]
					idx++
				}
			case "--template":
				if idx+1 < len(filteredArgs) {
					templateName = filteredArgs[idx+1]
					idx++
				}
			}
		}
		switch filteredArgs[1] {
		case "render":
			asset, err := pipeline.Render(context.Background(), filteredArgs[2], platform, templateName)
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			fmt.Fprint(stdout, formatResolvedConfig(asset))
			return 0
		case "validate":
			result, err := pipeline.Validate(context.Background(), filteredArgs[2], platform, templateName)
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
			fmt.Fprintf(stderr, "unknown formatting subcommand: %s\n", filteredArgs[1])
			return 2
		}
	case "review":
		svc, cleanup, err := runtimeReviewPublishServiceFactory(root)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		defer cleanup()
		reviewer, notes := parseReviewerNotesFlags(filteredArgs[3:])
		switch filteredArgs[1] {
		case "approve":
			review, err := svc.ApproveReview(context.Background(), filteredArgs[2], reviewer, notes)
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			fmt.Fprint(stdout, formatResolvedConfig(review))
			return 0
		case "reject":
			review, err := svc.RejectReview(context.Background(), filteredArgs[2], reviewer, notes)
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			fmt.Fprint(stdout, formatResolvedConfig(review))
			return 0
		default:
			fmt.Fprintf(stderr, "unknown review subcommand: %s\n", filteredArgs[1])
			return 2
		}
	case "publish":
		svc, cleanup, err := runtimeReviewPublishServiceFactory(root)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		defer cleanup()
		switch filteredArgs[1] {
		case "run":
			records, err := svc.PublishReview(context.Background(), filteredArgs[2])
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			fmt.Fprint(stdout, formatResolvedConfig(records))
			return 0
		case "history":
			records, err := svc.History(context.Background(), filteredArgs[2])
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			fmt.Fprint(stdout, formatResolvedConfig(records))
			return 0
		default:
			fmt.Fprintf(stderr, "unknown publish subcommand: %s\n", filteredArgs[1])
			return 2
		}
	case "rewrite":
		if len(filteredArgs) < 3 {
			fmt.Fprintln(stderr, "missing rewrite target")
			return 2
		}
		switch filteredArgs[1] {
		case "run":
			req, err := parseRewriteRunRequest(filteredArgs[2:])
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
			fmt.Fprintf(stderr, "unknown rewrite subcommand: %s\n", filteredArgs[1])
			return 2
		}
	case "automation":
		svc, cleanup, err := runtimeAutomationServiceFactory(root)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		defer cleanup()
		switch filteredArgs[1] {
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
			fmt.Fprintf(stderr, "unknown automation subcommand: %s\n", filteredArgs[1])
			return 2
		}
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", filteredArgs[0])
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

func extractGlobalRoot(args []string) (string, []string, error) {
	root := "."
	filtered := make([]string, 0, len(args))
	for idx := 0; idx < len(args); idx++ {
		if args[idx] == "--root" {
			value, next, err := requireFlagValue(args, idx, "--root")
			if err != nil {
				return "", nil, err
			}
			root = value
			idx = next
			continue
		}
		filtered = append(filtered, args[idx])
	}
	return root, filtered, nil
}

func parseNoArgs(args []string, scope string) error {
	if len(args) == 0 {
		return nil
	}
	return fmt.Errorf("unknown %s flag: %s", scope, args[0])
}

func parsePositionalIDAndFlags(args []string, missingMessage, scope string) (string, []string, error) {
	if len(args) == 0 {
		return "", nil, fmt.Errorf("missing %s", missingMessage)
	}
	positionals := make([]string, 0, 1)
	flags := make([]string, 0, len(args))
	for idx := 0; idx < len(args); idx++ {
		if strings.HasPrefix(args[idx], "--") {
			flags = append(flags, args[idx:]...)
			break
		}
		positionals = append(positionals, strings.TrimSpace(args[idx]))
	}
	if len(positionals) == 0 || positionals[0] == "" {
		return "", nil, fmt.Errorf("missing %s", missingMessage)
	}
	if len(positionals) > 1 {
		return "", nil, fmt.Errorf("unknown %s flag: %s", scope, positionals[1])
	}
	return positionals[0], flags, nil
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
