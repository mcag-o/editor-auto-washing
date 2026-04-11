package runtime

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"content-hub/collector/httpclient"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPFactory_BuildsClientWithConfiguredTimeoutHeadersAndRetry(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		assert.Equal(t, "collector-test", r.Header.Get("User-Agent"))
		if attempt == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	factory := NewHTTPFactory()
	resolved := ResolvedSourceRuntimeConfig{
		SourceID: "zhihu",
		BaseURL:  server.URL,
		Timeout:  12 * time.Second,
		Headers:  map[string]string{"User-Agent": "collector-test"},
		RetryPolicy: RetryRuntimeConfig{
			MaxAttempts: 4,
			Wait:        750 * time.Millisecond,
		},
	}

	client, err := factory.Build(resolved, nil)
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, 12*time.Second, client.Timeout())

	resp, err := client.Do(t.Context(), httpclient.Request{Method: http.MethodGet, Path: "/hotlist", Phase: "hotlist"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.EqualValues(t, 2, attempts.Load())
}
