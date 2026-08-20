# 041 日志体系补齐与治理

状态：已完成。研究门禁、计划确认、非文档实施、验证与提交前门禁均已通过。

## 范围

本变更系统审计当前项目日志使用情况，并在既有 Logger、Tracing、Execution Record、Health、Diagnostics 与 Observability 能力之上补齐关键运行边界的日志治理。

目标不是增加日志数量，而是让日志能回答：发生了什么、发生在哪里、为什么发生、影响了什么、系统采取了什么动作，以及最终是否恢复。

本轮涉及源码、测试和权威文档更新；实施授权来自计划报告后的明确确认“确认，实施”。实现只复用既有 Logger、Tracing、Execution Record、Health、Diagnostics 与 Observability 能力，未新增平行日志体系。

## 阅读顺序

1. [研究索引](research/README.md)：当前代码事实、日志覆盖矩阵、缺口和边界判断。
2. [需求规格](requirements.md)：目标、范围、非目标和验收标准。
3. [设计方案](design.md)：实施策略、文件影响、日志级别和验证方案。
4. [任务清单](tasks.md)：稳定任务 ID、依赖、完成条件和当前状态。

## 当前研究结论

- `pkg/logger`、Kernel baseline/replacement、`docs/development/logging.md` 和 028 的 Service/Generation/HTTP 日志基线已经存在，不需要新增 Logger API、全局 logger、第二套 sink 或平行观测体系。
- Service 启动、Generation load/prepare/commit/reload/stop、HTTP access/recovery、Auth audit、Execution 恢复状态、Scheduler 生命周期/任务失败、RabbitMQ Provider 状态已有结构化日志。
- 当前缺口集中在：one-shot migration/CLI 外部 I/O 缺少结构化 operation 日志；Execution 异步记录失败记录了原始 `err.Error()`；Messaging Consumer 关键处置缺少低敏结果事件；management health/diagnostics 的异常 outcome 主要靠 HTTP status 和 diagnostics，缺少 owner 事件；schedule/messaging 现有日志缺少足够测试门禁保护；日志规范需要把 execution/schedule/messaging/migration/management 场景写成可执行检查项。
