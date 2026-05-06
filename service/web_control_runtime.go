package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"fmt"
)

type WebControlRuntime struct {
	Config    *BusinessConfigService
	Control   *WebControlPlaneService
	Audit     *AuditLogService
	Intake    *WebIntakeService
	Articles  *ArticleQueryService
	Workflows *WorkflowTemplateService
	Templates *TemplateDefinitionService
}

type webControlProcessingCycleRunner interface {
	ProcessPending(context.Context, int) error
}

type WebControlPlaneService struct {
	control *ControlStateService
	audit   *AuditLogService
	runner  webControlProcessingCycleRunner
}

func NewWebControlPlaneService(control *ControlStateService, audit *AuditLogService, runner webControlProcessingCycleRunner) *WebControlPlaneService {
	return &WebControlPlaneService{control: control, audit: audit, runner: runner}
}

func (s *WebControlPlaneService) Get(ctx context.Context) (*domain.SystemControlState, error) {
	if s == nil || s.control == nil {
		return nil, domain.NewInternalErr("web control plane service is not configured", nil)
	}
	return s.control.Get(ctx)
}

func (s *WebControlPlaneService) Start(ctx context.Context, updatedBy string, concurrencyLimit int) (*domain.SystemControlState, error) {
	if s == nil || s.control == nil || s.audit == nil {
		return nil, domain.NewInternalErr("web control plane service is not configured", nil)
	}
	state, err := s.control.Start(ctx, updatedBy, concurrencyLimit)
	if err != nil {
		return nil, err
	}
	limit := concurrencyLimit
	if state != nil {
		if value, ok := state.Metadata["concurrency_limit"].(int); ok && value > 0 {
			limit = value
		}
	}
	stopState := func(reason string) (*domain.SystemControlState, error) {
		current, getErr := s.control.Get(ctx)
		if getErr != nil {
			return nil, getErr
		}
		now := current.UpdatedAt
		now = now.UTC()
		current.State = domain.SystemStateStopped
		current.Reason = reason
		current.UpdatedBy = updatedBy
		current.RequestedAt = &now
		current.UpdatedAt = now
		if err := s.control.repo.Upsert(ctx, current); err != nil {
			return nil, err
		}
		return s.control.repo.Get(ctx)
	}
	if s.runner != nil {
		if err := s.runner.ProcessPending(ctx, limit); err != nil {
			wrapped := fmt.Errorf("process pending source documents: %w", err)
			stoppedState, stopErr := stopState("cycle_failed")
			if stopErr != nil {
				return nil, stopErr
			}
			if auditErr := s.recordAudit(ctx, AuditLogCreateInput{
				Actor:    updatedBy,
				Action:   "control_plane.started",
				Resource: "system_control_state",
				Result:   "failure",
				Message:  wrapped.Error(),
				Metadata: map[string]any{"concurrency_limit": limit},
			}); auditErr != nil {
				return nil, auditErr
			}
			_ = stoppedState
			return nil, wrapped
		}
	}
	stoppedState, stopErr := stopState("cycle_completed")
	if stopErr != nil {
		return nil, stopErr
	}
	if err := s.recordAudit(ctx, AuditLogCreateInput{
		Actor:    updatedBy,
		Action:   "control_plane.started",
		Resource: "system_control_state",
		Result:   "success",
		Message:  "started web control plane processing",
		Metadata: map[string]any{"concurrency_limit": limit},
	}); err != nil {
		return nil, err
	}
	return stoppedState, nil
}

func (s *WebControlPlaneService) Pause(ctx context.Context, updatedBy string) (*domain.SystemControlState, error) {
	if s == nil || s.control == nil {
		return nil, domain.NewInternalErr("web control plane service is not configured", nil)
	}
	return s.control.Pause(ctx, updatedBy)
}

func (s *WebControlPlaneService) Resume(ctx context.Context, updatedBy string) (*domain.SystemControlState, error) {
	if s == nil || s.control == nil {
		return nil, domain.NewInternalErr("web control plane service is not configured", nil)
	}
	return s.control.Resume(ctx, updatedBy)
}

func (s *WebControlPlaneService) recordAudit(ctx context.Context, input AuditLogCreateInput) error {
	if _, err := s.audit.Create(ctx, input); err != nil {
		return fmt.Errorf("write control plane audit log: %w", err)
	}
	return nil
}

type sourceProcessingSchedulerCycleRunner struct {
	repo   repo.SourceDocumentRepo
	worker sourceProcessingSchedulerWorker
	owner  string
}

func newSourceProcessingSchedulerCycleRunner(repo repo.SourceDocumentRepo, worker sourceProcessingSchedulerWorker, owner string) *sourceProcessingSchedulerCycleRunner {
	return &sourceProcessingSchedulerCycleRunner{repo: repo, worker: worker, owner: owner}
}

func (r *sourceProcessingSchedulerCycleRunner) ProcessPending(ctx context.Context, concurrencyLimit int) error {
	scheduler := NewSourceProcessingScheduler(r.repo, r.worker, concurrencyLimit, r.owner)
	_, err := scheduler.ProcessPending(ctx)
	return err
}

func BuildWebControlRuntime(repos *RuntimeRepos) (*WebControlRuntime, error) {
	if repos == nil {
		return nil, domain.NewInternalErr("web control runtime repos are required", nil)
	}

	audit := NewAuditLogService(repos.AuditLogRepo)
	control := NewControlStateService(repos.SystemControlStateRepo)
	rewriteAssembly := buildRewriteAssembly(repos)
	articleIntake := NewArticleIntakeServiceWithWorkflows(repos.WorkspaceRepo, rewriteAssembly.orchestrator, repos.WorkflowDefinitionRepo)
	renderer := NewFormattingPipelineService(repos.DraftRepo, repos.AssetRepo, repos.WorkspaceRepo, repos.Formatter).WithRenderedDir(repos.RenderedDir)
	rewriteRunner := NewArticleIntakeSourceProcessingRewriteRunner(articleIntake)
	renderRunner := NewFormattingPipelineSourceProcessingRenderRunner(renderer, "")
	worker := NewSourceProcessingWorker(repos.SourceDocumentRepo, rewriteRunner, renderRunner)
	runner := newSourceProcessingSchedulerCycleRunner(repos.SourceDocumentRepo, worker, "web-control-runtime")

	return &WebControlRuntime{
		Config:    NewBusinessConfigService(repos.BusinessConfigRepo),
		Control:   NewWebControlPlaneService(control, audit, runner),
		Audit:     audit,
		Intake:    NewWebIntakeService(repos.SourceDocumentRepo, repos.AuditLogRepo),
		Articles:  NewArticleQueryService(repos.SourceDocumentRepo),
		Workflows: NewWorkflowTemplateService(repos.WorkflowDefinitionRepo),
		Templates: NewTemplateDefinitionService(repos.TemplateDefinitionRepo),
	}, nil
}
