# R002 统一 binding 契约与 i18n 接入设计

## 1. 研究问题

如何定义并固化业务模块统一 binding 契约，使每个业务模块按统一方式提供自身的 i18n 语言资源与 binding，同时保留 kernel/app/i18n 装配能力与 032 的 pkg/kernel-app 配置边界？

## 2. 方法与范围

- 只读取仓库内源码、composition、测试与权威文档作为证据，不编写实现。
- 目标是形成统一契约清单与 i18n binding 接入设计，供 requirements/design/tasks 落地。

## 3. 统一 binding 契约清单（目标）

业务模块按需提供，且文档统一声明其声明位置 / 接入方式 / 维护位置：

| Binding / 契约 | 声明位置 | 接入方式 | 维护位置 |
| --- | --- | --- | --- |
| HTTP binding | 模块顶层 `handler/`（适配）+ `binding/http`（`ModuleContract`/`RuntimeHandlers`） | 注册 contract-gen；composition 聚合；transport 绑定 | `internal/module/<name>/handler` 与 `binding/http` |
| config binding | `binding/config` | composition 连接 Config | `internal/module/<name>/binding/config` |
| cli binding | `binding/cli` | cmd 装配命令 | `internal/module/<name>/binding/cli` |
| migration binding | `binding/migration` | composition/migrate 使用 | `internal/module/<name>/binding/migration` |
| i18n binding | 模块内提供语言资源 + binding（见 §4） | 聚合进 kernel/app/i18n 或 composition，再按模块注入 Translator | 模块内语言资源；全局 `./locales` 仍聚合 |
| middleware | `middleware/`（横切） | composition 挂载 | `internal/module/<name>/middleware` |

仅当模块确有 HTTP operation 才要求 HTTP binding；确有用户可见翻译才要求 i18n binding；避免为空造对称空层（沿用 031/032 原则）。

## 4. i18n binding 设计（目标）

现状：语言资源统一在 `./locales/messages.*.yaml`，由 `kernel/app/i18n` 装配全局 Translator，业务模块只是消费注入的 Translator；没有「业务模块自提供语言资源 + binding」。

目标形态（候选方案，实施时定名）：

- 业务模块在模块内声明自身语言资源与 i18n binding，例如：
  - `internal/module/<name>/binding/i18n/locales/messages.zh-CN.yaml`（或模块内 `locales/`）；
  - `internal/module/<name>/binding/i18n/catalog.go`：暴露该模块的静态 message catalog 或 `[]pkg/i18n.MessageFile`/`fs.FS` 资源，供 kernel/app/i18n 或 composition 聚合。
- kernel/app/i18n 或 composition 聚合各业务模块的 i18n binding 资源，装配成统一全局 Translator，再按模块注入 `pkg/i18n.Translator`。这样业务模块以自己的 binding 提供翻译资源，而不只是依赖全局路径。
- 保留 032：`./locales` 仍是默认语言路径；kernel/app/i18n 集中声明默认配置；业务模块有自己的资源来源并可聚合。
- 保留 032 边界：pkg/i18n 只提供能力 + 基础默认；kernel/app/i18n 负责应用层默认与装配；业务模块通过注入的 Translator 消费，不直接读取 pkg/i18n 默认配置。

关键决策点（实施时确认）：
1. 语言资源放模块内 `binding/i18n/locales/` 还是继续统一 `./locales`？倾向：模块内自有资源 + binding，`./locales` 作为聚合/默认源。
2. 聚合机制：业务模块 i18n binding 通过 composition 显式传入 kernel/app/i18n 的 `MessageFiles`/`MessageFS`，还是 kernel/app/i18n 提供注册入口？倾向：composition 显式聚合（保持无全局注册），避免动态注册。
3. 是否在模块 dev guide 与门禁强制「业务模块必须提供 i18n binding」：仅当存在用户可见翻译时必需；纯内部/无翻译模块记录豁免。

## 5. Ops / Auth / Migration 对齐

- **Ops**：management HTTP 是独立 management 监听，不是公开 API。目标：不再用手写固定 ServeMux 的直接形态，统一为模块契约描述 + composition 挂载（或至少文档明确其为独立 management 且不纳入公开 contract-gen）；若要求纳入公开契约则定义 `ModuleContract`。Ops 提供 `HTTPMiddleware`、`Access` 端口、config。
- **Auth**：横切 middleware/port 模块，无自有业务 HTTP operation。文档明确其接入契约（HTTPMiddleware、Access、Authorizer、Audit、CredentialVerifier），不要求 `ModuleContract`。
- **Migration**：纯 CLI/binding 模块（cli+config），无 HTTP、无 i18n。文档明确其形态。

## 6. 保留 032 的 pkg/kernel-app 配置边界

- `pkg/*` 只提供通用能力 + 基础默认行为；`kernel/app/*` 负责应用环境/装配的默认配置与常量，不隐式依赖 `pkg/*.DefaultConfig()`；基础默认常量（如 `redisstore.DefaultTagPrefix`）可作为未声明时的回退保留。
- 本任务核心是业务模块契约补齐 + 统一 binding + 新增模块门禁；边界检查继续由架构门禁 `validateKernelAppConfigOwnership` 保持（032 产物）。

## 7. 新增模块门禁规范（目标）

在业务模块接入文档与项目门禁中明确声明：

- 新增业务模块必须提供哪些 binding：HTTP（若暴露 operation）、config、cli（若需命令）、migration（若持表）、i18n（若用户可见翻译）、middleware（若横切）。
- 必须接入哪些基础契约：代码优先 HTTP 契约（handler/ + binding/http + contract-gen 注册，若 HTTP）、i18n binding（若用户可见翻译）、模块纯内存装配（module.go）、contribution 校验。
- 每类 binding 的声明位置、接入方式、维护位置（见表 §3）。
- 架构门禁可扩展：若模块声明 HTTP binding，必须注册 ModuleContract；若声明 i18n binding，必须提供语言资源来源；防止旧式手写路由（如 Ops 旧形态）重新引入。

## 8. 适用与不适用场景

- 适用：统一 binding 契约、i18n binding 接入、模块门禁。
- 不适用：改变业务逻辑、引入服务注册/动态路由、更换翻译引擎、改变公开 HTTP 语义。

## 9. 局限与剩余未知

- i18n binding 的具体命名/聚合机制、Ops 是否纳入公开契约，需设计阶段决策。
- 当前为只读研究，未运行测试/生成。

## 10. 对 033 的影响

- requirements/design/tasks 必须按本报告形成：统一契约清单、现有模块补齐（Ops HTTP 形态、i18n binding 落地、Auth/Migration 文档说明）、契约固化到 devel 文档与门禁、保留 032 边界。
- 研究门禁通过。
