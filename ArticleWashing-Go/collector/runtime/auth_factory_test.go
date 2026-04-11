package runtime

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type secretMapResolver map[string]string

func (r secretMapResolver) Resolve(ref string) (string, error) {
	value, ok := r[ref]
	if !ok {
		return "", ErrSecretNotFound(ref)
	}
	return value, nil
}

func TestAuthFactory_BuildsCookieInjectorFromSecretRef(t *testing.T) {
	factory := NewAuthFactory(secretMapResolver{"env.WEIBO_COOKIE": "SUB=abc"})
	inj, err := factory.Build(ResolvedAuthConfig{Mode: "cookie", CookieSecretRef: "env.WEIBO_COOKIE"})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "https://example.com", nil)
	require.NoError(t, inj(req))
	assert.Equal(t, "SUB=abc", req.Header.Get("Cookie"))
}

func TestAuthFactory_RejectsMissingSecretRef(t *testing.T) {
	factory := NewAuthFactory(secretMapResolver{})
	_, err := factory.Build(ResolvedAuthConfig{Mode: "bearer", HeaderSecretRef: "env.MISSING", HeaderName: "Authorization", HeaderValuePrefix: "Bearer "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "env.MISSING")
}
