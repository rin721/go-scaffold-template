# 项目文档

本目录承载当前主题文档、研究报告和任务级变更记录。使用者先从根 [README](../README.md) 进入；AI Agent 的稳定协作规则见根 [AGENTS.md](../AGENTS.md)。

## 当前使用说明

- [本地启动指南](getting-started/local-development.md)
- [配置说明](configuration/README.md)
- [开发日志规范](development/logging.md)
- [交付与运维](operations/README.md)
- [API 文档](../api/README.md)

## 开发与架构主题

- [应用模块开发](development/application-module-development.md)
- [消息系统适配能力](development/messaging-capability.md)
- [定时调度能力](development/scheduled-task-capability.md)
- [底层能力库](../pkg/README.md)
- [Kernel 运行与配置](../internal/kernel/README.md)
- [Kernel App 组件](../internal/kernel/app/README.md)
- [CLI 契约](../pkg/cli/README.md)

## 研究与历史

- [研究报告索引](research/README.md)
- [Go 脚手架底层能力装配架构对比](research/001-go-capability-composition/README.md)
- [Kernel 底层组件手动装配与安全重载](research/002-kernel-app-manual-composition/README.md)
- [任务级变更索引](changes/README.md)

`docs/changes` 保存需求、设计、任务账本和实施证据，不替代当前使用说明或架构主题文档。
