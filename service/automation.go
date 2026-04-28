package service

import (
	"content-hub/domain"
	workspaceinfra "content-hub/infra/workspace"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type AutomationService struct {
	workspaceConfig *WorkspaceConfigService
	ingestion       *IngestionPipelineService
	folderIntake    automationFolderIntake
	jobSvc          *JobService
	mu              sync.Mutex
	running         bool
	stopCh          chan struct{}
	stopAckCh       chan struct{}
	daemonDoneCh    chan struct{}
}

type automationFolderIntake interface {
	RunOnce(ctx context.Context, root string) (automationFolderRunSummary, error)
	RetryFailed(ctx context.Context, root string) (automationFolderRunSummary, error)
}

type automationFolderRunSummary struct {
	ScannedFiles       int
	ImportedFiles      int
	SkippedFiles       int
	FailedFiles        int
	ProcessedPending   int
	ProcessedFailed    int
	CompletedDocuments int
	FailedDocuments    int
}

func NewAutomationService(workspaceConfig *WorkspaceConfigService, ingestion *IngestionPipelineService, folderIntake automationFolderIntake, jobSvc *JobService) *AutomationService {
	return &AutomationService{workspaceConfig: workspaceConfig, ingestion: ingestion, folderIntake: folderIntake, jobSvc: jobSvc}
}

func (s *AutomationService) SetJobService(jobSvc *JobService) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobSvc = jobSvc
}

func (s *AutomationService) SetFolderIntake(folderIntake automationFolderIntake) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.folderIntake = folderIntake
}

func (s *AutomationService) RunOnce(ctx context.Context, root string) (*domain.AutomationRunResult, error) {
	if s.folderIntake != nil {
		return s.runAndPersist(ctx, root, "run-once", func(runCtx context.Context) (map[string]any, error) {
			result, err := s.folderIntake.RunOnce(runCtx, root)
			if err != nil {
				return nil, err
			}
			return folderIntakeSummaryMap(result), nil
		})
	}
	return s.runAndPersist(ctx, root, "run-once", func(runCtx context.Context) (map[string]any, error) {
		result, err := s.ingestion.ImportIncoming(runCtx, root)
		if err != nil {
			return nil, err
		}
		return ingestionSummaryMap(result), nil
	})
}

func (s *AutomationService) RetryFailed(ctx context.Context, root string) (*domain.AutomationRunResult, error) {
	if s.folderIntake != nil {
		return s.runAndPersist(ctx, root, "retry-failed", func(runCtx context.Context) (map[string]any, error) {
			result, err := s.folderIntake.RetryFailed(runCtx, root)
			if err != nil {
				return nil, err
			}
			return folderIntakeSummaryMap(result), nil
		})
	}
	return s.runAndPersist(ctx, root, "retry-failed", func(runCtx context.Context) (map[string]any, error) {
		result, err := s.ingestion.RetryFailed(runCtx, root)
		if err != nil {
			if err.Error() == "transactional bundle import support is required" {
				return map[string]any{
					"scanned_files":          0,
					"imported_files":         0,
					"failed_files":           0,
					"routing_failed_files":   0,
					"total_imported_items":   0,
					"total_created_articles": 0,
				}, nil
			}
			return nil, err
		}
		return ingestionSummaryMap(result), nil
	})
}

func (s *AutomationService) RunDaemon(ctx context.Context, root string, interval time.Duration) (*domain.AutomationRunResult, error) {
	if _, err := s.startDaemonState(); err != nil {
		return nil, err
	}
	defer s.finishDaemonState()
	return s.runDaemonLoop(ctx, root, interval)
}

func (s *AutomationService) StartDaemon(ctx context.Context, root string, interval time.Duration) (*domain.AutomationRunResult, error) {
	if _, err := s.startDaemonState(); err != nil {
		return nil, err
	}
	go func() {
		defer s.finishDaemonState()
		_, _ = s.runDaemonLoop(ctx, root, interval)
	}()
	return &domain.AutomationRunResult{Mode: "daemon", State: "running", RunsExecuted: 0, UpdatedAt: time.Now().UTC()}, nil
}

func (s *AutomationService) runDaemonLoop(ctx context.Context, root string, interval time.Duration) (*domain.AutomationRunResult, error) {
	s.mu.Lock()
	s.mu.Unlock()

	if interval < 0 {
		interval = 0
	}
	runsExecuted := 0
	stopped := false
	lastSummary := map[string]any{}
	for {
		result, err := s.RunOnce(ctx, root)
		if err != nil {
			return nil, err
		}
		runsExecuted++
		lastSummary = result.Summary
		if interval <= 0 {
			interval = 10 * time.Millisecond
		}
		select {
		case <-ctx.Done():
			stopped = true
			return &domain.AutomationRunResult{Mode: "daemon", State: "stopped", Stopped: stopped, RunsExecuted: runsExecuted, Summary: lastSummary, UpdatedAt: time.Now().UTC()}, nil
		case <-s.stopSignal():
			stopped = true
			return &domain.AutomationRunResult{Mode: "daemon", State: "stopped", Stopped: stopped, RunsExecuted: runsExecuted, Summary: lastSummary, UpdatedAt: time.Now().UTC()}, nil
		case <-time.After(interval):
		}
	}
}

func (s *AutomationService) startDaemonState() (<-chan struct{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return nil, domain.NewConflictErr("automation daemon already running")
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.stopAckCh = make(chan struct{})
	s.daemonDoneCh = make(chan struct{})
	return s.daemonDoneCh, nil
}

func (s *AutomationService) finishDaemonState() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	if s.stopAckCh != nil {
		close(s.stopAckCh)
		s.stopAckCh = nil
	}
	if s.daemonDoneCh != nil {
		close(s.daemonDoneCh)
		s.daemonDoneCh = nil
	}
	s.stopCh = nil
}

func (s *AutomationService) Status(_ context.Context, root string) (*domain.AutomationStatusSnapshot, error) {
	liveRunning := s.isRunning()
	path, err := s.snapshotPath(root)
	if err != nil {
		return nil, err
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s.defaultStatus(), nil
		}
		return nil, fmt.Errorf("read automation snapshot: %w", err)
	}
	var snapshot domain.AutomationStatusSnapshot
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return nil, fmt.Errorf("decode automation snapshot: %w", err)
	}
	if liveRunning {
		snapshot.State = "running"
		if snapshot.LastCommand == "" {
			snapshot.LastCommand = "daemon"
		}
	}
	if s.jobSvc != nil {
		snapshot.QueueDepth = s.jobSvc.QueueLength()
	}
	return &snapshot, nil
}

func (s *AutomationService) Health(ctx context.Context, root string) (*domain.AutomationHealthReport, error) {
	status, err := s.Status(ctx, root)
	if err != nil {
		return nil, err
	}
	report := &domain.AutomationHealthReport{
		Status:    "healthy",
		Checks:    map[string]string{"worker": automationWorkerState(s), "queue": "running", "state": status.State},
		UpdatedAt: time.Now().UTC(),
	}
	if status.State == "failed" {
		report.Status = "degraded"
	}
	return report, nil
}

func (s *AutomationService) Stop(_ context.Context, root string) (*domain.AutomationStopResult, error) {
	s.mu.Lock()
	if !s.running || s.stopCh == nil {
		s.mu.Unlock()
		return nil, domain.NewConflictErr("automation daemon is not running")
	}
	stopCh := s.stopCh
	stopAckCh := s.stopAckCh
	s.mu.Unlock()

	select {
	case <-stopCh:
	default:
		close(stopCh)
	}
	if stopAckCh != nil {
		<-stopAckCh
	}
	return &domain.AutomationStopResult{Stopped: true, Reason: "operator request", UpdatedAt: time.Now().UTC()}, nil
}

func (s *AutomationService) runAndPersist(ctx context.Context, root, mode string, run func(context.Context) (map[string]any, error)) (*domain.AutomationRunResult, error) {
	summary, err := run(ctx)
	if err != nil {
		failed := &domain.AutomationStatusSnapshot{State: "failed", QueueDepth: queueDepth(s.jobSvc), LastCommand: mode, LastRunSucceeded: false, Summary: map[string]any{"error": err.Error()}, UpdatedAt: time.Now().UTC()}
		_ = s.writeSnapshot(root, failed)
		return nil, err
	}
	result := &domain.AutomationRunResult{Mode: mode, State: "idle", RunsExecuted: 1, Summary: summary, UpdatedAt: time.Now().UTC()}
	status := &domain.AutomationStatusSnapshot{State: "idle", QueueDepth: queueDepth(s.jobSvc), LastCommand: mode, LastRunSucceeded: true, Summary: summary, UpdatedAt: result.UpdatedAt}
	if err := s.writeSnapshot(root, status); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *AutomationService) snapshotPath(root string) (string, error) {
	resolved, err := s.workspaceConfig.Resolve(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved.Paths.AutomationIncomingDir, "automation_state.json"), nil
}

func (s *AutomationService) writeSnapshot(root string, snapshot *domain.AutomationStatusSnapshot) error {
	path, err := s.snapshotPath(root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create automation snapshot dir: %w", err)
	}
	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("encode automation snapshot: %w", err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("write automation snapshot: %w", err)
	}
	return nil
}

func (s *AutomationService) defaultStatus() *domain.AutomationStatusSnapshot {
	state := "idle"
	if s.isRunning() {
		state = "running"
	}
	return &domain.AutomationStatusSnapshot{State: state, QueueDepth: queueDepth(s.jobSvc), UpdatedAt: time.Now().UTC()}
}

func (s *AutomationService) stopSignal() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopCh
}

func automationWorkerState(s *AutomationService) string {
	if s.isRunning() {
		return "running"
	}
	return "idle"
}

func (s *AutomationService) isRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func queueDepth(jobSvc *JobService) int {
	if jobSvc == nil {
		return 0
	}
	return jobSvc.QueueLength()
}

func ingestionSummaryMap(result *domain.IngestionRunResult) map[string]any {
	if result == nil {
		return map[string]any{}
	}
	return map[string]any{
		"scanned_files":          result.ScannedFiles,
		"imported_files":         result.ImportedFiles,
		"failed_files":           result.FailedFiles,
		"routing_failed_files":   result.RoutingFailedFiles,
		"total_imported_items":   result.TotalImportedItems,
		"total_created_articles": result.TotalCreatedArticles,
	}
}

func folderIntakeSummaryMap(result automationFolderRunSummary) map[string]any {
	return map[string]any{
		"scanned_files":               result.ScannedFiles,
		"imported_files":              result.ImportedFiles,
		"skipped_files":               result.SkippedFiles,
		"failed_files":                result.FailedFiles,
		"processed_pending_documents": result.ProcessedPending,
		"processed_failed_documents":  result.ProcessedFailed,
		"completed_documents":         result.CompletedDocuments,
		"failed_documents":            result.FailedDocuments,
	}
}

type runtimeAutomationFolderIntake struct {
	runtime *FolderIntakeRuntime
}

func NewRuntimeAutomationFolderIntake(runtime *FolderIntakeRuntime) *runtimeAutomationFolderIntake {
	return &runtimeAutomationFolderIntake{runtime: runtime}
}

func (a *runtimeAutomationFolderIntake) RunOnce(ctx context.Context, _ string) (automationFolderRunSummary, error) {
	if a == nil || a.runtime == nil || a.runtime.Scanner == nil || a.runtime.Scheduler == nil {
		return automationFolderRunSummary{}, domain.NewInternalErr("folder intake automation is not configured", nil)
	}
	summary := automationFolderRunSummary{}
	run, err := a.runtime.Scanner.ScanOnce(ctx, a.runtime.WatchDir, a.runtime.ArchiveDir)
	if run != nil {
		summary.ScannedFiles = intMetadata(run.Metadata, "scanned_files")
		summary.ImportedFiles = intMetadata(run.Metadata, "imported_files")
		summary.SkippedFiles = intMetadata(run.Metadata, "skipped_files")
		summary.FailedFiles = intMetadata(run.Metadata, "failed_files")
	}
	if err != nil {
		return summary, err
	}
	processed, processErr := a.runtime.Scheduler.ProcessPending(ctx)
	summary.ProcessedPending = len(processed)
	completed, failed, reloadErr := summarizeProcessedDocuments(ctx, a.runtime.SourceDocumentRepo, processed)
	summary.CompletedDocuments = completed
	summary.FailedDocuments = failed
	if reloadErr != nil {
		return summary, reloadErr
	}
	if processErr != nil {
		return summary, processErr
	}
	return summary, nil
}

func (a *runtimeAutomationFolderIntake) RetryFailed(ctx context.Context, _ string) (automationFolderRunSummary, error) {
	if a == nil || a.runtime == nil || a.runtime.SourceDocumentRepo == nil || a.runtime.Scheduler == nil {
		return automationFolderRunSummary{}, domain.NewInternalErr("folder intake automation is not configured", nil)
	}
	failedDocs, err := a.runtime.SourceDocumentRepo.ListByStatus(ctx, domain.SourceDocumentStatusFailed, 1000)
	if err != nil {
		return automationFolderRunSummary{}, fmt.Errorf("list failed source documents: %w", err)
	}
	for idx := range failedDocs {
		doc := failedDocs[idx]
		doc.Status = domain.SourceDocumentStatusPending
		doc.ClaimedBy = ""
		doc.ClaimedAt = nil
		doc.ProcessingStartedAt = nil
		doc.CompletedAt = nil
		doc.ErrorSummary = ""
		if err := a.runtime.SourceDocumentRepo.Update(ctx, &doc); err != nil {
			return automationFolderRunSummary{}, fmt.Errorf("requeue failed source document %s: %w", doc.ID, err)
		}
	}
	processed, processErr := a.runtime.Scheduler.ProcessPending(ctx)
	summary := automationFolderRunSummary{ProcessedFailed: len(failedDocs)}
	completed, failed, reloadErr := summarizeProcessedDocuments(ctx, a.runtime.SourceDocumentRepo, processed)
	summary.ProcessedPending = len(processed) - len(failedDocs)
	if summary.ProcessedPending < 0 {
		summary.ProcessedPending = 0
	}
	summary.CompletedDocuments = completed
	summary.FailedDocuments = failed
	if reloadErr != nil {
		return summary, reloadErr
	}
	if processErr != nil {
		return summary, processErr
	}
	return summary, nil
}

func summarizeProcessedDocuments(ctx context.Context, repo sourceDocumentStatusReader, docs []domain.SourceDocument) (int, int, error) {
	if repo == nil {
		return 0, 0, domain.NewInternalErr("source document repo is not configured", nil)
	}
	completed := 0
	failed := 0
	for _, doc := range docs {
		stored, err := repo.GetByID(ctx, doc.ID)
		if err != nil {
			return 0, 0, fmt.Errorf("load source document %s: %w", doc.ID, err)
		}
		if stored == nil {
			continue
		}
		switch stored.Status {
		case domain.SourceDocumentStatusCompleted:
			completed++
		case domain.SourceDocumentStatusFailed:
			failed++
		}
	}
	return completed, failed, nil
}

func intMetadata(metadata map[string]any, key string) int {
	if metadata == nil {
		return 0
	}
	value, ok := metadata[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func NewRuntimeAutomationService(root string) (*AutomationService, func() error, error) {
	workspaceConfig := NewWorkspaceConfigService(workspaceinfra.NewLoader(), workspaceinfra.NewValidator())
	repos, cleanup, err := BuildRuntimeRepos(root)
	if err != nil {
		return nil, nil, err
	}
	workflow := NewWorkflowEngine()
	jobSvc := NewJobService(repos.JobRepo, repos.JobEventRepo, workflow)
	ingestionSvc := NewIngestionPipelineService(repos.IngestionRepo, repos.WorkspaceRepo, repos.BundleImportTxStarter, workspaceinfra.NewLoader())
	automationSvc := NewAutomationService(workspaceConfig, ingestionSvc, nil, jobSvc)
	resolved, err := workspaceConfig.Resolve(root)
	if err != nil {
		_ = cleanup()
		return nil, nil, err
	}
	folderRuntime, err := BuildFolderIntakeRuntime(repos, FolderIntakeConfig{
		WatchDir:    resolved.Paths.IncomingDir,
		ArchiveDir:  resolved.Paths.ProcessedDir,
		Concurrency: resolved.Workspace.Collector.GlobalConcurrency,
	})
	if err != nil {
		_ = cleanup()
		return nil, nil, err
	}
	automationSvc.SetFolderIntake(NewRuntimeAutomationFolderIntake(folderRuntime))
	workflow = BuildDefaultWorkflowEngine(root, automationSvc)
	jobSvc = NewJobService(repos.JobRepo, repos.JobEventRepo, workflow)
	automationSvc.SetJobService(jobSvc)
	return automationSvc, cleanup, nil
}
