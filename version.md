# Version Notes

## 概览

这一轮大改动完成后，仓库根目录下的 Go runtime 已成为当前默认主实现。

当前系统的整体形态是：

- 唯一 active operator surface 是 `8123` 上的中文 React + Vite web control plane
- 唯一文档化、默认、面向操作人员的 intake 路径是浏览器 upload / paste workflow
- 默认自动化主链路是 `intake -> rewrite -> draft -> render`
- `review / publish` 保持为后续可选人工步骤，不属于默认自动链路
- 业务配置、workflow/template 管理、工作流运行状态都以当前 Go runtime + DB 持久化状态为准

这一轮的核心完成点有三大块：

1. 控制台可交付化
2. 工作流编辑器专业化
3. 后端工作流执行引擎升级到 Phase D 完整版本

---

## 这次到底改了什么

### 1. 控制台可交付化

控制台从“辅助管理界面”变成了当前产品的唯一正式操作面。

现在的控制台具备这些定位：

- 中文优先的操作界面
- 基于 React + Vite 构建
- 使用 Material UI 作为基础组件体系
- 使用 React Flow 作为工作流/模板图编辑与展示基础设施
- 所有日常操作都以浏览器控制台为主，而不是 CLI

现在操作人员可以在控制台中完成：

- 浏览器上传文件
- 浏览器粘贴内容
- 管理 workflow/template
- 查看文章工作区状态
- 触发和观察 rewrite / draft / render 主链路
- 查看暂停中的工作流和人工任务
- 进行后续 review / publish 操作

这意味着控制台已经不是“演示层”或“壳”，而是实际可交付、可操作、可承载日常流程的正式产品面。

### 2. 工作流编辑器专业化

工作流不再只是后端配置数据，而是变成了真实 browser-backed 的组件化能力。

这一轮之后，工作流编辑器的特点是：

- 工作流和模板管理通过控制台完成
- 编辑器基于 React Flow 进行图式建模
- 后端运行时支持的控制流语义已经扩展到复杂图执行
- 前后端不再只是“能画图”，而是“图能真实执行”

编辑器专业化的含义不是只提升样式，而是它已经可以对应真实执行能力，包括：

- branch / priority / condition
- pause / resume / human node
- parallel branch progression
- fan-in / join
- inline subflow
- while-style loop

也就是说，编辑器不再只是展示 DAG，而是能表达当前运行时真实支持的工作流语言。

### 3. 后端工作流执行引擎升级

这一轮最重要的底层变化，是后端 workflow runtime 按计划完成了 Phase A 到 Phase D 的完整演进。

---

## 工作流引擎演进结果

### Phase A: 统一 rewrite orchestrator 与 workflow engine

#### 这一步之前

- rewrite orchestrator 和 workflow engine 还是分离思路
- rewrite 更像一条独立编排逻辑，不完全受统一工作流运行时约束

#### 这一步的决策

- 统一 rewrite orchestrator 与 workflow engine
- 让 rewrite 成为 workflow runtime 可以承载的一部分，而不是旁路系统

#### 现在后端变成了什么

- rewrite 主链路进入统一的工作流运行时模型
- workflow engine 成为更高层的统一执行骨架
- rewrite 阶段历史、stage run、prompt snapshot、draft linkage 都可以放在统一运行模型里理解

#### 现在具备什么能力

- imported workspace article 可以进入 rewrite pipeline
- rewrite 后自动 materialize draft
- draft 后继续 render

### Phase B: branch / priority / condition

#### 这一步之前

- 工作流更偏线性或单分支
- 图中边的优先级、条件和真实分支语义还不完整

#### 这一步的决策

- 引入 formal routing semantics
- 用 token 作为 branch semantic unit
- 支持 active-token-set checkpoint / resume

#### 现在后端变成了什么

- 分支运行不再是一次性跳转，而是有明确 token lineage
- priority / condition 成为正式路由决策规则
- 多个 active token 可以被持久化并恢复

#### 现在具备什么能力

- route-based branching
- branch-local mutable state
- multiple active tokens
- checkpoint + resume

### Phase C: pause / resume / human node

#### 这一步之前

- 工作流能分支，但暂停、恢复、人工节点、人工视图还不完整

#### 这一步的决策

- 引入完整 pause/resume state machine
- human node 进入正式 runtime contract
- manual pause / policy pause 都成为正式暂停来源
- 审计与 paused-run operator view 一并补齐

#### 现在后端变成了什么

- 暂停不再只是“停止执行”，而是有结构化 pause state
- human node 可以把执行流停在某个 token / node 上
- resume 不是 ad hoc 行为，而是受控恢复模式

#### 现在具备什么能力

- human review required 的暂停
- manual pause
- policy pause
- paused run summary
- human task list
- exact audit query support

### Phase D: parallel / fan-in / subflow / loop

这一轮最终完成的是 Phase D 完整版，而不是最初的最小基础版。

#### 这一步之前

- 已经有 branch、pause、resume、human node
- 但还没有真正的高级控制流运行时

#### 这一步的总决策

- 先做 token-local execution state
- 再做真实 worker-pool 并发调度
- 再做 join barrier / fan-in
- 再做 inline subflow
- 最后做 while-style loop

#### Phase D 现在后端变成了什么

现在的 workflow runtime 已经是一个高级控制流运行时，而不只是“能跑节点的分支图”。

它的核心运行时对象变成了：

- execution token
- join barrier
- subflow frame
- loop frame

这些对象都被纳入统一运行时状态空间，并支持 checkpoint / resume。

#### Phase D 现在具备什么能力

##### 1. Parallel execution

- 多个 active token 可同时存在
- worker pool 会并发推进多个 token
- token 仍然是语义分支单位
- worker 只是调度单位，不承载语义状态

##### 2. Fan-in / join

- join 不再允许单个上游 token 独立穿过
- 所有 required incoming branches 到齐后才继续
- join barrier 会记录：
  - 属于哪个 join node
  - 期待哪些 incoming branch
  - 已到达哪些 token
  - 是否 waiting / ready / blocked / paused
- join 继续时只生成一个 downstream continuation token
- join barrier 已纳入 checkpoint / resume 语义

##### 3. Inline subflow

- subflow 是显式 node 语义，不是随便挂在 edge 上
- child workflow 在同一个 run 内 inline 执行
- child 继承 parent token 当前 branch-local context
- child 输出只按 explicit output mapping 回传
- subflow frame 现在承载：
  - parent token
  - parent node
  - child workflow definition
  - child entry node
  - input/output mapping
  - child execution status
  - failure strategy
- subflow frame 已支持 dedicated snapshot / restore

当前支持的 failure strategy 包括：

- `fail_parent`
- `pause_parent`
- `continue_parent`

##### 4. Loop

- loop 是 while-style control flow
- 有显式 body edge 和 exit edge
- loop state 按 `loop node + run` 共享，而不是按 token 独立
- max iteration 命中时会 pause，而不是直接失败
- 默认 resume 继续当前 iteration
- loop frame 已进入 checkpoint / resume 语义

---

## 当前系统长什么样

### 后端

当前后端是一套以 Go runtime 为核心的内容处理系统。

主要后端能力包括：

- workspace config / runtime config
- source intake / article workspace lifecycle
- rewrite pipeline
- draft materialize
- render / validate / asset persistence
- review / publish gate
- workflow / jobs / automation
- pause / resume / audit
- advanced workflow control flow runtime

当前后端的关键特点：

- DB-backed runtime state
- 统一的 workflow runtime
- 支持复杂控制流
- 支持浏览器控制台驱动操作

### 控制台

当前控制台是：

- 中文优先
- React + Vite
- Material UI + React Flow
- 浏览器控制台是唯一 active operator surface

它不是辅助工具，而是实际产品主界面。

### 工作流系统

当前工作流系统已经不是简单 DAG。

它现在是一套支持以下语义的真实执行引擎：

- rewrite orchestrator unified workflow execution
- priority / condition routing
- multi-branch token runtime
- active token checkpoint / resume
- human node
- manual pause / policy pause
- parallel progression
- fan-in / join barrier
- inline subflow
- while-style loop

---

## 当前默认流程

### 操作人员默认流程

1. 打开浏览器控制台 `http://localhost:8123`
2. 在控制台中完成业务配置 / workflow/template 管理
3. 通过 upload / paste 导入 source 内容
4. 进入默认自动链路：

`intake -> rewrite -> draft -> render`

5. 如果工作流遇到 human node / manual pause / policy pause，则在控制台中查看 paused run 和 task items
6. 根据需要继续 resume / replay / submit human action
7. 自动链默认停在 draft + render
8. 若需要，后续人工执行 review / publish

### 默认自动链路

默认自动化主链路不是 publish-first，而是：

1. intake
2. rewrite
3. draft materialize
4. render

默认到这里结束。

以下步骤不是默认自动链的一部分：

- review
- publish

它们是后续可选人工步骤。

---

## 如何使用

### 1. 启动服务

前置条件：

- Go `>= 1.25`
- CGO 可用
- 本地可编译 SQLite

安装依赖：

```bash
go mod download
```

启动服务：

```bash
go run ./cmd/server
```

健康检查：

```bash
curl http://localhost:8123/health
curl http://localhost:8123/ready
```

### 2. 使用控制台

打开：

```text
http://localhost:8123
```

日常操作以控制台为主，主要包括：

- upload / paste intake
- workflow/template 管理
- 查看 workspace article
- 观察 workflow run / paused run / audit
- 人工 review / publish

### 3. 使用 CLI

CLI 现在是开发/调试支持工具，不是默认产品操作面。

常用命令：

```bash
go run ./cmd/cli workspace doctor --root .
go run ./cmd/cli automation run-once --root .
go run ./cmd/cli rewrite run <workspace-article-id> --target wechat-longform --source sspai --root .
go run ./cmd/cli formatting render <draft-id> --platform wechat --template daily-intelligence --root .
go run ./cmd/cli review approve <review-id> --reviewer alice --notes ok --root .
go run ./cmd/cli publish run <review-id> --root .
```

### 4. 使用 HTTP API

当前 HTTP server 同时承载：

- React + Vite web control plane
- HTTP API

常用接口包括：

- `GET /health`
- `GET /ready`
- `GET /config`
- `GET|POST|PUT|DELETE /content`
- `GET|POST /templates`
- `POST /drafts`
- `GET /workspace/articles`
- `POST /rewrite/runs`

---

## 这一轮重要架构决策汇总

### 决策 1

Go runtime 成为当前默认主实现，而不是继续以 Python 历史实现为产品真身。

结果：

- 根目录 Go 项目是当前主线
- Archive 下项目只保留为迁移参考与历史对照

### 决策 2

控制台成为唯一 active operator surface。

结果：

- 浏览器控制台成为正式操作入口
- CLI / TUI 转为开发调试支持

### 决策 3

唯一默认 intake 路径是 browser upload / paste。

结果：

- folder-intake、RSS、旧 shell 不再作为当前操作流程说明

### 决策 4

默认自动化主链停在 draft + render。

结果：

- review / publish 被明确放到后续人工步骤

### 决策 5

workflow/template 管理是真实 browser-backed capability，而不是后端文件配置的薄壳。

结果：

- 编辑器和运行时语义一体化

### 决策 6

workflow runtime 按 Phase A-D 逐步演进，而不是一次性堆特性。

结果：

- 当前 runtime 具备可解释、可 checkpoint、可 resume 的复杂控制流能力

---

## 当前不承诺的内容

这一轮虽然已经把系统推进到可交付状态，但仍然不承诺这些事情：

- 不保证完全复刻 Python 历史兼容层
- 不保证所有旧接口、旧命名、旧文档描述完全不变
- 不把当前 TUI 视为全部运维能力入口
- 不把旧的 folder-intake / RSS / ingestion / collector 表述成当前默认操作路径

---

## 总结

这一轮大改动完成后，系统已经从“迁移中的 Go 重写版”变成了“当前默认主实现”。

现在的最终形态是：

- 后端：Go runtime + DB-backed state + advanced workflow runtime
- 控制台：中文 React + Vite，Material UI + React Flow，唯一 active operator surface
- 工作流：支持 branch / pause / resume / human node / parallel / fan-in / subflow / loop
- 默认流程：browser upload/paste -> rewrite -> draft -> render
- 人工后续：review / publish

如果把这一轮用一句话概括：

**系统已经完成从“基础内容处理后端”到“带专业控制台和高级工作流引擎的可交付主实现”的转换。**
