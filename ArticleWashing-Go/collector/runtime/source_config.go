package runtime

import "time"

type RetryRuntimeConfig struct {
	MaxAttempts int
	Wait        time.Duration
	MaxWait     time.Duration
}

type ResolvedAuthConfig struct {
	Mode              string
	HeaderName        string
	HeaderValuePrefix string
	CookieSecretRef   string
	HeaderSecretRef   string
}

type ResolvedSourceRuntimeConfig struct {
	SourceID     string
	DisplayName  string
	BaseURL      string
	Timeout      time.Duration
	Headers      map[string]string
	HotlistLimit int
	RetryPolicy  RetryRuntimeConfig
	Auth         ResolvedAuthConfig
	Options      map[string]any
}
