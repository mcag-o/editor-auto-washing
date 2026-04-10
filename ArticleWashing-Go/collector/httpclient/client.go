package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultTimeout = 10 * time.Second

type AuthInjector func(req *http.Request) error

type RetryPolicy struct {
	MaxAttempts int
	Wait        time.Duration
}

type Options struct {
	BaseURL        string
	Timeout        time.Duration
	RetryPolicy    RetryPolicy
	DefaultHeaders map[string]string
	AuthInjector   AuthInjector
	HTTPClient     *http.Client
	Sleep          func(<-chan time.Time, time.Duration)
}

type Client struct {
	baseURL        string
	defaultHeaders map[string]string
	authInjector   AuthInjector
	retryPolicy    RetryPolicy
	httpClient     *http.Client
	sleepFn        func(<-chan time.Time, time.Duration)
}

type Request struct {
	Method  string
	Path    string
	Query   url.Values
	Headers map[string]string
	Body    []byte
}

type Response struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

func New(opts Options) (*Client, error) {
	if strings.TrimSpace(opts.BaseURL) == "" {
		return nil, fmt.Errorf("http client base url is required")
	}
	parsedBaseURL, err := url.Parse(strings.TrimSpace(opts.BaseURL))
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}
	if parsedBaseURL.Scheme == "" || parsedBaseURL.Host == "" {
		return nil, fmt.Errorf("parse base url: missing scheme or host")
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	} else if httpClient.Timeout == 0 {
		httpClient.Timeout = timeout
	}

	policy := opts.RetryPolicy
	if policy.MaxAttempts <= 0 {
		policy.MaxAttempts = 1
	}

	return &Client{
		baseURL:        strings.TrimRight(parsedBaseURL.String(), "/"),
		defaultHeaders: cloneHeaders(opts.DefaultHeaders),
		authInjector:   opts.AuthInjector,
		retryPolicy:    policy,
		httpClient:     httpClient,
		sleepFn:        resolveSleepFn(opts.Sleep),
	}, nil
}

func (c *Client) Do(ctx context.Context, req Request) (*Response, error) {
	method := strings.TrimSpace(req.Method)
	if method == "" {
		method = http.MethodGet
	}

	requestURL, err := c.resolveURL(req.Path, req.Query)
	if err != nil {
		return nil, err
	}

	var lastErr error
	var lastResp *Response
	for attempt := 1; attempt <= c.retryPolicy.MaxAttempts; attempt++ {
		httpReq, buildErr := http.NewRequestWithContext(ctx, method, requestURL, bytes.NewReader(req.Body))
		if buildErr != nil {
			return nil, fmt.Errorf("build request: %w", buildErr)
		}
		applyHeaders(httpReq.Header, c.defaultHeaders)
		applyHeaders(httpReq.Header, req.Headers)
		if c.authInjector != nil {
			if err := c.authInjector(httpReq); err != nil {
				return nil, fmt.Errorf("inject auth: %w", err)
			}
		}

		resp, doErr := c.httpClient.Do(httpReq)
		if doErr != nil {
			lastErr = fmt.Errorf("request %s %s failed after %d attempts: %w", method, requestURL, attempt, doErr)
		} else {
			body, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr != nil {
				return nil, fmt.Errorf("read response body: %w", readErr)
			}
			result := &Response{StatusCode: resp.StatusCode, Headers: resp.Header.Clone(), Body: body}
			lastResp = result
			if resp.StatusCode < http.StatusBadRequest {
				return result, nil
			}
			lastErr = fmt.Errorf("request %s %s failed after %d attempts with status %d", method, requestURL, attempt, resp.StatusCode)
			if !shouldRetry(resp.StatusCode) || attempt == c.retryPolicy.MaxAttempts {
				return result, lastErr
			}
		}

		wait := c.backoffDelay(attempt)
		if attempt < c.retryPolicy.MaxAttempts && wait > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			c.sleepFn(time.After(wait), wait)
		}
	}

	return lastResp, lastErr
}

func HeaderAuthInjector(headers map[string]string) AuthInjector {
	cloned := cloneHeaders(headers)
	return func(req *http.Request) error {
		applyHeaders(req.Header, cloned)
		return nil
	}
}

func (c *Client) resolveURL(path string, query url.Values) (string, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("parse base url: %w", err)
	}
	relative, err := url.Parse(strings.TrimSpace(path))
	if err != nil {
		return "", fmt.Errorf("parse request path: %w", err)
	}
	resolved := base.ResolveReference(relative)
	if len(query) > 0 {
		q := resolved.Query()
		for key, values := range query {
			for _, value := range values {
				q.Add(key, value)
			}
		}
		resolved.RawQuery = q.Encode()
	}
	return resolved.String(), nil
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

func applyHeaders(target http.Header, headers map[string]string) {
	for key, value := range headers {
		target.Set(key, value)
	}
}

func shouldRetry(statusCode int) bool {
	if statusCode == http.StatusRequestTimeout || statusCode == http.StatusTooManyRequests {
		return true
	}
	return statusCode >= http.StatusInternalServerError
}

func (c *Client) backoffDelay(attempt int) time.Duration {
	if c.retryPolicy.Wait <= 0 {
		return 0
	}
	// 这里改为指数退避，避免网络抖动期间所有请求以固定节奏同时重试，
	// 从而放大上游故障或造成自身雪崩。
	// 当前策略：base * 2^(attempt-1)，后续如需更强稳态表现，可再引入 jitter 与上限配置。
	if attempt <= 1 {
		return c.retryPolicy.Wait
	}
	scale := math.Pow(2, float64(attempt-1))
	wait := float64(c.retryPolicy.Wait) * scale
	if wait > float64(math.MaxInt64) {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(wait)
}

func resolveSleepFn(override func(<-chan time.Time, time.Duration)) func(<-chan time.Time, time.Duration) {
	if override != nil {
		return override
	}
	return func(ch <-chan time.Time, _ time.Duration) {
		<-ch
	}
}
