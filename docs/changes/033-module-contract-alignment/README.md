# 033 业务模块统一契约与 binding 对齐

## 状态

- 当前阶段：研究完成，计划待确认。
- 研究门禁：已通过，证据为 `R001`、`R002`。
- 计划状态：待确认。本任务会修改业务模块源码、binding、测试与权威文档，属于非纯文档实施，必须由用户在计划报告后的后续消息中明确确认后才能开始实施。
- 本轮交付：只新增本变更目录的研究与计划文档，不修改实现文件，不执行状态变更命令，不暂存、不提交、不推送。
- 外部副作用：无。不启动服务、不写数据库、不 push、不 tag、不 release。

## 问题

现有业务模块对「模块统一契约」的落地不一致，主要体现为：

1. **HTTP 契约分层未对齐**：Todo 已按 030/031 采用「模块顶层 `handler/` + `binding/http` 只做契约声明与装箱 + contract-gen 生成」；但 **Ops 仍在 `binding/http/handler.go` 用手写 `http.Handler` + 自建 ServeMux**（hardcoded `/startupz /livez /readyz /build /diagnostics /metrics`），没有 `ModuleContract`、不参与 contract-gen 生成，仍是 030 之前的旧模式。
2. **Auth 无自有 HTTP 契约**：Auth 只有 middleware 与 adapter，没有自己的 `Operations`/binding/http 代码优先契约。
3. **i18n 未作为业务模块 binding 契约接入**：目前只有 Todo 通过注入的 `pkg/i18n.Translator` 消费，语言资源统一放在全局 `./locales/messages.*.yaml`，由 `kernel/app/i18n` 处理；**业务模块没有提供自己的 i18n 语言资源与 binding**。目标：每个业务模块按统一方式提供自身 i18n 语言资源与对应 binding，而非仅由底层统一处理。
4. **其他模块级 binding 未统一检查/对齐**：config/cli/migration/admin 等 binding 的声明位置、接入方式、维护位置没有统一的约束说明。
5. **新增模块门禁规范缺失**：没有明确「新增业务模块必须提供哪些 binding、必须接入哪些基础契约（如 i18n binding）、每类 binding 的声明位置/接入方式/维护位置」。

## 计划结论

以 Todo 为现行完整参考，建立并落地一份「业务模块统一契约与 binding 规范」，让现有模块全部对齐：

- 定义统一契约清单：HTTP binding（顶层 `handler/` + `binding/http` 契约/装箱 + contract-gen 注册）、config binding、cli binding、migration binding、**i18n binding（业务模块自有语言资源 + binding）**、middleware（横切）。
- 补齐现有模块：
  - **Ops**：从手写 `http.Handler` 改为代码优先契约路径（若其 HTTP operation 应纳入公开契约），或明确其为独立 management HTTP 而不纳入 contract-gen（需在文档说明适用边界）。
  - **Auth / Migration**：按实际形态对齐契约清单，缺失的 binding（如 i18n binding 若非空则补齐）统一接入。
  - **i18n binding**：设计业务模块自有语言资源 + binding 的标准形态，替换「仅由 kernel/app/i18n 统一处理」，同时保留 kernel/app/i18n 装配全局 Translator 的能力。
- 把契约清单固化到业务模块接入文档与门禁规范，明确新增模块必须提供的 binding、必须接入的基础契约、声明位置/接入方式/维护位置。
- 保留 032 的 kernel/app vs pkg 配置职责边界检查：`kernel/app/*` 封装 `pkg/*` 时不直接依赖应由应用层/使用者声明的默认配置、动态值、可变参数。

## 阅读顺序

1. [R001 业务模块统一契约清单与现状缺口](research/R001-module-contract-inventory/report.md)
2. [R002 统一 binding 契约与 i18n 接入设计](research/R002-unified-binding-contract-and-i18n/report.md)
3. [需求](requirements.md)
4. [设计](design.md)
5. [任务与确认状态](tasks.md)

本记录是任务级计划，不替代 [应用模块开发指南](../../development/application-module-development.md)、[模块边界说明](../../../internal/module/README.md)、[032 i18n 配置职责边界](../../../docs/changes/032-i18n-config-boundary/README.md) 或 [HTTP API 契约](../../../api/README.md)。确认实施后必须同步这些当前权威文档。
