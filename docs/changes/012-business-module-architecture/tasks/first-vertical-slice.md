# VSL：首个真实业务垂直切片

## 后续实施状态

本文件保留 012 当时尚未绑定业务的候选任务，不再是当前 Todo 实施清单。FND/GOV-F 与 `AC-BIZ-GATE-001` 已完成；真实 Todo 用例随后在 [014](../../014-todo-business-vertical-slice/README.md) 中完成独立研究、设计、确认和实施。014 使用真实 SQLite Repository，不以空 CRUD 或内存假 Repository 冒充验收。

以下 VSL-001..007 保留为历史拆分，不回写成 014 的任务证据。未来新业务仍需独立研究、计划和确认。

## VSL-001：确认业务用例与验收

- 工作量：M。
- 依赖：FND/GOV-F 已实现并通过 `AC-BIZ-GATE-001`；用户提供真实需求；业务详细设计重新确认。
- 完成条件：模块所有者、actor、输入/结果、不变量、错误、数据/事务、入口、I18n、缓存必要性和验收数据明确；非目标清晰。
- 当前状态：阻塞。

## VSL-002：Domain/Application 与 caller-owned ports

- 工作量：M-L，取决于用例。
- 依赖：VSL-001。
- 完成条件：
  - 业务目录按能力命名，只建立必要层；
  - Command/Query/Result、Service、Repository/跨模块 port 使用项目自有类型；
  - 不导入 Kernel、HTTP/CLI、GORM/Redis 等基础设施；
  - 成功、业务错误、取消、超时和依赖失败单元测试通过。

## VSL-003：Database Adapter 与事务

- 工作量：L。
- 依赖：VSL-002；真实 schema/数据源确认。
- 完成条件：
  - persistence Record 与 domain/result 显式转换；
  - Access `Use/WithinTx` 边界内完成访问，Client/Tx/Repository 不逃逸；
  - Repository 或模块专用 UnitOfWork 合约覆盖提交、回滚、取消和清理错误；
  - migration/外部数据库操作必须另有明确授权和回滚计划。

## VSL-004：Cache Adapter（条件任务）

- 工作量：M。
- 依赖：VSL-001 有量化缓存需求；VSL-002/003。
- 完成条件：key/TTL/version/失效 owner、hit/miss/stale/unavailable 语义和 Client Close 全部可测试。
- 跳过条件：没有性能、负载或可用性证据时明确记录“不需要”，不创建占位实现。

## VSL-005：HTTP Adapter 与 I18n

- 工作量：L。
- 依赖：FND/GOV-F 已通过、业务 HTTP 详细设计已重新确认、VSL-002，且 VSL-001 需要 HTTP。
- 完成条件：
  - DTO/Handler/Route/Presenter 边界明确；Handler 不直接访问 Repository；
  - 全局技术 Middleware 与 route policy 顺序测试通过；
  - fault/reason 到安全 HTTP 协议和 namespaced I18n message 的映射确认；
  - message 资源加载方式已解决，不使用自动扫描；
  - 真实 API 成功/失败/取消验收通过。

## VSL-006：Application CLI Adapter（条件任务）

- 工作量：M。
- 依赖：FND-006 已验证、业务 CLI 详细设计已重新确认、VSL-002，且 VSL-001 需要 CLI。
- 完成条件：CLI 调用同一 Service，不调用 Handler；参数、输出、错误/exit/I18n、取消和资源停止通过验收。
- 跳过条件：首个用例不需要 CLI 时不创建占位命令。

## VSL-007：Composition、生命周期与端到端验收

- 工作量：L。
- 依赖：VSL-002 至少一个入站 Adapter；所需基础设施任务完成。
- 完成条件：
  - composition 显式传入最小 Capability 和跨模块 port；
  - contribution 在监听前验证；
  - Kernel/module/HTTP 或 command 的正序启动、反序停止可执行；
  - 真实数据/协议路径通过端到端验收，未执行的 Docker/远程门禁明确标注；
  - 没有旧入口、重复实现或资源泄漏。

## 工作量与拆分原则

业务复杂度未知，因此不承诺人日。每个 VSL 任务在 VSL-001 后按可独立验证、可单一提交审阅的范围细化，但必须仍属于 012 已确认目标；若拆分会留下新旧双轨或不可运行中间态，应在同一变更中完成迁移。
