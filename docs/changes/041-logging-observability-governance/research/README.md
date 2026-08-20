# 041 研究索引

## 研究范围

本研究复核当前仓库在日志、Tracing、Execution Record、Health、Diagnostics 和 Observability 之间的真实分工，重点覆盖：

- 进程入口、Service 启动/停止、Application Generation、配置加载与 reload；
- Logger 注入、Kernel baseline/replacement 和结构化字段；
- HTTP access/recovery、Auth audit、management health/diagnostics；
- Execution、异步记录、恢复治理、调度任务和消息消费；
- RabbitMQ Provider 状态、Consumer admission、Broker disposition；
- 现有文档、测试和架构门禁。

## 检索与复用

已按 `docs/research/README.md` 要求检索既有 `metadata.yaml`。本轮复用了并刷新以下历史事实：

- `028-required-development-logging/R001`：Logger 能力、development 默认级别、Service/Generation/HTTP 日志基线。
- `035-background-task-capabilities`：Execution、RecoveringStore、AsyncRecorder、Health 和日志观测。
- `037-scheduled-task-capability`：Scheduler、Execution、Tracing、diagnostics 和任务日志。
- `038-messaging-adapter-capability`：Messaging/RabbitMQ、Execution、Consumer admission、Health 与运维边界。

历史记录不替代当前代码。本轮以 HEAD `f86825b52eb19e8dd807c0db7f59d5d7c7e7102a` 的静态代码、测试和文档为当前事实。

## 记录

- [R001 当前日志覆盖与治理缺口审计](R001-current-logging-observability-audit/report.md)

