# 027 研究档案

本目录复核新增业务能力的职责收口、模块内第三方依赖、导出契约泄漏、底层 Capability 双条件和现有架构门禁缺口，并形成“业务能力完整收口、专属第三方模块内封装、双条件资源底层装配”的计划依据。

检索顺序为：既有 metadata -> 当前权威文档 -> `internal/module` 生产 import -> 模块导出签名 -> `pkg`/Kernel App 能力模型 -> package graph 门禁。本任务不新增外部技术选型。

## 记录

- [R001 当前第三方边界复核](R001-current-third-party-shadow/report.md)：确认模块私有第三方 Adapter 合法；Ops 已发生具体类型泄漏，Observability 因同时跨业务复用并由进程统一选择而符合底层装配条件。
