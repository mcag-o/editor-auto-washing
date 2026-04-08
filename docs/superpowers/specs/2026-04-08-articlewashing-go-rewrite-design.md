# ArticleWashing Go 重构技术实施方案 (v1.1 Final)

**文档编号**: AW-GO-20260408-001
**版本**: 1.1
**状态**: 已评审通过，准备进入实施规划阶段
**日期**: 2026-04-08T23:23+08:00

---

## 1. 架构设计原则

本重构方案遵循以下核心设计原则，确保系统具备高内聚、低耦合、可测试、可观测的企业级特质。

### 1.1 依赖倒置 (Dependency Inversion)
接口定义在消费方（`service/`），实现在提供方（`infra/`）。业务服务层仅依赖自身定义的 `ArticleRepo`、`LLMProvider` 等接口，不感知底层是 SQLite、内存存储还是 PostgreSQL。这使得服务层单元测试可完全脱离 I/O，实现毫秒级执行。

### 1.2 插件化子模块化
存储 (Storage)、采集 (Collector)、发布 (Publisher)、LLM 客户端均为可替换的基础设施组件。更换数据源或采集策略仅需替换 `infra/` 下对应实现并修改 `cmd/server/main.go` 的组装逻辑，无需触碰业务服务代码。

### 1.3 零运行时动态加载
Go 的静态编译特性决定了插件机制在运行时是冗余的。本方案摒弃微内核注册表 (PluginRegistry) 和事件总线 (EventBus) 等过度封装，采用**编译期多态 + 入口手动组装**模式。组件数量固定、拓扑清晰，直接在 `main()` 中声明依赖链。

### 1.4 显式错误分类
摒弃裸 `error` 传递，建立五类业务错误域：`ValidationError` (400)、`NotFoundError` (404)、`ConflictError` (409)、`ExternalError` (LLM/采集器调用失败)、`InternalError` (500)。所有错误携带 TraceID，贯穿 HTTP → Service → Infra 全链路。

---

## 2. 系统边界与技术选型

| 层级 | 组件 | 技术选型 | 选型依据 |
|------|------|----------|----------|
| **HTTP 网关** | 路由/中间件 | `gin-gonic/gin` | 路由树 O(1)，中间件生态成熟，泛化能力强 |
| **数据持久化** | 主存储 | `mattn/go-sqlite3` + WAL 模式 | 零运维、单文件、ACID 完整。通过 `PRAGMA busy_timeout=5000` 处理写锁竞态 |
| **配置管理** | 热重载 | 原生 `encoding/json` + `fsnotify/fsnotify` | 轻量无依赖。SHA256 防重，原子写保证文件一致性 |
| **日志系统** | 结构化日志 | `log/slog` (Go 1.21+) + `lumberjack` | 标准库 JSON 序列化，零分配优化。lumberjack 支持日志轮转 |
| **终端界面** | TUI | `charmbracelet/bubbletea` | Elm 架构范式，跨平台渲染，组件生态丰富 |
| **LLM 接入** | HTTP 客户端 | 标准库 `net/http` + SSE 流式解析 | OpenAI-compatible 协议简单稳定，避免引入重型 SDK |
| **外部进程** | 采集调度 | `os/exec` + `CommandContext` | 原生进程组管理，超时中断，stdout/stderr 分离捕获 |

---

## 3. 核心服务分层模型

```
cmd/server/                     # 启动入口（信号处理、依赖组装、配置加载、Graceful Shutdown）
├── main.go

domain/                         # 领域层：纯 Go struct + 业务不变量校验
├── article.go                  #   ContentDocument, ListQuery
├── draft.go                    #   ArticleDraft, Status
├── template.go                 #   TemplateAsset
├── review.go                   #   ReviewTask, ReviewState
├── publish.go                  #   PublishRecord, PublishResult
├── workflow.go                 #   WorkflowDefinition, WorkflowContext
├── job.go                      #   JobRun, JobEvent, JobStatus
├── ingestion.go                #   IngestionBundle, IngestionItem
├── workspace.go                #   WorkspaceArticle, StateTransition
└── error.go                    #   统一错误域定义 (5 类 AppError)

service/                        # 业务用例层：编排领域对象，依赖 repo/interface
├── content.go                  #   → ArticleRepo, PublishRepo
├── template.go                 #   → TemplateRepo
├── draft.go                    #   → DraftRepo
├── formatting.go               #   → DraftRepo, AssetRepo, LLMProvider
├── review.go                   #   → ReviewRepo, PublishRepo
├── publish.go                  #   → PublishRepo, PublisherProvider[]
├── workflow.go                 #   → WorkflowNode[], ArticleRepo
├── job.go                      #   → WorkflowExecutor, JobRepo, 有界队列
├── ingestion.go                #   → IngestionRepo, WorkspaceService
├── workspace.go                #   → WorkspaceRepo
├── collector.go                #   → CollectorProvider, IngestionRepo
├── interfaces.go               #   全部 Repository 接口定义

infra/                          # 基础设施层：外部依赖适配
├── sqlite/
│   ├── provider.go             #   实现 service/interfaces.go 中所有 Repo 接口
│   └── migrations/
│       ├── 001_init.up.sql
│       └── migrations.go       #   embed.FS 内嵌迁移脚本，启动时自动执行
├── memory/
│   ├── provider.go             #   内存实现，所有 Service 测试依赖此 Provider
│   └── tx.go                   #   MemoryTx: 支持 Commit/Rollback 保证事务可测
├── llm/
│   └── openai.go               #   OpenAI-compatible HTTP 客户端，SSE 流式支持
├── collector/
│   └── nodejs.go               #   exec.CommandContext 包装 DataCollection 进程
├── publisher/
│   ├── wechat.go               #   微信 OpenAPI 调用
│   └── record.go               #   纯本地记录 (测试/降级用)
├── formatter/
│   └── wechat_html.go          #   HTML 模板渲染，支持 8 种结构化模板
├── config/
│   ├── config.go               #   配置结构体定义 + 默认值 + Secret 解析
│   ├── loader.go               #   Load/Save/Watch (原子写 + fsnotify 热重载 + 回调链)
│   └── auditor.go              #   diff3 变更审计，写入 JSON 日志文件
└── logger/
    └── logger.go               #   slog 封装，JSON/Text 格式切换，日志轮转

transport/                      # 传输层：外部协议适配
├── http/
│   ├── server.go               #   Gin 初始化，TLS, 中间件挂载
│   ├── middleware/
│   │   ├── traceid.go          #   X-Trace-ID 生成与注入
│   │   ├── recovery.go         #   panic 捕获，堆栈日志，500 响应
│   │   ├── ratelimit.go        #   golang.org/x/time/rate IP 限流
│   │   └── auth.go             #   Bearer Token 鉴权
│   ├── handlers/               #   HTTP Handler → Service 调用
│   └── errors.go               #   Domain Error → HTTP Status 映射
└── tui/
    ├── app.go                  #   bubbletea 主循环 (7 面板)
    ├── client.go               #   HTTP Client 封装 (本地缓存 + 断线重试)
    ├── dashboard.go            #   概览面板
    ├── articles.go             #   文章列表 + 状态流转
    ├── templates.go            #   模板浏览 + 内嵌编辑器
    ├── jobs.go                 #   作业监控
    ├── config.go               #   配置树浏览器
    ├── logs.go                 #   SSE 日志流实时查看
    └── collector.go            #   采集控制面板
```

---

## 4. 关键数据结构定义

### 4.1 基础实体

```go
type ContentDocument struct {
    ID        string             `json:"id"`
    Title     string             `json:"title"`
    Body      string             `json:"body"`
    Format    string             `json:"format"`
    Metadata  map[string]any     `json:"metadata"`
    CreatedAt time.Time          `json:"created_at"`
}

type ArticleDraft struct {
    ID              string           `json:"id"`
    Template        string           `json:"template"`
    Meta            map[string]any   `json:"meta"`
    Headline        map[string]any   `json:"headline"`
    Sections        []map[string]any `json:"sections"`
    Conclusion      string           `json:"conclusion"`
    Status          string           `json:"status"`
}

type WorkflowContext struct {
    Document    *ContentDocument
    Payload     map[string]any
    TraceID     string              // 贯穿工作流节点的追踪 ID
}

type JobRun struct {
    ID           string    `json:"id"`
    Topic        string    `json:"topic"`
    Status       string    `json:"status"`       // pending | running | completed | failed | cancelled
    StartedAt    *time.Time `json:"started_at"`
    CompletedAt  *time.Time `json:"completed_at"`
}
```

### 4.2 错误域

```go
type AppError struct {
    Code    ErrorCode   // VALIDATION_ERROR | NOT_FOUND | CONFLICT | EXTERNAL_ERROR | INTERNAL_ERROR
    Message string
    Cause   error
    TraceID string
}

func (e AppError) HTTPStatus() int {
    switch e.Code {
    case ErrValidation: return 400
    case ErrNotFound:   return 404
    case ErrConflict:   return 409
    case ErrExternal:   return 502
    default:            return 500
    }
}
```

---

## 5. 高并发处理与容错机制

### 5.1 SQLite 写锁竞态缓解
- **WAL 模式**：`PRAGMA journal_mode=WAL` 允许读写并行
- **连接策略**：`db.SetMaxOpenConns(1)` 写操作串行化，`db.SetMaxIdleConns(10)` 读操作复用
- **Busy Timeout**：`PRAGMA busy_timeout=5000` 底层自动重试 5 秒

### 5.2 异步作业队列
- 有界通道 (Bounded Channel)，容量 100，溢出返回 503
- 后台 Worker 协程逐个消费，每个 Job 绑定独立 TraceID
- 运行期间定期写入 `JobEvent`，TUI 可实时查询进度

### 5.3 采集子进程隔离
- `exec.CommandContext(ctx, ...)` 绑定上下文，超时自动 `SIGKILL`
- stdout/stderr 分离缓冲，设置 `SysProcAttr.Setpgid = true` 防止孤儿进程

### 5.4 TUI 本地缓存与断线重连
- **本地缓存**：LRU 缓存 (TTL 30s)，标记 `stale` 状态
- **断线重连**：指数退避重试 (500ms → 5s, 最多 5 次)，幂等设计保证数据一致性
- UI 显示 `Connection Lost - Reconnecting...`，恢复后台自动刷新

### 5.5 Graceful Shutdown
1. HTTP Server `Shutdown(ctx)` 停止接收新请求 (10s 超时)
2. `context.CancelFunc()` 通知所有后台 goroutine 停止
3. 清空作业队列，当前正执行的 Job 等待上下文取消自然终止
4. 数据库 `PRAGMA wal_checkpoint(TRUNCATE)` 刷入主库，关闭连接
5. **自动备份**：拷贝 `content_hub.db` 至 `data/backups/content_hub-pre-shutdown.db`
6. TUI 显示 `Server Shutting Down...` 后进入离线模式

---

## 6. 错误容错与异常处理策略

### 6.1 分级错误处理

| 错误级别 | 代表场景 | 处理策略 |
|----------|----------|----------|
| ValidationErr | 必填字段为空、格式非法 | 400，记录 warn 日志，不重试 |
| NotFoundErr | 不存在的 Article / Template ID | 404，记录 warn 日志 |
| ConflictErr | 并发写同一配置/文档 | 409，返回当前最新版本，提示用户重试 |
| ExternalErr | LLM API 超时、采集器退出码非 0 | 502，记录 error 日志 (含 TraceID)，支持重试 |
| InternalErr | 数据库文件损坏、Panic Recovery | 500，触发告警，返回通用错误文案 |

### 6.2 统一错误映射
`transport/http/errors.go` 集中处理 Domain Error → HTTP Status 映射。Handler 层仅负责 `return nil, appErr`，由中间件统一拦截并格式化 JSON 响应。

### 6.3 Panic Recovery
Gin `CustomRecovery` 中间件捕获所有未被处理的 panic，进程不退出。

---

## 7. 配置管理与热重载机制

### 7.1 config.json 原子写 + 校验
1. **校验**：检查必填字段、交叉约束
2. **备份**：`config.json` → `config.bak.json`
3. **临时写入**：写入 `config.tmp.json`
4. **原子重命名**：`os.Rename("config.tmp.json", "config.json")`
5. **触发重载**：fsnotify 监听 → SHA256 防重 → 触发回调链

### 7.2 配置竞态保护 (回调链模型)
配置在内存中为不可变引用 (`atomic.Value`)。变更热重载时：
1. 解析新配置到临时变量
2. `r3labs/diff` 生成变更 diff
3. 写入审计日志 `data/audit/config-change-YYYYMMDD-HHMMSS.json`
4. `atomic.Value.Store(newConfig)` 替换全局指针
5. 触发**字段级回调链**：`loader.OnChange("llm", callback)`, `loader.OnChange("workflow", callback)`
各 Service 不持 Config 引用，彻底消除并发读写的竞态风险。

### 7.3 Secret 解析
格式: `"api_key_env": "LLM_API_KEY"`。启动时调用 `os.Getenv("LLM_API_KEY")` 解析。TUI 显示 `[HIDDEN]`。

### 7.4 变更审计
每次 `config.json` 变更生成带时间戳的 JSON 审计文件，记录 old_hash, new_hash, 变更字段列表 (old_value → new_value)。

---

## 8. 依赖管理与接口契约规范

### 8.1 接口定义位置
接口定义在**消费方** (`service/interfaces.go`)，由**实现方** (`infra/`) 实现。

### 8.2 Memory Provider 事务支持 (Test Double)
- `MemoryTx` 持有启动时刻的**数据快照** (Deep Copy)
- `Commit()`: 原子性地将变更应用到主状态
- `Rollback()`: 不修改主状态
保证 Service 层 `Tx(fn func(Tx))` 包裹的事务逻辑在测试中可验证。

### 8.3 go.mod 管理
- 严格版本锁定，最小依赖集
- `database/sql` + 参数化查询，禁用 ORM
- CGO 控制：仅 `mattn/go-sqlite3` 依赖 CGO，使用 `zig cc` 跨平台编译

---

## 9. 可观测性与安全设计

| 维度 | 方案 |
|------|------|
| **TraceID** | 每个请求生成 UUID `X-Trace-ID`，贯穿全链路 |
| **日志格式** | JSON：含 level, msg, trace_id, duration_ms |
| **Health** | `/health` (Liveness)，`/ready` (Readiness) |
| **限流** | `golang.org/x/time/rate.TokenBucket` 按 IP 60 req/s |
| **鉴权** | Bearer Token，环境变量注入 |
| **SQL 注入防护** | 全量使用 `?` 参数化查询 |

---

## 10. 平滑迁移路径与实施路线图

| 阶段 | 时间 | 交付物 | 里程碑 |
|------|------|----------|-----------|
| **Phase 1: 骨架与配置** | D1-D2 | go.mod, config.json, Loader+Auditor, Logger | 配置可热重载，变更审计可用 |
| **Phase 2: 接口定义与存储** | D3-D5 | `service/` 接口, `infra/sqlite/` CRUD, `infra/memory/` + Tx | 数据库初始化 + Repo 单元测试通过 |
| **Phase 3: 服务层** | D6-D8 | 全部 `service/` (9 个), 异步 Job 队列 | 业务逻辑测试通过 (memory repo) |
| **Phase 4: HTTP 网关** | D9-D11 | Gin Server, Middleware, 全量 API, Error Mapper | `/health` 通，API 对标 Python |
| **Phase 5: LLM + Formatter** | D12-D13 | LLM 流式客户端, WechatHtmlFormatter 移植 | 微信模板渲染验证通过 |
| **Phase 6: TUI 工具** | D14-D16 | 7 面板, API Client + 缓存 + 断线重连 | TUI 可独立操作所有核心业务 |
| **Phase 7: 测试集成** | D17-D18 | 单元测试 >75%，集成测试，基准测试 | `make test` 全绿 |
| **Phase 8: 平滑切换** | D19 | 旧 Python 数据迁移 (JSON → SQLite)，上线验证 | 业务数据无损导入，HTTP 兼容旧版 |

---

## 11. 测试策略

| 测试类型 | 覆盖范围 | 标准 |
|----------|----------|------|
| **单元测试** | Domain, Service, MemoryRepo+Tx | >75% 行覆盖，<10ms/case |
| **集成测试** | HTTP 端到端, SQLite 迁移 | JSON 结构匹配 Python 版 |
| **基准测试** | Repo CRUD, LLM 连接 | GetByID <2ms, List <10ms |
| **安全测试** | SQL 注入, XSS | 0 个拼接 SQL |

---

## 12. 附录：平滑切换数据迁移

```bash
go run cmd/migrate/main.go --from-dir ./ArticleWashing/data/ --to-db ./data/content_hub.db
go run cmd/migrate/main.go verify --db ./data/content_hub.db --count-check
```

迁移策略：
1. 扫描旧 `data/` 目录下 JSON 文件
2. 开启 SQLite 事务
3. 逐文件解析为 Domain Struct，批量 INSERT
4. Commit 前校验 (Count Check)
5. 成功 Commit / 失败全量 Rollback
