# Archive 归档说明

本目录包含两个已从主开发链路中归档的历史项目。它们不再参与当前默认开发流程。

## 归档项目

### `ArticleWashing/` (Python 版)

- 技术栈：Python、FastAPI、`unittest`
- 状态：已归档。其主链路功能已由当前 Go 实现（仓库根目录）完成等价替代。
- 历史作用：曾作为 service-first 内容中转站运行时，承载 API、工作流和内容服务。

快速使用（仅供历史参考）：

```bash
cd Archive/ArticleWashing
python3 -m pip install -r requirements.txt
PYTHONPATH="src" python3 -m unittest discover -s "tests/content_hub" -p "test_workflow.py"
```

### `DataCollection/` (Node.js 版)

- 技术栈：Node.js、ESM、Vitest
- 状态：已归档。其平台采集能力正逐步迁移至当前 Go collector runtime。
- 历史作用：曾负责热榜和内容源采集、调度、重试等逻辑。

快速使用（仅供历史参考）：

```bash
cd Archive/DataCollection
npm install
npm test
npm run collect
```

## 为什么归档

当前 Go 实现（仓库根目录）已完成以下替代：

- workspace 驱动配置
- collector source registry / hotlist run / scheduler control
- article workspace 生命周期
- 结构化 draft / render / validate / asset persistence
- review / publish gate / publish history
- workflow / jobs / automation
- HTTP API / CLI / 基础 TUI

对于历史参考、审计和迁移对照，这两个归档项目仍然保留完整代码。
