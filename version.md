# Version Notes

## 当前版本摘要

本文件只保留当前版本的变更摘要；运行方式、能力边界和操作说明以 `README.md` 为准。

## 本轮重点变更

- 仓库根目录 Go runtime 明确为当前交付形态。
- 浏览器中的中文 React + Vite web control plane 作为唯一文档化操作面，默认端口统一为 `8123`。
- 唯一默认操作人员 intake 路径为浏览器 upload / paste workflow。
- workflow/template 管理确认为同一浏览器界面中的真实 browser-backed 能力。
- 默认自动化主链路明确为 `intake -> rewrite -> draft -> render`。
- `review / publish` 保留为后续可选人工步骤，不属于默认自动链路。
- workflow runtime 已具备 Phase D 级别控制流能力，包括 branch、pause/resume、human node、parallel、fan-in、subflow、loop。
- 仓库已移除历史 `Archive/` 目录与迁移期中间文档，当前只保留现项目本身的结构和说明。

## 一致性说明

- collector source 元数据统一使用 `implementation_reference`。
- 仓库默认 HTTP 端口、示例配置与当前文档均统一为 `8123`。

## 参考

- 运行与使用说明：`README.md`
- 站点接入规范：`爬虫开发和接入.md`
- 单站点执行模板：`单站点接入模板.md`
