package config

import (
	"fmt"
	"sort"
	"strings"

	"content-hub/domain"
)

// CollectorConfig 描述当前采集器的外部化配置。
//
// 设计目标：
// 1. 将采集运行时所需的源定义和默认策略统一收敛到配置层；
// 2. 与当前 Go 运行时直接兼容，避免后续再维护第二套平台清单；
// 3. 保持 schema 仅覆盖运行时会读取和持久化的字段，避免再混入占位/进度类元数据。
type CollectorConfig struct {
	Defaults      CollectorDefaults             `json:"defaults"`
	HTTPClients   map[string]HTTPClientProfile  `json:"http_clients"`
	RetryPolicies map[string]RetryPolicyProfile `json:"retry_policies"`
	AuthProfiles  map[string]AuthProfileConfig  `json:"auth_profiles"`
	Sources       map[string]CollectorSourceDef `json:"sources"`
}

type CollectorDefaults struct {
	HTTPClient   string `json:"http_client"`
	RetryPolicy  string `json:"retry_policy"`
	AuthProfile  string `json:"auth_profile"`
	TimeoutMS    int    `json:"timeout_ms"`
	IntervalMins int    `json:"interval_minutes"`
	HotlistLimit int    `json:"hotlist_limit"`
	Concurrency  int    `json:"concurrency"`
}

type HTTPClientProfile struct {
	UserAgent string            `json:"user_agent,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
}

type RetryPolicyProfile struct {
	MaxAttempts int `json:"max_attempts"`
	BaseWaitMS  int `json:"base_wait_ms"`
	MaxWaitMS   int `json:"max_wait_ms"`
}

type AuthProfileConfig struct {
	Mode              string `json:"mode"`
	HeaderName        string `json:"header_name,omitempty"`
	HeaderValuePrefix string `json:"header_value_prefix,omitempty"`
}

// CollectorSourceDef 是采集源运行时配置的最小持久化表达。
type CollectorSourceDef struct {
	DisplayName         string            `json:"display_name"`
	Aliases             []string          `json:"aliases"`
	SourceType          string            `json:"source_type"`
	SourceURL           string            `json:"source_url"`
	Enabled             bool              `json:"enabled"`
	ScheduleEnabled     bool              `json:"schedule_enabled"`
	IntervalMinutes     int               `json:"interval_minutes"`
	TimeoutMS           int               `json:"timeout_ms"`
	HotlistLimit        int               `json:"hotlist_limit"`
	DetailFetchEnabled  bool              `json:"detail_fetch_enabled"`
	Concurrency         int               `json:"concurrency"`
	AuthMode            string            `json:"auth_mode"`
	HTTPClient          string            `json:"http_client,omitempty"`
	RetryPolicy         string            `json:"retry_policy,omitempty"`
	AuthProfile         string            `json:"auth_profile,omitempty"`
	CookieSecretRef     string            `json:"cookie_secret_ref,omitempty"`
	HeaderSecretRef     string            `json:"header_secret_ref,omitempty"`
	Headers             map[string]string `json:"headers,omitempty"`
	SupportsArticle     bool              `json:"supports_article"`
}

func DefaultCollectorConfig() CollectorConfig {
	return CollectorConfig{
		Defaults: CollectorDefaults{
			HTTPClient:   defaultCollectorHTTPClientProfileID,
			RetryPolicy:  defaultCollectorRetryPolicyProfileID,
			AuthProfile:  defaultCollectorAuthProfileID,
			TimeoutMS:    10000,
			IntervalMins: 30,
			HotlistLimit: 50,
			Concurrency:  1,
		},
		HTTPClients: map[string]HTTPClientProfile{
			defaultCollectorHTTPClientProfileID: {
				Headers: map[string]string{},
			},
		},
		RetryPolicies: map[string]RetryPolicyProfile{
			defaultCollectorRetryPolicyProfileID: {
				MaxAttempts: 3,
				BaseWaitMS:  500,
				MaxWaitMS:   5000,
			},
		},
		AuthProfiles: map[string]AuthProfileConfig{
			defaultCollectorAuthProfileID: {
				Mode: domain.CollectorAuthModeNone,
			},
			"header": {
				Mode:       domain.CollectorAuthModeHeader,
				HeaderName: "Authorization",
			},
			"cookie": {
				Mode: domain.CollectorAuthModeCookie,
			},
		},
		Sources: defaultCollectorSources(),
	}
}

func (c CollectorConfig) Validate() error {
	if c.Defaults.IntervalMins <= 0 {
		return fmt.Errorf("collector.defaults.interval_minutes must be positive")
	}
	if c.Defaults.TimeoutMS <= 0 {
		return fmt.Errorf("collector.defaults.timeout_ms must be positive")
	}
	if c.Defaults.HotlistLimit <= 0 {
		return fmt.Errorf("collector.defaults.hotlist_limit must be positive")
	}
	if c.Defaults.Concurrency <= 0 {
		return fmt.Errorf("collector.defaults.concurrency must be positive")
	}
	if _, ok := c.HTTPClients[c.Defaults.HTTPClient]; !ok {
		return fmt.Errorf("collector.defaults.http_client references unknown profile %q", c.Defaults.HTTPClient)
	}
	if _, ok := c.RetryPolicies[c.Defaults.RetryPolicy]; !ok {
		return fmt.Errorf("collector.defaults.retry_policy references unknown profile %q", c.Defaults.RetryPolicy)
	}
	if _, ok := c.AuthProfiles[c.Defaults.AuthProfile]; !ok {
		return fmt.Errorf("collector.defaults.auth_profile references unknown profile %q", c.Defaults.AuthProfile)
	}
	for name, policy := range c.RetryPolicies {
		if err := validateRetryPolicyProfile(name, policy); err != nil {
			return err
		}
	}
	for name, profile := range c.AuthProfiles {
		if err := validateAuthProfile(name, profile); err != nil {
			return err
		}
	}
	if len(c.Sources) == 0 {
		return fmt.Errorf("collector.sources cannot be empty")
	}
	for id, source := range c.Sources {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("collector.sources contains empty source id")
		}
		if strings.TrimSpace(source.DisplayName) == "" {
			return fmt.Errorf("collector.sources.%s.display_name cannot be empty", id)
		}
		if strings.TrimSpace(source.SourceType) == "" {
			return fmt.Errorf("collector.sources.%s.source_type cannot be empty", id)
		}
		if source.IntervalMinutes < 0 {
			return fmt.Errorf("collector.sources.%s.interval_minutes cannot be negative", id)
		}
		if source.TimeoutMS < 0 {
			return fmt.Errorf("collector.sources.%s.timeout_ms cannot be negative", id)
		}
		if source.Concurrency < 0 {
			return fmt.Errorf("collector.sources.%s.concurrency cannot be negative", id)
		}
		if source.HTTPClient != "" {
			if _, ok := c.HTTPClients[source.HTTPClient]; !ok {
				return fmt.Errorf("collector.sources.%s.http_client references unknown profile %q", id, source.HTTPClient)
			}
		}
		if source.RetryPolicy != "" {
			if _, ok := c.RetryPolicies[source.RetryPolicy]; !ok {
				return fmt.Errorf("collector.sources.%s.retry_policy references unknown profile %q", id, source.RetryPolicy)
			}
		}
		if strings.TrimSpace(source.AuthMode) != "" && strings.TrimSpace(source.AuthProfile) == "" {
			return fmt.Errorf("collector.sources.%s.auth_mode requires explicit auth_profile", id)
		}
		if source.AuthProfile != "" {
			profile, ok := c.AuthProfiles[source.AuthProfile]
			if !ok {
				return fmt.Errorf("collector.sources.%s.auth_profile references unknown profile %q", id, source.AuthProfile)
			}
			if mode := strings.TrimSpace(profile.Mode); mode != "" && strings.TrimSpace(source.AuthMode) != "" && source.AuthMode != mode {
				return fmt.Errorf("collector.sources.%s.auth_mode conflicts with auth_profile %q", id, source.AuthProfile)
			}
		}
	}
	return nil
}

func validateRetryPolicyProfile(name string, policy RetryPolicyProfile) error {
	if policy.MaxAttempts <= 0 {
		return fmt.Errorf("collector.retry_policies.%s.max_attempts must be positive", name)
	}
	if policy.BaseWaitMS < 0 {
		return fmt.Errorf("collector.retry_policies.%s.base_wait_ms cannot be negative", name)
	}
	if policy.MaxWaitMS < policy.BaseWaitMS {
		return fmt.Errorf("collector.retry_policies.%s.max_wait_ms cannot be smaller than base_wait_ms", name)
	}
	return nil
}

func validateAuthProfile(name string, profile AuthProfileConfig) error {
	mode := strings.TrimSpace(profile.Mode)
	if mode == "" {
		return fmt.Errorf("collector.auth_profiles.%s.mode cannot be empty", name)
	}
	validModes := map[string]bool{
		domain.CollectorAuthModeNone:   true,
		domain.CollectorAuthModeHeader: true,
		domain.CollectorAuthModeCookie: true,
	}
	if !validModes[mode] {
		return fmt.Errorf("collector.auth_profiles.%s.mode must be one of none,header,cookie", name)
	}
	if mode == domain.CollectorAuthModeHeader && strings.TrimSpace(profile.HeaderName) == "" {
		return fmt.Errorf("collector.auth_profiles.%s.header_name cannot be empty for header auth", name)
	}
	return nil
}

func (c CollectorConfig) CanonicalSourceIDs() []string {
	ids := make([]string, 0, len(c.Sources))
	for id := range c.Sources {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (c CollectorConfig) SourceOrDefault(id string) (CollectorSourceDef, bool) {
	source, ok := c.Sources[id]
	if !ok {
		return CollectorSourceDef{}, false
	}
	if strings.TrimSpace(source.HTTPClient) == "" {
		source.HTTPClient = c.Defaults.HTTPClient
	}
	if strings.TrimSpace(source.RetryPolicy) == "" {
		source.RetryPolicy = c.Defaults.RetryPolicy
	}
	if strings.TrimSpace(source.AuthProfile) == "" {
		source.AuthProfile = c.Defaults.AuthProfile
	}
	if source.IntervalMinutes <= 0 {
		source.IntervalMinutes = c.Defaults.IntervalMins
	}
	if source.TimeoutMS <= 0 {
		source.TimeoutMS = c.Defaults.TimeoutMS
	}
	if source.HotlistLimit <= 0 {
		source.HotlistLimit = c.Defaults.HotlistLimit
	}
	if source.Concurrency <= 0 {
		source.Concurrency = c.Defaults.Concurrency
	}
	if profile, ok := c.AuthProfiles[source.AuthProfile]; ok && strings.TrimSpace(profile.Mode) != "" {
		source.AuthMode = profile.Mode
	} else if strings.TrimSpace(source.AuthMode) == "" {
		source.AuthMode = domain.CollectorAuthModeNone
	}
	if source.Headers == nil {
		source.Headers = map[string]string{}
	}
	if source.Aliases == nil {
		source.Aliases = []string{id}
	}
	return source, true
}

func defaultCollectorSources() map[string]CollectorSourceDef {
	return map[string]CollectorSourceDef{
		"36kr":          collectorSourceDef("36Kr", []string{"36kr", "tskr"}, "json-api", "https://gateway.36kr.com/api/mis/nav/home/nav/rank/hot", false, false, domain.CollectorAuthModeNone),
		"52pojie":       collectorSourceDef("吾爱破解", []string{"52pojie", "ftpojie"}, "html", "https://www.52pojie.cn/forum.php?mod=guide&view=hot", false, true, domain.CollectorAuthModeNone),
		"baidu":         collectorSourceDef("百度热搜", []string{"baidu"}, "json-api", "https://top.baidu.com/api/board?platform=wise&tab=realtime", true, true, domain.CollectorAuthModeNone),
		"bilibili":      collectorSourceDef("哔哩哔哩", []string{"bilibili"}, "json-api", "https://api.bilibili.com/x/web-interface/popular", true, false, domain.CollectorAuthModeNone),
		"cls":           collectorSourceDef("财联社", []string{"cls"}, "json-api", "https://www.cls.cn/featured/v1/column/list", false, false, domain.CollectorAuthModeNone),
		"douban":        collectorSourceDef("豆瓣", []string{"douban"}, "html", "https://www.douban.com/group/explore", false, true, domain.CollectorAuthModeNone),
		"douyin":        collectorSourceDef("抖音", []string{"douyin"}, "json-api", "https://www.douyin.com/aweme/v1/web/hot/search/list/", false, false, domain.CollectorAuthModeNone),
		"eastmoney":     collectorSourceDef("东方财富", []string{"eastmoney"}, "json-api", "https://np-weblist.eastmoney.com/comm/web/getFastNewsList", false, false, domain.CollectorAuthModeNone),
		"github":        collectorSourceDef("GitHub Trending", []string{"github"}, "json-api", "https://api.github.com/search/repositories?q=stars:%3E1&sort=stars", true, true, domain.CollectorAuthModeHeader),
		"hackernews":    collectorSourceDef("Hacker News", []string{"hackernews"}, "html", "https://news.ycombinator.com/", false, true, domain.CollectorAuthModeNone),
		"hupu":          collectorSourceDef("虎扑", []string{"hupu"}, "html", "https://bbs.hupu.com/all-gambia", false, true, domain.CollectorAuthModeNone),
		"jinritoutiao":  collectorSourceDef("今日头条", []string{"jinritoutiao"}, "json-api", "https://www.toutiao.com/hot-event/hot-board/?origin=toutiao_pc", false, false, domain.CollectorAuthModeHeader),
		"juejin":        collectorSourceDef("掘金", []string{"juejin"}, "json-api", "https://api.juejin.cn/content_api/v1/content/article_rank?category_id=1&type=hot", false, true, domain.CollectorAuthModeNone),
		"shaoshupai":    collectorSourceDef("少数派", []string{"shaoshupai", "sspai"}, "json-api", "https://sspai.com/api/v1/article/index/page/get?limit=20&offset=0&created_at=0", false, true, domain.CollectorAuthModeNone),
		"sina_finance":  collectorSourceDef("新浪财经", []string{"sina_finance"}, "json-api", "https://zhibo.sina.com.cn/api/zhibo/feed?page=1&page_size=20&zhibo_id=152&tag_id=0&dire=f&dpc=1&pagesize=20", false, false, domain.CollectorAuthModeNone),
		"stackoverflow": collectorSourceDef("Stack Overflow", []string{"stackoverflow"}, "json-api", "https://api.stackexchange.com/2.3/questions?order=desc&sort=hot&site=stackoverflow", true, false, domain.CollectorAuthModeNone),
		"tenxunwang":    collectorSourceDef("腾讯网", []string{"tenxunwang"}, "json-api", "https://i.news.qq.com/gw/event/pc_hot_ranking_list?ids_hash=&offset=0&page_size=51&appver=15.5_qqnews_7.1.60&rank_id=hot", false, false, domain.CollectorAuthModeNone),
		"tieba":         collectorSourceDef("百度贴吧", []string{"tieba"}, "json-api", "http://tieba.baidu.com/hottopic/browse/topicList", false, false, domain.CollectorAuthModeNone),
		"v2ex":          collectorSourceDef("V2EX", []string{"v2ex", "vtex"}, "html", "https://www.v2ex.com/?tab=hot", true, true, domain.CollectorAuthModeNone),
		"weibo":         withCookie(collectorSourceDef("微博热搜", []string{"weibo"}, "json-api", "https://weibo.com/ajax/side/hotSearch", true, false, domain.CollectorAuthModeCookie), "env.WEIBO_COOKIE"),
		"xueqiu":        withCookie(collectorSourceDef("雪球", []string{"xueqiu"}, "json-api", "https://xueqiu.com/hot_event/list.json?count=10", false, false, domain.CollectorAuthModeCookie), "env.XUEQIU_COOKIE"),
		"zhihu":         collectorSourceDef("知乎热榜", []string{"zhihu"}, "json-api", "https://www.zhihu.com/api/v3/explore/guest/feeds?limit=30&ws_qiangzhisafe=0", false, true, domain.CollectorAuthModeNone),
	}
}

func collectorSourceDef(displayName string, aliases []string, sourceType string, sourceURL string, enabled bool, supportsArticle bool, authMode string) CollectorSourceDef {
	return CollectorSourceDef{
		DisplayName:         displayName,
		Aliases:             aliases,
		SourceType:          sourceType,
		SourceURL:           sourceURL,
		Enabled:             enabled,
		ScheduleEnabled:     enabled,
		IntervalMinutes:     30,
		TimeoutMS:           10000,
		HotlistLimit:        50,
		DetailFetchEnabled:  supportsArticle && enabled,
		Concurrency:         1,
		AuthMode:            authMode,
		HTTPClient:          defaultCollectorHTTPClientProfileID,
		RetryPolicy:         defaultCollectorRetryPolicyProfileID,
		AuthProfile:         authMode,
		SupportsArticle:     supportsArticle,
		Headers:             map[string]string{},
	}
}

func withCookie(source CollectorSourceDef, secretRef string) CollectorSourceDef {
	source.CookieSecretRef = secretRef
	return source
}
