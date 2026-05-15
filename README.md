# content-hub

> 当前运行时为仓库根目录的 Go 实现，默认操作入口是 `8123` 上的中文 React + Vite web control plane，覆盖浏览器上传/粘贴 intake、browser-backed workflow/template 管理，以及默认停在 draft + render 的自动处理链路。

---

## 项目定位

- 活跃项目：仓库根目录 Go 模块 `content-hub`
- 当前唯一文档化的操作面：`http://localhost:8123` 上的中文 React + Vite web control plane
- 当前唯一文档化、默认且面向操作人员的 intake 路径：浏览器中的 upload / paste workflow
- workflow/template 管理通过同一浏览器界面完成，前端基础设施以 Material UI 与 React Flow 为主
- 业务配置以数据库中的 runtime state 为准
- 默认自动处理结果停在 draft + render；review / publish 为后续可选人工步骤
- folder-intake 保留为后端/内部兼容能力，不作为默认操作入口
- 当前版本变更摘要见 `version.md`

---

## 当前能力

### 1. 工作区与配置

- workspace 初始化、加载、解析、校验、doctor 检查
- provider/article/publish profile 解析
- secret 引用与环境变量解析
- runtime state 持久化与浏览器配置管理

相关代码：

- `service/workspace_config.go`
- `infra/workspace/loader.go`
- `infra/workspace/validator.go`

### 2. Intake 与文章工作区

- 浏览器 upload / paste workflow 为默认 intake 主路径
- source document 导入后进入 intake、rewrite、draft materialize、render 组成的默认处理链
- article workspace record 与状态流转由 Go runtime 持久化
- workflow/template 管理通过 `8123` 上的 browser UI 完成

相关代码：

- `service/folder_intake_runtime.go`
- `service/source_processing_scheduler.go`
- `service/source_processing_worker.go`
- `service/article_intake.go`
- `infra/sqlite/source_document_repo.go`
- `infra/sqlite/article_workspace_repo.go`

### 3. Rewrite

- imported workspace article 可在 draft 创建前进入独立 rewrite pipeline
- rewrite run 按 `target type + source profile + version` 选择 profile
- rewrite 会持久化 stage history、prompt snapshot 与 draft linkage
- rewrite 成功后由 materializer 创建 draft，随后执行 render 并结束默认链路

相关代码：

- `service/rewrite_orchestrator.go`
- `service/rewrite_stage_executor.go`
- `service/draft_materializer.go`
- `infra/llm/client.go`

### 4. 排版、审核与发布

- draft 创建、读取、校验
- WeChat HTML 渲染与 rendered asset 持久化
- review create / approve / reject
- publish gate、publish outcome 与 publish history 持久化

相关代码：

- `service/formatting_pipeline.go`
- `infra/formatter/wechat_html.go`
- `service/review.go`
- `service/publish_gate.go`

### 5. Workflow / Jobs / Automation

- workflow nodes 注册与执行
- job queue / worker / cancel / event history
- automation `run-once / daemon / retry-failed / status / health / stop`

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
- 本机具备 SQLite 编译链

### 安装依赖

```bash
go mod download
```

### 启动 Web Control Plane

```bash
go run ./cmd/server
```

默认监听 `http://localhost:8123`。

可检查：

```bash
curl http://localhost:8123/health
curl http://localhost:8123/ready
```

---

## HTTP API

核心路由定义：`transport/http/server.go`

- `GET /health`
- `GET /ready`
- `GET /config`
- `GET|POST|PUT|DELETE /content`
- `GET|POST /templates`
- `GET /templates/categories`
- `POST /drafts`
- `GET /drafts/:id`
- `POST /drafts/:id/render`
- `POST /drafts/:id/validate`
- `GET /assets/:id`
- `GET /workspace/articles`
- `POST /rewrite/runs`
- `POST /reviews`
- `GET /reviews`
- `POST /reviews/:id/approve`
- `POST /reviews/:id/reject`
- `POST /publish`
- `GET /publish/history`
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

- 默认持久化路径：SQLite
- runtime repos：`service/runtime_repos.go`
- migrations：`infra/sqlite/migrations/`
- `infra/memory/` 用于测试和内存替身场景

---

## 验证命令

```bash
go test ./service -run 'TestSource|TestFolder|TestRewrite|TestBuildWebControlRuntime|TestWorkflowTemplate|TestTemplateDefinition|TestWebControlPlaneService'
go test ./transport/http/... -run 'TestAPI|TestAdminFrontend|TestRewrite'
npm --prefix webapp run test
npm --prefix webapp run build
go test ./integration -run 'TestWebControlPlanePasteToRenderedResult|TestWebControlPlaneUploadToRenderedResultWithWorkflowTemplate|TestReactControlPlanePasteToRenderedResultWithWorkflowTemplate|TestRewritePipelineMainlineMaterializesDraft'
go test ./...
```

---

## 当前边界

- browser upload / paste workflow 是默认操作人员 intake 路径
- review / publish 不会自动进入默认链路
- automation daemon 当前为单进程内模型
- publish provider 的完整能力取决于具体 provider 实现

---

## 目录概览

```text
├── cmd/
│   └── server/
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
    └── http/
```
