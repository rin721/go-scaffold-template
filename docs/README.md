# 项目手册

本手册把项目资料收束成一条连续路径。根 [README](../README.md) 保留项目定位和最短启动入口；这里承接完整说明、开发约束、能力使用、架构脉络、调试运维和历史证据。

当前操作说明优先放在主题文档中。`docs/changes/**` 是任务账本，`docs/research/**` 是研究快照；它们用于追溯为什么这样做，不替代这里的当前使用说明。

## 1. 认识项目

- [根 README](../README.md)：项目定位、五分钟启动、架构摘要和文档权威边界。
- [AGENTS.md](../AGENTS.md)：AI Agent 协作规则、工程红线和研究计划门禁。
- [API 文档](../api/README.md)：当前 OpenAPI、operation inventory 和 code-first 契约生成入口。

## 2. 启动项目

- [本地启动指南](getting-started/local-development.md)：`config init`、数据库迁移、Service 启动、readiness 和常见启动错误。
- [配置说明](configuration/README.md)：配置来源、环境变量覆盖、owner、strict binding、reload 与密钥边界。
- [数据库迁移与回滚](operations/migration-and-rollback.md)：迁移状态、前滚、回滚和 dirty 处理。

## 3. 使用能力

- [pkg 封装规范与能力清单](../pkg/README.md)：面向业务模块开放的通用能力、第三方封装边界和暂缓路线。
- [CLI 契约](../pkg/cli/README.md)：命令、flag、交互式提示和执行边界。
- [HTTP 能力](../pkg/httpx/README.md)：HTTP client、router、server、middleware 和 contract 边界。
- [Database 能力](../pkg/database/README.md)：数据库、迁移、Repository 和事务边界。
- [Execution 能力](../pkg/execution/README.md)：幂等、失败重试、执行记录、Trace 和恢复治理的公共契约。
- [Schedule 能力](../pkg/schedule/README.md)：定时任务声明契约。
- [Messaging 能力](../pkg/messaging/README.md)：项目自有消息契约与模块声明能力。

## 4. 开发业务

- [开发指南](development/README.md)：业务模块开发和能力接入的统一入口。
- [应用模块开发指南](development/application-module-development.md)：新增业务模块时的能力评估、目录职责、HTTP/config/cli/migration/i18n/schedule/message binding。
- [开发日志规范](development/logging.md)：结构化日志事件、级别、字段、脱敏和验证要求。
- [业务模块接入 execution 能力](development/execution-capability.md)：幂等、失败重试、执行记录、Trace 和多实例边界。
- [业务模块接入定时调度能力](development/scheduled-task-capability.md)：cron/fixedDelay、分布式执行权、恢复和运维覆盖。
- [业务模块接入消息系统适配能力](development/messaging-capability.md)：Message Contract/Binding、Provider、Publisher、Consumer 和可靠性语义。

## 5. 接入基础设施

- [配置说明](configuration/README.md)：所有外部资源配置节、owner 和 reload 行为的当前 authority。
- [交付与运维](operations/README.md)：构建、容器、迁移、发布、复制、安全、定时任务、消息系统和排障入口。
- [定时任务运维](operations/scheduled-tasks.md)：调度配置、执行权、健康和恢复观察。
- [消息系统运维](operations/messaging.md)：Provider 配置、RabbitMQ topology、恢复和未验证门禁。

## 6. 理解架构

- [架构说明](architecture/README.md)：composition、Application Generation、Kernel App、模块边界、pkg 能力链路和生命周期治理。
- [应用模块边界](../internal/module/README.md)：业务模块目录职责、Contribution 和允许依赖。
- [Kernel 与 App 组件装配](../internal/kernel/README.md)：底层 App 组件、Plan、typed Input、生命周期、重载和诊断。
- [Kernel App 组件开发](../internal/kernel/app/README.md)：底层组件形态、Definition、Lease、Replacement 和接入验收。

## 7. 扩展能力

- 业务模块扩展先从 [应用模块开发指南](development/application-module-development.md) 开始，确认真实用例、能力归属、Binding 和 owner。
- 跨业务复用且由进程统一选择的底层能力，再进入 [Kernel App 组件开发](../internal/kernel/app/README.md)。
- 新增公开 HTTP operation 时，遵守 [API 文档](../api/README.md) 的 code-first 生成链。

## 8. 调试排障

- 本地启动失败先回到 [本地启动指南](getting-started/local-development.md) 和 [配置说明](configuration/README.md)。
- 运行期状态、readiness、诊断、调度和消息故障从 [交付与运维](operations/README.md) 进入。
- 日志事件、错误 owner 和低敏字段遵守 [开发日志规范](development/logging.md)。

## 9. 运行维护

- [构建与容器](operations/build-and-container.md)：本地构建、跨平台构建和容器验证。
- [本地候选与正式发布](operations/release.md)：本地候选、发布约束和 Release 证据。
- [复制为独立项目](operations/copying.md)：把模板复制成独立项目时的身份、配置和验证要求。
- [安全响应](operations/security.md)：凭据、漏洞和安全响应边界。

## 10. 深入底层设计

- [研究档案与报告](research/README.md)：结构化研究格式、复用规则和项目级研究索引。
- [任务级变更记录](changes/README.md)：每个变更的需求、设计、任务和验证证据。
- [040 项目文档体系系统重构](changes/040-documentation-system-rebuild/README.md)：当前文档体系重构的研究、计划、审计矩阵与验证账本。

## 维护边界

- 项目级契约、Binding、生命周期、配置、装配方式、业务模块接入、基础能力使用、架构原则和门禁规范，只写入对应主题 authority。
- `pkg/**/README.md` 和 `internal/**/README.md` 只说明本包或本模块局部实现、资源所有权和链接入口。
- `docs/changes/**` 和 `docs/research/**` 保存历史证据，不替代当前手册。
