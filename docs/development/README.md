# 开发指南

本目录收口当前开发主题。开发资料按能力形成闭环：先理解模块如何进入应用，再按需要接入日志、执行、调度、消息等底层能力，最后回到验证和运维文档确认交付边界。

## 开发闭环

1. [应用模块开发指南](application-module-development.md)：新增业务模块的研究问题、能力评估、目录职责、binding、composition 接入和完成标准。
2. [开发日志规范](logging.md)：Service 生命周期、外部 I/O、状态转换、错误边界和低敏结构化日志。
3. [业务模块接入 execution 能力](execution-capability.md)：幂等、失败重试、执行记录、Trace、恢复治理和多实例边界。
4. [业务模块接入定时调度能力](scheduled-task-capability.md)：cron/fixedDelay 声明、分布式执行权、Execution 协作和 Generation 切换。
5. [业务模块接入消息系统适配能力](messaging-capability.md)：Message Contract/Binding、Provider、Publisher、Consumer、RabbitMQ 可靠性和运维协作。

## 相关入口

- [架构说明](../architecture/README.md)：理解 Kernel、Application Generation、composition 和模块边界。
- [pkg 封装规范与能力清单](../../pkg/README.md)：确认当前可复用的底层能力和第三方封装边界。
- [应用模块边界](../../internal/module/README.md)：确认 `internal/module` 的职责、Contribution 和允许依赖。
- [API 文档](../../api/README.md)：确认当前 HTTP 契约和生成产物。
- [交付与运维](../operations/README.md)：确认构建、迁移、发布、复制和运行期运维要求。

## 权威边界

本目录描述当前开发方式。历史任务中的设计、研究和验证证据保存在 [任务级变更记录](../changes/README.md)，用于追溯，不替代这里的现行开发说明。
