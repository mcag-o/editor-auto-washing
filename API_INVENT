# API Inventory

> 目的：汇总当前 [`hot_news`](.) 项目与相邻前端项目 [`HotList-Web`](../HotList-Web) 涉及到的**全部已识别 API / 数据源地址**，并明确区分：
> 1. 项目自己的对外 API
> 2. 前端直接调用的外部 API
> 3. 各平台上游抓取源 API / 页面源地址
>
> 说明：
> - “是否当前可直接使用”是按**当前仓库与当前环境状态**判断，不等同于公网某个已部署版本一定不可用。
> - [`hot_news`](.) 当前本地环境存在依赖兼容性与外部服务阻塞，因此其“项目对外 API”大多标为**否**。
> - [`HotList-Web`](../HotList-Web) 相关源码文件此前已读到，但当前绝对目录状态异常；下文对其接口描述基于已读取源码内容整理。

---

## 1. [`hot_news`](.) 项目对外 API

### 1.1 基础信息

- 默认监听地址来自 [`config/config.yaml`](config/config.yaml)：`0.0.0.0:18080`
- 路由注册位置：[`app.include_router()`](app/main.py:81)
- 健康检查：[`health_check()`](app/main.py:86)
- 热榜接口：[`app/api/v1/daily_news.py`](app/api/v1/daily_news.py)
- 网站元信息接口：[`app/api/v1/web_tools.py`](app/api/v1/web_tools.py)
- 分析接口：[`app/api/v1/analysis.py`](app/api/v1/analysis.py)

### 1.2 对外接口清单

| 分组 | 接口 | 方法 | 用途 | 主要参数 | 信息类型 | 内容来源方式 | 返回格式/字段 | 是否当前可直接使用 |
|---|---|---|---|---|---|---|---|---|
| 健康检查 | `/health` | GET | 检查服务存活与版本 | 无 | 服务状态 | 本地应用内存 | JSON：`status`, `version` | 否（当前本地服务未成功跑起） |
| 热榜 | `/api/v1/dailynews/` | GET | 获取单个平台热榜 | `platform`, `date?` | 单平台热榜列表 | 读取 Redis/缓存中的爬虫结果 | JSON：`status`, `data[]`, `msg`；`data` 内通常含 `title`, `url`, `content/desc`, `source`, `publish_time` | 否 |
| 热榜 | `/api/v1/dailynews/all` | GET | 获取所有平台热榜 | `date?` | 多平台热榜映射 | 遍历缓存键读取 | JSON：`status`, `data{platform:list}`, `msg` | 否 |
| 热榜 | `/api/v1/dailynews/multi` | GET | 获取指定多个平台热榜 | `platforms`, `date?` | 多平台热榜映射 | 读取多个平台缓存 | JSON：`status`, `data{platform:list}`, `msg` | 否 |
| 热榜 | `/api/v1/dailynews/search` | GET | 按关键词检索热榜标题 | `keyword`, `date?`, `platforms?`, `limit?` | 检索结果列表 | 读取缓存后在应用层过滤标题 | JSON：`status`, `data[]`, `msg`, `total`, `search_results`；结果项含 `id`, `title`, `source`, `rank`, `category`, `sub_category`, `url` | 否 |
| 网站工具 | `/api/v1/tools/website-meta/` | GET | 获取任意页面 meta 信息与 favicon | `url` | 页面元信息 | 先查缓存；未命中则直接请求目标页面，失败时回退 [`cloudscraper`](app/api/v1/web_tools.py:60) | JSON：`status`, `data.meta_info`, `data.favicon_url`, `msg`, `cache` | 否（依赖本地服务） |
| 分析 | `/api/v1/analysis/trend` | GET | 热点聚合分析 | `date?`, `type?` | 趋势/主题分析 | 先查缓存，未命中调用 [`TrendAnalyzer`](app/api/v1/analysis.py:37) | 动态 JSON；错误时返回 `status=error`, `message`, `date` | 否 |
| 分析 | `/api/v1/analysis/platform-comparison` | GET | 平台对比分析 | `date?` | 平台比较结果 | 缓存 + [`TrendAnalyzer.get_platform_comparison()`](app/api/v1/analysis.py:71) | 动态 JSON / 错误 JSON | 否 |
| 分析 | `/api/v1/analysis/cross-platform` | GET | 跨平台热点分析 | `date?`, `refresh?` | 跨平台共现/传播分析 | 缓存 + [`TrendAnalyzer.get_cross_platform_analysis()`](app/api/v1/analysis.py:107) | 动态 JSON / 错误 JSON | 否 |
| 分析 | `/api/v1/analysis/advanced` | GET | 高级分析 | `date?`, `refresh?` | 高级分析结果 | 缓存 + [`TrendAnalyzer.get_advanced_analysis()`](app/api/v1/analysis.py:143) | 动态 JSON / 错误 JSON | 否 |
| 分析 | `/api/v1/analysis/prediction` | GET | 热点趋势预测 | `date?` | 预测结果 | 缓存 + [`TrendPredictor.get_prediction()`](app/api/v1/analysis.py:176) | 动态 JSON / 错误 JSON | 否 |
| 分析 | `/api/v1/analysis/keyword-cloud` | GET | 关键词云 | `date?`, `refresh?`, `platforms?`, `category?`, `keyword_count?` | 关键词/词云数据 | 缓存 + [`TrendAnalyzer.get_keyword_cloud()`](app/api/v1/analysis.py:221) | 动态 JSON / 错误 JSON | 否 |
| 分析 | `/api/v1/analysis/data-visualization` | GET | 数据可视化分析 | `date?`, `refresh?`, `platforms?` | 可视化数据 | 缓存 + [`TrendAnalyzer.get_data_visualization()`](app/api/v1/analysis.py:263) | 动态 JSON / 错误 JSON | 否 |
| 分析 | `/api/v1/analysis/trend-forecast` | GET | 趋势预测分析 | `date?`, `refresh?`, `time_range?` | 未来时间范围趋势预测 | 缓存 + [`TrendAnalyzer.get_trend_forecast()`](app/api/v1/analysis.py:305) | 动态 JSON / 错误 JSON | 否 |

### 1.3 已观察到的项目外消费方式

| 调用方 | 实际调用地址 | 用途 | 备注 |
|---|---|---|---|
| Telegram 机器人 [`tg_bot.py`](tg_bot.py) | `https://orz.ai/api/v1/dailynews/?platform=...` | 获取线上部署版热榜 | 这是**已部署公网接口**，不是本地仓库自动可用 |

### 1.4 [`hot_news`](.) 项目对外接口的统一数据特点

- 热榜类接口最终依赖缓存键：`crawler:{platform}:{date}`，参考 [`get_hot_news()`](app/api/v1/daily_news.py:17)
- 爬虫内部产出的统一新闻对象通常为：

```json
{
  "title": "标题",
  "url": "原始链接",
  "content": "摘要或补充信息",
  "source": "平台代码",
  "publish_time": "YYYY-MM-DD HH:MM:SS"
}
```

- [`/search`](app/api/v1/daily_news.py:134) 会把多平台缓存数据再转换为统一检索对象：`id`, `title`, `source`, `rank`, `category`, `sub_category`, `url`

---

## 2. [`HotList-Web`](../HotList-Web) 前端实际调用的外部 API

> 该项目不是后端，不提供自己的 API；它是前端页面，直接请求外部热榜服务。

### 2.1 实际调用接口

| 接口 | 方法 | 调用位置 | 用途 | 主要参数 | 信息类型 | 内容来源方式 | 前端实际使用的返回字段 | 是否当前可直接使用 |
|---|---|---|---|---|---|---|---|---|
| `https://hot-api.vhan.eu.org/v2?type=all` | GET | [`vhInit()`](../HotList-Web/src/App.vue:75) | 初始化加载所有榜单 | `type=all` | 全部榜单聚合结果 | 前端直接调用第三方 API | 顶层使用 `data[]`；每个榜单项匹配 `name`, `subtitle`, `data`, `update_time` | 条件式（依赖第三方 API 可达） |
| `https://hot-api.vhan.eu.org/v2?type={item.key}` | GET | [`refreshFn()`](../HotList-Web/src/App.vue:95) | 刷新单个榜单 | `type=<榜单 key>` | 单榜单热榜结果 | 前端直接调用第三方 API | 使用 `data`, `update_time` | 条件式 |

### 2.2 [`HotList-Web`](../HotList-Web) 页面实际展示的数据格式

前端最终渲染字段来自 [`ListItem`](../HotList-Web/src/components/ListItem/ListItem.vue:12)：

```json
{
  "index": 1,
  "title": "热榜标题",
  "url": "跳转链接",
  "hot": "热度值"
}
```

外层榜单对象在 [`hotlistKey`](../HotList-Web/src/App.vue:55) 中与上游返回值按 `name + subtitle` 匹配，常见结构为：

```json
{
  "name": "微博",
  "subtitle": "热搜榜",
  "data": [{"index":1,"title":"...","url":"...","hot":"..."}],
  "update_time": "时间字符串"
}
```

### 2.3 文档中出现但不是前端运行时直接调用的地址

| 地址 | 类型 | 说明 |
|---|---|---|
| `https://api.vvhan.com/article/wbHot.html` | 文档/说明页 | 出现在 [`README.md`](../HotList-Web/README.md) 中，属于 API 说明入口，不是前端运行时代码里直接 `fetch` 的地址 |
| `https://api.vvhan.com/article/hotlist.html` | 文档页 | 出现在页面公告文案中，用于介绍 API 来源 |

---

## 3. [`hot_news`](.) 各平台抓取源 API / 页面源地址

> 这一部分是 [`hot_news`](.) 的**上游数据源**，不是它对外暴露给调用方的 API。
> 项目会把这些源地址返回的数据或页面解析为统一热榜对象，再写入缓存，最后由 [`/api/v1/dailynews`](app/api/v1/daily_news.py:16) 等接口对外提供。

### 3.1 统一输出格式

无论上游是 JSON API、HTML 页面还是浏览器抓取，项目最终都尽量转成统一结构：

```json
{
  "title": "标题",
  "url": "跳转链接",
  "content": "摘要/热度/来源说明",
  "source": "平台代码",
  "publish_time": "YYYY-MM-DD HH:MM:SS"
}
```

### 3.2 站点 API / JSON 接口型数据源

| 平台代码 | 爬虫文件 | 上游地址 | 请求方式 | 信息类型 | 内容来源方式 | 上游格式 | 项目内转换后格式 | 是否当前可直接使用 |
|---|---|---|---|---|---|---|---|---|
| `baidu` | [`baidu.py`](app/services/sites/baidu.py) | `https://top.baidu.com/api/board?platform=wise&tab=realtime` | GET | 百度实时热搜 | 直接请求百度 JSON API | JSON | `title`, `url`, `content(desc)`, `source`, `publish_time` | 条件式（需本地服务和网络） |
| `bilibili` | [`bilibili.py`](app/services/sites/bilibili.py) | `https://api.bilibili.com/x/web-interface/popular` | GET | B 站热门视频 | 直接请求 B 站 API | JSON | 统一新闻对象 | 条件式 |
| `cls` | [`cls.py`](app/services/sites/cls.py) | `https://www.cls.cn/featured/v1/column/list` | GET | 财联社快讯/电报 | 直接请求站点 API | JSON | 统一新闻对象 | 条件式 |
| `douyin` | [`douyin.py`](app/services/sites/douyin.py) | `https://www.douyin.com/aweme/v1/web/hot/search/list/?...` | GET | 抖音热搜/热榜 | 直接请求抖音 Web API | JSON | 统一新闻对象 | 条件式 |
| `eastmoney` | [`eastmoney.py`](app/services/sites/eastmoney.py) | `https://np-weblist.eastmoney.com/comm/web/getFastNewsList` | GET | 东方财富快讯 | 直接请求站点 API | JSON | 统一新闻对象 | 条件式 |
| `github` | [`github.py`](app/services/sites/github.py) | `https://api.github.com/search/repositories?q=stars:%3E1&sort=stars` | GET | GitHub 热门仓库/高星仓库 | 直接请求 GitHub API | JSON | 统一新闻对象 | 条件式 |
| `jinritoutiao` | [`jinritoutiao.py`](app/services/sites/jinritoutiao.py) | `https://www.toutiao.com/hot-event/hot-board/?origin=toutiao_pc` | GET | 今日头条热点榜 | 直接请求头条热榜接口 | JSON | 统一新闻对象 | 条件式 |
| `juejin` | [`juejin.py`](app/services/sites/juejin.py) | `https://api.juejin.cn/content_api/v1/content/article_rank?category_id=1&type=hot` | GET | 掘金热门文章 | 直接请求掘金 API | JSON | 统一新闻对象 | 条件式 |
| `shaoshupai` | [`sspai.py`](app/services/sites/sspai.py) | `https://sspai.com/api/v1/article/index/page/get?limit=20&offset=0&created_at=0` | GET | 少数派文章流 | 直接请求少数派 API | JSON | 统一新闻对象 | 条件式 |
| `stackoverflow` | [`stackoverflow.py`](app/services/sites/stackoverflow.py) | `https://api.stackexchange.com/2.3/questions?order=desc&sort=hot&site=stackoverflow` | GET | Stack Overflow 热门问题 | 直接请求 StackExchange API | JSON | 统一新闻对象 | 条件式 |
| `tenxunwang` | [`tenxunwang.py`](app/services/sites/tenxunwang.py) | `https://i.news.qq.com/gw/event/pc_hot_ranking_list?ids_hash=&offset=0&page_size=51&appver=15.5_qqnews_7.1.60&rank_id=hot` | GET | 腾讯新闻热榜 | 直接请求腾讯新闻接口 | JSON | 统一新闻对象 | 条件式 |
| `tieba` | [`tieba.py`](app/services/sites/tieba.py) | `http://tieba.baidu.com/hottopic/browse/topicList` | GET | 百度贴吧热门话题 | 直接请求贴吧接口 | JSON | 统一新闻对象 | 条件式 |
| `weibo` | [`weibo.py`](app/services/sites/weibo.py) | `https://weibo.com/ajax/side/hotSearch` | GET | 微博热搜 | 直接请求微博 Ajax 接口 | JSON | 统一新闻对象 | 条件式 |
| `zhihu` | [`zhihu.py`](app/services/sites/zhihu.py) | `https://www.zhihu.com/api/v3/explore/guest/feeds?limit=30&ws_qiangzhisafe=0` | GET | 知乎热点内容流 | 直接请求知乎 API | JSON | 统一新闻对象 | 条件式 |
| `sina_finance` | [`sina_finance.py`](app/services/sites/sina_finance.py) | `https://zhibo.sina.com.cn/api/zhibo/feed?page=1&page_size=20&zhibo_id=152&tag_id=0&dire=f&dpc=1&pagesize=20` | GET | 新浪财经直播/快讯 | 直接请求新浪财经 API | JSON | 统一新闻对象 | 条件式 |
| `xueqiu` | [`xueqiu.py`](app/services/sites/xueqiu.py) | `https://xueqiu.com/hot_event/list.json?count=10` | GET | 雪球热门事件 | 先访问主页/热榜页拿 Cookie，再请求 JSON 接口 | JSON | 统一新闻对象 | 条件式（依赖 Cookie 预热） |
| `36kr` | [`tskr.py`](app/services/sites/tskr.py) | `https://gateway.36kr.com/api/mis/nav/home/nav/rank/hot` | GET | 36Kr 热榜 | 直接请求 36Kr 接口 | JSON | 统一新闻对象 | 条件式 |

### 3.3 页面抓取 / HTML 解析型数据源

| 平台代码 | 爬虫文件 | 页面地址 | 抓取方式 | 信息类型 | 内容来源方式 | 上游格式 | 项目内转换后格式 | 是否当前可直接使用 |
|---|---|---|---|---|---|---|---|---|
| `52pojie` | [`ftpojie.py`](app/services/sites/ftpojie.py) | `https://www.52pojie.cn/forum.php?mod=guide&view=hot` | GET + HTML 解析 | 吾爱破解热门帖子 | 请求网页后用 BeautifulSoup 解析 | HTML | 统一新闻对象 | 条件式 |
| `douban` | [`douban.py`](app/services/sites/douban.py) | `https://www.douban.com/group/explore` | GET + HTML 解析 | 豆瓣小组/探索内容 | 请求网页后解析 DOM | HTML | 统一新闻对象 | 条件式 |
| `hupu` | [`hupu.py`](app/services/sites/hupu.py) | `https://bbs.hupu.com/all-gambia` | GET + HTML 解析 | 虎扑步行街/热门帖 | 请求网页后解析 DOM | HTML | 统一新闻对象 | 条件式 |
| `v2ex` | [`vtex.py`](app/services/sites/vtex.py) | `https://www.v2ex.com/?tab=hot` | GET + HTML 解析 | V2EX 热门主题 | 请求网页后解析 DOM | HTML | 统一新闻对象 | 条件式 |
| `hackernews` | [`hackernews.py`](app/services/sites/hackernews.py) | `https://news.ycombinator.com/` | GET + HTML 解析 | Hacker News 热门列表 | 优先 requests 抓首页后解析表格行 | HTML | 统一新闻对象（含来源站点、得分、作者、评论摘要） | 条件式 |

### 3.4 浏览器抓取 / Selenium 型数据源

| 平台代码 | 爬虫文件 | 页面地址 | 抓取方式 | 信息类型 | 内容来源方式 | 上游格式 | 项目内转换后格式 | 是否当前可直接使用 |
|---|---|---|---|---|---|---|---|---|
| `weixin` | [`weixin.py`](app/services/sites/weixin.py) | `https://k.weixin.qq.com/` | Selenium | 微信看一看热点 | 浏览器打开页面、切换“热点”Tab、抓取文章列表 | 动态网页 | 统一新闻对象 | 否（依赖本地浏览器环境；且当前工厂未注册） |
| `weixin` 备用 | [`weixin.py`](app/services/sites/weixin.py) | `https://weread.qq.com/web/category/all` | Selenium | 微信读书热门书籍/书评 | 浏览器打开页面，抓排行榜或书籍列表 | 动态网页 | 统一新闻对象 | 否（同上） |
| `douyin` 旧方案 | [`douyin.py`](app/services/sites/douyin.py) | `https://www.douyin.com/hot` | Selenium | 抖音热榜页面 | 浏览器抓取页面 DOM | 动态网页 | 统一新闻对象 | 否（当前默认未启用，`fetch()` 实际走 `fetch_v2()`） |
| `hackernews` 备用 | [`hackernews.py`](app/services/sites/hackernews.py) | `https://news.ycombinator.com/` | Selenium | Hacker News 热门列表 | requests 失败时回退浏览器抓取 | 动态/静态皆可 | 统一新闻对象 | 条件式（仅备用方案） |

### 3.5 额外的预热/辅助地址

| 平台 | 地址 | 用途 | 备注 |
|---|---|---|---|
| `xueqiu` | `https://xueqiu.com` | 先拿基础 Cookie | 见 [`main_url`](app/services/sites/xueqiu.py:24) |
| `xueqiu` | `https://xueqiu.com/hot_event` | 热榜页预热 | 见 [`hot_page_url`](app/services/sites/xueqiu.py:48) |
| `baidu` | `https://top.baidu.com/board?tab=realtime` | 旧版页面抓取备用方案 | 见 [`fetch_v0()`](app/services/sites/baidu.py:57)；默认未使用 |

---

## 4. 哪些 API 是“项目自己的”，哪些只是“上游来源”

### 4.1 项目自己的 API

属于 [`hot_news`](.) 自己暴露的接口：

- [`/health`](app/main.py:86)
- [`/api/v1/dailynews/`](app/api/v1/daily_news.py:16)
- [`/api/v1/dailynews/all`](app/api/v1/daily_news.py:44)
- [`/api/v1/dailynews/multi`](app/api/v1/daily_news.py:79)
- [`/api/v1/dailynews/search`](app/api/v1/daily_news.py:134)
- [`/api/v1/tools/website-meta/`](app/api/v1/web_tools.py:19)
- [`/api/v1/analysis/*`](app/api/v1/analysis.py:14)

### 4.2 前端直接调用的第三方 API

属于 [`HotList-Web`](../HotList-Web) 直接请求的外部接口：

- `https://hot-api.vhan.eu.org/v2?type=all`
- `https://hot-api.vhan.eu.org/v2?type={item.key}`

### 4.3 上游抓取源

属于 [`hot_news`](.) 背后用来抓数据的站点 API / 页面，并不直接暴露给最终调用方。调用方最终看到的是 [`/api/v1/dailynews`](app/api/v1/daily_news.py:16) 返回的加工结果，而不是这些上游接口原始返回值。

---

## 5. 最后给你的直接结论

- 如果你想**直接拿数据用**，对当前环境来说最容易直接请求的是：
  - `https://hot-api.vhan.eu.org/v2?type=all`
  - `https://hot-api.vhan.eu.org/v2?type=<榜单类型>`
  - `https://orz.ai/api/v1/dailynews/?platform=<平台>`（这是已部署公网实例，不是当前本地仓库自动可用）
- 如果你想**看当前仓库自己设计出来的接口体系**，重点看 [`app/api/v1/daily_news.py`](app/api/v1/daily_news.py)、[`app/api/v1/web_tools.py`](app/api/v1/web_tools.py)、[`app/api/v1/analysis.py`](app/api/v1/analysis.py)
- 如果你想**追溯每个平台的真实上游来源**，重点看 [`app/services/sites/factory.py`](app/services/sites/factory.py) 和 [`app/services/sites`](app/services/sites) 目录下各平台爬虫文件
