# ArticleWashing-Go

> `ArticleWashing` 的 Go 主实现，当前已经覆盖工作区配置、bundle 导入、结构化排版、审核发布、作业与自动化主链路。

---

## 项目定位

`ArticleWashing-Go` 不是 Python 版的逐文件翻译，而是面向当前主链路的 Go 实现。

当前已经完成的替代范围：

- workspace 驱动配置
- `DataCollection` bundle ingestion
- article workspace 生命周期
- 结构化 draft / render / validate / asset persistence
- review / publish gate / publish history
- workflow / jobs / automation
- HTTP API / CLI / 基础 TUI

这意味着：在“功能等价替代 + 覆盖实际可用主链路”的标准下，`ArticleWashing-Go` 已可以替代 `ArticleWashing`。

不包含的承诺：

- 不保证 100% 复刻 Python 历史兼容层
- 不保证所有旧接口路径、旧命名、旧文档描述完全不变
- 不把当前 TUI 视为全部运维能力入口

---

## 当前能力

### 1. 工作区与配置

- workspace 初始化、加载、解析、校验、doctor 检查
- provider/article/publish profile
- secret 引用与环境变量解析
- incoming/processed/failed/rendered 等路径解析

相关代码：

- `ArticleWashing-Go/service/workspace_config.go`
- `ArticleWashing-Go/infra/workspace/loader.go`
- `ArticleWashing-Go/infra/workspace/validator.go`

### 2. Ingestion 与文章工作区

- 导入 `DataCollection` 输出的 bundle 文件
- 处理 `incoming / processed / failed`
- 持久化 ingestion record
- 持久化 article workspace record 与状态流转
- 支持 retry-failed

相关代码：

- `ArticleWashing-Go/service/ingestion_pipeline.go`
- `ArticleWashing-Go/infra/filesystem/bundle_router.go`
- `ArticleWashing-Go/infra/sqlite/ingestion_repo.go`
- `ArticleWashing-Go/infra/sqlite/article_workspace_repo.go`

### 3. 结构化排版

- draft 创建与读取
- 模板目录加载
- WeChat HTML 渲染
- draft 校验与 rendered output 校验
- rendered asset 持久化与读取

相关代码：

- `ArticleWashing-Go/service/formatting_pipeline.go`
- `ArticleWashing-Go/infra/formatter/wechat_html.go`
- `ArticleWashing-Go/infra/formatter/template_catalog.go`
- `ArticleWashing-Go/infra/sqlite/formatting_repo.go`

### 4. 审核与发布

- review create / list / approve / reject
- 未审核禁止发布
- asset 非 `ready` 禁止发布
- publish outcome 与 publish history 持久化
- workspace 生命周期与 review/publish 同步

相关代码：

- `ArticleWashing-Go/service/review.go`
- `ArticleWashing-Go/service/publish_gate.go`
- `ArticleWashing-Go/infra/sqlite/review_repo.go`
- `ArticleWashing-Go/infra/sqlite/runtime_repo.go`

### 5. Workflow / Jobs / Automation

- concrete workflow nodes 已注册
- job queue / worker / cancel / event history
- automation `run-once / daemon / retry-failed / status / health / stop`
- automation snapshot 持久化

相关代码：

- `ArticleWashing-Go/service/workflow.go`
- `ArticleWashing-Go/service/workflow_nodes.go`
- `ArticleWashing-Go/service/job.go`
- `ArticleWashing-Go/service/automation.go`

---

## 运行方式

### 前置条件

- Go `>= 1.25`
- CGO 可用
- 本机有 SQLite 编译链（例如 `gcc`）

### 安装依赖

```bash
cd ArticleWashing-Go
go mod download
```

### 启动服务

```bash
go run ./cmd/server
```

默认监听后可检查：

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

### 启动 TUI

```bash
go run ./cmd/tui --api http://localhost:8080
```

说明：当前 TUI 是基础监控/浏览入口，不覆盖全部 automation 控制能力。

---

## CLI

CLI 入口：`ArticleWashing-Go/cmd/cli/main.go`

### Workspace

```bash
go run ./cmd/cli workspace init --root .
go run ./cmd/cli workspace show-config --root .
go run ./cmd/cli workspace resolve-config --root .
go run ./cmd/cli workspace doctor --root .
```

### Ingestion

```bash
go run ./cmd/cli ingestion import --root .
go run ./cmd/cli ingestion retry-failed --root .
```

### Formatting

```bash
go run ./cmd/cli formatting render <draft-id> --platform wechat --template daily-intelligence --root .
go run ./cmd/cli formatting validate <draft-id> --platform wechat --template daily-intelligence --root .
```

### Review / Publish

```bash
go run ./cmd/cli review approve <review-id> --reviewer alice --notes ok --root .
go run ./cmd/cli review reject <review-id> --reviewer alice --notes retry --root .
go run ./cmd/cli publish run <review-id> --root .
go run ./cmd/cli publish history <article-id> --root .
```

### Automation

```bash
go run ./cmd/cli automation run-once --root .
go run ./cmd/cli automation daemon --root .
go run ./cmd/cli automation retry-failed --root .
go run ./cmd/cli automation status --root .
go run ./cmd/cli automation health --root .
go run ./cmd/cli automation stop --root .
```

说明：CLI `automation daemon` 是前台阻塞模式；HTTP 的 `daemon` 是进程内异步启动模式。这是有意保持的“真实语义”。

---

## HTTP API

核心路由定义在：`ArticleWashing-Go/transport/http/server.go`

### 基础

- `GET /health`
- `GET /ready`
- `GET /config`

### Content / Templates / Drafts / Assets

- `GET|POST|PUT|DELETE /content`
- `GET|POST /templates`
- `GET /templates/categories`
- `POST /drafts`
- `GET /drafts/:id`
- `POST /drafts/:id/render`
- `POST /drafts/:id/validate`
- `GET /assets/:id`

### Ingestion / Workspace

- `POST /ingestion/import`
- `POST /ingestion/retry-failed`
- `GET /ingestion`
- `GET /ingestion/:id`
- `GET /workspace/articles`

### Review / Publish

- `POST /reviews`
- `GET /reviews`
- `POST /reviews/:id/approve`
- `POST /reviews/:id/reject`
- `POST /publish`
- `GET /publish/history`

### Jobs / Workflows / Automation

- `POST /jobs`
- `GET /jobs`
- `GET /jobs/:id`
- `POST /jobs/:id/cancel`
- `GET /jobs/:id/events`
- `POST /workflows/execute`
- `POST /automation/run-once`
- `POST /automation/daemon`
- `POST /automation/retry-failed`
- `GET /automation/status`
- `GET /automation/health`
- `POST /automation/stop`

---

## 存储与运行时

当前默认主路径是 SQLite，而不是纯内存实现。

- SQLite provider：`ArticleWashing-Go/infra/sqlite/provider.go`
- runtime repos：`ArticleWashing-Go/service/runtime_repos.go`
- migrations：`ArticleWashing-Go/infra/sqlite/migrations/`

`infra/memory/` 仍然保留，用于测试和内存替身场景。

---

## 验证状态

当前仓库已经在 Go 主项目上通过完整测试：

```bash
go test ./...
```

覆盖范围包括：

- workspace/config
- ingestion/article workspace
- formatting/render/validate/assets
- review/publish
- workflow/jobs/automation
- HTTP handlers
- CLI
- integration mainline
- TUI 基础行为

---

## 当前限制与边界

- Go 版已经可以替代 Python 主链路，但不是历史兼容层逐字复刻
- automation daemon 目前是单进程内模型，不是外部 supervisor 模型
- TUI 范围有意收敛，不覆盖全部 automation 管理面
- publish provider 当前仍以现有 provider boundary 为主，外部平台能力是否完整取决于具体 provider 实现

---

## 目录概览

```text
ArticleWashing-Go/
├── cmd/
│   ├── cli/
│   ├── server/
│   └── tui/
├── domain/
├── infra/
│   ├── config/
│   ├── filesystem/
│   ├── formatter/
│   ├── memory/
│   ├── sqlite/
│   └── workspace/
├── integration/
├── pkg/repo/
├── service/
├── testdata/
└── transport/
    ├── http/
    └── tui/
```

---

## 当前结论

`ArticleWashing-Go` 现在的准确表述是：

- 已完成对 `ArticleWashing` 的主链路功能等价替代
- 可以作为当前默认主实现使用
- 文档、运维切换和最终远端发布是剩余的主要收尾事项
