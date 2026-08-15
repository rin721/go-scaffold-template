# 028 研究档案

本目录复核当前 Logger 契约、默认级别、启动与重载调用链、实际生产日志调用点、错误边界、文档规范和验证缺口。

检索顺序为：既有 metadata -> 根文档与 Logger/Kernel 主题文档 -> `cmd/app` 入口 -> application composition -> GenerationCoordinator/Kernel/Supervisor -> HTTP 与 Auth 消费者 -> 测试和配置示例。本任务不需要重新选择日志技术或引入外部资料。

## 记录

- [R001 当前日志可见性复核](R001-current-logging-visibility/report.md)：确认能力已经存在，问题主要来自开发默认过滤、启动阶段事件稀疏、无真实 Warn 调用和开发规范缺位。
