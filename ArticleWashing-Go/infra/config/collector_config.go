package config

import (
	"fmt"
	"sort"
	"strings"

	"content-hub/domain"
)

// CollectorConfig 描述 Go 原生采集器的外部化配置。
//
// 设计目标：
// 1. 将原先散落在代码中的平台注册元数据统一迁移到配置层；
// 2. 与 ArticleWashing-Go 运行时直接兼容，避免后续再维护第二套平台清单；
// 3. 对尚未实现的平台保留明确占位信息和中文说明，方便后续逐个平台落地。
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
	Mode string `json:"mode"`
}

// CollectorSourceDef 是平台注册元数据的配置表达。
//
// 注意：这里既包含运行时可直接落库的字段，也包含 sourceType/sourceURL 等“占位型说明字段”。
// 后者的目标是帮助后续开发者快速理解这个平台当前处于什么阶段、下一步该做什么。
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
	Status              string            `json:"status"`
	Goal                string            `json:"goal"`
	Todo                []string          `json:"todo"`
	Notes               []string          `json:"notes"`
	MigrationReference  string            `json:"migration_reference,omitempty"`
	SupportsArticle     bool              `json:"supports_article"`
	PlaceholderRequired bool              `json:"placeholder_required"`
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
				Mode: domain.CollectorAuthModeHeader,
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
	if source.Todo == nil {
		source.Todo = []string{}
	}
	if source.Notes == nil {
		source.Notes = []string{}
	}
	if source.Aliases == nil {
		source.Aliases = []string{id}
	}
	return source, true
}

func defaultCollectorSources() map[string]CollectorSourceDef {
	return map[string]CollectorSourceDef{
		"36kr":          collectorSourceDef("36Kr", []string{"36kr", "tskr"}, "json-api", "https://gateway.36kr.com/api/mis/nav/home/nav/rank/hot", false, "placeholder", "补齐 36Kr 热榜接口映射、错误语义和测试夹具", []string{"补充请求头与接口参数确认", "实现 JSON 解码与标准字段映射", "为详情抓取预留设计"}, []string{"原 DataCollection 已有实现，可直接对照迁移。"}, false, domain.CollectorAuthModeNone, true, "DataCollection/src/platforms/36kr.js"),
		"52pojie":       collectorSourceDef("吾爱破解", []string{"52pojie", "ftpojie"}, "html", "https://www.52pojie.cn/forum.php?mod=guide&view=hot", false, "placeholder", "补齐 HTML 抓取、列表选择器和反爬校验逻辑", []string{"确认 HTML 结构和分页规则", "实现热榜解析与详情页正文抽取", "增加页面结构变更回归测试"}, []string{"该平台为 HTML 类站点，后续重点是选择器稳定性。"}, true, domain.CollectorAuthModeNone, true, "DataCollection/src/platforms/52pojie.js"),
		"baidu":         collectorSourceDef("百度热搜", []string{"baidu"}, "json-api", "https://top.baidu.com/api/board?platform=wise&tab=realtime", true, "implemented", "已接入主链路，继续维护接口稳定性与详情抓取质量", []string{"持续跟踪上游字段变化", "必要时补充更多 detail fixture"}, []string{"当前 Go 版已实现 hotlist + detail fetch。"}, true, domain.CollectorAuthModeNone, false, "DataCollection/src/platforms/baidu.js"),
		"bilibili":      collectorSourceDef("哔哩哔哩", []string{"bilibili"}, "json-api", "https://api.bilibili.com/x/web-interface/popular", true, "partial", "当前仅支持 hotlist，后续补齐详情采集与正文标准化", []string{"确认详情接口或页面抓取方案", "增加 article fetch 的 fixture 与集成测试"}, []string{"现阶段可用于热点发现，不可直接 bridge 为完整文章。"}, false, domain.CollectorAuthModeNone, false, "DataCollection/src/platforms/bilibili.js"),
		"cls":           collectorSourceDef("财联社", []string{"cls"}, "json-api", "https://www.cls.cn/featured/v1/column/list", false, "placeholder", "补齐财经快讯类 JSON 平台实现", []string{"确认分页与字段稳定性", "补充时间字段和摘要映射"}, []string{"可能需要区分栏目类型与快讯类型。"}, false, domain.CollectorAuthModeNone, true, "DataCollection/src/platforms/cls.js"),
		"douban":        collectorSourceDef("豆瓣", []string{"douban"}, "html", "https://www.douban.com/group/explore", false, "placeholder", "补齐豆瓣 HTML 热帖抓取与正文提取", []string{"确认反爬策略与 UA 需求", "实现列表解析和详情正文抽取"}, []string{"后续若 HTML 不稳定，可评估浏览器回退。"}, true, domain.CollectorAuthModeNone, true, "DataCollection/src/platforms/douban.js"),
		"douyin":        collectorSourceDef("抖音", []string{"douyin"}, "json-api", "https://www.douyin.com/aweme/v1/web/hot/search/list/", false, "placeholder", "补齐抖音热榜 JSON 平台实现", []string{"确认接口参数和签名约束", "补充限流与失败分类处理"}, []string{"该平台后续可能需要更严格的 headers/cookie 组合。"}, false, domain.CollectorAuthModeNone, true, "DataCollection/src/platforms/douyin.js"),
		"eastmoney":     collectorSourceDef("东方财富", []string{"eastmoney"}, "json-api", "https://np-weblist.eastmoney.com/comm/web/getFastNewsList", false, "placeholder", "补齐东方财富快讯平台实现", []string{"确认 query 参数", "实现标准字段归一化"}, []string{"可能需要增加财经类字段映射到 Metadata。"}, false, domain.CollectorAuthModeNone, true, "DataCollection/src/platforms/eastmoney.js"),
		"github":        collectorSourceDef("GitHub Trending", []string{"github"}, "json-api", "https://api.github.com/search/repositories?q=stars:%3E1&sort=stars", true, "implemented", "已实现热榜和 README 详情抓取，继续提升异常说明", []string{"根据 API 限额补充 fallback 方案", "持续维护 fixture"}, []string{"当前 Go 版已实现 hotlist + detail fetch。"}, true, domain.CollectorAuthModeHeader, false, "DataCollection/src/platforms/github.js"),
		"hackernews":    collectorSourceDef("Hacker News", []string{"hackernews"}, "html", "https://news.ycombinator.com/", false, "placeholder", "补齐 HTML 热榜解析并定义详情抓取策略", []string{"实现列表选择器与评论页兼容策略", "视需要评估 article fetch 是否取正文还是评论摘要"}, []string{"该平台是典型 HTML 占位源，优先补足热榜解析。"}, true, domain.CollectorAuthModeNone, true, "DataCollection/src/platforms/hackernews.js"),
		"hupu":          collectorSourceDef("虎扑", []string{"hupu"}, "html", "https://bbs.hupu.com/all-gambia", false, "placeholder", "补齐虎扑论坛热门帖抓取", []string{"实现列表页结构解析", "确认详情页正文与作者字段位置"}, []string{"HTML 结构变更概率较高，建议后续重点补 fixture。"}, true, domain.CollectorAuthModeNone, true, "DataCollection/src/platforms/hupu.js"),
		"jinritoutiao":  collectorSourceDef("今日头条", []string{"jinritoutiao"}, "json-api", "https://www.toutiao.com/hot-event/hot-board/?origin=toutiao_pc", false, "placeholder", "补齐今日头条热榜接口接入", []string{"确认接口鉴权与 headers 依赖", "实现响应字段映射"}, []string{"可能需要特别处理反爬 headers。"}, false, domain.CollectorAuthModeHeader, true, "DataCollection/src/platforms/jinritoutiao.js"),
		"juejin":        collectorSourceDef("掘金", []string{"juejin"}, "json-api", "https://api.juejin.cn/content_api/v1/content/article_rank?category_id=1&type=hot", false, "placeholder", "补齐掘金文章热榜实现", []string{"实现列表字段映射", "评估详情抓取接口或正文回源方案"}, []string{"该平台适合作为下一批 JSON 平台迁移目标。"}, true, domain.CollectorAuthModeNone, true, "DataCollection/src/platforms/juejin.js"),
		"shaoshupai":    collectorSourceDef("少数派", []string{"shaoshupai", "sspai"}, "json-api", "https://sspai.com/api/v1/article/index/page/get?limit=20&offset=0&created_at=0", false, "placeholder", "补齐少数派平台实现", []string{"实现列表字段映射", "评估正文详情接口"}, []string{"原平台偏内容站，后续 bridge 价值较高。"}, true, domain.CollectorAuthModeNone, true, "DataCollection/src/platforms/shaoshupai.js"),
		"sina_finance":  collectorSourceDef("新浪财经", []string{"sina_finance"}, "json-api", "https://zhibo.sina.com.cn/api/zhibo/feed?page=1&page_size=20&zhibo_id=152&tag_id=0&dire=f&dpc=1&pagesize=20", false, "placeholder", "补齐新浪财经 feed 采集实现", []string{"确认 feed 类型与字段含义", "补充稳定性测试"}, []string{"后续可补财经标签到 Metadata。"}, false, domain.CollectorAuthModeNone, true, "DataCollection/src/platforms/sina_finance.js"),
		"stackoverflow": collectorSourceDef("Stack Overflow", []string{"stackoverflow"}, "json-api", "https://api.stackexchange.com/2.3/questions?order=desc&sort=hot&site=stackoverflow", true, "partial", "当前仅支持 hotlist，后续补齐 question detail 与正文抽取", []string{"确认详情接口与速率限制策略", "补 article fetch fixture"}, []string{"现有 Go 版仅完成热榜归一化。"}, false, domain.CollectorAuthModeNone, false, "DataCollection/src/platforms/stackoverflow.js"),
		"tenxunwang":    collectorSourceDef("腾讯网", []string{"tenxunwang"}, "json-api", "https://i.news.qq.com/gw/event/pc_hot_ranking_list?ids_hash=&offset=0&page_size=51&appver=15.5_qqnews_7.1.60&rank_id=hot", false, "placeholder", "补齐腾讯网热榜实现", []string{"校验接口稳定性", "实现列表映射与摘要提取"}, []string{"命名沿用现有 DataCollection 平台 id，避免迁移映射混乱。"}, false, domain.CollectorAuthModeNone, true, "DataCollection/src/platforms/tenxunwang.js"),
		"tieba":         collectorSourceDef("百度贴吧", []string{"tieba"}, "json-api", "http://tieba.baidu.com/hottopic/browse/topicList", false, "placeholder", "补齐贴吧热议话题平台实现", []string{"确认列表字段与 topic URL 规则", "增加回归 fixture"}, []string{"接口为 http，迁移时需要确认是否存在 https 替代。"}, false, domain.CollectorAuthModeNone, true, "DataCollection/src/platforms/tieba.js"),
		"v2ex":          collectorSourceDef("V2EX", []string{"v2ex", "vtex"}, "html", "https://www.v2ex.com/?tab=hot", true, "implemented", "已完成 HTML 热榜和详情页解析，后续主要维护稳定性", []string{"持续维护 selector fixture", "必要时补充结构变更监控"}, []string{"当前 Go 版已具备 hotlist + detail fetch。"}, true, domain.CollectorAuthModeNone, false, "DataCollection/src/platforms/v2ex.js"),
		"weibo":         withCookie(collectorSourceDef("微博热搜", []string{"weibo"}, "json-api", "https://weibo.com/ajax/side/hotSearch", true, "partial", "已实现 Cookie 鉴权热榜抓取，后续继续补齐详情抓取或 bridge 方案", []string{"补充认证失败与过期的运维说明", "评估详情抓取是否必要"}, []string{"当前 Go 版支持 Cookie 注入与健康检查。"}, false, domain.CollectorAuthModeCookie, false, "DataCollection/src/platforms/weibo.js"), "env.WEIBO_COOKIE"),
		"xueqiu":        withCookie(collectorSourceDef("雪球", []string{"xueqiu"}, "json-api", "https://xueqiu.com/hot_event/list.json?count=10", false, "placeholder", "补齐雪球平台实现并接入 Cookie 鉴权", []string{"实现 Cookie 注入和鉴权失效判断", "补充热榜映射和错误测试"}, []string{"该平台与微博类似，需要先打通 Cookie 配置链路。"}, false, domain.CollectorAuthModeCookie, true, "DataCollection/src/platforms/xueqiu.js"), "env.XUEQIU_COOKIE"),
		"zhihu":         collectorSourceDef("知乎热榜", []string{"zhihu"}, "json-api", "https://www.zhihu.com/api/v3/explore/guest/feeds?limit=30&ws_qiangzhisafe=0", false, "placeholder", "补齐知乎热榜实现与后续详情正文抽取", []string{"实现列表字段标准化", "确认详情抓取接口或页面回源方式"}, []string{"适合作为下一批重点迁移平台之一。"}, true, domain.CollectorAuthModeNone, true, "DataCollection/src/platforms/zhihu.js"),
	}
}

func collectorSourceDef(displayName string, aliases []string, sourceType string, sourceURL string, enabled bool, status string, goal string, todo []string, notes []string, supportsArticle bool, authMode string, placeholderRequired bool, migrationRef string) CollectorSourceDef {
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
		Status:              status,
		Goal:                goal,
		Todo:                append([]string(nil), todo...),
		Notes:               append([]string(nil), notes...),
		MigrationReference:  migrationRef,
		SupportsArticle:     supportsArticle,
		PlaceholderRequired: placeholderRequired,
		Headers:             map[string]string{},
	}
}

func withCookie(source CollectorSourceDef, secretRef string) CollectorSourceDef {
	source.CookieSecretRef = secretRef
	return source
}
