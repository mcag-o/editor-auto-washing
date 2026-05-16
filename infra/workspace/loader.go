package workspace

import (
	"content-hub/domain"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	WorkspaceConfigFileName  = "workspace.yaml"
	WorkspaceSecretsFileName = "secrets.yaml"
)

type Loader struct{}

func NewLoader() *Loader {
	return &Loader{}
}

func (l *Loader) Load(root string) (domain.WorkspaceSettings, error) {
	configPath := filepath.Join(root, WorkspaceConfigFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.DefaultWorkspaceSettings(), nil
		}
		return domain.WorkspaceSettings{}, fmt.Errorf("read workspace config: %w", err)
	}

	settings := domain.DefaultWorkspaceSettings()
	if err := yaml.Unmarshal(data, &settings); err != nil {
		return domain.WorkspaceSettings{}, fmt.Errorf("parse workspace config: %w", err)
	}
	applyWorkspaceDefaults(&settings)
	return settings, nil
}

func (l *Loader) Resolve(root string) (domain.ResolvedWorkspaceSettings, error) {
	settings, err := l.Load(root)
	if err != nil {
		return domain.ResolvedWorkspaceSettings{}, err
	}

	secrets, err := l.LoadSecrets(root)
	if err != nil {
		return domain.ResolvedWorkspaceSettings{}, err
	}
	resolveEnvironmentSecretRefs(secrets, collectEnvironmentSecretRefs(settings))

	resolvedPaths := resolvePaths(root, settings)
	return domain.ResolvedWorkspaceSettings{
		Workspace: settings,
		Paths:     resolvedPaths,
		Secrets:   secrets,
	}, nil
}

func collectEnvironmentSecretRefs(settings domain.WorkspaceSettings) []string {
	refs := []string{}
	for _, profile := range settings.ProviderProfiles {
		if strings.HasPrefix(profile.SecretRef, "env.") {
			refs = append(refs, profile.SecretRef)
		}
	}
	for _, profile := range settings.PublishProfiles {
		if strings.HasPrefix(profile.SecretRef, "env.") {
			refs = append(refs, profile.SecretRef)
		}
	}
	return refs
}

func resolveEnvironmentSecretRefs(secrets map[string]string, refs []string) {
	for _, ref := range refs {
		name := strings.TrimPrefix(ref, "env.")
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			secrets[ref] = value
		}
	}
}

func (l *Loader) LoadSecrets(root string) (map[string]string, error) {
	secretsPath := filepath.Join(root, WorkspaceSecretsFileName)
	data, err := os.ReadFile(secretsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("read workspace secrets: %w", err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse workspace secrets: %w", err)
	}

	secrets := map[string]string{}
	flattenSecrets(raw, "", secrets)
	return secrets, nil
}

func (l *Loader) Save(root string, settings domain.WorkspaceSettings) error {
	configPath := filepath.Join(root, WorkspaceConfigFileName)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("create workspace root: %w", err)
	}
	data, err := yaml.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal workspace config: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return fmt.Errorf("write workspace config: %w", err)
	}
	return nil
}

func resolvePaths(root string, settings domain.WorkspaceSettings) domain.ResolvedWorkspacePaths {
	paths := settings.Paths
	incoming := resolvePath(root, paths.IncomingDir)
	automationIncoming := incoming
	if settings.Automation.IncomingDir != "" {
		automationIncoming = resolvePath(root, settings.Automation.IncomingDir)
	}
	automationProcessed := filepath.Join(automationIncoming, "processed")
	if settings.Automation.ProcessedDir != "" {
		automationProcessed = resolvePath(root, settings.Automation.ProcessedDir)
	}
	automationFailed := filepath.Join(automationIncoming, "failed")
	if settings.Automation.FailedDir != "" {
		automationFailed = resolvePath(root, settings.Automation.FailedDir)
	}

	return domain.ResolvedWorkspacePaths{
		Root:                   root,
		ConfigFile:             filepath.Join(root, WorkspaceConfigFileName),
		SecretsFile:            filepath.Join(root, WorkspaceSecretsFileName),
		DataDir:                resolvePath(root, paths.DataDir),
		IncomingDir:            incoming,
		ProcessedDir:           filepath.Join(incoming, "processed"),
		FailedDir:              filepath.Join(incoming, "failed"),
		ArticlesDir:            resolvePath(root, paths.ArticlesDir),
		DraftsDir:              resolvePath(root, paths.DraftsDir),
		TemplateDirs:           resolvePathList(root, paths.TemplateDirs),
		RenderedDir:            resolvePath(root, paths.RenderedDir),
		ReviewsDir:             resolvePath(root, paths.ReviewsDir),
		PublishRecordsDir:      resolvePath(root, paths.PublishRecordsDir),
		LogsDir:                resolvePath(root, paths.LogsDir),
		AutomationIncomingDir:  automationIncoming,
		AutomationProcessedDir: automationProcessed,
		AutomationFailedDir:    automationFailed,
	}
}

func resolvePath(root, value string) string {
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(root, value)
}

func resolvePathList(root string, values []string) []string {
	resolved := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		resolved = append(resolved, resolvePath(root, trimmed))
	}
	return resolved
}

func flattenSecrets(raw map[string]any, prefix string, out map[string]string) {
	for key, value := range raw {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}
		switch typed := value.(type) {
		case map[string]any:
			flattenSecrets(typed, fullKey, out)
		case map[any]any:
			normalized := map[string]any{}
			for nestedKey, nestedValue := range typed {
				normalized[fmt.Sprint(nestedKey)] = nestedValue
			}
			flattenSecrets(normalized, fullKey, out)
		default:
			trimmed := strings.TrimSpace(fmt.Sprint(value))
			if trimmed != "" {
				out[fullKey] = trimmed
			}
		}
	}
}

func applyWorkspaceDefaults(settings *domain.WorkspaceSettings) {
	defaults := domain.DefaultWorkspaceSettings()
	if settings.Name == "" {
		settings.Name = defaults.Name
	}
	if settings.Paths.DataDir == "" {
		settings.Paths.DataDir = defaults.Paths.DataDir
	}
	if settings.Paths.IncomingDir == "" {
		settings.Paths.IncomingDir = defaults.Paths.IncomingDir
	}
	if settings.Paths.ArticlesDir == "" {
		settings.Paths.ArticlesDir = defaults.Paths.ArticlesDir
	}
	if settings.Paths.DraftsDir == "" {
		settings.Paths.DraftsDir = defaults.Paths.DraftsDir
	}
	if settings.Paths.TemplateDirs == nil {
		settings.Paths.TemplateDirs = defaults.Paths.TemplateDirs
	}
	if settings.Paths.RenderedDir == "" {
		settings.Paths.RenderedDir = defaults.Paths.RenderedDir
	}
	if settings.Paths.ReviewsDir == "" {
		settings.Paths.ReviewsDir = defaults.Paths.ReviewsDir
	}
	if settings.Paths.PublishRecordsDir == "" {
		settings.Paths.PublishRecordsDir = defaults.Paths.PublishRecordsDir
	}
	if settings.Paths.LogsDir == "" {
		settings.Paths.LogsDir = defaults.Paths.LogsDir
	}
	if settings.DefaultProviderProfile == "" {
		settings.DefaultProviderProfile = defaults.DefaultProviderProfile
	}
	if settings.DefaultArticleProfile == "" {
		settings.DefaultArticleProfile = defaults.DefaultArticleProfile
	}
	if settings.DefaultPublishProfile == "" {
		settings.DefaultPublishProfile = defaults.DefaultPublishProfile
	}
	if len(settings.ProviderProfiles) == 0 {
		settings.ProviderProfiles = defaults.ProviderProfiles
	}
	if len(settings.ArticleProfiles) == 0 {
		settings.ArticleProfiles = defaults.ArticleProfiles
	}
	if len(settings.PublishProfiles) == 0 {
		settings.PublishProfiles = defaults.PublishProfiles
	}
	applyReviewPolicyDefaults(&settings.ReviewPolicy, defaults.ReviewPolicy)
	applyAutomationDefaults(&settings.Automation, defaults.Automation)
	applyProviderProfileDefaults(settings.ProviderProfiles, defaults.ProviderProfiles[defaults.DefaultProviderProfile])
	applyArticleProfileDefaults(settings.ArticleProfiles, defaults.ArticleProfiles[defaults.DefaultArticleProfile])
	applyPublishProfileDefaults(settings.PublishProfiles, defaults.PublishProfiles[defaults.DefaultPublishProfile])
}

func applyReviewPolicyDefaults(policy *domain.ReviewPolicy, defaults domain.ReviewPolicy) {
	if policy.DefaultMode == "" {
		policy.DefaultMode = defaults.DefaultMode
	}
	if policy.AutoPublishProfiles == nil {
		policy.AutoPublishProfiles = defaults.AutoPublishProfiles
	}
	if policy.BlockingErrors == nil {
		policy.BlockingErrors = defaults.BlockingErrors
	}
}

func applyAutomationDefaults(policy *domain.AutomationPolicy, defaults domain.AutomationPolicy) {
	if policy.IntervalSeconds == 0 {
		policy.IntervalSeconds = defaults.IntervalSeconds
	}
}

func applyProviderProfileDefaults(profiles map[string]domain.ProviderProfile, defaults domain.ProviderProfile) {
	for name, profile := range profiles {
		if profile.Temperature == 0 {
			profile.Temperature = defaults.Temperature
		}
		if profile.MaxTokens == 0 {
			profile.MaxTokens = defaults.MaxTokens
		}
		if profile.EnabledSet == nil {
			profile.Enabled = defaults.Enabled
			profile.EnabledSet = boolPtr(defaults.Enabled)
		} else {
			profile.Enabled = *profile.EnabledSet
		}
		profiles[name] = profile
	}
}

func applyArticleProfileDefaults(profiles map[string]domain.ArticleProfile, defaults domain.ArticleProfile) {
	for name, profile := range profiles {
		if profile.Length == "" {
			profile.Length = defaults.Length
		}
		if profile.ImagePolicy == "" {
			profile.ImagePolicy = defaults.ImagePolicy
		}
		profiles[name] = profile
	}
}

func applyPublishProfileDefaults(profiles map[string]domain.PublishProfile, defaults domain.PublishProfile) {
	for name, profile := range profiles {
		if profile.RetryCount == 0 {
			profile.RetryCount = defaults.RetryCount
		}
		if profile.FallbackSet == nil {
			profile.FallbackToReview = defaults.FallbackToReview
			profile.FallbackSet = boolPtr(defaults.FallbackToReview)
		} else {
			profile.FallbackToReview = *profile.FallbackSet
		}
		profiles[name] = profile
	}
}

func boolPtr(value bool) *bool {
	return &value
}
