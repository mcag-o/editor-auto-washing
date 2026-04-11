package plugin

import (
	"content-hub/collector/httpclient"
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

type SourceHTTPClientConfigurable interface {
	WithHTTPClient(client *httpclient.Client) SourcePlugin
}

type SourceHTTPClientAccessor interface {
	HTTPClient() *httpclient.Client
}

// SourceDescriptor 用于把平台元数据从插件层投影回 registry / 持久化层。
//
// 设计原因：
// - 解决“平台注册信息硬编码在代码里、数据库记录又缺少外部配置语义”的问题；
// - 让 placeholder 和真实插件都能返回统一描述，便于 Sync 时落库；
// - 为后续把更多平台从骨架升级为正式实现提供稳定 contract。
type SourceDescriptor interface {
	Descriptor() SourceDefinition
}

type SourceDefinition struct {
	Enabled            bool
	ScheduleEnabled    bool
	IntervalMinutes    int
	TimeoutMS          int
	HotlistLimit       int
	DetailFetchEnabled bool
	Concurrency        int
	AuthMode           string
	CookieSecretRef    string
	HeaderSecretRef    string
	Headers            map[string]string
	RetryPolicy        map[string]any
	Options            map[string]any
	Metadata           map[string]any
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
