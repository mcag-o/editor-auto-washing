package runtime

import (
	"fmt"
	"strings"
	"time"

	"content-hub/infra/config"
)

type SecretResolver interface {
	Resolve(ref string) (string, error)
}

type PolicyResolver struct {
	secrets SecretResolver
}

func NewPolicyResolver(secrets SecretResolver) *PolicyResolver {
	return &PolicyResolver{secrets: secrets}
}

func (r *PolicyResolver) ResolveSource(cfg config.Config, sourceID string) (ResolvedSourceRuntimeConfig, error) {
	source, ok := cfg.Collector.SourceOrDefault(sourceID)
	if !ok {
		return ResolvedSourceRuntimeConfig{}, fmt.Errorf("collector source %s not found", sourceID)
	}

	httpProfile, err := resolveHTTPProfile(cfg.Collector, source)
	if err != nil {
		return ResolvedSourceRuntimeConfig{}, err
	}
	retryProfile, err := resolveRetryProfile(cfg.Collector, source)
	if err != nil {
		return ResolvedSourceRuntimeConfig{}, err
	}
	authProfile, err := resolveAuthProfile(cfg.Collector, source)
	if err != nil {
		return ResolvedSourceRuntimeConfig{}, err
	}

	return r.buildResolvedSourceConfig(sourceID, source, httpProfile, retryProfile, authProfile)
}

func resolveHTTPProfile(cfg config.CollectorConfig, source config.CollectorSourceDef) (config.HTTPClientProfile, error) {
	profileID := strings.TrimSpace(source.HTTPClient)
	profile, ok := cfg.HTTPClients[profileID]
	if !ok {
		return config.HTTPClientProfile{}, fmt.Errorf("collector source %s references unknown http client profile %q", source.DisplayName, profileID)
	}
	return profile, nil
}

func resolveRetryProfile(cfg config.CollectorConfig, source config.CollectorSourceDef) (config.RetryPolicyProfile, error) {
	profileID := strings.TrimSpace(source.RetryPolicy)
	profile, ok := cfg.RetryPolicies[profileID]
	if !ok {
		return config.RetryPolicyProfile{}, fmt.Errorf("collector source %s references unknown retry policy %q", source.DisplayName, profileID)
	}
	return profile, nil
}

func resolveAuthProfile(cfg config.CollectorConfig, source config.CollectorSourceDef) (config.AuthProfileConfig, error) {
	profileID := strings.TrimSpace(source.AuthProfile)
	profile, ok := cfg.AuthProfiles[profileID]
	if !ok {
		return config.AuthProfileConfig{}, fmt.Errorf("collector source %s references unknown auth profile %q", source.DisplayName, profileID)
	}
	return profile, nil
}

func (r *PolicyResolver) buildResolvedSourceConfig(sourceID string, source config.CollectorSourceDef, httpProfile config.HTTPClientProfile, retryProfile config.RetryPolicyProfile, authProfile config.AuthProfileConfig) (ResolvedSourceRuntimeConfig, error) {
	headers := cloneHeaders(httpProfile.Headers)
	for key, value := range source.Headers {
		headers[key] = value
	}
	if userAgent := strings.TrimSpace(httpProfile.UserAgent); userAgent != "" {
		headers["User-Agent"] = userAgent
	}

	auth, err := r.resolveAuthConfig(source, authProfile)
	if err != nil {
		return ResolvedSourceRuntimeConfig{}, err
	}

	return ResolvedSourceRuntimeConfig{
		SourceID:     sourceID,
		DisplayName:  source.DisplayName,
		BaseURL:      strings.TrimSpace(source.SourceURL),
		Timeout:      time.Duration(source.TimeoutMS) * time.Millisecond,
		Headers:      headers,
		HotlistLimit: source.HotlistLimit,
		RetryPolicy: RetryRuntimeConfig{
			MaxAttempts: retryProfile.MaxAttempts,
			Wait:        time.Duration(retryProfile.BaseWaitMS) * time.Millisecond,
			MaxWait:     time.Duration(retryProfile.MaxWaitMS) * time.Millisecond,
		},
		Auth: auth,
		Options: map[string]any{
			"detail_fetch_enabled": source.DetailFetchEnabled,
			"goal":                 source.Goal,
			"placeholder_required": source.PlaceholderRequired,
			"source_type":          source.SourceType,
			"status":               source.Status,
			"supports_article":     source.SupportsArticle,
		},
	}, nil
}

func (r *PolicyResolver) resolveAuthConfig(source config.CollectorSourceDef, authProfile config.AuthProfileConfig) (ResolvedAuthConfig, error) {
	resolved := ResolvedAuthConfig{
		Mode:              strings.TrimSpace(authProfile.Mode),
		Headers:           map[string]string{},
		HeaderName:        strings.TrimSpace(authProfile.HeaderName),
		HeaderValuePrefix: authProfile.HeaderValuePrefix,
	}

	if resolved.Mode == "" {
		resolved.Mode = strings.TrimSpace(source.AuthMode)
	}

	if ref := strings.TrimSpace(source.CookieSecretRef); ref != "" {
		secret, err := r.resolveSecret(ref)
		if err != nil {
			return ResolvedAuthConfig{}, fmt.Errorf("resolve cookie secret %s: %w", ref, err)
		}
		resolved.Cookie = secret
	}

	if ref := strings.TrimSpace(source.HeaderSecretRef); ref != "" {
		secret, err := r.resolveSecret(ref)
		if err != nil {
			return ResolvedAuthConfig{}, fmt.Errorf("resolve header secret %s: %w", ref, err)
		}
		if secret != "" {
			headerName := resolved.HeaderName
			if headerName == "" {
				headerName = "Authorization"
			}
			resolved.Headers = map[string]string{headerName: resolved.HeaderValuePrefix + secret}
		}
	}

	return resolved, nil
}

func (r *PolicyResolver) resolveSecret(ref string) (string, error) {
	if r == nil || r.secrets == nil {
		return "", nil
	}
	return r.secrets.Resolve(ref)
}

func cloneHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(headers))
	for key, value := range headers {
		cloned[key] = value
	}
	return cloned
}
