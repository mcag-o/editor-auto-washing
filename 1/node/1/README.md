# wechat-claw-node

Node.js 版本的微信公众号文章排版、校验、模板输出与流程编排工具。

## 功能

- 结构化文章 JSON 渲染为公众号 HTML
- 发布前结构校验
- 8 套模板完整迁移
- 本地 dry-run 流程编排
- 外部图片生成脚本与微信脚本适配
- 作为库或 CLI 使用

## 环境要求

- Node.js 18+

## 安装

在 [`1/package.json`](1/package.json) 所在目录安装依赖：

```bash
cd 1
npm install
```

## CLI 用法

### 渲染 HTML

```bash
node ./1/src/cli/index.js render ./1/examples/article.sample.json -o ./1/.tmp/article.html --check
```

### 校验文章

```bash
node ./1/src/cli/index.js validate ./1/examples/article.sample.json
```

### 运行本地流程

```bash
node ./1/src/cli/index.js pipeline ./1/examples/article.sample.json --output-dir ./1/.tmp/build --dry-run
```

## 目录说明

- [`1/src/core/`](1/src/core/)：核心业务逻辑
- [`1/src/adapters/`](1/src/adapters/)：外部脚本适配层
- [`1/src/cli/`](1/src/cli/)：CLI 入口
- [`1/templates/`](1/templates/)：模板资源
- [`1/examples/`](1/examples/)：示例输入
- [`1/test/`](1/test/)：测试

## 当前迁移状态

已完成：

- 模板迁移
- 元数据与公共逻辑迁移
- 图片规划逻辑迁移
- HTML 渲染逻辑迁移
- 校验逻辑迁移
- 外部脚本适配层
- 流程编排器
- CLI 基础命令
