package runtime

import "content-hub/collector/httpclient"

type HTTPFactory struct{}

func NewHTTPFactory() *HTTPFactory {
	return &HTTPFactory{}
}

func (f *HTTPFactory) Build(cfg ResolvedSourceRuntimeConfig, auth httpclient.AuthInjector) (*httpclient.Client, error) {
	return httpclient.New(httpclient.Options{
		BaseURL:        cfg.BaseURL,
		Timeout:        cfg.Timeout,
		DefaultHeaders: cfg.Headers,
		AuthInjector:   auth,
		RetryPolicy: httpclient.RetryPolicy{
			MaxAttempts: cfg.RetryPolicy.MaxAttempts,
			Wait:        cfg.RetryPolicy.Wait,
			Backoff: httpclient.ExponentialBackoff{
				BaseWait: cfg.RetryPolicy.Wait,
				MaxWait:  cfg.RetryPolicy.MaxWait,
			},
		},
	})
}
