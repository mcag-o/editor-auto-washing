package main

import (
	"bytes"
	"content-hub/domain"
	"content-hub/infra/memory"
	workspaceinfra "content-hub/infra/workspace"
	"content-hub/service"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceInitCommandCreatesWorkspace(t *testing.T) {
	root := t.TempDir()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := run([]string{"workspace", "init", "--root", root}, stdout, stderr)

	assert.Equal(t, 0, exitCode)
	assert.FileExists(t, filepath.Join(root, "workspace.yaml"))
	assert.FileExists(t, filepath.Join(root, "secrets.yaml"))
	assert.Contains(t, stdout.String(), "workspace initialized")
	assert.Empty(t, stderr.String())
}

func TestCLIIsDownscopedFromPrimaryOperatorSurface(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := run(nil, stdout, stderr)

	assert.Equal(t, 2, exitCode)
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), "web control plane")
	assert.Contains(t, stderr.String(), "http://localhost:8123")
	assert.Contains(t, stderr.String(), "development/debug")
}

func TestWorkspaceShowConfigCommandPrintsWorkspaceConfig(t *testing.T) {
	root := t.TempDir()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	require.Equal(t, 0, run([]string{"workspace", "init", "--root", root}, &bytes.Buffer{}, &bytes.Buffer{}))

	exitCode := run([]string{"workspace", "show-config", "--root", root}, stdout, stderr)

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout.String(), "content-workspace")
	assert.Contains(t, stdout.String(), "provider_profiles")
	assert.Empty(t, stderr.String())
}

func TestWorkspaceDoctorCommandReturnsNonZeroWhenDiagnosticsHaveErrors(t *testing.T) {
	root := t.TempDir()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	require.Equal(t, 0, run([]string{"workspace", "init", "--root", root}, &bytes.Buffer{}, &bytes.Buffer{}))

	exitCode := run([]string{"workspace", "doctor", "--root", root}, stdout, stderr)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, stdout.String(), "missing secret for provider profile")
	assert.Contains(t, stdout.String(), "missing secret for publish profile")
	assert.Empty(t, stderr.String())
}

func TestWorkspaceResolveConfigAndDoctorHandlePublishProfileEnvironmentSecret(t *testing.T) {
	root := t.TempDir()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	require.Equal(t, 0, run([]string{"workspace", "init", "--root", root}, &bytes.Buffer{}, &bytes.Buffer{}))
	require.NoError(t, os.WriteFile(filepath.Join(root, "workspace.yaml"), []byte(`
name: env-secret-workspace
default_provider_profile: openai-main
default_article_profile: wechat-daily
default_publish_profile: wechat-main
provider_profiles:
  openai-main:
    provider: openai-compatible
    model: gpt-4.1
    secret_ref: env.LLM_API_KEY
article_profiles:
  wechat-daily:
    style: news-rewrite
    output_format: html
    template: daily-intelligence
publish_profiles:
  wechat-main:
    platform: wechat
    account: main
    secret_ref: env.WECHAT_TOKEN
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "secrets.yaml"), []byte("wechat:\n  main: existing-secret\n"), 0o600))
	t.Setenv("LLM_API_KEY", "llm-secret")
	t.Setenv("WECHAT_TOKEN", "wechat-env-token")

	resolveExitCode := run([]string{"workspace", "resolve-config", "--root", root}, stdout, stderr)

	assert.Equal(t, 0, resolveExitCode)
	assert.Contains(t, stdout.String(), "env.WECHAT_TOKEN: wechat-env-token")
	assert.Empty(t, stderr.String())

	stdout.Reset()
	stderr.Reset()
	doctorExitCode := run([]string{"workspace", "doctor", "--root", root}, stdout, stderr)

	assert.Equal(t, 0, doctorExitCode)
	assert.Contains(t, stdout.String(), "workspace diagnostics: ok")
	assert.Empty(t, stderr.String())
}

func TestFormattingRenderCommandPrintsRenderedAssetMetadata(t *testing.T) {
	root := t.TempDir()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	originalFactory := runtimeFormattingServiceFactory
	runtimeFormattingServiceFactory = func(root string) (formattingCLIService, func() error, error) {
		return &cliFormatterServiceStub{
			renderResult: &domain.RenderedAssetRecord{AssetID: "asset-1", ArticleID: "draft-1", Platform: "wechat", OutputFormat: "html", Content: "<html></html>"},
		}, func() error { return nil }, nil
	}
	defer func() { runtimeFormattingServiceFactory = originalFactory }()

	exitCode := run([]string{"formatting", "render", "draft-1", "--platform", "wechat", "--template", "daily-intelligence", "--root", root}, stdout, stderr)

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout.String(), "asset-1")
	assert.Empty(t, stderr.String())
}

func TestFormattingValidateCommandPrintsValidationReport(t *testing.T) {
	root := t.TempDir()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	originalFactory := runtimeFormattingServiceFactory
	runtimeFormattingServiceFactory = func(root string) (formattingCLIService, func() error, error) {
		return &cliFormatterServiceStub{
			validationResult: domain.DraftValidationResult{Warnings: []string{"meta.thumb_media_id is missing"}},
		}, func() error { return nil }, nil
	}
	defer func() { runtimeFormattingServiceFactory = originalFactory }()

	exitCode := run([]string{"formatting", "validate", "draft-1", "--platform", "wechat", "--template", "daily-intelligence", "--root", root}, stdout, stderr)

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout.String(), "meta.thumb_media_id is missing")
	assert.Empty(t, stderr.String())
}

func TestReviewApproveRejectPublishAndHistoryCommands(t *testing.T) {
	root := t.TempDir()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	originalFactory := runtimeReviewPublishServiceFactory
	runtimeReviewPublishServiceFactory = func(root string) (reviewPublishCLIService, func() error, error) {
		return &cliReviewPublishServiceStub{
			review:  &domain.ReviewTask{ID: "review-1", ArticleID: "draft-1", Status: domain.ReviewStatusApproved},
			history: []domain.PublishRecord{{ReviewID: "review-1", ArticleID: "draft-1", AssetID: "asset-1", Success: true, Message: "published"}},
		}, func() error { return nil }, nil
	}
	defer func() { runtimeReviewPublishServiceFactory = originalFactory }()

	exitCode := run([]string{"review", "approve", "review-1", "--reviewer", "alice", "--notes", "ok", "--root", root}, stdout, stderr)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout.String(), domain.ReviewStatusApproved)
	assert.Empty(t, stderr.String())

	stdout.Reset()
	stderr.Reset()
	exitCode = run([]string{"review", "reject", "review-1", "--reviewer", "bob", "--notes", "retry", "--root", root}, stdout, stderr)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout.String(), domain.ReviewStatusRejected)

	stdout.Reset()
	stderr.Reset()
	exitCode = run([]string{"publish", "run", "review-1", "--root", root}, stdout, stderr)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout.String(), "asset-1")

	stdout.Reset()
	stderr.Reset()
	exitCode = run([]string{"publish", "history", "draft-1", "--root", root}, stdout, stderr)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout.String(), "review-1")
	assert.Contains(t, stdout.String(), "asset-1")
	assert.Empty(t, stderr.String())
}

func TestCLIRewriteRunInvokesRuntimeRewriteService(t *testing.T) {
	root := t.TempDir()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	originalFactory := runtimeRewriteServiceFactory
	called := false
	var received service.RewriteRunRequest
	runtimeRewriteServiceFactory = func(root string) (rewriteCLIService, func() error, error) {
		return &stubRewriteCLIService{runFn: func(_ context.Context, req service.RewriteRunRequest) (*domain.RewritePipelineRun, error) {
			called = true
			received = req
			return &domain.RewritePipelineRun{ID: "run-1", Status: domain.RewriteRunSucceeded}, nil
		}}, func() error { return nil }, nil
	}
	defer func() { runtimeRewriteServiceFactory = originalFactory }()

	exitCode := run([]string{"rewrite", "run", "article-1", "--target", "wechat-longform", "--source", "sspai", "--root", root}, stdout, stderr)

	assert.Equal(t, 0, exitCode)
	assert.True(t, called)
	assert.Equal(t, service.RewriteRunRequest{
		WorkspaceArticleID: "article-1",
		TargetType:         "wechat-longform",
		SourceProfile:      "sspai",
		Version:            "latest",
	}, received)
	assert.Contains(t, stdout.String(), "run-1")
	assert.Empty(t, stderr.String())
}

func TestCLIRewriteRunRequiresPositionalWorkspaceArticleID(t *testing.T) {
	root := t.TempDir()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := run([]string{"rewrite", "run", "--target", "wechat-longform", "--source", "sspai", "--root", root}, stdout, stderr)

	assert.Equal(t, 2, exitCode)
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), "missing positional workspace article id")
}

func TestCLIRewriteRunArgumentFailureDoesNotInvokeRuntimeFactory(t *testing.T) {
	root := t.TempDir()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	originalFactory := runtimeRewriteServiceFactory
	called := false
	runtimeRewriteServiceFactory = func(root string) (rewriteCLIService, func() error, error) {
		called = true
		return nil, func() error { return nil }, nil
	}
	defer func() { runtimeRewriteServiceFactory = originalFactory }()

	exitCode := run([]string{"rewrite", "run", "--target", "wechat-longform", "--source", "sspai", "--root", root}, stdout, stderr)

	assert.Equal(t, 2, exitCode)
	assert.False(t, called)
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), "missing positional workspace article id")
}

func TestRuntimeRewriteCLIServiceRunDerivesWorkspaceFields(t *testing.T) {
	called := false
	var received service.RewriteRunRequest
	svc := &runtimeRewriteCLIService{
		workspaceRepo: stubWorkspaceReader{workspace: &domain.ArticleWorkspaceRecord{
			ID:       "article-1",
			Title:    "Source Title",
			Metadata: map[string]any{"collector_article_id": "collector-1"},
		}},
		runner: &stubRewriteCLIService{runFn: func(_ context.Context, req service.RewriteRunRequest) (*domain.RewritePipelineRun, error) {
			called = true
			received = req
			return &domain.RewritePipelineRun{ID: "run-1", Status: domain.RewriteRunSucceeded}, nil
		}},
	}

	result, err := svc.Run(t.Context(), service.RewriteRunRequest{WorkspaceArticleID: "article-1", TargetType: "wechat-longform", SourceProfile: "sspai", Version: "latest"})

	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, &domain.RewritePipelineRun{ID: "run-1", Status: domain.RewriteRunSucceeded}, result)
	assert.Equal(t, service.RewriteRunRequest{WorkspaceArticleID: "article-1", CollectorArticleID: "collector-1", Title: "Source Title", TargetType: "wechat-longform", SourceProfile: "sspai", Version: "latest"}, received)
}

func TestRuntimeRewriteCLIServiceRunReturnsErrorWhenCollectorArticleIDMissing(t *testing.T) {
	runtime := &runtimeRewriteCLIService{
		workspaceRepo: stubWorkspaceReader{workspace: &domain.ArticleWorkspaceRecord{ID: "article-1", Title: "Source Title", Metadata: map[string]any{}}},
		runner: &stubRewriteCLIService{runFn: func(_ context.Context, req service.RewriteRunRequest) (*domain.RewritePipelineRun, error) {
			return nil, nil
		}},
	}

	result, err := runtime.Run(t.Context(), service.RewriteRunRequest{WorkspaceArticleID: "article-1", TargetType: "wechat-longform", SourceProfile: "sspai", Version: "latest"})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "collector_article_id")
}

func TestAutomationCommandsRunOnceDaemonStatusHealthRetryFailedAndStop(t *testing.T) {
	root := t.TempDir()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	originalFactory := runtimeAutomationServiceFactory
	runtimeAutomationServiceFactory = func(root string) (automationCLIService, func() error, error) {
		return &cliAutomationServiceStub{
			runResult: &domain.AutomationRunResult{
				Mode:         "run-once",
				Stopped:      false,
				RunsExecuted: 1,
				Summary: map[string]any{
					"imported_files": 1,
				},
			},
			daemonResult: &domain.AutomationRunResult{
				Mode:         "daemon",
				State:        "stopped",
				Stopped:      true,
				RunsExecuted: 2,
			},
			statusResult: &domain.AutomationStatusSnapshot{
				State:            "idle",
				QueueDepth:       0,
				LastCommand:      "run-once",
				LastRunSucceeded: true,
			},
			healthResult: &domain.AutomationHealthReport{
				Status: "healthy",
				Checks: map[string]string{"worker": "running"},
			},
			stopResult: &domain.AutomationStopResult{
				Stopped: true,
				Reason:  "operator request",
			},
		}, func() error { return nil }, nil
	}
	defer func() { runtimeAutomationServiceFactory = originalFactory }()

	exitCode := run([]string{"automation", "run-once", "--root", root}, stdout, stderr)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout.String(), "run-once")
	assert.Empty(t, stderr.String())

	stdout.Reset()
	stderr.Reset()
	exitCode = run([]string{"automation", "retry-failed", "--root", root}, stdout, stderr)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout.String(), "run-once")

	stdout.Reset()
	stderr.Reset()
	exitCode = run([]string{"automation", "daemon", "--root", root}, stdout, stderr)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout.String(), "daemon")
	assert.Contains(t, stdout.String(), "stopped")
	assert.Contains(t, stdout.String(), "true")

	stdout.Reset()
	stderr.Reset()
	exitCode = run([]string{"automation", "status", "--root", root}, stdout, stderr)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout.String(), "idle")
	assert.Contains(t, stdout.String(), "run-once")

	stdout.Reset()
	stderr.Reset()
	exitCode = run([]string{"automation", "health", "--root", root}, stdout, stderr)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout.String(), "healthy")
	assert.Contains(t, stdout.String(), "worker")

	stdout.Reset()
	stderr.Reset()
	exitCode = run([]string{"automation", "stop", "--root", root}, stdout, stderr)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout.String(), "operator request")
	assert.Empty(t, stderr.String())
}

func TestNewCollectorSourceDefaultsDetailFetchDisabled(t *testing.T) {
	source := domain.NewCollectorSource("baidu", "Baidu")

	assert.False(t, source.DetailFetchEnabled)
}

func TestRuntimeAutomationCLIServiceRunDaemonBlocksUntilContextStops(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceConfigFileName), []byte("name: cli-test\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))
	for _, dir := range []string{"incoming", "incoming/processed", "incoming/failed"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, dir), 0o755))
	}

	provider := memory.NewProvider()
	ingestionSvc := service.NewIngestionPipelineService(provider.IngestionRepo(), provider.WorkspaceRepo(), provider, workspaceinfra.NewLoader())
	automationSvc := service.NewAutomationService(service.NewWorkspaceConfigService(workspaceinfra.NewLoader(), workspaceinfra.NewValidator()), ingestionSvc, nil, nil)
	cliSvc := &runtimeAutomationCLIService{root: root, svc: automationSvc}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resultCh := make(chan *domain.AutomationRunResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := cliSvc.RunDaemon(ctx)
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	select {
	case err := <-errCh:
		t.Fatalf("unexpected daemon error: %v", err)
	case <-resultCh:
		t.Fatal("RunDaemon returned before context cancellation")
	case <-time.After(50 * time.Millisecond):
	}

	cancel()

	select {
	case err := <-errCh:
		t.Fatalf("unexpected daemon error after cancel: %v", err)
	case result := <-resultCh:
		assert.Equal(t, "daemon", result.Mode)
		assert.True(t, result.Stopped)
	case <-time.After(2 * time.Second):
		t.Fatal("RunDaemon did not stop after context cancellation")
	}
}

func TestLegacyRSSCLICommandsRemoved(t *testing.T) {
	root := t.TempDir()
	testCases := [][]string{
		{"rss", "subscriptions", "list", "--root", root},
		{"rss", "subscriptions", "add", "--name", "Daily Feed", "--feed-url", "https://example.com/feed.xml", "--target", "wechat-longform", "--source", "sspai", "--root", root},
		{"rss", "subscriptions", "update", "sub-1", "--name", "Updated Feed", "--root", root},
		{"rss", "subscriptions", "remove", "sub-1", "--root", root},
		{"rss", "run", "sub-1", "--root", root},
		{"rss", "run-all", "--root", root},
		{"rss", "runs", "list", "--limit", "5", "--root", root},
		{"rss", "items", "list", "--limit", "7", "--root", root},
	}

	for _, args := range testCases {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		exitCode := run(args, stdout, stderr)
		assert.Equal(t, 2, exitCode, "%v should be rejected", args)
		assert.Empty(t, stdout.String())
		assert.Contains(t, stderr.String(), "unknown command: rss")
	}
}

func TestCLIRSSCommandReplacesLegacyIntakeAndCollectorSurface(t *testing.T) {
	root := t.TempDir()

	t.Run("ingestion removed", func(t *testing.T) {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		exitCode := run([]string{"ingestion", "import", "--root", root}, stdout, stderr)
		assert.Equal(t, 2, exitCode)
		assert.Empty(t, stdout.String())
		assert.Contains(t, stderr.String(), "unknown command: ingestion")
	})

	t.Run("collector removed", func(t *testing.T) {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		exitCode := run([]string{"collector", "sources", "list", "--root", root}, stdout, stderr)
		assert.Equal(t, 2, exitCode)
		assert.Empty(t, stdout.String())
		assert.Contains(t, stderr.String(), "unknown command: collector")
	})
}

type cliFormatterServiceStub struct {
	renderResult     *domain.RenderedAssetRecord
	validationResult domain.DraftValidationResult
}

func (s *cliFormatterServiceStub) Render(_ context.Context, draftID, platform, templateName string) (*domain.RenderedAssetRecord, error) {
	return s.renderResult, nil
}

func (s *cliFormatterServiceStub) Validate(_ context.Context, draftID, platform, templateName string) (domain.DraftValidationResult, error) {
	return s.validationResult, nil
}

func (s *cliFormatterServiceStub) GetAsset(_ context.Context, assetID string) (*domain.RenderedAssetRecord, error) {
	return nil, nil
}

type cliReviewPublishServiceStub struct {
	review  *domain.ReviewTask
	history []domain.PublishRecord
}

type cliAutomationServiceStub struct {
	runResult    *domain.AutomationRunResult
	daemonResult *domain.AutomationRunResult
	statusResult *domain.AutomationStatusSnapshot
	healthResult *domain.AutomationHealthReport
	stopResult   *domain.AutomationStopResult
}

type stubRewriteCLIService struct {
	runFn func(context.Context, service.RewriteRunRequest) (*domain.RewritePipelineRun, error)
}

type stubWorkspaceReader struct {
	workspace *domain.ArticleWorkspaceRecord
	err       error
}

func (s *cliReviewPublishServiceStub) ApproveReview(_ context.Context, id, reviewer, notes string) (*domain.ReviewTask, error) {
	result := *s.review
	result.ID = id
	result.Status = domain.ReviewStatusApproved
	result.Reviewer = reviewer
	result.Notes = notes
	return &result, nil
}

func (s *cliReviewPublishServiceStub) RejectReview(_ context.Context, id, reviewer, notes string) (*domain.ReviewTask, error) {
	result := *s.review
	result.ID = id
	result.Status = domain.ReviewStatusRejected
	result.Reviewer = reviewer
	result.Notes = notes
	return &result, nil
}

func (s *cliReviewPublishServiceStub) PublishReview(_ context.Context, reviewID string) (*domain.PublishOutcome, error) {
	result := append([]domain.PublishRecord(nil), s.history...)
	for idx := range result {
		result[idx].ReviewID = reviewID
	}
	return &domain.PublishOutcome{Success: true, WorkspaceSynced: true, Records: result}, nil
}

func (s *cliReviewPublishServiceStub) History(_ context.Context, articleID string) ([]domain.PublishRecord, error) {
	return append([]domain.PublishRecord(nil), s.history...), nil
}

func (s *cliAutomationServiceStub) RunOnce(_ context.Context) (*domain.AutomationRunResult, error) {
	return s.runResult, nil
}

func (s *cliAutomationServiceStub) RunDaemon(_ context.Context) (*domain.AutomationRunResult, error) {
	return s.daemonResult, nil
}

func (s *cliAutomationServiceStub) RetryFailed(_ context.Context) (*domain.AutomationRunResult, error) {
	return s.runResult, nil
}

func (s *cliAutomationServiceStub) Status(_ context.Context) (*domain.AutomationStatusSnapshot, error) {
	return s.statusResult, nil
}

func (s *cliAutomationServiceStub) Health(_ context.Context) (*domain.AutomationHealthReport, error) {
	return s.healthResult, nil
}

func (s *cliAutomationServiceStub) Stop(_ context.Context) (*domain.AutomationStopResult, error) {
	return s.stopResult, nil
}

func (s *stubRewriteCLIService) Run(ctx context.Context, req service.RewriteRunRequest) (*domain.RewritePipelineRun, error) {
	return s.runFn(ctx, req)
}

func (s stubWorkspaceReader) GetByID(_ context.Context, id string) (*domain.ArticleWorkspaceRecord, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.workspace == nil || s.workspace.ID != id {
		return nil, domain.NewNotFoundErr("workspace article", id)
	}
	return s.workspace, nil
}
