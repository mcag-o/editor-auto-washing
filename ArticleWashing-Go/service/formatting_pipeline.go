package service

import (
	"content-hub/domain"
	"content-hub/infra/formatter"
	workspaceinfra "content-hub/infra/workspace"
	"content-hub/pkg/repo"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var removeRenderedArtifact = os.Remove

type DraftFormatter interface {
	Render(draft *domain.ArticleDraft, templateName string) (string, error)
	ValidateDraft(draft *domain.ArticleDraft, templateName string) domain.DraftValidationResult
	ValidateRenderedOutput(html string) []string
}

type FormattingPipelineService struct {
	drafts               repo.DraftRepo
	assets               repo.AssetRepo
	workspaces           repo.WorkspaceRepo
	formatter            DraftFormatter
	renderedDir          string
	persistRenderedAsset func(*domain.RenderedAssetRecord, string) error
}

func NewFormattingPipelineService(drafts repo.DraftRepo, assets repo.AssetRepo, workspaces repo.WorkspaceRepo, formatter DraftFormatter) *FormattingPipelineService {
	return &FormattingPipelineService{drafts: drafts, assets: assets, workspaces: workspaces, formatter: formatter, persistRenderedAsset: persistRenderedAsset}
}

func (s *FormattingPipelineService) WithRenderedDir(dir string) *FormattingPipelineService {
	s.renderedDir = strings.TrimSpace(dir)
	return s
}

func (s *FormattingPipelineService) Render(ctx context.Context, draftID, platform, templateName string) (*domain.RenderedAssetRecord, error) {
	if s.drafts == nil || s.assets == nil || s.formatter == nil {
		return nil, domain.NewInternalErr("formatting pipeline is not configured", nil)
	}
	draft, err := s.drafts.GetByID(ctx, draftID)
	if err != nil {
		return nil, err
	}
	templateName = resolvedTemplateName(draft, templateName)
	validation := s.formatter.ValidateDraft(draft, templateName)
	if len(validation.Errors) > 0 {
		return nil, domain.NewValidationErr(strings.Join(validation.Errors, "; "), nil)
	}
	rendered, err := s.formatter.Render(draft, templateName)
	if err != nil {
		return nil, err
	}
	outputErrors := s.formatter.ValidateRenderedOutput(rendered)
	if len(outputErrors) > 0 {
		return nil, domain.NewValidationErr(strings.Join(outputErrors, "; "), nil)
	}
	asset := domain.NewRenderedAssetRecord(draft.ID, strings.TrimSpace(platform), "html", templateName, rendered, "")
	asset.Metadata["template"] = templateName
	asset.Metadata["warnings"] = append([]string{}, validation.Warnings...)
	if err := s.persistRenderedAsset(asset, s.renderedDir); err != nil {
		return nil, err
	}
	if err := s.assets.Create(ctx, asset); err != nil {
		cleanupErr := cleanupRenderedArtifact(asset)
		if cleanupErr != nil {
			return nil, fmt.Errorf("%w; rendered file cleanup failed: %v", err, cleanupErr)
		}
		return nil, err
	}
	if s.workspaces != nil {
		if err := s.workspaces.TransitionStatus(ctx, draft.ID, domain.ArticleWorkspaceStatusRendered, "rendered draft asset"); err != nil {
			cleanupErr := cleanupRenderedArtifact(asset)
			rollbackErr := rollbackAssetRecord(ctx, s.assets, asset)
			if cleanupErr != nil && rollbackErr != nil {
				return nil, fmt.Errorf("%w; rendered file cleanup failed: %v; asset rollback failed: %v", err, cleanupErr, rollbackErr)
			}
			if cleanupErr != nil {
				return nil, fmt.Errorf("%w; rendered file cleanup failed: %v", err, cleanupErr)
			}
			if rollbackErr != nil {
				return nil, fmt.Errorf("%w; asset rollback failed: %v", err, rollbackErr)
			}
			return nil, err
		}
	}
	return asset, nil
}

func (s *FormattingPipelineService) Validate(ctx context.Context, draftID, platform, templateName string) (domain.DraftValidationResult, error) {
	if s.drafts == nil || s.formatter == nil {
		return domain.DraftValidationResult{}, domain.NewInternalErr("formatting pipeline is not configured", nil)
	}
	draft, err := s.drafts.GetByID(ctx, draftID)
	if err != nil {
		return domain.DraftValidationResult{}, err
	}
	_ = platform
	templateName = resolvedTemplateName(draft, templateName)
	result := s.formatter.ValidateDraft(draft, templateName)
	if len(result.Errors) > 0 {
		return result, nil
	}
	rendered, err := s.formatter.Render(draft, templateName)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result, nil
	}
	for _, issue := range s.formatter.ValidateRenderedOutput(rendered) {
		result.Errors = domain.AppendUniqueIssue(result.Errors, issue)
	}
	return result, nil
}

func (s *FormattingPipelineService) GetAsset(ctx context.Context, assetID string) (*domain.RenderedAssetRecord, error) {
	if s.assets == nil {
		return nil, domain.NewInternalErr("formatting pipeline is not configured", nil)
	}
	return s.assets.GetByID(ctx, assetID)
}

func NewRuntimeFormattingPipelineService(root string) (*FormattingPipelineService, func() error, error) {
	repos, cleanup, err := BuildRuntimeRepos(root)
	if err != nil {
		return nil, nil, err
	}
	workspaceSvc := NewWorkspaceConfigService(workspaceinfra.NewLoader(), workspaceinfra.NewValidator())
	resolved, err := workspaceSvc.Resolve(root)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	formatter := formatter.NewWechatHtmlFormatter(resolved.Paths.TemplateDirs)
	return NewFormattingPipelineService(repos.DraftRepo, repos.AssetRepo, repos.WorkspaceRepo, formatter).WithRenderedDir(resolved.Paths.RenderedDir), cleanup, nil
}

func persistRenderedAsset(asset *domain.RenderedAssetRecord, renderedDir string) error {
	if asset == nil {
		return domain.NewValidationErr("asset is required", nil)
	}
	if strings.TrimSpace(renderedDir) == "" {
		return nil
	}
	if err := os.MkdirAll(renderedDir, 0o755); err != nil {
		return fmt.Errorf("create rendered dir: %w", err)
	}
	path := filepath.Join(renderedDir, fmt.Sprintf("%s-%s.%s", asset.ArticleID, asset.Platform, asset.OutputFormat))
	if err := os.WriteFile(path, []byte(asset.Content), 0o644); err != nil {
		return fmt.Errorf("write rendered asset: %w", err)
	}
	asset.ArtifactPath = path
	asset.Metadata["artifact_path"] = path
	return nil
}

func cleanupRenderedArtifact(asset *domain.RenderedAssetRecord) error {
	if asset == nil {
		return nil
	}
	var cleanupErr error
	if strings.TrimSpace(asset.ArtifactPath) != "" {
		cleanupErr = removeRenderedArtifact(asset.ArtifactPath)
	}
	asset.ArtifactPath = ""
	delete(asset.Metadata, "artifact_path")
	return cleanupErr
}

func rollbackAssetRecord(ctx context.Context, assetRepo repo.AssetRepo, asset *domain.RenderedAssetRecord) error {
	if assetRepo == nil || asset == nil || strings.TrimSpace(asset.AssetID) == "" {
		return nil
	}
	return assetRepo.Delete(ctx, asset.AssetID)
}

func resolvedTemplateName(draft *domain.ArticleDraft, templateName string) string {
	if strings.TrimSpace(templateName) != "" {
		return strings.TrimSpace(templateName)
	}
	if draft == nil {
		return ""
	}
	return strings.TrimSpace(draft.Template)
}
