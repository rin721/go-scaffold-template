# 034 业务模块装配纯度与文档一致性

## 状态

- 当前阶段：研究完成，计划待确认。
- 研究门禁：已通过，证据为 `R001`、`R002`。
- 计划状态：待确认。本任务会修改 `internal/composition`、可能涉及 `contract-gen`/权威文档/门禁，属于非纯文档实施，必须由用户在计划报告后的后续消息中明确确认后才能开始实施。
- 本轮交付：只新增本变更目录的研究与计划文档，不修改实现文件，不执行状态变更命令，不暂存、不提交、不推送。
- 外部副作用：无。不启动服务、不写数据库、不 push、不 tag、不 release。

## 问题

用户要求评估项目是否真正贯彻「先通用，再使用」，并检查各层职责、接入流程纯度：

1. **装配流程不够纯粹**：理想上，业务模块完成后只应在 `internal/composition` 一处手动装配 + 依赖注入即可接入；但审计发现，接入一个新 HTTP 业务模块需要跨多个文件手改：
   - `http_api.go` 的 `newContractDispatcher(todoOperations todohandler.Operations)` / `operationGateAdapter` 是 Todo 命名、Todo 专属；
   - `service.go` 的 `operationPolicies()` 硬编码 `todohttp.ModuleContract()` 复制契约 policy；
   - `ops.go` 反手 import `todohttp.ModuleContract()` 只为推导 observability operations（跨模块反向读取 Todo 契约）；
   - `todo.go` CLI executor、`generation.go` 装配都需随模块扩展；
   - `internal/tools/contract-gen/main.go` 的 `registeredModules()` 是 composition 之外的注册点。
2. **是否存在反向依赖 / 职责越界**：`composition/service.go` 与 `ops.go` 直接依赖 `internal/module/todo/binding/http`（Todo 的契约包），使 composition 与 Todo 强耦合、Todo 契约被多处扩散消费。
3. **文档与实现一致性**：需要核对架构说明、模块接入方式、binding 契约、装配流程、配置方式、扩展方式与门禁规范是否与当前代码一致，避免“文档描述尚未具备的能力”或“代码已变但文档停留在旧设计”。

## 计划结论

- 建立一份「现有装配被多处扩散消费 / Todo 专用涤具」的现状梳理（R001），确认哪些是合理装配职责、哪些是职责/依赖偏移。
- 建立一份「权威文档 vs 当前实现」一致性核对（R002），列出所有偏移点。
- 提出修正方案：让模块接入收敛为“在 composition 一处加装配/注入”，消除 Todo 命名涤具与跨模块契约读取、收敛 `registeredModules()` 的注册职责，并同步文档。
- 保留 030/031/032/033 已建立的契约分层、i18n binding、pkg/kernel-app 边界与既有门禁。

## 阅读顺序

1. [R001 业务模块装配流程与职责审计](research/R001-module-wiring-and-responsibility/report.md)
2. [R002 文档与实现一致性核对](research/R002-doc-implementation-consistency/report.md)
3. [需求](requirements.md)
4. [设计](design.md)
5. [任务与确认状态](tasks.md)

本记录是任务级计划，不替代 [应用模块开发指南](../../development/application-module-development.md)、[模块边界说明](../../../internal/module/README.md)、[HTTP API 契约](../../../api/README.md) 或 [配置说明](../../configuration/README.md)。确认实施后必须同步这些当前权威文档。
