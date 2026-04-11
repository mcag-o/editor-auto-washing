package httpclient_test

import (
	"context"
	"errors"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"
	"time"

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

func TestClient_UsesExponentialBackoffBetweenRetryAttempts(t *testing.T) {
	var attempts atomic.Int32
	var waits []time.Duration
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		if attempt < 3 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"temporary upstream failure"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(server.Close)

	client, err := httpclient.New(httpclient.Options{
		BaseURL: server.URL,
		RetryPolicy: httpclient.RetryPolicy{
			MaxAttempts: 3,
			Wait:        10 * time.Millisecond,
		},
		Sleep: func(_ <-chan time.Time, d time.Duration) {
			waits = append(waits, d)
		},
	})
	require.NoError(t, err)

	resp, err := client.Do(t.Context(), httpclient.Request{Method: http.MethodGet, Path: "/backoff"})

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.EqualValues(t, 3, attempts.Load())
	assert.True(t, slices.Equal(waits, []time.Duration{10 * time.Millisecond, 20 * time.Millisecond}), "unexpected waits: %v", waits)
	assert.Greater(t, waits[1], waits[0])
	assert.NotEqual(t, waits[0], waits[1])
}

func TestRetryPolicy_UsesMaxWaitAndBoundedJitter(t *testing.T) {
	policy := httpclient.ExponentialBackoff{
		BaseWait:   500 * time.Millisecond,
		Multiplier: 2,
		MaxWait:    2 * time.Second,
		Jitter: httpclient.JitterConfig{
			Mode:  httpclient.JitterBounded,
			Ratio: 0.2,
			Rand:  rand.New(rand.NewSource(7)),
		},
	}

	d1 := policy.NextDelay(1)
	d4 := policy.NextDelay(4)
	overflowPolicy := httpclient.ExponentialBackoff{
		BaseWait:   500 * time.Millisecond,
		Multiplier: 2,
		MaxWait:    2 * time.Second,
		Jitter: httpclient.JitterConfig{
			Mode:  httpclient.JitterBounded,
			Ratio: 0.2,
			Rand:  rand.New(rand.NewSource(0)),
		},
	}
	d5 := overflowPolicy.NextDelay(5)

	assert.GreaterOrEqual(t, d1, 400*time.Millisecond)
	assert.LessOrEqual(t, d1, 600*time.Millisecond)
	assert.GreaterOrEqual(t, d4, 1600*time.Millisecond)
	assert.LessOrEqual(t, d4, 2*time.Second)
	assert.Equal(t, 2*time.Second, d5)
}

func TestRetryClassifier_ClassifiesHTTPAndNetworkErrors(t *testing.T) {
	classifier := httpclient.DefaultRetryClassifier(httpclient.DefaultRetryClassifierConfig())

	decision429 := classifier.Classify(&httpclient.Response{StatusCode: 429}, nil, "hotlist")
	decision400 := classifier.Classify(&httpclient.Response{StatusCode: 400}, nil, "hotlist")
	decisionTimeout := classifier.Classify(nil, context.DeadlineExceeded, "detail")

	assert.True(t, decision429.Retryable)
	assert.False(t, decision400.Retryable)
	assert.True(t, decisionTimeout.Retryable)
	assert.Equal(t, httpclient.ErrKindNetworkTimeout, decisionTimeout.Kind)
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
