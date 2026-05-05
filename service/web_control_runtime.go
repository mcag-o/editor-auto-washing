package service

import (
	"content-hub/domain"
	"context"
	"fmt"
)

type WebControlRuntime struct {
	Config   *BusinessConfigService
	Control  *WebControlPlaneService
	Audit    *AuditLogService
	Intake   *WebIntakeService
	Articles *ArticleQueryService
}

type WebControlPlaneService struct {
	control   *ControlStateService
	audit     *AuditLogService
	scheduler *SourceProcessingScheduler
}

func NewWebControlPlaneService(control *ControlStateService, audit *AuditLogService, scheduler *SourceProcessingScheduler) *WebControlPlaneService {
	return &WebControlPlaneService{control: control, audit: audit, scheduler: scheduler}
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
	if err := s.recordAudit(ctx, AuditLogCreateInput{
		Actor:    updatedBy,
		Action:   "control_plane.started",
		Resource: "system_control_state",
		Result:   "success",
		Message:  "started web control plane processing",
		Metadata: map[string]any{"concurrency_limit": concurrencyLimit},
	}); err != nil {
		return nil, err
	}
	if s.scheduler != nil {
		if _, err := s.scheduler.ProcessPending(ctx); err != nil {
			return nil, fmt.Errorf("process pending source documents: %w", err)
		}
	}
	return state, nil
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

func BuildWebControlRuntime(repos *RuntimeRepos) (*WebControlRuntime, error) {
	if repos == nil {
		return nil, domain.NewInternalErr("web control runtime repos are required", nil)
	}

	audit := NewAuditLogService(repos.AuditLogRepo)
	control := NewControlStateService(repos.SystemControlStateRepo)
	rewriteAssembly := buildRewriteAssembly(repos)
	articleIntake := NewArticleIntakeService(repos.WorkspaceRepo, rewriteAssembly.orchestrator)
	renderer := NewFormattingPipelineService(repos.DraftRepo, repos.AssetRepo, repos.WorkspaceRepo, repos.Formatter).WithRenderedDir(repos.RenderedDir)
	rewriteRunner := NewArticleIntakeSourceProcessingRewriteRunner(articleIntake)
	renderRunner := NewFormattingPipelineSourceProcessingRenderRunner(renderer, "")
	worker := NewSourceProcessingWorker(repos.SourceDocumentRepo, rewriteRunner, renderRunner)
	scheduler := NewSourceProcessingScheduler(repos.SourceDocumentRepo, worker, 1, "web-control-runtime")

	return &WebControlRuntime{
		Config:   NewBusinessConfigService(repos.BusinessConfigRepo),
		Control:  NewWebControlPlaneService(control, audit, scheduler),
		Audit:    audit,
		Intake:   NewWebIntakeService(repos.SourceDocumentRepo, repos.AuditLogRepo),
		Articles: NewArticleQueryService(repos.SourceDocumentRepo),
	}, nil
}
