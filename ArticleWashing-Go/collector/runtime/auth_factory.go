package runtime

import (
	"fmt"
	"os"
	"strings"

	"content-hub/collector/httpclient"
	"content-hub/domain"
)

type SecretResolver interface {
	Resolve(ref string) (string, error)
}

type AuthFactory struct {
	secrets SecretResolver
}

func NewAuthFactory(secrets SecretResolver) *AuthFactory {
	return &AuthFactory{secrets: secrets}
}

func (f *AuthFactory) Build(profile ResolvedAuthConfig) (httpclient.AuthInjector, error) {
	mode := strings.TrimSpace(profile.Mode)
	switch mode {
	case "", domain.CollectorAuthModeNone:
		return nil, nil
	case domain.CollectorAuthModeCookie:
		cookie, err := f.resolveRequired(profile.CookieSecretRef)
		if err != nil {
			return nil, err
		}
		return httpclient.HeaderAuthInjector(map[string]string{"Cookie": cookie}), nil
	case domain.CollectorAuthModeHeader, "bearer":
		token, err := f.resolveRequired(profile.HeaderSecretRef)
		if err != nil {
			return nil, err
		}
		headerName := strings.TrimSpace(profile.HeaderName)
		if headerName == "" {
			headerName = "Authorization"
		}
		return httpclient.HeaderAuthInjector(map[string]string{headerName: profile.HeaderValuePrefix + token}), nil
	default:
		return nil, fmt.Errorf("unsupported auth mode %q", profile.Mode)
	}
}

func (f *AuthFactory) resolveRequired(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("missing secret ref")
	}
	if f == nil || f.secrets == nil {
		return "", fmt.Errorf("resolve secret %s: no secret resolver configured", ref)
	}
	value, err := f.secrets.Resolve(ref)
	if err != nil {
		return "", fmt.Errorf("resolve secret %s: %w", ref, err)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ErrSecretNotFound(ref)
	}
	return value, nil
}

func ErrSecretNotFound(ref string) error {
	return fmt.Errorf("secret %s not found", strings.TrimSpace(ref))
}

type envSecretResolver struct{}

func NewEnvSecretResolver() SecretResolver {
	return envSecretResolver{}
}

func (envSecretResolver) Resolve(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", ErrSecretNotFound(ref)
	}
	if strings.HasPrefix(ref, "env.") {
		if value := strings.TrimSpace(os.Getenv(strings.TrimPrefix(ref, "env."))); value != "" {
			return value, nil
		}
		return "", ErrSecretNotFound(ref)
	}
	return "", ErrSecretNotFound(ref)
}
