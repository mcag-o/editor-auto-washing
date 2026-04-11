package httpclient

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"
)

type RetryableErrorKind string

const (
	ErrKindNetworkTimeout RetryableErrorKind = "network_timeout"
	ErrKindHTTPStatus     RetryableErrorKind = "http_status"
	ErrKindConfigInvalid  RetryableErrorKind = "config_invalid"
)

type RetryDecision struct {
	Retryable bool
	Kind      RetryableErrorKind
	Reason    string
}

type RetryClassifier interface {
	Classify(resp *Response, err error, phase string) RetryDecision
}

type JitterMode string

const (
	JitterNone    JitterMode = "none"
	JitterFull    JitterMode = "full"
	JitterBounded JitterMode = "bounded"
)

type JitterConfig struct {
	Mode  JitterMode
	Ratio float64
	Rand  *rand.Rand

	mu sync.Mutex
}

func (j *JitterConfig) randFloat64() float64 {
	if j == nil || j.Rand == nil {
		return rand.Float64()
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.Rand.Float64()
}

func (j JitterConfig) Apply(delay time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}

	switch j.Mode {
	case JitterFull:
		return time.Duration(j.randFloat64() * float64(delay))
	case JitterBounded:
		ratio := j.Ratio
		if ratio < 0 {
			ratio = 0
		}
		minV := float64(delay) * (1 - ratio)
		maxV := float64(delay) * (1 + ratio)
		if maxV < minV {
			maxV = minV
		}
		return time.Duration(minV + j.randFloat64()*(maxV-minV))
	default:
		return delay
	}
}

type ExponentialBackoff struct {
	BaseWait   time.Duration
	Multiplier float64
	MaxWait    time.Duration
	Jitter     JitterConfig
}

func (b ExponentialBackoff) NextDelay(attempt int) time.Duration {
	if b.BaseWait <= 0 {
		return 0
	}

	multiplier := b.Multiplier
	if multiplier <= 0 {
		multiplier = 2
	}

	power := math.Pow(multiplier, float64(maxInt(0, attempt-1)))
	base := float64(b.BaseWait) * power
	if base > float64(math.MaxInt64) {
		base = float64(math.MaxInt64)
	}

	capped := time.Duration(base)
	if b.MaxWait > 0 {
		capped = minDuration(capped, b.MaxWait)
	}

	return b.Jitter.Apply(capped)
}

type DefaultRetryClassifierOptions struct {
	RetryableStatusCodes map[int]struct{}
	RetryableStatusFloor int
}

type defaultRetryClassifier struct {
	statusCodes map[int]struct{}
	statusFloor int
}

func DefaultRetryClassifierConfig() DefaultRetryClassifierOptions {
	return DefaultRetryClassifierOptions{
		RetryableStatusCodes: map[int]struct{}{
			http.StatusRequestTimeout:  {},
			http.StatusTooManyRequests: {},
		},
		RetryableStatusFloor: http.StatusInternalServerError,
	}
}

func DefaultRetryClassifier(cfg DefaultRetryClassifierOptions) RetryClassifier {
	statusCodes := make(map[int]struct{}, len(cfg.RetryableStatusCodes))
	for code := range cfg.RetryableStatusCodes {
		statusCodes[code] = struct{}{}
	}
	return defaultRetryClassifier{
		statusCodes: statusCodes,
		statusFloor: cfg.RetryableStatusFloor,
	}
}

func (c defaultRetryClassifier) Classify(resp *Response, err error, phase string) RetryDecision {
	if isTimeoutError(err) {
		return RetryDecision{
			Retryable: true,
			Kind:      ErrKindNetworkTimeout,
			Reason:    "request timed out",
		}
	}

	if resp == nil {
		return RetryDecision{}
	}

	if _, ok := c.statusCodes[resp.StatusCode]; ok || (c.statusFloor > 0 && resp.StatusCode >= c.statusFloor) {
		reason := "retryable http status"
		if phase != "" {
			reason = phase + ": " + reason
		}
		return RetryDecision{
			Retryable: true,
			Kind:      ErrKindHTTPStatus,
			Reason:    reason,
		}
	}

	return RetryDecision{
		Retryable: false,
		Kind:      ErrKindHTTPStatus,
		Reason:    "non-retryable http status",
	}
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
