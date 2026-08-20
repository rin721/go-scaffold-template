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
- [025 业务模块边界收口](025-business-module-boundary-closure/README.md)：已完成；Todo 手写 HTTP Adapter 已收回模块，Auth/Todo 通过窄端口连接，入口与跨模块导入门禁已加固，OpenAPI 与运行行为保持不变。
- [026 Handler-first HTTP 路由绑定](026-handler-first-http-route-binding/README.md)：研究与计划已完成，待确认；拟把模块 operation Handler、应用静态 aggregate、单一生成 route binding 与外层 Router 分责，消除当前单模块假设。
- [027 第三方封装与分轨装配](027-business-module-third-party-isolation/README.md)：已确认并实施；新增业务能力先完整收口到模块，专属第三方留在模块 Adapter 并零泄漏，只有跨业务复用且由进程统一选择的资源才进入完整底层链。
- [028 开发日志基线与启动可见性](028-required-development-logging/README.md)：已完成；development 默认输出 Debug 及以上，production 默认保持 Info，Service/Generation/HTTP 已形成分级低敏事件链并由开发规范和架构测试守护。
- [029 本地启动与配置闭环](029-local-startup-config-closure/README.md)：已完成；generated config、Migration、Todo CLI 与 Service 共用 application-owned 配置集合，本地启动与配置说明已收束到根 README、本地启动指南和配置说明。
- [030 模块自有代码优先契约](030-module-owned-code-first-contract/README.md)：已完成；契约 authority 反转为模块自有 typed 声明，`contract-gen` 从代码生成 `api/openapi.yaml` 与 operation inventory，删除 oapi-codegen 生成链与 nethttp-middleware，transport 从同一份契约单一绑定。
- [031 模块顶层 HTTP Handler 分责](031-module-top-level-http-handler/README.md)：已完成；Todo 的 HTTP handler 层从 `binding/http` 迁移到模块顶层 `handler`，`binding/http` 只做代码优先契约与运行期装箱，每层职责分明。
- [032 i18n 配置职责边界与集中声明](032-i18n-config-boundary/README.md)：已完成；`kernel/app/i18n` 集中声明默认配置并统一 `./locales`，logger/database 组件自声明默认值（不再复用 `pkg/*.DefaultConfig()`），cache 的 `redisstore.DefaultTagPrefix` 作为基础默认常量回退保留，并把「应用层不得隐式依赖通用库默认值」纳入架构门禁与业务 i18n 接入文档。
- [033 业务模块统一契约与 binding 对齐](033-module-contract-alignment/README.md)：已完成；把统一绑定契约（HTTP/config/cli/migration/i18n/middleware）固化到模块开发指南，落地业务流程模块自有 i18n binding（Todo `binding/i18n`），文档化 Ops（独立 management）/Auth（横切）/Migration（纯 CLI）形态，并保留 032 的 pkg/kernel-app 配置边界。
- [034 业务模块装配纯度与文档一致性](034-module-wiring-purity/README.md)：已完成；`internal/composition` 通过 `applicationHTTPModules()` 收敛 HTTP 契约接入，消除 `ops.go`/`service.go` 对 `todohttp.ModuleContract()` 的直接反向读取，生成器注册点 `registeredModules()` 与装配流程文档化，并让权威文档与实现一致。
- [035 后台任务能力装配（幂等 / 重试 / 执行记录）](035-background-task-capabilities/README.md)：已完成；为需要幂等、失败重试、执行记录的业务模块（订单、支付、库存为例）装配 `pkg/execution -> kernel/app/execution -> composition` 底层执行能力（默认内存 backend + 组件开关），重试复用 `pkg/resilience`（`d42e044`）；并在 `pkg/execution` 追加外部依赖故障恢复治理 `RecoveringStore`（Healthy/Degraded/Recovering 状态机、有界记录缓冲 + 溢出策略、退避/抖动/最大频率探测、可用性验证、恢复后回放并原子切回主实现）与执行记录异步持久化 `AsyncRecorder`，`kernel/app/execution` 装配恢复治理 + 异步记录、按 `Config.Policies`/`Execution.PolicyName` 提供命令式按模块策略隔离，并经注入 Logger 输出状态变化日志、`Access.Recovery()`/`Access.Health()` 导出恢复治理观测；缓存 Cache-primary/数据库外部主存储接入列为下一增量。
- [036 业务模块接入 execution（Todo 落地）](036-business-module-execution-adoption/README.md)：接入指南（纯文档）已完成；为需要幂等、失败重试、执行记录的业务模块沉淀单一权威入口 `docs/development/execution-capability.md`（声明式命名策略、`OperationExecutor` 用法、错误语义、Trace、观测与多实例边界，并由模块开发指南索引）；非文档的 Todo 真实接入（service 窄 port + composition 注入 + 命名策略）待计划确认。

下一个任务序号为 `037`。已完成记录只保存历史证据；当前行为必须回到根 [README](../../README.md) 和对应主题文档确认。
