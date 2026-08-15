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
- [015 Todo 路由中间件示例](015-todo-route-middleware-example/README.md)：已实现模块级 JSON Content-Type middleware、创建路由显式绑定、415 安全错误与进程验收。
- [016 应用模块命名迁移](016-application-module-naming/README.md)：已完成；`internal/module`、`module.ID` 与 `module.todo` 已成为唯一现行命名。
- [017 应用模块能力评估门禁](017-module-capability-assessment-gate/README.md)：已完成通用 Agent 研究语境、项目级应用模块开发指南、能力评估表和生命周期契约缺口升级路径。
- [018 Cordis 启发的插件架构](018-cordis-inspired-plugin-architecture/README.md)：已废除；研究快照作为历史保留，所有插件架构实施任务失效。
- [019 HTTP API 成熟度缺口评估](019-http-api-maturity-gap-assessment/README.md)：已完成当前 HTTP API 运行链审计、成熟度参考、缺口优先级和分阶段路线；没有非文档实施授权。
- [020 复制型脚手架产品形态](020-scaffold-product-form/README.md)：已完成 copy-owned 单轨决策、两个独立副本的身份迁移、Todo 保留/移除和 Windows 门禁；Linux、正式复制指南与 release 能力仍待独立实施。
- [021 仓库身份迁移](021-repository-identity-migration/README.md)：已将 canonical remote、Go module/import、运行品牌与当前使用文档统一为 `go-scaffold-template`；另一个 `go-scaffold` 仓库未进入范围。
- [022 HTTP API 脚手架成熟就绪度](022-http-api-template-readiness/README.md)：`Foundation-closed(current synchronous HTTP/CLI profile)` 已通过；024 已完成产品能力与 Windows 本地证据，但 Linux、容器、服务器数据库和远端 CI 总验收尚未通过。
- [023 全配置无感重载](023-full-configuration-seamless-reload/README.md)：已完成本地实施验收；Application Generation/ListenerHub 与七节配置重载已落地，Linux 真实 runtime 和真实 Redis 经用户批准跳过并保持未验证，未 push。
- [024 生产就绪模板一次性竣工](024-production-ready-one-shot-completion/README.md)：已确认并实施中；连续完成 `ONE-001..025` 与本地检查点提交，禁止 push、tag、远端 Release、GHCR 和外部 attestation。

下一个任务序号为 `025`。已完成记录只保存历史证据；当前行为必须回到根 [README](../../README.md) 和对应主题文档确认。
