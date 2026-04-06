# 一致性说明

当前一致性验证覆盖以下层面：

- 渲染后的关键 HTML 片段同时出现在 Python 与 Node 输出中
- 校验语义保持一致：示例文章在 Node 与 Python 中都应通过
- dry-run pipeline summary 结构保持一致，至少对齐关键字段：
  - `html`
  - `resolved_article`

## 已知差异

- Node 版本当前更偏向结构性一致，而非逐字符 HTML 完全一致
- Python 脚本的完整输出细节与 Node 版可能存在换行、空格和部分默认字段差异
- 目前 parity 测试主要以“关键片段”和“结果语义”作为对齐基准

## 后续可增强项

- 做更严格的 HTML 归一化对比
- 对 warning 文案逐条对齐
- 对更多模板分别做 parity 验证
