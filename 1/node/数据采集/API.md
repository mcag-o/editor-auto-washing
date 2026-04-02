# API

## Node 采集系统接口

`数据采集/` 当前提供的是命令行与编程调用接口，不直接暴露 HTTP 服务。

### CLI 入口

文件：`数据采集/src/cli/run.js`

#### 单平台采集

```bash
node 数据采集/src/cli/run.js --platform baidu
```

#### 多平台采集

```bash
node 数据采集/src/cli/run.js --platforms baidu,weibo,zhihu
```

#### 采集全部平台

```bash
node 数据采集/src/cli/run.js --all
```

### 编程调用入口

文件：`数据采集/src/index.js`

导出内容：

- `loadEnv`
- `env`
- `createPlatformRegistry`
- `collectPlatform`
- `collectMany`

调用示例：

```js
import { env, createPlatformRegistry, collectMany } from './数据采集/src/index.js';

const registry = createPlatformRegistry(env);
const result = await collectMany(['baidu', 'hackernews'], { registry, env });
console.log(result);
```

## 统一返回字段字典

### 聚合结果字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `requestedPlatforms` | `string[]` | 原始请求平台列表 |
| `resolvedPlatforms` | `string[]` | 解析别名后的平台列表 |
| `startedAt` | `string` | 开始时间，ISO 8601 |
| `finishedAt` | `string` | 结束时间，ISO 8601 |
| `successCount` | `number` | 成功的平台数 |
| `failureCount` | `number` | 失败的平台数 |
| `results` | `object[]` | 每个平台的标准化结果 |

### 单平台结果字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `platform` | `string` | 请求使用的平台名或别名 |
| `canonicalPlatform` | `string` | 规范平台 ID |
| `aliases` | `string[]` | 允许的兼容别名 |
| `displayName` | `string` | 平台显示名 |
| `sourceType` | `json-api \| html \| browser` | 数据来源类型 |
| `sourceUrl` | `string` | 上游地址 |
| `fetchedAt` | `string` | 抓取时间 |
| `success` | `boolean` | 是否成功 |
| `items` | `object[]` | 热榜条目列表 |
| `warnings` | `string[]` | 警告信息 |
| `error` | `object?` | 失败时的错误对象 |

### 条目字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | `string` | 规范化条目标识 |
| `rank` | `number` | 排名 |
| `title` | `string` | 标题 |
| `url` | `string` | 主跳转链接 |
| `mobileUrl` | `string \| null` | 移动端链接 |
| `hot` | `string \| null` | 热度值 |
| `summary` | `string \| null` | 摘要 |
| `author` | `string \| null` | 作者/发布者 |
| `category` | `string \| null` | 分类 |
| `tags` | `string[]` | 标签 |
| `publishTime` | `string \| null` | 原始发布时间 |
| `metadata` | `object` | 补充元数据 |
| `raw` | `any` | 原始上游记录 |

## 平台 API 定义

### `baidu`

端点：`https://top.baidu.com/api/board?platform=wise&tab=realtime`
方法：`GET`
鉴权：无
参数：`platform=wise`、`tab=realtime`
响应字段：`data.cards[0].content[0].content[]`，包含 `word`、`url`、`desc`、`hotScore`
可用性：可用
调用范例：`node 数据采集/src/cli/run.js --platform baidu`

### `shaoshupai` / `sspai`

端点：`https://sspai.com/api/v1/article/index/page/get?limit=20&offset=0&created_at=0`
方法：`GET`
鉴权：无
参数：`limit`、`offset`、`created_at`
响应字段：`data[]`，包含 `id`、`title`、`summary`
可用性：可用
调用范例：`node 数据采集/src/cli/run.js --platform sspai`

### `weibo`

端点：`https://weibo.com/ajax/side/hotSearch`
方法：`GET`
鉴权：建议 Cookie
参数：无
响应字段：`data.realtime[]`，包含 `word`、`raw_hot`
可用性：条件可用
调用范例：`WEIBO_COOKIE='...' node 数据采集/src/cli/run.js --platform weibo`

### `zhihu`

端点：`https://www.zhihu.com/api/v3/explore/guest/feeds?limit=30&ws_qiangzhisafe=0`
方法：`GET`
鉴权：无
参数：`limit`、`ws_qiangzhisafe`
响应字段：`data[]`，深层字段 `target.question.title`、`target.question.id`、`target.excerpt`
可用性：可用
调用范例：`node 数据采集/src/cli/run.js --platform zhihu`

### `36kr` / `tskr`

端点：`https://gateway.36kr.com/api/mis/nav/home/nav/rank/hot`
方法：`POST`
鉴权：无
参数：JSON body，包含 `partner_id`、`param.siteId`、`param.platformId`、`timestamp`
响应字段：`data.hotRankList[]`
可用性：条件可用
调用范例：`node 数据采集/src/cli/run.js --platform 36kr`

### `52pojie` / `ftpojie`

端点：`https://www.52pojie.cn/forum.php?mod=guide&view=hot`
方法：`GET`
鉴权：无
参数：`mod=guide`、`view=hot`
响应字段：HTML DOM 节点 `tbody[id^="normalthread_"]`
可用性：条件可用
调用范例：`node 数据采集/src/cli/run.js --platform ftpojie`

### `bilibili`

端点：`https://api.bilibili.com/x/web-interface/popular`
方法：`GET`
鉴权：无
参数：无
响应字段：`data.list[]`，包含 `title`、`bvid`、`desc`、`owner`、`stat`
可用性：可用
调用范例：`node 数据采集/src/cli/run.js --platform bilibili`

### `douban`

端点：`https://www.douban.com/group/explore`
方法：`GET`
鉴权：无
参数：无
响应字段：HTML DOM 节点 `div.channel-item`
可用性：条件可用
调用范例：`node 数据采集/src/cli/run.js --platform douban`

### `hupu`

端点：`https://bbs.hupu.com/all-gambia`
方法：`GET`
鉴权：无
参数：无
响应字段：HTML DOM 节点 `div.t-info`
可用性：条件可用
调用范例：`node 数据采集/src/cli/run.js --platform hupu`

### `tieba`

端点：`http://tieba.baidu.com/hottopic/browse/topicList`
方法：`GET`
鉴权：无
参数：无
响应字段：`data.bang_topic.topic_list[]`
可用性：可用
调用范例：`node 数据采集/src/cli/run.js --platform tieba`

### `juejin`

端点：`https://api.juejin.cn/content_api/v1/content/article_rank?category_id=1&type=hot`
方法：`GET`
鉴权：无
参数：`category_id`、`type`
响应字段：`data[]`，深层字段 `content.title`、`content.content_id`
可用性：可用
调用范例：`node 数据采集/src/cli/run.js --platform juejin`

### `douyin`

端点：`https://www.douyin.com/aweme/v1/web/hot/search/list/`
方法：`GET`
鉴权：无
参数：可扩展 Web API query
响应字段：`data.word_list[]`，字段包含 `word`、`sentence_id`、`hot_value`
可用性：条件可用
调用范例：`node 数据采集/src/cli/run.js --platform douyin`

### `v2ex` / `vtex`

端点：`https://www.v2ex.com/?tab=hot`
方法：`GET`
鉴权：无
参数：`tab=hot`
响应字段：HTML DOM 节点 `div.cell.item`
可用性：可用
调用范例：`node 数据采集/src/cli/run.js --platform vtex`

### `jinritoutiao`

端点：`https://www.toutiao.com/hot-event/hot-board/?origin=toutiao_pc`
方法：`GET`
鉴权：无
参数：`origin=toutiao_pc`
响应字段：`data[]`，字段存在大小写漂移
可用性：条件可用
调用范例：`node 数据采集/src/cli/run.js --platform jinritoutiao`

### `tenxunwang`

端点：`https://i.news.qq.com/gw/event/pc_hot_ranking_list?ids_hash=&offset=0&page_size=51&appver=15.5_qqnews_7.1.60&rank_id=hot`
方法：`GET`
鉴权：无
参数：`offset`、`page_size`、`rank_id`
响应字段：`idlist[0].newslist` 或 `data.newslist`
可用性：条件可用
调用范例：`node 数据采集/src/cli/run.js --platform tenxunwang`

### `stackoverflow`

端点：`https://api.stackexchange.com/2.3/questions?order=desc&sort=hot&site=stackoverflow`
方法：`GET`
鉴权：无
参数：`order`、`sort`、`site`
响应字段：`items[]`
可用性：可用
调用范例：`node 数据采集/src/cli/run.js --platform stackoverflow`

### `github`

端点：`https://api.github.com/search/repositories?q=stars:%3E1&sort=stars`
方法：`GET`
鉴权：无，可扩展 token
参数：`q`、`sort`
响应字段：`items[]`
可用性：可用
调用范例：`node 数据采集/src/cli/run.js --platform github`

### `hackernews`

端点：`https://news.ycombinator.com/`
方法：`GET`
鉴权：无
参数：无
响应字段：HTML DOM 节点 `tr.athing`
可用性：可用
调用范例：`node 数据采集/src/cli/run.js --platform hackernews`

### `sina_finance`

端点：`https://zhibo.sina.com.cn/api/zhibo/feed?page=1&page_size=20&zhibo_id=152&tag_id=0&dire=f&dpc=1&pagesize=20`
方法：`GET`
鉴权：无
参数：`page`、`page_size`、`zhibo_id`
响应字段：`result.data.feed.list` 或 `data.feed.list`
可用性：条件可用
调用范例：`node 数据采集/src/cli/run.js --platform sina_finance`

### `eastmoney`

端点：`https://np-weblist.eastmoney.com/comm/web/getFastNewsList`
方法：`GET`
鉴权：无
参数：依上游策略可扩展
响应字段：`data.fastNewsList` 或 `data.list`
可用性：条件可用
调用范例：`node 数据采集/src/cli/run.js --platform eastmoney`

### `xueqiu`

端点：`https://xueqiu.com/hot_event/list.json?count=10`
方法：`GET`
鉴权：建议 Cookie
参数：`count`
响应字段：`list[]`
可用性：条件可用
调用范例：`XUEQIU_COOKIE='...' node 数据采集/src/cli/run.js --platform xueqiu`

### `cls`

端点：`https://www.cls.cn/featured/v1/column/list`
方法：`GET`
鉴权：无
参数：建议带官网风格 query
响应字段：`data.column_list[]`
可用性：条件可用
调用范例：`node 数据采集/src/cli/run.js --platform cls`
