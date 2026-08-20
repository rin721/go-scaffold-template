# 040 项目文档体系系统重构

状态：纯文档任务；研究门禁已通过，计划已确认，文档实施已完成。

## 范围

本变更在 039 的轻量入口闭环基础上，系统审计并重构当前项目文档体系。目标是让读者从项目本身自然进入，按真实使用路径连续理解：认识项目、启动项目、使用能力、开发业务、接入基础设施、理解架构、扩展能力、调试排障、运行维护和深入底层设计。

本任务只修改 Markdown 文档和任务证据，不修改 Go 源码、配置格式、脚本、生成物、OpenAPI、数据库迁移或运行行为。

## 阅读顺序

1. [研究索引](research/README.md)：审计范围、代码事实、文档问题和矩阵入口。
2. [需求规格](requirements.md)：目标、范围、非目标和验收标准。
3. [设计方案](design.md)：正式文档体系、authority 收敛和文件影响。
4. [任务清单](tasks.md)：稳定任务 ID、实施证据和验证记录。

## 当前实现结论

- 根 [README](../../../README.md) 继续只承担项目自然入口、五分钟启动、项目手册入口和权威边界。
- [项目手册](../../README.md) 改为连续路径，不按读者身份分流，并显式连接正式主题文档、局部 README、研究快照和变更历史。
- [架构说明](../../architecture/README.md) 收口 composition、Application Generation、Kernel App、module boundary、pkg capability 和生命周期治理。
- [开发指南](../../development/README.md) 收口业务模块、Binding 契约、execution、schedule、messaging、logging 与 API contract。
- [交付与运维](../../operations/README.md) 收口构建、迁移、发布、复制、安全、调度、消息、排障和运行维护。
- [pkg 能力清单](../../../pkg/README.md) 补齐 `execution` 局部入口，并明确包级 README 不承担项目级公共契约 authority。
- `docs/changes/**` 和 `docs/research/**` 继续保存历史证据与研究快照；已经成为正式能力的内容回到主题文档，未验证或目标设计不得混入当前操作说明。
