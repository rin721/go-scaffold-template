# 任务级变更记录

每项新变更使用递增三位序号和语义名称，固定包含 `README.md`、`research/`、`requirements.md`、`design.md` 和 `tasks.md`。所有任务先通过研究门禁，再形成计划；非纯文档实现必须在计划报告后的独立消息中获得确认。完整规则以根 [AGENTS.md](../../AGENTS.md) 为准。

## 记录

- [001 默认配置契约与可选 CLI](001-default-config-cli-contracts/README.md)：已完成。
- [002 应用启动入口](002-application-entrypoint/README.md)：实现已保留，存在确认流程偏差，最终运行验证未完成。
- [003 变更确认流程](003-change-confirmation-workflow/README.md)：已完成。
- [004 Logger Capability 注入](004-logger-capability-injection/README.md)：已完成。
- [005 全量配置示例](005-full-config-example/README.md)：已完成。
- [006 Kernel App 多态装配基础](006-kernel-app-polymorphic-composition/README.md)：已完成。
- [007 Kernel 内置 Logger 的可选 App 替换](007-app-component-logger-injection/README.md)：已完成。
- [009 配置重载与生命周期修复](009-config-reload-lifecycle-repair/README.md)：已完成。
- [010 数据库单轨 GORM 与稳定访问边界](010-database-gorm-boundary/README.md)：已完成。
- [011 Cache、I18n 与 Storage 装配](011-cache-i18n-storage-composition/README.md)：已完成。
- [012 业务模块架构](012-business-module-architecture/README.md)：底层 CLI/Config/单候选/Supervisor/HTTP/诊断与治理闭环已实施；业务解锁条件已由 014 的真实 Todo 用例满足。
- [013 研究优先任务门禁](013-research-plan-implementation-gate/README.md)：已完成；将 012 的结构化研究方法提升为“研究 -> 计划 -> 实现”的仓库级前置门禁。
- [014 Todo 业务垂直切片](014-todo-business-vertical-slice/README.md)：已实现 Todo Model/Service/Repository、SQLite migration、HTTP 路由、Application CLI、配置绑定与进程组合闭环。

下一个任务序号为 `015`。已完成记录只保存历史证据；当前行为必须回到根 [README](../../README.md) 和对应主题文档确认。
