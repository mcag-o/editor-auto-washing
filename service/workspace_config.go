package service

import (
	"content-hub/domain"
	"content-hub/infra/config"
	workspaceinfra "content-hub/infra/workspace"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type WorkspaceConfigService struct {
	loader    *workspaceinfra.Loader
	validator *workspaceinfra.Validator
}

func NewWorkspaceConfigService(loader *workspaceinfra.Loader, validator *workspaceinfra.Validator) *WorkspaceConfigService {
	return &WorkspaceConfigService{loader: loader, validator: validator}
}

func (s *WorkspaceConfigService) Init(root string) (domain.ResolvedWorkspaceSettings, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return domain.ResolvedWorkspaceSettings{}, fmt.Errorf("create workspace root: %w", err)
	}
	configPath := filepath.Join(root, workspaceinfra.WorkspaceConfigFileName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		settings := domain.DefaultWorkspaceSettings()
		if err := s.loader.Save(root, settings); err != nil {
			return domain.ResolvedWorkspaceSettings{}, err
		}
	} else if err != nil {
		return domain.ResolvedWorkspaceSettings{}, fmt.Errorf("stat workspace config: %w", err)
	}
	secretsPath := filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName)
	if _, err := os.Stat(secretsPath); os.IsNotExist(err) {
		if err := os.WriteFile(secretsPath, []byte(""), 0o600); err != nil {
			return domain.ResolvedWorkspaceSettings{}, fmt.Errorf("write workspace secrets: %w", err)
		}
	} else if err != nil {
		return domain.ResolvedWorkspaceSettings{}, fmt.Errorf("stat workspace secrets: %w", err)
	}
	resolved, err := s.loader.Resolve(root)
	if err != nil {
		return domain.ResolvedWorkspaceSettings{}, err
	}
	if err := s.ensureDirectories(resolved.Paths); err != nil {
		return domain.ResolvedWorkspaceSettings{}, err
	}
	return resolved, nil
}

func (s *WorkspaceConfigService) ShowConfig(root string) (string, error) {
	settings, err := s.loader.Load(root)
	if err != nil {
		return "", err
	}
	data, err := yaml.Marshal(settings)
	if err != nil {
		return "", fmt.Errorf("marshal workspace config: %w", err)
	}
	return string(data), nil
}

func (s *WorkspaceConfigService) Resolve(root string) (domain.ResolvedWorkspaceSettings, error) {
	return s.loader.Resolve(root)
}

func (s *WorkspaceConfigService) Doctor(root string) (workspaceinfra.DiagnosticReport, error) {
	resolved, err := s.loader.Resolve(root)
	if err != nil {
		return workspaceinfra.DiagnosticReport{}, err
	}
	return s.validator.Validate(resolved), nil
}

func (s *WorkspaceConfigService) RuntimeConfig(root string) (config.Config, error) {
	resolved, err := s.loader.Resolve(root)
	if err != nil {
		return config.Config{}, err
	}
	return BuildRuntimeConfig(resolved)
}

func BuildRuntimeConfig(resolved domain.ResolvedWorkspaceSettings) (config.Config, error) {
	cfg := config.DefaultConfig()
	providerProfile, ok := resolved.Workspace.ProviderProfiles[resolved.Workspace.DefaultProviderProfile]
	if !ok {
		return config.Config{}, fmt.Errorf("missing provider profile: %s", resolved.Workspace.DefaultProviderProfile)
	}
	articleProfile, ok := resolved.Workspace.ArticleProfiles[resolved.Workspace.DefaultArticleProfile]
	if !ok {
		return config.Config{}, fmt.Errorf("missing article profile: %s", resolved.Workspace.DefaultArticleProfile)
	}
	publishProfile, ok := resolved.Workspace.PublishProfiles[resolved.Workspace.DefaultPublishProfile]
	if !ok {
		return config.Config{}, fmt.Errorf("missing publish profile: %s", resolved.Workspace.DefaultPublishProfile)
	}

	cfg.Storage.BasePath = resolved.Paths.DataDir
	cfg.Database.Path = filepath.Join(resolved.Paths.DataDir, "content-hub.db")
	if len(resolved.Paths.TemplateDirs) > 0 {
		cfg.Template.PromptDir = resolved.Paths.TemplateDirs[0]
	}
	cfg.Template.DefaultPrompt = articleProfile.Template
	if providerProfile.Provider != "" {
		cfg.LLM.Provider = providerProfile.Provider
	}
	if providerProfile.Model != "" {
		cfg.LLM.Model = providerProfile.Model
	}
	if providerProfile.BaseURL != "" {
		cfg.LLM.BaseURL = providerProfile.BaseURL
	}
	if providerProfile.Temperature != 0 {
		cfg.LLM.Temperature = providerProfile.Temperature
	}
	if providerProfile.MaxTokens != 0 {
		cfg.LLM.MaxTokens = providerProfile.MaxTokens
	}
	cfg.LLM.APIKey = resolved.Secrets[providerProfile.SecretRef]
	cfg.LLM.DefaultProfile = ""
	cfg.LLM.Profiles = nil
	cfg.Platforms.WeChat.Enabled = publishProfile.Platform == "wechat"
	cfg.Platforms.WeChat.Token = resolved.Secrets[publishProfile.SecretRef]

	return cfg, cfg.Validate()
}

func (s *WorkspaceConfigService) ensureDirectories(paths domain.ResolvedWorkspacePaths) error {
	for _, dir := range []string{
		paths.DataDir,
		paths.IncomingDir,
		paths.ArticlesDir,
		paths.DraftsDir,
		paths.RenderedDir,
		paths.ReviewsDir,
		paths.PublishRecordsDir,
		paths.LogsDir,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create workspace directory %s: %w", dir, err)
		}
	}
	return nil
}
