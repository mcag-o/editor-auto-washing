# DataCollection → ArticleWashing-Go 迁移对比分析

**审计日期**: 2026-04-11
**审计范围**: `DataCollection/` (Node.js 原版) → `ArticleWashing-Go/collector/` (Go 重写版)

---

## 结论先行

| 维度 | 结论 |
|------|------|
| **架构设计** | ✅ Go 版接口设计优于原版，职责清晰、类型安全 |
| **平台覆盖** | ❌ Go 版仅 7/22 (32%)，缺 15 个平台实现 |
| **核心能力** | ⚠️ Scheduler 状态机完整，但缺 Worker 池、速率限制、指数退避 |
| **HTML 采集** | ❌ 原版支持 HTML 平台 (12 个)，Go 版缺 HTML 解析器 |
| **测试覆盖** | ✅ Go 版 5 个子包全绿，覆盖率良好 |
| **生产就绪** | ❌ 当前不能替代，缺 Phase A 核心能力后方可替代 |

---

## 一、替代判断标准

判断 Go 版 Collector 能否替代 `DataCollection/` 的标准：

1. 是否覆盖全部 22 个采集平台
2. 是否具备 JSON API + HTML 两种采集能力
3. 是否具备超时、重试、速率限制、并发控制
4. 是否具备调度器（定时/手动触发）
5. 是否具备健康检查与采集指标
6. 是否支持 Cookie/代理/自定义 Headers 鉴权
7. 是否具备标准化数据输出（与原版兼容）

当前满足：1、5、6(部分)、7。不满足：2、3(部分)、4。

补充说明：当前 Go collector runtime 的 transport、retry、auth 已通过 `config/config.json` 策略驱动；source-specific timeout、auth profile、retry policy、HTTP client selection 可通过配置切换，secret 通过外部引用提供，不再需要硬编码在 collector plugin 中。

---

## 二、对比结果总览

| 能力域 | DataCollection (Node.js) | ArticleWashing-Go (Go) | 当前结论 |
|--------|--------------------------|------------------------|----------|
| **平台覆盖率** | 22/22 (100%) | 7/22 (32%) | ❌ Go 缺 15 个 |
| **JSON API 采集** | ✅ defineJsonCrawler() | ✅ SourcePlugin 接口实现 | ✅ 对等 |
| **HTML 采集** | ✅ defineHtmlCrawler() + cheerio | ❌ 无 HTML 解析器 | ❌ 缺失 |
| **浏览器回退** | ✅ Playwright 可选回退 | ❌ 无 | ❌ 缺失 |
| **HTTP 客户端** | undici + 超时 + 重试 | net/http + 超时 + 固定延迟重试 | ⚠️ 缺指数退避 |
| **速率限制** | ✅ per-key rateLimiter | ❌ 无 | ❌ 缺失 |
| **并发控制** | ✅ p-limit 并发池 | ❌ 顺序执行 | ❌ 缺失 |
| **指数退避** | ✅ getBackoffDelay | ❌ 固定延迟 | ❌ 缺失 |
| **Scheduler 调度** | ✅ collectMany() 遍历 | ✅ Service + loop() + 状态持久化 | ✅ Go 更健壮 |
| **Cron 定时** | ❌ 无内置 | ❌ 无内置 (仅固定 interval) | ⚠️ 持平 |
| **健康检查** | ❌ 无 | ✅ 每个 SourcePlugin 独立 HealthCheck | ✅ Go 更优 |
| **Cookie 鉴权** | ✅ env.WEIBO_COOKIE / XUEQIU_COOKIE | ⚠️ 仅 HealthCheck 报告，未注入请求 | ⚠️ 不完整 |
| **代理支持** | ✅ env.HTTP_PROXY / HTTPS_PROXY | ❌ 无 | ❌ 缺失 |
| **数据标准化** | ✅ Zod schema 验证 | ✅ struct + Normalize 方法 | ✅ 对等 |
| **配置外部化** | ✅ sources.yaml + .env | ❌ 硬编码注册 | ❌ 缺失 |
| **测试覆盖** | ✅ Vitest, 15 test files, fixtures | ✅ Go test, 5 packages, 全部通过 | ✅ 对等 |
| **CLI 入口** | ✅ node cli/run.js --platform/--all | ❌ 无独立 CLI | ❌ 缺失 |
| **CI/CD 集成** | ✅ npm test/smoke/collect | ⚠️ 依赖主项目 go test | ⚠️ 待完善 |

---

## 三、详细对比分析

### 1. 平台覆盖

#### Node.js 原版 (22 个平台)

| 平台 | 类型 | 状态 | 端点 |
|------|------|------|------|
| baidu | json-api | ✅ | `https://top.baidu.com/api/board` |
| weibo | json-api | ⚠️ 需 Cookie | `https://weibo.com/ajax/side/hotSearch` |
| zhihu | json-api | ✅ | `https://www.zhihu.com/api/v3/explore/guest/feeds` |
| bilibili | json-api | ✅ | `https://api.bilibili.com/x/web-interface/popular` |
| github | json-api | ✅ | `https://api.github.com/search/repositories` |
| stackoverflow | json-api | ✅ | `https://api.stackexchange.com/2.3/questions` |
| hackernews | html | ✅ | `https://news.ycombinator.com/` |
| douban | html | ⚠️ 条件可用 | `https://www.douban.com/group/explore` |
| v2ex | html | ✅ | `https://www.v2ex.com/?tab=hot` |
| juejin | json-api | ✅ | `https://api.juejin.cn/content_api/v1/content/article_rank` |
| douyin | json-api | ⚠️ 条件可用 | `https://www.douyin.com/aweme/v1/web/hot/search/list/` |
| tenxunwang | json-api | ⚠️ 条件可用 | `https://i.news.qq.com/gw/event/pc_hot_ranking_list` |
| jinritoutiao | json-api | ⚠️ 条件可用 | `https://www.toutiao.com/hot-event/hot-board/` |
| 36kr | json-api | ⚠️ 条件可用 | `https://gateway.36kr.com/api/mis/nav/home/nav/rank/hot` |
| 52pojie | html | ⚠️ 条件可用 | `https://www.52pojie.cn/forum.php?mod=guide&view=hot` |
| tieba | json-api | ✅ | `http://tieba.baidu.com/hottopic/browse/topicList` |
| cls | json-api | ⚠️ 条件可用 | `https://www.cls.cn/featured/v1/column/list` |
| eastmoney | json-api | ⚠️ 条件可用 | `https://np-weblist.eastmoney.com/comm/web/getFastNewsList` |
| sina_finance | json-api | ⚠️ 条件可用 | `https://zhibo.sina.com.cn/api/zhibo/feed` |
| xueqiu | json-api | ⚠️ 需 Cookie | `https://xueqiu.com/hot_event/list.json` |
| hupu | html | ⚠️ 条件可用 | `https://bbs.hupu.com/all-gambia` |
| shaoshupai | json-api | ✅ | `https://sspai.com/api/v1/article/index/page/get` |

#### Go 重写版 (7 个平台)

| 平台 | 类型 | 状态 | 端点 |
|------|------|------|------|
| baidu | json-api | ✅ | `https://top.baidu.com/api/board` |
| bilibili | json-api | ✅ | `https://api.bilibili.com/x/web-interface/popular` |
| github | json-api | ✅ | `https://api.github.com/search/repositories` |
| stackoverflow | json-api | ✅ | `https://api.stackexchange.com/2.3/questions` |
| v2ex | json-api | ✅ | `https://v2ex.com/api/topics/hot.json` |
| weibo | json-api | ✅ (含 Cookie) | `https://weibo.com/ajax/side/hotSearch` |
| HTML Base | html (base) | ✅ | 通用 HTML 解析框架 |

**缺失平台 (15 个)**: zhihu, douban, hackernews, juejin, douyin, tenxunwang, jinritoutiao, 36kr, 52pojie, tieba, cls, eastmoney, sina_finance, xueqiu, hupu, shaoshupai

---

### 2. HTTP 客户端对比

#### Node.js 原版

```javascript
// DataCollection/src/core/httpClient.js
export async function request(url, options = {}) {
  for (let attempt = 0; attempt <= retryCount; attempt += 1) {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), timeoutMs);
    try {
      const response = await fetchImpl(url, { method, headers, body, signal: controller.signal });
      if (!response.ok) throw new UpstreamHttpError(...);
      return response;
    } catch (error) {
      if (attempt === retryCount || wrapped.retryable === false) throw wrapped;
      await sleep(getBackoffDelay(attempt, retryBaseMs)); // ← 指数退避
    }
  }
}
```

**特点**:
- `undici` fetch 实现
- `AbortController` 超时控制
- **指数退避** (`getBackoffDelay`)
- 自定义错误类 (`CollectorError`, `UpstreamHttpError`) + `retryable` 标记

#### Go 重写版

```go
// ArticleWashing-Go/collector/httpclient/client.go
func (c *Client) Do(ctx context.Context, req Request) (*Response, error) {
    for attempt := 1; attempt <= c.retryPolicy.MaxAttempts; attempt++ {
        resp, doErr := c.httpClient.Do(httpReq)
        // 成功/失败判断
        if !shouldRetry(resp.StatusCode) || attempt == c.retryPolicy.MaxAttempts {
            return result, lastErr
        }
        select {
        case <-time.After(c.retryPolicy.Wait):  // ← 固定延迟
        case <-ctx.Done():
            return nil, ctx.Err()
        }
    }
}
```

**优点**:
- 标准库 `net/http`，成熟稳定
- `AuthInjector` 回调模式优雅
- 自动 URL 解析 + Query 合并
- `shouldRetry()` 对 5xx/408/429 判断清晰

**缺点**:
- ⚠️ **固定延迟** (`c.retryPolicy.Wait`)，非指数退避
- ⚠️ 无自定义错误类型标记 (`retryable`)
- ⚠️ 无 User-Agent 轮换
- ❌ 无代理支持

---

### 3. 爬虫架构对比

#### Node.js 原版——工厂模式

```javascript
// 工厂函数：一行定义一个平台
export const createBaiduCrawler = defineJsonCrawler({
  sourceUrl: 'https://top.baidu.com/api/board?platform=wise&tab=realtime',
  mapEntries: (payload) => payload?.data?.cards?.[0]?.content?.[0]?.content ?? [],
  mapItem: (entry, index, platform) => createItem(platform, index + 1, entry.word, url, {
    hot: stringOrNull(entry.hotScore),
    summary: stringOrNull(entry.desc),
    raw: entry
  })
});
```

**优点**:
- 极致简洁，每个平台 10-20 行
- 统一通过 `defineJsonCrawler()` / `defineHtmlCrawler()` 构造
- `mapEntries` + `mapItem` 两层抽象，易于理解

**缺点**:
- 类型不严格 (任何字段名都靠约定)
- HTML 解析依赖 cheerio selector，上游改动易失效

#### Go 重写版——接口实现

```go
// Go: 每个平台需要实现 SourcePlugin 接口
type BaiduPlugin struct{ client *httpclient.Client }

func (p *BaiduPlugin) SourceID() string { return "baidu" }
func (p *BaiduPlugin) FetchHotlist(ctx context.Context, req plugin.FetchHotlistRequest) ([]plugin.HotEntry, error) {
    resp, err := p.client.Do(ctx, httpclient.Request{Path: "/api/board", Query: query})
    // 手动解析 JSON，逐字段映射到 HotEntry
}
func (p *BaiduPlugin) NormalizeHotEntry(raw any) (plugin.HotEntry, error) { ... }
func (p *BaiduPlugin) HealthCheck(ctx context.Context) (plugin.SourceHealth, error) { ... }
func (p *BaiduPlugin) Capabilities() plugin.SourceCapabilities { ... }
```

**优点**:
- **类型安全**——所有字段编译期校验
- **接口统一**——FetchHotlist, FetchArticle, Normalize, HealthCheck, Capabilities
- **HealthCheck**——每个平台独立健康检查 (原版无)
- **配置化**——`SourceConfigurablePlugin` 支持动态注入配置

**缺点**:
- 样板代码多，每个平台需要 50-80 行
- 没有工厂函数，新增平台需手写全部方法

---

### 4. 调度器对比

#### Node.js 原版

```javascript
// DataCollection/src/scheduler/collectMany.js
export async function collectMany(platforms, options) {
  const startedAt = new Date().toISOString();
  const results = await Promise.allSettled(
    platforms.map(platform => collectPlatform(platform, options))
  );
  return { startedAt, finishedAt: new Date().toISOString(), results: results.map(...) };
}
```

**特点**:
- `collectMany()` 遍历平台，Promise.allSettled 并发
- 无持久化状态
- 无定时调度

#### Go 重写版

```go
// ArticleWashing-Go/collector/scheduler/service.go
type Service struct {
    repo       repo.CollectorSchedulerStateRepo  // 状态持久化
    runs       runService
    interval   time.Duration                     // 默认 30 分钟
    mu         sync.Mutex                        // 并发安全
    running    bool
    stopCh     chan struct{}
    loopCtx    context.Context
}

func (s *Service) loop(ctx context.Context) {
    for {
        if _, err := s.RunOnce(ctx); err != nil { return }
        select {
        case <-time.After(s.interval):
        case <-ctx.Done(): return
        case <-s.stopCh: return
        }
    }
}
```

**优点**:
- **状态持久化**——Running/Failed/Stopped 状态写入 DB
- **优雅停机**——`Stop()` 带 ackCh 确认机制
- **幂等控制**——重复 StartDaemon 返回 Conflict
- **心跳**——LastHeartbeat 持续更新

**缺点**:
- 顺序执行——`loop()` 中每个平台串行
- 无并发池——`worker_pools.go` 仅存注释

---

### 5. 鉴权模式对比

| 鉴权方式 | Node.js 原版 | Go 重写版 | 状态 |
|----------|-------------|-----------|------|
| 无鉴权 | ✅ 默认 | ✅ 默认 (无 AuthInjector) | ✅ |
| Cookie | ✅ `env.WEIBO_COOKIE` | ⚠️ 可配置，但未注入到请求 | ❌ |
| Proxy | ✅ `env.HTTP_PROXY` | ❌ 无 | ❌ |
| Custom Headers | ✅ 每个平台自定义 | ✅ `AuthInjector` 回调 | ✅ |
| Token | ❌ | ✅ `HeaderAuthInjector()` | ✅ Go 更优 |

---

### 6. 数据标准化对比

| 维度 | Node.js 原版 | Go 重写版 | 状态 |
|------|-------------|-----------|------|
| **标准化方法** | `createItem(platform, rank, title, url, overrides)` | `domain.NewCollectorEntry(...)` | ✅ 对等 |
| **校验** | Zod schema (`resultSchema.js`) | struct 字段 + Normalize 方法 | ⚠️ Go 无运行时 schema 校验 |
| **字段映射** | `mapItem()` 函数 | `NormalizeHotEntry()` / `NormalizeArticle()` | ✅ 对等 |
| **错误处理** | `CollectorError` + retryable | `domain.AppError` + errorCode | ✅ Go 更结构化 |

---

### 7. 性能与并发对比

| 维度 | Node.js 原版 | Go 重写版 | 评估 |
|------|-------------|-----------|------|
| **并发模型** | Promise.allSettled | 顺序 loop() | ❌ Go 当前更慢 |
| **超时控制** | AbortController (10s) | http.Client.Timeout (10s) | ✅ 对等 |
| **速率限制** | per-key rateLimiter | 无 | ❌ Go 无保护 |
| **重试策略** | 指数退避 (250ms × 2^n) | 固定延迟 (5s) | ❌ Go 策略不佳 |
| **22 平台预计耗时** | 30s-120s (并发) | 22 × 10s = 220s (串行) | ❌ Go 慢 2× |

---

### 8. 代码质量对比

| 维度 | DataCollection (Node.js) | ArticleWashing-Go (Go) | 评估 |
|------|--------------------------|------------------------|------|
| 代码行数 | src (43 文件, ~3500 行) | collector (27 文件, ~3800 行) | Go 稍多但类型安全 |
| 测试覆盖率 | 15 test files, Vitest, fixtures | 5 test packages, 全部通过 | ✅ Go 全部通过 |
| CI/CD | npm test / smoke / collect | go test ./collector/... | ⚠️ Go 缺独立 CI 脚本 |
| TypeScript | JSDoc (弱类型) | Go 原生强类型 | ✅ Go 更优 |
| 运行时 | Node.js ≥20 + undici + cheerio | Go 1.25 + 标准库 | ✅ Go 零额外依赖 |
| 错误模型 | 自定义 Error 类 + retryable | domain.AppError + ErrorCode | ✅ 风格不同，Go 更统一 |

---

## 四、Go 版亮点

1. **HealthCheck 接口**——每个 `SourcePlugin` 独立健康检查，返回 `healthy/auth_missing/unavailable` 状态
2. **Bridge Service**——采集器与工作区的双向桥接，带分布式事务补偿
3. **Scheduler 状态持久化**——运行状态写入 DB，重启不丢失
4. **SourceConfigurablePlugin**——接口支持动态注入配置，新增平台无需重新编译
5. **标准化重试逻辑**——统一 HTTP 重试 + 可配置策略
6. **测试覆盖率**——5 个子包全部通过，包括真实 fixture 测试

---

## 五、待补充项清单

### 🔴 阻塞替代

| 编号 | 项目 | 复杂度 | 预估工时 | 优先级 |
|------|------|--------|----------|--------|
| P1 | 补全 15 个缺失平台 (zhihu, douban, hackernews, ...) | 低 (每平台 20-30 行) | 2 天 | P0 |
| P2 | HTML 解析器集成 (goquery/cheerio 等效) | 中 | 1 天 | P0 |
| P3 | 指数退避重试 | 低 | 0.5 天 | P0 |
| P4 | 速率限制器 (per-source) | 中 | 1 天 | P0 |

### 🟡 重要但不阻塞

| 编号 | 项目 | 复杂度 | 预估工时 | 优先级 |
|------|------|--------|----------|--------|
| P5 | Worker Pool 并发调度 | 高 | 2 天 | P1 |
| P6 | Browser Fall back (Playwright) | 高 | 2 天 | P1 |
| P7 | 代理支持 (HTTP/HTTPS_PROXY) | 低 | 0.5 天 | P1 |
| P8 | Cookie 自动注入请求 | 低 | 0.5 天 | P1 |
| P9 | cron 表达式定时 | 低 | 0.5 天 | P2 |
| P10 | CLI 入口 (`go run collector --platform baidu`) | 中 | 1 天 | P2 |
| P11 | 采集指标暴露 (Prometheus/metrics) | 中 | 1 天 | P2 |
| P12 | 配置外部化 (sources.yaml → 动态注册) | 中 | 1 天 | P1 |

---

## 六、实施路线图

```
Phase 1: 核心补齐 (D1-D4)
  ┌─ HTML 解析器 (goquery)
  ├─ 指数退避重试
  ├─ 速率限制器 (per-source)
  └─ 补 15 个平台插件
  → 里程碑: 22/22 平台覆盖，HTML 采集可用

Phase 2: 并发与配置 (D5-D7)
  ├─ Worker Pool 并发调度
  ├─ 配置外部化 (sources.yaml)
  ├─ 代理 + Cookie 注入
  └─ CLI 入口
  → 里程碑: 采集时间 <60s，配置零代码

Phase 3: 运维与可观测 (D8-D10)
  ├─ 浏览器回退 (Playwright)
  ├─ cron 表达式定时
  ├─ 采集指标暴露
  └─ TUI 采集面板
  → 里程碑: 生产就绪，运维可视
```

**总计: 10 个工作日**

---

## 七、最终结论

> `ArticleWashing-Go/collector/` 在**架构设计上已优于原版**，接口清晰、类型安全、测试覆盖完整、状态管理健壮。但 **功能覆盖率仅 32% (7/22)**，核心缺失包括平台实现数量、HTML 解析器、速率限制、并发调度和代理支持。
>
> **不能立即替代**——需要在 Phase 1 (HTML 解析 + 退避 + 速率限制 + 15 平台补齐) 完成后，方可完全替代 Python DataCollection 模块。
