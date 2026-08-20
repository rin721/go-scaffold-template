# 036 业务模块接入 execution（Todo 落地）

状态：研究门禁已过；纯文档交付（接入指南）已就绪；非文档的 Todo 真实接入**待计划确认**。

## 范围

- 纯文档交付：新增 [业务模块接入 execution 能力](../../development/execution-capability.md)权威文档，
  并在模块开发指南建立入口链接。
- 非文档交付（待确认）：把 Todo 业务模块接入 `execution`（幂等 / 重试 / 执行记录），经 composition 注入，
  在某真实用例上以窄 port 使用（候选：`Complete`）。

## 阅读顺序

1. `research/R001-.../report.md` —— Todo 模块装配与能力注入现状、接入点分析。
2. `requirements.md` —— 目标 / 范围 / 约束 / 非目标 / 验收。
3. `design.md` —— 窄 port 设计、composition 注入、错误与边界语义。
4. `tasks.md` —— 任务清单与确认状态。
5. 权威接入文档：`docs/development/execution-capability.md`。
