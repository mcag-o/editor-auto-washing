# content-hub

> 当前项目是仓库根目录的 Go runtime，默认操作入口是 `http://localhost:8123` 上的中文 React + Vite web control plane。默认面向操作人员的链路是浏览器上传/粘贴文章，自动完成 intake、rewrite、draft materialize 与 render；review / publish 保留为后续人工步骤。

---

## 项目定位

- 活跃项目：仓库根目录 Go 模块 `content-hub`
- 默认操作面：中文 React + Vite web control plane，监听端口 `8123`
- 默认 intake：浏览器 upload / paste workflow
- 默认自动链路：`intake -> rewrite -> draft -> render`
- workflow/template 管理：同一浏览器界面中的真实 browser-backed 能力
- 业务配置：以 SQLite 中的 runtime state 为准，文件配置用于启动与 workspace 初始化
- 当前边界：review / publish 不自动进入默认链路，automation daemon 是单进程内模型

---

## 架构总览

```text
React/Vite 中文控制台
        |
        | /api/*
        v
Gin HTTP server
        |
        v
service 编排层
  |        |          |
intake   rewrite    workflow/job
  |        |          |
workspace repo   LLM + prompt/profile + quality gate
  |        |
draft materializer
  |
formatter/render
  |
SQLite repos + rendered files
```

主要分层：

- `cmd/server/`：服务启动入口，组装 runtime repos、services、workflow engine、job worker 和 HTTP server
- `domain/`：核心领域模型、状态机、校验和错误类型
- `service/`：业务编排层，覆盖 intake、rewrite、workflow、formatting、web control、review/publish、automation
- `infra/`：SQLite、workspace/config、formatter、LLM、memory provider、filesystem 等基础设施
- `transport/http/`：Gin server、handlers、middleware、React/legacy static serving
- `pkg/repo/`：repository/provider interfaces
- `integration/`：默认主链路与跨层集成测试
- `webapp/`：React + Vite + Material UI + React Flow 前端源码
- `web/`：嵌入式静态资源，包含当前 dist 与 legacy static fallback
- `testdata/`：测试夹具

---

## 核心业务链路

### 1. 浏览器导入

操作人员通过控制台上传 `.txt` / `.md` / `.json` 文件，或直接粘贴正文。浏览器入口由 Go runtime 承接并创建 `workspace article`，source type 为 `upload` 或 `paste`。

相关代码：

- `service/web_intake.go`
- `transport/http/handlers/api_intake.go`
- `webapp/src/features/intake/IntakePage.tsx`

### 2. Intake 与 workspace 状态流转

导入后的文章进入 workspace，默认 metadata 包含 target type、render platform、rewrite profile version、source profile 和 source body。系统启动处理时会扫描 imported 的 browser workspace articles。

相关代码：

- `service/article_intake.go`
- `service/web_control_runtime.go`
- `infra/sqlite/article_workspace_repo.go`

### 3. Rewrite pipeline

rewrite run 根据 `target type + profile + version` 解析 profile，按 stage 执行 prompt rendering、LLM 调用、structured decode 和 quality gate，并持久化 run/stage history、prompt snapshot 与 final output。

相关代码：

- `service/rewrite_runtime.go`
- `service/rewrite_orchestrator.go`
- `service/rewrite_stage_executor.go`
- `service/prompt_registry.go`
- `service/rewrite_profile_registry.go`
- `service/quality_gate.go`
- `infra/llm/client.go`
- `infra/llm/openai.go`

### 4. Draft 与 Render

rewrite final output materialize 成 draft，随后通过 formatter 渲染 WeChat HTML，生成 rendered asset，并将 workspace 状态推进到 rendered。

相关代码：

- `service/draft_materializer.go`
- `service/formatting_pipeline.go`
- `infra/formatter/wechat_html.go`
- `infra/sqlite/formatting_repo.go`

### 5. Workflow / Job / Automation

workflow runtime 支持 branch、pause/resume、human node、parallel、fan-in、subflow、loop 等控制流能力。job service 负责 workflow job 的队列、执行、取消与事件记录。automation 支持 `run-once`、`retry-failed` 和 `daemon` 命令路径。

相关代码：

- `service/workflow_runtime_kernel.go`
- `service/workflow_nodes.go`
- `service/workflow_template.go`
- `service/job.go`
- `service/automation.go`

---

## 前端控制台

前端位于 `webapp/`，技术栈：

- React 19
- TypeScript
- Vite 7
- Material UI 7
- Emotion
- React Flow
- Vitest + Testing Library

页面入口：

- `webapp/src/main.tsx`
- `webapp/src/App.tsx`
- `webapp/src/routes/AppRoutes.tsx`

当前页面：

- `overview`：运行总览、关键指标、近期事件
- `intake`：文件上传与正文粘贴
- `articles`：文章队列、stage 查看、retry/stop/resume/delete
- `control`：system start/pause/resume/status
- `workflows`：workflow template 列表、React Flow 编辑器、节点/边配置
- `templates`：输出模板管理
- `audit`：审计日志查询与详情
- `config`：浏览器配置管理

前端 API client 固定使用 `/api`：

- `webapp/src/lib/api/client.ts`
- `webapp/src/lib/api/types.ts`
- `webapp/src/lib/mappers/workflow.ts`
- `webapp/src/lib/mappers/template.ts`
- `webapp/src/lib/mappers/config.ts`

---

## HTTP API

核心路由定义在 `transport/http/server.go`。

### Health / Ready

- `GET /health`
- `GET /ready`

### Browser control plane API

- `POST /api/intake/upload`
- `POST /api/intake/paste`
- `GET /api/articles`
- `GET /api/articles/:id`
- `GET /api/articles/:id/stages`
- `POST /api/articles/:id/workflow-template`
- `POST /api/articles/:id/retry`
- `POST /api/articles/:id/stop`
- `POST /api/articles/:id/resume`
- `DELETE /api/articles/:id`
- `GET /api/config`
- `PUT /api/config`
- `POST /api/system/start`
- `POST /api/system/pause`
- `POST /api/system/resume`
- `GET /api/system/status`
- `GET /api/audit`
- `GET /api/audit/:id`
- `GET /api/workflow-runs/:id/audit`
- `POST /api/workflow-runs/:id/pause`
- `POST /api/workflow-runs/:id/resume`
- `GET|POST|PUT|DELETE /api/workflows`
- `GET|POST|PUT|DELETE /api/templates`

### Legacy / lower-level API

这些路由仍存在于 Go runtime，但不是当前文档化的默认 operator surface：

- `/content`
- `/templates`
- `/drafts`
- `/assets`
- `/automation`
- `/workspace/articles`
- `/jobs`
- `/reviews`
- `/publish`
- `/rewrite/runs`
- `/workflows/execute`

---

## 配置、存储与静态资源

### Runtime config

默认 standalone 配置：

- `config/config.json`

启动时优先解析 workspace runtime config，失败后 fallback 到 `config/config.json`。`CONTENT_HUB_WORKSPACE_ROOT` 可用于指定 workspace root。

### Workspace config

workspace loader 支持：

- `workspace.yaml`
- `secrets.yaml`
- `env.X` 环境变量 secret 引用
- workspace 路径解析与默认目录初始化

相关代码：

- `service/workspace_config.go`
- `infra/workspace/loader.go`
- `infra/workspace/validator.go`

### SQLite

默认持久化使用 SQLite：

- runtime repos：`service/runtime_repos.go`
- provider：`infra/sqlite/provider.go`
- migrations：`infra/sqlite/migrations/`

SQLite schema 覆盖 content、draft、asset、review、publish、job、ingestion、workspace、rewrite pipeline、web control plane、workflow template 与 workflow runtime。

### Frontend assets

Go server 优先服务嵌入式 React dist。若 dist 不可用，会 fallback 到 legacy static shell。

相关代码：

- `web/embed.go`
- `web/dist/`
- `web/static/`
- `transport/http/server.go`

---

## 运行方式

### 前置条件

- Go `>= 1.25`
- CGO 可用
- 本机具备 SQLite 编译链
- Node.js / npm，用于前端开发与构建

### 安装依赖

```bash
go mod download
npm --prefix webapp install
```

### 启动 Go runtime / Web Control Plane

```bash
go run ./cmd/server
```

默认访问：

```text
http://localhost:8123
```

健康检查：

```bash
curl http://localhost:8123/health
curl http://localhost:8123/ready
```

### 前端开发

```bash
npm --prefix webapp run dev
```

### 构建

```bash
make build
npm --prefix webapp run build
```

`npm --prefix webapp run build` 会将 Vite 产物输出到 `web/dist`，供 Go server 嵌入/服务。

---

## 验证命令

推荐从窄到宽验证：

```bash
go test ./service -run 'TestRewrite|TestBuildWebControlRuntime|TestWorkflowTemplate|TestTemplateDefinition|TestWebControlPlaneService'
go test ./transport/http/... -run 'TestAPI|TestAdminFrontend|TestRewrite'
npm --prefix webapp run test
npm --prefix webapp run build
go test ./integration -run 'TestWebControlPlanePasteToRenderedResult|TestWebControlPlaneUploadToRenderedResultWithWorkflowTemplate|TestReactControlPlanePasteToRenderedResultWithWorkflowTemplate|TestRewritePipelineMainlineMaterializesDraft'
go test ./...
```

Makefile：

```bash
make build
make run
make test
make clean
```

`make test` 会执行：

```bash
go test -race -coverprofile=coverage.out ./...
```

---

## 当前边界与注意事项

- browser upload / paste workflow 是当前唯一文档化、默认且面向操作人员的 intake 路径。
- React + Vite web control plane 是当前唯一文档化 operator surface。
- review / publish 需要人工触发，不属于默认自动链路。
- automation daemon 是单进程内模型。
- 默认 HTTP host 来自配置，可为 `0.0.0.0`；如暴露到非可信网络，需要额外网络隔离或鉴权策略。
- HTTP CORS 当前允许所有 origin，适合本地/受信环境，不应直接作为公网默认配置。
- `secrets.yaml`、`.env`、`workspace.yaml`、SQLite 数据库、runtime data、构建产物应保持本地忽略。
- 根目录存在的 n8n workflow JSON 可能包含 webhook/credential 元数据，提交或共享前应复核敏感信息。

---

## 当前文档

- `README.md`：当前项目手册与运行说明
- `version.md`：当前版本摘要
- `AGENTS.md`：面向 agent 的仓库协作规则
