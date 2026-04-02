# API 说明

## 核心 API

### [`ensureMetaDefaults()`](1/src/core/article.js:110)
补齐 article 的元数据默认值。

### [`normalizeParagraphs()`](1/src/core/article.js:34)
将字符串或数组归一为段落数组。

### [`attachMissingImagePlans()`](1/src/core/image-plan.js:71)
补齐封面图、头图与 section 图计划。

### [`renderArticle()`](1/src/core/render.js:278)
将 article JSON 渲染为 HTML 字符串。

### [`validateArticle()`](1/src/core/validate.js:18)
校验 article 数据与渲染结果。

返回结构：

```js
{
  ok: true,
  errors: [],
  warnings: []
}
```

### [`runPipeline()`](1/src/core/pipeline.js:11)
执行本地流程编排，返回 summary：

```js
{
  html: 'build/article.html',
  resolved_article: 'build/article.resolved.json',
  draft_media_id: null,
  publish_id: null,
  cover_media_id: null
}
```

## 适配器 API

### [`runCommand()`](1/src/adapters/subprocess.js:5)
执行外部命令并返回输出。

### [`generateImage()`](1/src/adapters/image-generator.js:3)
调用图片生成脚本。

### [`uploadCover()`](1/src/adapters/wechat-publisher.js:21)
上传封面图并解析 `media_id`。

### [`uploadContentImage()`](1/src/adapters/wechat-publisher.js:28)
上传正文图并解析 URL。

### [`createDraft()`](1/src/adapters/wechat-publisher.js:35)
创建草稿并解析 draft media id。

### [`publishDraft()`](1/src/adapters/wechat-publisher.js:42)
发布草稿并解析 publish id。
