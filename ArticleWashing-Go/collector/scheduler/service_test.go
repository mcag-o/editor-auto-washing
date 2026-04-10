package scheduler_test

import (
	"content-hub/collector/plugin"
	"content-hub/collector/scheduler"
	collectorsvc "content-hub/collector/service"
	"content-hub/domain"
	"content-hub/infra/sqlite"
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduler_RunOnceCreatesCollectorRunAndSourceRuns(t *testing.T) {
	provider := newSchedulerProvider(t)
	registry := plugin.NewRegistry()
	require.NoError(t, registry.Register(&schedulerStubPlugin{
		sourceID:    "scheduler-source",
		displayName: "Scheduler Source",
		hotlist:     []plugin.HotEntry{{SourceID: "scheduler-source", ExternalID: "entry-1", Title: "Entry", CanonicalURL: "https://example.com/entry"}},
	}))

	registrySvc := collectorsvc.NewSourceRegistryService(provider.CollectorSourceRepo(), registry)
	require.NoError(t, registrySvc.Sync(t.Context()))
	runSvc := collectorsvc.NewRunService(provider.CollectorSourceRepo(), provider.CollectorRunRepo(), provider.CollectorEntryRepo(), registry)
	svc := scheduler.NewService(provider.CollectorSchedulerStateRepo(), runSvc, 30*time.Minute)

	result, err := svc.RunOnce(t.Context())

	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.SourceCount, 1)
	assert.Equal(t, domain.CollectorRunSucceeded, result.Status)

	status, err := svc.Status(t.Context())
	require.NoError(t, err)
	assert.Equal(t, domain.CollectorSchedulerIdle, status.State)
	assert.Equal(t, result.RunID, status.LastRunID)

	health, err := svc.Health(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)
	assert.Equal(t, domain.CollectorSchedulerIdle, health.Checks["state"])
}

func TestScheduler_DaemonStatusHealthAndStop(t *testing.T) {
	provider := newSchedulerProvider(t)
	registry := plugin.NewRegistry()
	require.NoError(t, registry.Register(&schedulerStubPlugin{
		sourceID:    "daemon-source",
		displayName: "Daemon Source",
		hotlist:     []plugin.HotEntry{{SourceID: "daemon-source", ExternalID: "entry-1", Title: "Entry", CanonicalURL: "https://example.com/entry"}},
	}))

	registrySvc := collectorsvc.NewSourceRegistryService(provider.CollectorSourceRepo(), registry)
	require.NoError(t, registrySvc.Sync(t.Context()))
	runSvc := collectorsvc.NewRunService(provider.CollectorSourceRepo(), provider.CollectorRunRepo(), provider.CollectorEntryRepo(), registry)
	svc := scheduler.NewService(provider.CollectorSchedulerStateRepo(), runSvc, 10*time.Millisecond)

	start, err := svc.StartDaemon(t.Context())
	require.NoError(t, err)
	assert.True(t, start.Started)
	assert.Equal(t, domain.CollectorSchedulerRunning, start.State)

	require.Eventually(t, func() bool {
		status, statusErr := svc.Status(t.Context())
		return statusErr == nil && status.State == domain.CollectorSchedulerRunning
	}, time.Second, 10*time.Millisecond)

	health, err := svc.Health(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)
	assert.Equal(t, domain.CollectorSchedulerRunning, health.Checks["state"])

	stop, err := svc.Stop(t.Context())
	require.NoError(t, err)
	assert.True(t, stop.Stopped)
	assert.Equal(t, domain.CollectorSchedulerStopped, stop.State)

	require.Eventually(t, func() bool {
		status, statusErr := svc.Status(t.Context())
		return statusErr == nil && status.State == domain.CollectorSchedulerStopped
	}, time.Second, 10*time.Millisecond)
}

func TestScheduler_DaemonSurvivesStartContextCancellation(t *testing.T) {
	provider := newSchedulerProvider(t)
	registry := plugin.NewRegistry()
	require.NoError(t, registry.Register(&schedulerStubPlugin{
		sourceID:    "ctx-source",
		displayName: "Context Source",
		hotlist:     []plugin.HotEntry{{SourceID: "ctx-source", ExternalID: "entry-1", Title: "Entry", CanonicalURL: "https://example.com/entry"}},
	}))

	registrySvc := collectorsvc.NewSourceRegistryService(provider.CollectorSourceRepo(), registry)
	require.NoError(t, registrySvc.Sync(t.Context()))
	runSvc := collectorsvc.NewRunService(provider.CollectorSourceRepo(), provider.CollectorRunRepo(), provider.CollectorEntryRepo(), registry)
	svc := scheduler.NewService(provider.CollectorSchedulerStateRepo(), runSvc, 25*time.Millisecond)

	startCtx, cancel := context.WithCancel(t.Context())
	start, err := svc.StartDaemon(startCtx)
	require.NoError(t, err)
	assert.True(t, start.Started)

	cancel()

	require.Eventually(t, func() bool {
		status, statusErr := svc.Status(t.Context())
		return statusErr == nil && status.Running && status.State == domain.CollectorSchedulerRunning
	}, time.Second, 10*time.Millisecond)

	stop, err := svc.Stop(t.Context())
	require.NoError(t, err)
	assert.True(t, stop.Stopped)
}

func TestScheduler_StartDaemonFailsWhenAlreadyRunning(t *testing.T) {
	provider := newSchedulerProvider(t)
	registry := plugin.NewRegistry()
	require.NoError(t, registry.Register(&schedulerStubPlugin{sourceID: "dup-source", displayName: "Duplicate Source", hotlist: []plugin.HotEntry{{SourceID: "dup-source", ExternalID: "entry-1", Title: "Entry", CanonicalURL: "https://example.com/entry"}}}))
	registrySvc := collectorsvc.NewSourceRegistryService(provider.CollectorSourceRepo(), registry)
	require.NoError(t, registrySvc.Sync(t.Context()))
	runSvc := collectorsvc.NewRunService(provider.CollectorSourceRepo(), provider.CollectorRunRepo(), provider.CollectorEntryRepo(), registry)
	svc := scheduler.NewService(provider.CollectorSchedulerStateRepo(), runSvc, 10*time.Millisecond)

	_, err := svc.StartDaemon(t.Context())
	require.NoError(t, err)

	_, err = svc.StartDaemon(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	_, stopErr := svc.Stop(t.Context())
	require.NoError(t, stopErr)
}

func TestScheduler_StopFailsWhenNotRunning(t *testing.T) {
	provider := newSchedulerProvider(t)
	svc := scheduler.NewService(provider.CollectorSchedulerStateRepo(), &blockingRunService{}, 10*time.Millisecond)

	_, err := svc.Stop(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestScheduler_StopHonorsContextWhenRunInFlight(t *testing.T) {
	provider := newSchedulerProvider(t)
	blocker := newNonInterruptibleRunService()
	svc := scheduler.NewService(provider.CollectorSchedulerStateRepo(), blocker, 10*time.Millisecond)

	_, err := svc.StartDaemon(t.Context())
	require.NoError(t, err)
	blocker.waitUntilStarted(t)

	stopCtx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	_, err = svc.Stop(stopCtx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	blocker.unblock()
	require.Eventually(t, func() bool {
		status, statusErr := svc.Status(t.Context())
		return statusErr == nil && !status.Running
	}, time.Second, 10*time.Millisecond)
}

func newSchedulerProvider(t *testing.T) *sqlite.Provider {
	t.Helper()
	dbPath := fmt.Sprintf("%s/test_collector_scheduler_%d.db", t.TempDir(), os.Getpid())
	provider, err := sqlite.NewProvider(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	return provider
}

type schedulerStubPlugin struct {
	sourceID    string
	displayName string
	hotlist     []plugin.HotEntry
	healthErr   error
}

type blockingRunService struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

type nonInterruptibleRunService struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func newNonInterruptibleRunService() *nonInterruptibleRunService {
	return &nonInterruptibleRunService{started: make(chan struct{}), release: make(chan struct{})}
}

func (s *nonInterruptibleRunService) RunHotlist(_ context.Context, trigger string) (*domain.CollectorRunSummary, error) {
	s.once.Do(func() { close(s.started) })
	<-s.release
	now := time.Now().UTC()
	return &domain.CollectorRunSummary{RunID: "non-interruptible-run", Trigger: trigger, Status: domain.CollectorRunSucceeded, StartedAt: now, CompletedAt: now}, nil
}

func (s *nonInterruptibleRunService) waitUntilStarted(t *testing.T) {
	t.Helper()
	select {
	case <-s.started:
	case <-time.After(time.Second):
		t.Fatal("non-interruptible run service did not start")
	}
}

func (s *nonInterruptibleRunService) unblock() {
	select {
	case <-s.release:
	default:
		close(s.release)
	}
}

func newBlockingRunService() *blockingRunService {
	return &blockingRunService{started: make(chan struct{}), release: make(chan struct{})}
}

func (s *blockingRunService) RunHotlist(ctx context.Context, trigger string) (*domain.CollectorRunSummary, error) {
	s.once.Do(func() { close(s.started) })
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.release:
		now := time.Now().UTC()
		return &domain.CollectorRunSummary{RunID: "blocking-run", Trigger: trigger, Status: domain.CollectorRunSucceeded, StartedAt: now, CompletedAt: now}, nil
	}
}

func (s *blockingRunService) waitUntilStarted(t *testing.T) {
	t.Helper()
	select {
	case <-s.started:
	case <-time.After(time.Second):
		t.Fatal("run service did not start")
	}
}

func (s *blockingRunService) unblock() {
	select {
	case <-s.release:
	default:
		close(s.release)
	}
}

func (p *schedulerStubPlugin) SourceID() string { return p.sourceID }

func (p *schedulerStubPlugin) DisplayName() string { return p.displayName }

func (p *schedulerStubPlugin) Aliases() []string { return nil }

func (p *schedulerStubPlugin) FetchHotlist(_ context.Context, _ plugin.FetchHotlistRequest) ([]plugin.HotEntry, error) {
	return append([]plugin.HotEntry(nil), p.hotlist...), nil
}

func (p *schedulerStubPlugin) FetchArticle(_ context.Context, _ plugin.FetchArticleRequest) (*plugin.RawArticle, error) {
	return nil, plugin.ErrArticleFetchNotSupported(p.sourceID)
}

func (p *schedulerStubPlugin) NormalizeHotEntry(raw any) (plugin.HotEntry, error) {
	entry, _ := raw.(plugin.HotEntry)
	return entry, nil
}

func (p *schedulerStubPlugin) NormalizeArticle(any) (*plugin.NormalizedArticle, error) {
	return nil, plugin.ErrArticleFetchNotSupported(p.sourceID)
}

func (p *schedulerStubPlugin) HealthCheck(_ context.Context) (plugin.SourceHealth, error) {
	health := plugin.SourceHealth{SourceID: p.sourceID, OK: p.healthErr == nil, CheckedAt: time.Now().UTC()}
	if p.healthErr != nil {
		health.Message = p.healthErr.Error()
	}
	return health, p.healthErr
}

func (p *schedulerStubPlugin) Capabilities() plugin.SourceCapabilities {
	return plugin.SourceCapabilities{SupportsHotlist: true, SupportsArticle: false, AuthModes: []string{domain.CollectorAuthModeNone}}
}
