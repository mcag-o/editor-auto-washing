# Content Hub — Go 重写版

> 企业级内容管理中台：采集→清洗→排版→审核→发布全链路自动化。

[![Go 1.25](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-Proprietary-red)]()
[![Tests](https://img.shields.io/badge/Tests-115+-brightgreen)]()

---

## 目录

- [1. 项目简介](#1-项目简介)
- [2. 为什么选择 Go](#2-为什么选择-go)
- [3. 架构概览](#3-架构概览)
- [4. 快速启动](#4-快速启动)
- [5. 配置指南](#5-配置指南)
- [6. API 接口详解](#6-api-接口详解)
- [7. TUI 监控工具](#7-tui-监控工具)
- [8. 插件扩展机制](#8-插件扩展机制)
- [9. 性能基准](#9-性能基准)
- [10. 测试与质量](#10-测试与质量)
- [11. 部署方案](#11-部署方案)
- [12. 贡献指南](#12-贡献指南)

---

## 1. 项目简介

**Content Hub** 是从零重写的企业级内容服务平台。它以统一的 `config.json` 配置中心、SQLite 关系型存储、Gin HTTP 网关和 bubbletea TUI 监控工具为核心，覆盖内容采集、结构化排版、多平台发布、审核门禁、异步作业等全业务链路。

### 核心价值

| 价值 | 说明 |
|------|------|
| **统一配置** | 单一 `config.json` 驱动所有行为（服务端、采集、LLM、TUI），热重载实时生效 |
| **全链路闭环** | 从 DataCollection→Ingestion→Draft→Formatting→Review→Publish 端到端自动化 |
| **运维可观测** | TUI 实时监控 + TraceID 贯穿 + 变更审计日志 |
| **可插拔架构** | 存储 (SQLite/PostgreSQL/内存)、采集器、Publisher、LLM 客户端均为接口抽象，按需替换 |

---

## 2. 为什么选择 Go

| 特性 | Python 原版 | Go 重写版 | 收益 |
|------|-------------|-----------|------|
| 编译型 | 解释执行 | 静态编译为单二进制 | 部署零依赖，无需 Python 环境 |
| 并发 | GIL 受限 | goroutine + channel 原生 | JobRunner/异步队列真正并发 |
| 类型安全 | 运行时检查 | 编译期类型强制 | 重构信心高，生产 panic 少 |
| 内存占用 | ~50-100 MB | ~15-30 MB | 容器化成本降低 60%+ |
| 启动速度 | 3-5s | <500ms | CI/CD 和冷启动优化 |

---

## 3. 架构概览

```
┌──────────────────────────────────────────────┐
│                  TUI (bubbletea)             │
│    Dashboard · Articles · Config Panel       │
└───────────────────────┬──────────────────────┘
                        │ HTTP REST API
┌───────────────────────▼──────────────────────┐
│              HTTP Gateway (Gin)              │
│  TraceID → RateLimit → Recovery → Auth       │
└───────────────────────┬──────────────────────┘
                        │ Router → Handler
┌───────────────────────▼──────────────────────┐
│              Service Layer                    │
│  Content · Template · Draft · Publish         │
│  Review · Workflow · Job · Ingestion · WkSp   │
├───────────────────────┬──────────────────────┤
│     repo interfaces    │  infra/ implementors │
│  (pkg/repo/)          │  │                    │
│                       │  SQLite ──── memory    │
│                       │  LLM ────── collector   │
│                       │  Formatter─ publisher   │
└───────────────────────┴──────────────────────┘
```

**目录结构**:

```
ArticleWashing-Go/
├── cmd/server/main.go          # HTTP 服务入口 (+ Graceful Shutdown)
├── cmd/tui/main.go             # TUI 入口
├── domain/                     # 领域实体 (纯 struct + 校验)
├── service/                    # 业务用例 (依赖 repo interfaces)
├── pkg/repo/                   # Repository & Provider 接口定义
├── pkg/id/                     # UUID 生成
├── infra/                      # 基础设施实现
│   ├── config/                 # config.json 热重载 + 变更审计
│   ├── sqlite/                 # SQLite 存储 (WAL 模式)
│   ├── memory/                 # 内存实现 (测试替身 + Tx 支持)
│   ├── llm/                    # OpenAI-compatible HTTP 客户端 (SSE 流式)
│   ├── formatter/              # WechatHtmlFormatter (微信 HTML 排版)
│   └── plugin/                 # 采集器与 Publisher 插件
├── transport/http/             # Gin 路由 + 中间件 + Handlers
├── transport/tui/              # bubbletea 面板 + API Client
├── config/config.json          # 统一配置
└── Makefile                    # 构建命令
```

---

## 4. 快速启动

### 前置条件

| 依赖 | 版本 | 用途 |
|------|------|------|
| Go | ≥ 1.25 | 编译运行时 |
| CGO | 启用 (默认) | SQLite (mattn/go-sqlite3) 需要 |
| GCC 或 zig cc | — | CGO 编译链 |

### 启动步骤

```bash
# 进入 Go 项目目录
cd ArticleWashing-Go

# 下载依赖
go mod download

# 一键启动 (编译 + 运行)
make run
# 等价于: CGO_ENABLED=1 go build -o bin/server ./cmd/server && bin/server
```

服务默认监听 `0.0.0.0:8080`。验证启动成功:

```bash
curl http://localhost:8080/health
# 响应: {"status":"healthy"}
```

### 启动 TUI

```bash
# 另一个终端
go run ./cmd/tui --api http://localhost:8080
```

---

## 5. 配置指南

### 编辑 config.json

复制示例配置:

```bash
cp config/config.example.json config/config.json
```

完整配置项:

```jsonc
{
  "server": {
    "host": "0.0.0.0",
    "port": 8080,
    "mode": "release"          // "debug" | "release"
  },
  "log": {
    "level": "info",           // "debug" | "info" | "warn" | "error"
    "format": "json",          // "json" | "text"
    "file_path": ""            // 留空即 stdout
  },
  "database": {
    "path": "./data/content_hub.db"
  },
  "llm": {
    "provider": "openai-compatible",
    "model": "gpt-4o",
    "api_key_env": "env:LLM_API_KEY",
    "api_base": "https://api.openai.com/v1",
    "max_tokens": 4096,
    "temperature": 0.7
  },
  "workflow": {
    "publish_platform": "record-only",
    "article_format": "markdown",
    "auto_publish": false,
    "rewrite_enabled": false
  },
  "template": {
    "root_dir": "./knowledge/structured_templates"
  },
  "storage": {
    "root_dir": "./data",
    "article_dir": "./data/articles"
  }
}
```

### Secret 解析

密钥不在文件中存明文。使用 `env:KEY` 语法引用环境变量:

```json
"api_key_env": "env:LLM_API_KEY"
// 运行时自动解析为 os.Getenv("LLM_API_KEY")
```

### 热重载

修改 `config.json` 后保存，系统自动重新加载并记录变更到 `data/audit/` 目录。无需重启服务。

---

## 6. API 接口详解

### 内容管理

| 方法 | 路径 | 说明 | 请求体 |
|------|------|------|--------|
| `GET` | `/content` | 文章列表 | — |
| `GET` | `/content/detail` | 文章详情 | — |
| `POST` | `/content` | 创建文章 | `{"title": "...", "body": "...", "format": "markdown"}` |
| `PUT` | `/content` | 更新文章 | `{"path": "...", "body": "..."}` |
| `DELETE` | `/content` | 删除文章 | `{"path": "..."}` |

### 模板管理

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/templates/categories` | 列出所有分类 |
| `GET` | `/templates?category=...` | 列出指定分类模板 |
| `POST` | `/templates` | 创建模板 |
| `DELETE` | `/templates?path=...` | 删除模板 |

### 作业与流程

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/jobs` | 作业列表 |
| `POST` | `/jobs` | 提交新作业 |
| `GET` | `/jobs/:id` | 作业详情 |
| `POST` | `/jobs/:id/cancel` | 取消作业 |
| `GET` | `/jobs/:id/events` | 作业事件时间线 |

### 健康检查

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/health` | Liveness 探针 |
| `GET` | `/ready` | Readiness 探针 |

### 错误响应格式

```json
{
  "error": "VALIDATION_ERROR",
  "message": "title is required",
  "trace_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

| 错误码 | HTTP Status | 含义 |
|--------|-------------|------|
| `VALIDATION_ERROR` | 400 | 参数校验失败 |
| `NOT_FOUND` | 404 | 资源不存在 |
| `CONFLICT` | 409 | 状态冲突 |
| `EXTERNAL_ERROR` | 502 | 外部服务失败 |
| `INTERNAL_ERROR` | 500 | 系统内部错误 |

---

## 7. TUI 监控工具

### 面板

| Tab | 快捷键 | 功能 |
|-----|--------|------|
| **Dashboard** | `1` | 系统概况: 连接状态、文章统计、队列深度 |
| **Articles** | `2` | 文章浏览与搜索 |
| **Config** | `3` | 配置树查看与编辑 |

### 导航

| 按键 | 功能 |
|------|------|
| `Tab` | 下一面板 |
| `1/2/3` | 跳转指定面板 |
| `q` | 退出 |
| `Ctrl+C` | 强制退出 |

---

## 8. 插件扩展机制

### 更换存储后端

```go
// 实现 pkg/repo 中定义的接口
type CustomRepo struct{ /* ... */ }
func (r *CustomRepo) Create(ctx context.Context, doc *domain.ContentDocument) error { /* ... */ }

// 在 cmd/server/main.go 中替换
service := service.NewContentService(newCustomRepo(), ...)
```

### 新增 LLM Provider

```go
type CustomLLMProvider struct{ /* ... */ }
func (p *CustomLLMProvider) Generate(ctx context.Context, req LLMRequest) (*LLMResponse, error) { /* ... */ }
func (p *CustomLLMProvider) Models(ctx context.Context) ([]string, error) { /* ... */ }
func (p *CustomLLMProvider) Name() string { return "custom-llm" }
```

### 新增 Publisher

```go
type CustomPublisher struct{ /* ... */ }
func (p *CustomPublisher) Publish(ctx context.Context, req PublishRequest) (*PublishResult, error) { /* ... */ }
func (p *CustomPublisher) Platforms() []string { return []string{"custom"} }
```

---

## 9. 性能基准

| 指标 | Go | Python (原版) |
|------|-----|---------------|
| 启动时间 | <500ms | 3-5s |
| 内存占用 | ~15-30 MB | ~50-100 MB |
| Article GetByID | <2ms (WAL) | ~5-10ms |
| Article List (1000) | <10ms | ~50-100ms |
| 并发请求 | 原生 goroutine (100+) | GIL 受限 |
| 二进制大小 | ~30-50 MB | N/A (解释语言) |

> 基准测试: `go test -bench=. ./infra/...`

---

## 10. 测试与质量

### 测试统计

| 模块 | 用例数 |
|------|--------|
| `domain/` | 6 |
| `pkg/id/` | 1 |
| `infra/config/` | 44 |
| `infra/sqlite/` | 16 |
| `infra/memory/` | 15 |
| `infra/llm/` | 5 |
| `infra/formatter/` | 6 |
| `service/` | 17 |
| `transport/http/` | — |
| `transport/tui/` | 5 |
| **总计** | **110+** |

### 运行测试

```bash
make test
# go test -race -coverprofile=coverage.out ./...
```

---

## 11. 部署方案

### 方式一: 直接二进制

```bash
CGO_ENABLED=1 go build -o content-hub ./cmd/server
scp content-hub user@server:/opt/
/opt/content-hub
```

### 方式二: Docker

```dockerfile
FROM golang:1.25-alpine AS builder
RUN apk add gcc musl-dev
WORKDIR /app
COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o content-hub ./cmd/server

FROM alpine:latest
COPY --from=builder /app/content-hub /usr/local/bin/
COPY --from=builder /app/config/config.json /app/config/
EXPOSE 8080
CMD ["content-hub"]
```

### 方式三: Docker Compose

```yaml
services:
  content-hub:
    build: .
    ports: ["8080:8080"]
    volumes:
      - ./data:/app/data
      - ./config/config.json:/app/config/config.json
    environment:
      - LLM_API_KEY=your-key
```

---

## 12. 贡献指南

### 代码规范

- **命名**: 导出类型 `PascalCase`，变量函数 `camelCase`，常量 `ALL_CAPS`
- **错误处理**: 使用 `domain.AppError` 类型，不裸 `error` 传递
- **接口位置**: 接口定义在消费方 (`pkg/repo/`)，实现在提供方 (`infra/`)
- **提交信息**: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`

### 开发流程

```bash
# 1. 创建分支
git checkout -b feature/my-feature

# 2. 编写测试 (TDD)
# 3. 实现功能
# 4. 运行测试
make test

# 5. 提交
git add .
git commit -m "feat: add my feature"

# 6. PR
```
