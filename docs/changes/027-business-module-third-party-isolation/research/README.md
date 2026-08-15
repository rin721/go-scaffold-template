# 027 研究档案

本目录复核业务模块内第三方依赖的真实分布、导出契约泄漏、非业务能力归属和现有架构门禁缺口，并形成“业务模块内封装、非业务能力底层装配”的分轨计划依据。

检索顺序为：既有 metadata -> 当前权威文档 -> `internal/module` 生产 import -> 模块导出签名 -> `pkg`/Kernel App 能力模型 -> package graph 门禁。本任务不新增外部技术选型。

## 记录

- [R001 当前第三方边界复核](R001-current-third-party-shadow/report.md)：确认模块私有第三方 Adapter 合法，但 Ops 已发生具体类型泄漏且 Observability 归属层级错误。
