# 交付与运维

本目录是当前构建、迁移、发布、复制、安全、调度、消息、排障和运行维护的使用入口，承接 [项目手册](../README.md) 的验证与运维部分。`docs/changes/**` 保存施工证据，不替代这里的现行操作说明。

## 运行路径

1. [构建与容器](build-and-container.md)：本地构建、跨平台构建、容器验证和 CI 证据边界。
2. [数据库迁移与回滚](migration-and-rollback.md)：迁移状态、前滚、回滚、dirty 处理和生产约束。
3. [本地候选与正式发布](release.md)：本地候选、发布约束、Release 证据和不允许冒充的门禁。
4. [复制为独立项目](copying.md)：模板复制后的身份、配置、Git 历史和验证要求。
5. [安全响应](security.md)：凭据、漏洞、安全修复和低敏交付边界。
6. [定时任务运维](scheduled-tasks.md)：调度配置、执行权、健康、恢复观察和多实例边界。
7. [消息系统运维](messaging.md)：Provider 配置、RabbitMQ topology、恢复、DLX 和真实协议门禁。

## 排障入口

- 启动前配置、未知字段、环境变量覆盖和 reload 语义见 [配置说明](../configuration/README.md)。
- Service readiness、diagnostics、management listener 和构建信息从本目录与 Ops 模块说明进入。
- 日志级别、唯一错误 owner、字段脱敏和验证要求见 [开发日志规范](../development/logging.md)。
- execution、schedule、messaging 的业务接入语义分别回到 [execution 能力](../development/execution-capability.md)、[定时调度能力](../development/scheduled-task-capability.md) 和 [消息系统适配能力](../development/messaging-capability.md)。

所有命令都从仓库根目录运行。Linux、容器、PostgreSQL/MySQL 和 keyless release 的最终证据来自 CI；没有对应日志时不得用 cross-build 或未运行的 workflow 代替。

## 权威边界

本目录只描述当前可操作的运维方式。历史任务中的未验证门禁、目标设计和发布计划保留在 [任务级变更记录](../changes/README.md)，不能作为当前运维步骤直接执行。
