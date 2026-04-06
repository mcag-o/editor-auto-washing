# 项目简介

这是一个围绕内容采集、结构化处理与内容服务化运行构建的多项目工作区，不是单一应用。

仓库当前包含三个活跃子项目：

- `文章中转站/`：面向服务化运行的内容中转站，也是当前推荐的主开发目录
- `数据采集/`：负责热榜与内容来源采集、平台适配和调度逻辑的 Node.js 项目
- `结构化文章/`：负责结构化文章渲染、校验与 CLI 流程的 Node.js 工具项目

如果你是第一次进入这个仓库，建议先把它理解为“多个相关项目组成的工作区”。如果你准备继续开发核心服务能力，优先从 `文章中转站/` 开始。

如果你只想先用一个最小步骤确认环境和项目入口，推荐先从 `文章中转站/` 开始：

```bash
python3 -m pip install -r "文章中转站/requirements.txt"
PYTHONPATH="文章中转站/src" python3 -m unittest discover -s "文章中转站/tests/content_hub" -p "test_workflow.py"
```

判断从哪个目录开始，可以先用下面这条简单规则：

- 偏服务运行、API、工作流：去 `文章中转站/`
- 偏抓取输入、平台采集、调度：去 `数据采集/`
- 偏渲染输出、结构校验、CLI：去 `结构化文章/`

## 仓库结构

### `文章中转站/`

- 技术栈：Python、FastAPI、`unittest`
- 作用：承载 service-first 内容中转站运行时，对外提供 API、工作流和内容服务能力
- 适合改动：API、工作流节点、内容域模型、存储层、发布流程、任务系统

### `数据采集/`

- 技术栈：Node.js、ESM、Vitest
- 作用：负责热榜和内容源采集，处理平台适配、采集调度与请求重试等运行逻辑
- 适合改动：抓取器、平台适配器、调度策略、HTTP 客户端、重试与超时控制

### `结构化文章/`

- 技术栈：Node.js、ESM、`node --test`
- 作用：负责结构化文章的渲染、校验与 CLI 流程编排
- 适合改动：渲染模板、文章结构校验、CLI 命令、本地 pipeline 与输出流程

## 快速开始

### 1. `文章中转站/`

从仓库根目录执行。

安装依赖：

```bash
python3 -m pip install -r "文章中转站/requirements.txt"
```

运行主 API（用于本地启动服务）：

```bash
PYTHONPATH="文章中转站/src" uvicorn content_hub.interfaces.api.main:app --reload
```

运行独立入口（用于快速验证运行时入口；与 API 启动二选一即可）：

```bash
python3 "文章中转站/main.py"
```

适用场景：继续服务端、API、工作流主线能力开发。

### 2. `数据采集/`

从仓库根目录执行：

```bash
(cd 数据采集 && npm install)
(cd 数据采集 && npm test)
(cd 数据采集 && npm run collect)
```

适用场景：继续抓取器、平台适配和采集调度相关开发。

### 3. `结构化文章/`

从仓库根目录执行：

```bash
(cd 结构化文章 && npm install)
(cd 结构化文章 && npm test)
(cd 结构化文章 && node ./src/cli/index.js validate ./examples/article.sample.json)
```

适用场景：继续渲染、结构校验、CLI 和本地 pipeline 相关开发。

## 测试与开发

三个子项目的测试方式并不相同：

- `文章中转站/`：使用 Python `unittest`，要求 Python `>=3.10,<3.13`
- `数据采集/`：使用 npm + Vitest，要求 Node `>=20`
- `结构化文章/`：使用 npm + `node --test`，要求 Node `>=18`

单文件测试示例：

```bash
(PYTHONPATH="文章中转站/src" python3 -m unittest discover -s "文章中转站/tests/content_hub" -p "test_workflow.py")
```

```bash
(cd 数据采集 && npx vitest run test/core/httpClient.test.js)
```

```bash
(cd 结构化文章 && node --test test/core/io.test.js)
```

开发时建议遵循以下顺序：

1. 先确认目标子项目。
2. 先运行该子项目最窄的相关测试，再开始修改。
3. 做最小、针对性的改动，尽量保持修改范围局部。
4. 先回归单文件测试，再回归对应子项目的完整测试集。

## 推荐开发路径

- 如果你在做新的 service、API 和工作流能力，优先进入 `文章中转站/`
- 如果你在做站点抓取、平台适配、采集调度，进入 `数据采集/`
- 如果你在做文章渲染、结构校验、CLI 或本地流程，进入 `结构化文章/`

## 补充说明

- 根目录 `README.md` 只负责仓库级导航，更多实现细节请查看各子项目自己的 README 和目录文档。
- `文章中转站/` 是当前推荐的主开发目录；如果没有明确理由，新的核心服务能力应优先落在这里。
- `结构化文章/` 的部分历史文档仍可能保留旧的 `1/...` 路径，使用时请以当前真实目录为准。
- 当前仓库根目录没有统一的 build、lint、test 入口；执行命令前请先确认自己所在的目标子项目。
