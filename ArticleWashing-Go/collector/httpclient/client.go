package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
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
}

type Client struct {
	baseURL        string
	defaultHeaders map[string]string
	authInjector   AuthInjector
	retryPolicy    RetryPolicy
	httpClient     *http.Client
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

		if attempt < c.retryPolicy.MaxAttempts && c.retryPolicy.Wait > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.retryPolicy.Wait):
			}
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
