package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"fmt"
	"strings"
)

type WebControlRuntime struct {
	Config         *BusinessConfigService
	Control        *WebControlPlaneService
	Audit          *AuditLogService
	Intake         *WebIntakeService
	ExternalIntake ExternalIntakeProcessor
	Articles       *ArticleQueryService
	Workflows      *WorkflowTemplateService
	Templates      *TemplateDefinitionService
}

type webControlProcessingCycleRunner interface {
	ProcessPending(context.Context, int) error
}

type browserWorkspaceContinuationIntake interface {
	IntakeResultIntoWorkspace(ctx context.Context, workspaceArticleID string, article domain.IntakeArticle) (*ArticleIntakeResult, error)
	ResumeResult(ctx context.Context, rewriteRunID string, article domain.IntakeArticle) (*SourceProcessingRewriteResult, error)
}

type browserWorkspaceContinuationRenderer interface {
	Render(ctx context.Context, draftID, platform, templateName string) (*domain.RenderedAssetRecord, error)
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
			wrapped := fmt.Errorf("process pending browser intake items: %w", err)
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

type browserWorkspaceContinuationRunner struct {
	workspaces repo.WorkspaceRepo
	intake     browserWorkspaceContinuationIntake
	renderer   browserWorkspaceContinuationRenderer
}

func newBrowserWorkspaceContinuationRunner(workspaces repo.WorkspaceRepo, intake browserWorkspaceContinuationIntake, renderer browserWorkspaceContinuationRenderer) *browserWorkspaceContinuationRunner {
	return &browserWorkspaceContinuationRunner{workspaces: workspaces, intake: intake, renderer: renderer}
}

func (r *browserWorkspaceContinuationRunner) ProcessPending(ctx context.Context, concurrencyLimit int) error {
	if r == nil || r.workspaces == nil || r.intake == nil || r.renderer == nil {
		return domain.NewInternalErr("browser workspace continuation runner is not configured", nil)
	}
	status := domain.ArticleWorkspaceStatusImported
	items, err := r.workspaces.List(ctx, &status)
	if err != nil {
		return fmt.Errorf("list imported workspace articles: %w", err)
	}
	processed := 0
	for _, item := range items {
		if concurrencyLimit > 0 && processed >= concurrencyLimit {
			break
		}
		if !isBrowserIntakeWorkspace(item) {
			continue
		}
		if err := r.processWorkspaceItem(ctx, item, "browser"); err != nil {
			return err
		}
		processed++
	}
	return nil
}

func (r *browserWorkspaceContinuationRunner) ProcessWorkspace(ctx context.Context, workspaceArticleID string) error {
	if r == nil || r.workspaces == nil || r.intake == nil || r.renderer == nil {
		return domain.NewInternalErr("browser workspace continuation runner is not configured", nil)
	}
	workspaceArticleID = strings.TrimSpace(workspaceArticleID)
	if workspaceArticleID == "" {
		return domain.NewValidationErr("workspace article id is required", nil)
	}
	item, err := r.workspaces.GetByID(ctx, workspaceArticleID)
	if err != nil {
		return err
	}
	if item.Status != domain.ArticleWorkspaceStatusImported {
		return domain.NewConflictErr("workspace article is not imported")
	}
	if !isExternalIntakeWorkspace(*item) {
		return domain.NewValidationErr("workspace article is not an external API intake item", nil)
	}
	return r.processWorkspaceItem(ctx, *item, "external API")
}

func (r *browserWorkspaceContinuationRunner) processWorkspaceItem(ctx context.Context, item domain.ArticleWorkspaceRecord, label string) error {
	if rewriteRunID := browserWorkspaceResumeRewriteRunID(item); rewriteRunID != "" {
		result, err := r.intake.ResumeResult(ctx, rewriteRunID, browserWorkspaceToIntakeArticle(item))
		if err != nil {
			return fmt.Errorf("resume %s workspace %s: %w", label, item.ID, err)
		}
		if result == nil || strings.TrimSpace(result.DraftID) == "" {
			return domain.NewInternalErr(label+" workspace continuation did not return a draft id", nil)
		}
		updated, getErr := r.workspaces.GetByID(ctx, item.ID)
		if getErr != nil {
			return fmt.Errorf("load resumed %s workspace %s: %w", label, item.ID, getErr)
		}
		delete(updated.Metadata, "resume_rewrite_run_id")
		updated.UpdatedAt = updated.UpdatedAt.UTC()
		if err := r.workspaces.Update(ctx, updated); err != nil {
			return fmt.Errorf("clear %s workspace resume marker %s: %w", label, item.ID, err)
		}
		if _, err := r.renderer.Render(ctx, strings.TrimSpace(result.DraftID), browserWorkspaceRenderPlatform(item), ""); err != nil {
			return fmt.Errorf("render %s workspace %s: %w", label, item.ID, err)
		}
		return nil
	}
	result, err := r.intake.IntakeResultIntoWorkspace(ctx, item.ID, browserWorkspaceToIntakeArticle(item))
	if err != nil {
		return fmt.Errorf("continue %s workspace %s: %w", label, item.ID, err)
	}
	if result == nil || strings.TrimSpace(result.DraftID) == "" {
		return domain.NewInternalErr(label+" workspace continuation did not return a draft id", nil)
	}
	if _, err := r.renderer.Render(ctx, strings.TrimSpace(result.DraftID), browserWorkspaceRenderPlatform(item), ""); err != nil {
		return fmt.Errorf("render %s workspace %s: %w", label, item.ID, err)
	}
	return nil
}

type combinedWebControlProcessingRunner struct {
	runners []webControlProcessingCycleRunner
}

func newCombinedWebControlProcessingRunner(runners ...webControlProcessingCycleRunner) *combinedWebControlProcessingRunner {
	return &combinedWebControlProcessingRunner{runners: runners}
}

func (r *combinedWebControlProcessingRunner) ProcessPending(ctx context.Context, concurrencyLimit int) error {
	for _, runner := range r.runners {
		if runner == nil {
			continue
		}
		if err := runner.ProcessPending(ctx, concurrencyLimit); err != nil {
			return err
		}
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
	articleIntake := NewArticleIntakeServiceWithWorkflows(repos.WorkspaceRepo, rewriteAssembly.orchestrator, repos.WorkflowDefinitionRepo)
	renderer := NewFormattingPipelineService(repos.DraftRepo, repos.AssetRepo, repos.WorkspaceRepo, repos.Formatter).WithRenderedDir(repos.RenderedDir)
	browserRunner := newBrowserWorkspaceContinuationRunner(repos.WorkspaceRepo, articleIntake, renderer)
	runner := newCombinedWebControlProcessingRunner(browserRunner)

	return &WebControlRuntime{
		Config:         NewBusinessConfigService(repos.BusinessConfigRepo),
		Control:        NewWebControlPlaneService(control, audit, runner),
		Audit:          audit,
		Intake:         NewWebIntakeService(repos.WorkspaceRepo, repos.AuditLogRepo),
		ExternalIntake: browserRunner,
		Articles:       NewBrowserArticleQueryService(repos.WorkspaceRepo, repos.RewritePipelineRunRepo, repos.WorkflowRunRepo),
		Workflows:      NewWorkflowTemplateService(repos.WorkflowDefinitionRepo),
		Templates:      NewTemplateDefinitionService(repos.TemplateDefinitionRepo),
	}, nil
}

func isBrowserIntakeWorkspace(item domain.ArticleWorkspaceRecord) bool {
	return item.Source.SourceType == "paste" || item.Source.SourceType == "upload"
}

func isExternalIntakeWorkspace(item domain.ArticleWorkspaceRecord) bool {
	if item.Metadata == nil {
		return false
	}
	origin, _ := item.Metadata["intake_origin"].(string)
	return strings.TrimSpace(origin) == "external_api"
}

func browserWorkspaceToIntakeArticle(item domain.ArticleWorkspaceRecord) domain.IntakeArticle {
	metadata := map[string]any{}
	for key, value := range item.Metadata {
		metadata[key] = value
	}
	body, _ := metadata["source_body"].(string)
	targetType, _ := metadata["target_type"].(string)
	sourceProfile, _ := metadata["source_profile"].(string)
	rewriteProfileVersion, _ := metadata["rewrite_profile_version"].(string)
	return domain.IntakeArticle{
		SourceType:            strings.TrimSpace(item.Source.SourceType),
		Title:                 strings.TrimSpace(item.Title),
		Body:                  body,
		Summary:               strings.TrimSpace(item.Summary),
		OriginalURL:           strings.TrimSpace(item.Source.URL),
		TargetType:            strings.TrimSpace(targetType),
		SourceProfile:         strings.TrimSpace(sourceProfile),
		RewriteProfileVersion: strings.TrimSpace(rewriteProfileVersion),
		Metadata:              metadata,
	}
}

func browserWorkspaceRenderPlatform(item domain.ArticleWorkspaceRecord) string {
	if value, ok := item.Metadata["render_platform"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return defaultWebIntakeRenderPlatform
}

func browserWorkspaceResumeRewriteRunID(item domain.ArticleWorkspaceRecord) string {
	if value, ok := item.Metadata["resume_rewrite_run_id"].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}
