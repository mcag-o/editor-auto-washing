# 架构说明

## 分层结构

Node.js 迁移版位于 [`1/`](1/)，按以下层次组织：

- [`1/src/core/`](1/src/core/)：纯业务逻辑
- [`1/src/adapters/`](1/src/adapters/)：外部脚本适配器
- [`1/src/cli/index.js`](1/src/cli/index.js:1)：命令行入口
- [`1/templates/`](1/templates/)：模板资源
- [`1/test/`](1/test/)：测试

## 核心模块

### [`1/src/core/article.js`](1/src/core/article.js)

负责：
- 元数据默认值补齐
- 段落归一化
- 新闻与来源计数
- 文本转义

### [`1/src/core/image-plan.js`](1/src/core/image-plan.js)

负责：
- 自动补齐封面图和正文图计划
- 生成 prompt、caption 与 local path

### [`1/src/core/render.js`](1/src/core/render.js)

负责：
- 模板读取
- 占位符替换
- headline 渲染
- section 与 block 渲染
- 模板风格分支处理

### [`1/src/core/validate.js`](1/src/core/validate.js)

负责：
- article JSON 结构校验
- HTML 未解析占位符校验
- 微信图片 URL 预检查
- warnings / errors 输出

### [`1/src/core/pipeline.js`](1/src/core/pipeline.js)

负责：
- 本地 dry-run 流程编排
- 可选图片生成与微信上传对接
- HTML 和 resolved article 输出

## 适配器模块

### [`1/src/adapters/subprocess.js`](1/src/adapters/subprocess.js)
统一封装子进程执行。

### [`1/src/adapters/image-generator.js`](1/src/adapters/image-generator.js)
封装外部图片生成脚本调用。

### [`1/src/adapters/wechat-publisher.js`](1/src/adapters/wechat-publisher.js)
封装封面上传、正文图上传、草稿创建与发布动作。

## 数据流

1. 读取 article JSON
2. 规划图片信息
3. 渲染 HTML
4. 校验 article 与 HTML
5. 可选调用图片生成脚本
6. 可选调用微信发布脚本
7. 输出 HTML 与 resolved article
