package httpclient_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"content-hub/collector/httpclient"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_RetriesRetryableStatuses(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		if attempt == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"slow down"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(server.Close)

	client, err := httpclient.New(httpclient.Options{
		BaseURL: server.URL,
		RetryPolicy: httpclient.RetryPolicy{
			MaxAttempts: 2,
		},
	})
	require.NoError(t, err)

	resp, err := client.Do(t.Context(), httpclient.Request{Method: http.MethodGet, Path: "/status"})

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.EqualValues(t, 2, attempts.Load())
}

func TestClient_InjectsDefaultAndAuthHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "collector-test", r.Header.Get("X-Client"))
		assert.Equal(t, "Bearer secret-token", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(server.Close)

	client, err := httpclient.New(httpclient.Options{
		BaseURL: server.URL,
		DefaultHeaders: map[string]string{
			"X-Client": "collector-test",
		},
		AuthInjector: httpclient.HeaderAuthInjector(map[string]string{
			"Authorization": "Bearer secret-token",
		}),
	})
	require.NoError(t, err)

	resp, err := client.Do(t.Context(), httpclient.Request{Method: http.MethodGet, Path: "/headers"})

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestClient_DoesNotRetryNonRetryable4xx(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad request"}`))
	}))
	t.Cleanup(server.Close)

	client, err := httpclient.New(httpclient.Options{
		BaseURL: server.URL,
		RetryPolicy: httpclient.RetryPolicy{
			MaxAttempts: 3,
		},
	})
	require.NoError(t, err)

	resp, err := client.Do(t.Context(), httpclient.Request{Method: http.MethodGet, Path: "/bad"})

	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.EqualValues(t, 1, attempts.Load())
	assert.ErrorContains(t, err, "after 1 attempt")
	assert.ErrorContains(t, err, "status 400")
}

func TestClient_ReturnsRetryExhaustionDetailsFor5xx(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"upstream failed"}`))
	}))
	t.Cleanup(server.Close)

	client, err := httpclient.New(httpclient.Options{
		BaseURL: server.URL,
		RetryPolicy: httpclient.RetryPolicy{
			MaxAttempts: 2,
		},
	})
	require.NoError(t, err)

	resp, err := client.Do(t.Context(), httpclient.Request{Method: http.MethodGet, Path: "/unstable"})

	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	assert.EqualValues(t, 2, attempts.Load())
	assert.ErrorContains(t, err, "after 2 attempts")
	assert.ErrorContains(t, err, "GET")
	assert.ErrorContains(t, err, "/unstable")
	assert.ErrorContains(t, err, "status 502")
}

func TestClient_NewRejectsInvalidBaseURL(t *testing.T) {
	client, err := httpclient.New(httpclient.Options{BaseURL: "://bad-url"})

	require.Error(t, err)
	assert.Nil(t, client)
	assert.ErrorContains(t, err, "base url")
}

func TestClient_DoReturnsAuthInjectionFailure(t *testing.T) {
	client, err := httpclient.New(httpclient.Options{
		BaseURL: "https://example.com",
		AuthInjector: func(req *http.Request) error {
			return errors.New("missing token")
		},
	})
	require.NoError(t, err)

	resp, err := client.Do(t.Context(), httpclient.Request{Method: http.MethodGet, Path: "/auth"})

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorContains(t, err, "inject auth")
	assert.ErrorContains(t, err, "missing token")
}
