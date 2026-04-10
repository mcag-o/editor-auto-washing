package plugin

import (
	"content-hub/domain"
	"context"
	"fmt"
	"time"
)

type SourcePlugin interface {
	SourceID() string
	DisplayName() string
	Aliases() []string
	FetchHotlist(ctx context.Context, req FetchHotlistRequest) ([]HotEntry, error)
	FetchArticle(ctx context.Context, req FetchArticleRequest) (*RawArticle, error)
	NormalizeHotEntry(raw any) (HotEntry, error)
	NormalizeArticle(raw any) (*NormalizedArticle, error)
	HealthCheck(ctx context.Context) (SourceHealth, error)
	Capabilities() SourceCapabilities
}

type SourceConfigurablePlugin interface {
	WithSourceConfig(source domain.CollectorSource) SourcePlugin
}

type FetchHotlistRequest struct {
	Limit   int
	Headers map[string]string
}

type FetchArticleRequest struct {
	Entry HotEntry
}

type HotEntry struct {
	SourceID     string
	ExternalID   string
	CanonicalURL string
	Title        string
	Summary      string
	Author       string
	Tags         []string
	Rank         *int
	PublishedAt  *time.Time
	RawJSON      []byte
	Metadata     map[string]any
}

type RawArticle struct {
	SourceID     string
	ExternalID   string
	CanonicalURL string
	Body         []byte
	ContentType  string
	Metadata     map[string]any
}

type NormalizedArticle struct {
	SourceID     string
	ExternalID   string
	CanonicalURL string
	Title        string
	Body         string
	Summary      string
	Author       string
	Tags         []string
	PublishedAt  *time.Time
	RawJSON      []byte
	Metadata     map[string]any
}

type SourceCapabilities struct {
	SupportsHotlist bool
	SupportsArticle bool
	AuthModes       []string
}

type SourceHealth struct {
	SourceID  string
	OK        bool
	Code      string
	CheckedAt time.Time
	Message   string
}

const (
	HealthCodeHealthy     = "healthy"
	HealthCodeAuthMissing = "auth_missing"
	HealthCodeAuthExpired = "auth_expired"
	HealthCodeUnavailable = "unavailable"
)

func ErrArticleFetchNotSupported(sourceID string) error {
	return fmt.Errorf("article fetch not supported for source %s", sourceID)
}
