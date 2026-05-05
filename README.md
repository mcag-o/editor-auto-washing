# content-hub

> 当前默认主实现（Go 版，仓库根目录），已覆盖 web control plane、浏览器上传/粘贴 intake、rewrite/draft/render、审核发布能力、作业与自动化主链路。

---

## 项目定位

仓库根目录下的 `content-hub` 不是 Python 版的逐文件翻译，而是面向当前主链路的 Go 实现。

当前已经完成的主链路替代范围：

- DB-backed business config / workspace runtime config
- browser upload / paste intake
- article workspace 生命周期
- 结构化 draft / render / validate / asset persistence
- review / publish gate / publish history
- workflow / jobs / automation
- Web control plane / HTTP API / CLI（开发调试）/ 基础 TUI

这意味着：在“功能等价替代 + 覆盖实际可用主链路”的标准下，仓库根目录下的 Go runtime 已可以作为当前默认主实现；当前默认操作入口是监听在 `8123` 的 web control plane。当前 active runtime 唯一文档化且默认的 intake 路径是浏览器中的 upload / paste workflow，业务配置以数据库中的 runtime state 为准，自动处理默认产物会停在 draft + render，review / publish 保持为后续可选人工步骤。旧的 RSS、collector、ingestion surface 不再属于 active runtime 当前能力。

不包含的承诺：

- 不保证 100% 复刻 Python 历史兼容层
- 不保证所有旧接口路径、旧命名、旧文档描述完全不变
- 不把当前 TUI 视为全部运维能力入口

---

## 当前能力

### 1. 工作区与配置

- workspace 初始化、加载、解析、校验、doctor 检查
- business config 通过数据库中的 runtime state 持久化并由 web control plane 管理
- provider/article/publish profile
- secret 引用与环境变量解析
- incoming/processed/failed/rendered 等路径解析

相关代码：

- `service/workspace_config.go`
- `infra/workspace/loader.go`
- `infra/workspace/validator.go`

### 2. Web Intake 与文章工作区

- 浏览器 upload / paste workflow 是当前 active runtime 默认 intake 主路径
- 上传或粘贴 source document 后会进入 intake、rewrite、draft materialize、render 组成的默认自动化处理链
- 默认自动化链在 render 结束；review / publish 保持可选，由后续人工或显式调用触发
- article workspace record 与状态流转仍由当前 Go runtime 持久化

相关代码：

- `service/folder_intake_runtime.go`
- `service/folder_intake_worker.go`
- `service/source_processing_scheduler.go`
- `service/source_processing_worker.go`
- `service/article_intake.go`
- `infra/sqlite/source_document_repo.go`
- `infra/sqlite/article_workspace_repo.go`

说明：旧的 RSS 与 collector / ingestion 相关实现仅作为迁移参考或历史上下文保留；当前文档中的唯一 active/default intake 叙述是 browser upload / paste workflow。

### 2.6. AI Rewrite Pipeline

- imported workspace article 可以在 draft 创建前进入独立 rewrite pipeline
- rewrite run 由 `target type + source profile + version` 选择对应 profile
- rewrite 执行会持久化 stage history、prompt snapshot 与最终 draft linkage
- rewrite 成功后由 materializer 创建 draft，默认自动化链随后执行 render 并结束

相关代码：

- `service/rewrite_orchestrator.go`
- `service/rewrite_stage_executor.go`
- `service/draft_materializer.go`
- `infra/llm/client.go`

### 3. 结构化排版

- draft 创建与读取
- 模板目录加载
- WeChat HTML 渲染
- draft 校验与 rendered output 校验
- rendered asset 持久化与读取

相关代码：

- `service/formatting_pipeline.go`
- `infra/formatter/wechat_html.go`
- `infra/formatter/template_catalog.go`
- `infra/sqlite/formatting_repo.go`

### 4. 审核与发布

- review create / list / approve / reject
- 未审核禁止发布
- asset 非 `ready` 禁止发布
- publish outcome 与 publish history 持久化
- workspace 生命周期与 review/publish 同步

相关代码：

- `service/review.go`
- `service/publish_gate.go`
- `infra/sqlite/review_repo.go`
- `infra/sqlite/runtime_repo.go`

### 5. Workflow / Jobs / Automation

- concrete workflow nodes 已注册
- job queue / worker / cancel / event history
- automation `run-once / daemon / retry-failed / status / health / stop`
- automation snapshot 持久化

相关代码：

- `service/workflow.go`
- `service/workflow_nodes.go`
- `service/job.go`
- `service/automation.go`

---

## 运行方式

### 前置条件

- Go `>= 1.25`
- CGO 可用
- 本机有 SQLite 编译链（例如 `gcc`）

### 安装依赖

```bash
go mod download
```

### 启动 Web Control Plane

```bash
go run ./cmd/server
```

默认监听 `http://localhost:8123`，浏览器操作以 web control plane 为主。业务配置在运行时由数据库持久化，日常 intake 通过浏览器上传或粘贴进入。启动后可检查：

```bash
curl http://localhost:8123/health
curl http://localhost:8123/ready
```

### 启动 TUI

```bash
go run ./cmd/tui --api http://localhost:8123
```

说明：当前 TUI 是基础监控/浏览入口，不覆盖全部 automation 控制能力。

---

## CLI

CLI 入口：`cmd/cli/main.go`

说明：CLI 不再是默认产品操作面，当前仅保留为开发/调试支持工具。日常操作以浏览器访问 `http://localhost:8123` 的 web control plane 为主。

### Workspace

```bash
go run ./cmd/cli workspace init --root .
go run ./cmd/cli workspace show-config --root .
go run ./cmd/cli workspace resolve-config --root .
go run ./cmd/cli workspace doctor --root .
```

### Intake / Automation Debug

```bash
go run ./cmd/cli automation run-once --root .
```

说明：当前运行时的唯一文档化 intake 路径是 web control plane 中的 browser upload / paste workflow；这里的 `automation` 命令仅用于开发/调试支持，不是默认操作面。默认自动处理结果停在 draft + render。review / publish 是后续可选人工步骤，不属于默认自动链路。旧的 RSS、`ingestion ...` 与 `collector ...` CLI 入口不再作为当前运行时文档化接口。

### Formatting

```bash
go run ./cmd/cli formatting render <draft-id> --platform wechat --template daily-intelligence --root .
go run ./cmd/cli formatting validate <draft-id> --platform wechat --template daily-intelligence --root .
```

### Rewrite

```bash
go run ./cmd/cli rewrite run <workspace-article-id> --target wechat-longform --source sspai --root .
```

说明：rewrite CLI 会读取 workspace article 元数据，并按 `target + source + version` 解析 rewrite profile；当前 CLI 不提供 `--version` 参数，版本选择走运行时默认值。默认 intake 路径仍是 browser upload / paste workflow，rewrite 编排衔接其导入结果继续执行。

### Review / Publish

```bash
go run ./cmd/cli review approve <review-id> --reviewer alice --notes ok --root .
go run ./cmd/cli review reject <review-id> --reviewer alice --notes retry --root .
go run ./cmd/cli publish run <review-id> --root .
go run ./cmd/cli publish history <article-id> --root .
```

说明：review / publish CLI 仍然保留用于开发/调试或手工补救，但它们不是默认操作路径，也不是默认自动化主链的一部分；默认自动处理结果停在 draft + render。

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

核心路由定义在：`transport/http/server.go`

说明：HTTP server 同时承载 web control plane 与 API，当前默认操作入口是根路径 `/` 提供的 browser UI，默认端口为 `8123`。

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

### Workspace

- `GET /workspace/articles`

### Rewrite

- `POST /rewrite/runs`

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

说明：browser upload / paste workflow 是当前 active runtime 唯一文档化且默认的 intake 路径；当前 HTTP server 通过 web control plane 承载该主操作面，并继续暴露 automation、rewrite、workspace article、review/publish 等可调用 API。review 与 publish API 仍可单独调用，但不会由默认自动化链自动进入。旧的 `/ingestion/*` 与 `/collector/*` 路径不再作为支持中的运行时入口。

---

## 存储与运行时

当前默认主路径是 SQLite，而不是纯内存实现；业务配置与运行时状态以数据库持久化结果为准。

- SQLite provider：`infra/sqlite/provider.go`
- runtime repos：`service/runtime_repos.go`
- migrations：`infra/sqlite/migrations/`

`infra/memory/` 仍然保留，用于测试和内存替身场景。

---

## 验证状态

当前仓库的最终验证矩阵如下：

```bash
go test ./service -run 'TestSource|TestFolder|TestRewrite|TestBuildWebControlRuntime'
go test ./transport/http/... -run 'TestAPI|TestAdminFrontend|TestRewrite'
go test ./integration -run 'TestWebControlPlanePasteToRenderedResult|TestRewritePipelineMainlineMaterializesDraft'
go test ./...
```

覆盖范围包括：

- workspace/config
- browser upload/paste intake
- rewrite orchestrator/stage execution/draft materialization
- article intake/workspace
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
- web control plane（`http://localhost:8123`）是当前默认主操作面；browser upload / paste workflow 是当前唯一文档化且默认的 intake 路径；CLI 仅作为开发/调试支持保留，对外可调用入口以该 browser UI 与现有 automation、rewrite、workspace/review/publish 接口为主
- 默认自动化结果停在 draft + render；review / publish 作为后续可选人工步骤保留
- 归档项目里的采集/ingestion 文档仍保留历史语义，不代表根目录 Go runtime 的当前对外接口
- automation daemon 目前是单进程内模型，不是外部 supervisor 模型
- TUI 范围有意收敛，不覆盖全部 automation 管理面
- publish provider 当前仍以现有 provider boundary 为主，外部平台能力是否完整取决于具体 provider 实现

---

## 目录概览

```text
├── Archive/
│   ├── ArticleWashing/
│   └── DataCollection/
├── cmd/
│   ├── cli/
│   ├── server/
│   └── tui/
├── collector/
│   ├── httpclient/
│   ├── plugin/
│   ├── runtime/
│   ├── scheduler/
│   └── service/
├── config/
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

当前仓库根目录 Go runtime 的准确表述是：

- 已完成对 `Archive/ArticleWashing/`（Python 版）的主链路功能等价替代
- 可以作为当前默认主实现使用
- 当前默认主操作面是 `8123` 上的 web control plane
- 根目录 Go runtime 当前以 browser upload / paste 作为默认 intake 主路径
- 业务配置以数据库中的 runtime state 为准
- 默认自动化链产物为 draft + render，review / publish 不会自动触发
- 旧的 RSS、collector、ingestion surface 已从 active runtime 对外接口集合中移除
