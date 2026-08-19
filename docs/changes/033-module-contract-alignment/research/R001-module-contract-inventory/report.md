# R001 业务模块统一契约清单与现状缺口

## 1. 研究问题

现有业务模块（Todo/Auth/Ops/Migration）应遵循的统一模块契约是什么？各模块当前的 binding / 接入现状是否完整实现，缺失或仍采用旧方式的有哪些？

## 2. 方法与范围

- 只读取仓库内源码、composition、测试与权威文档作为证据，不编写实现。
- 复核对象：`internal/module/{todo,auth,ops,migration}`、`internal/composition`、`internal/tools/contract-gen`、`internal/module/README.md`、模块开发指南。

## 3. 统一契约清单（推断）

基于 `internal/module/README.md` 与模块开发指南，业务模块应提供/遵循以下 binding 与接入契约：

1. **HTTP binding**（若模块暴露 HTTP operation）：
   - 模块顶层 `handler/` 承载 HTTP 应用语义适配（`Operations`/`Handler`、DTO 与映射、错误呈现、`ActorAccess`）。
   - `binding/http` 只做代码优先契约声明（`pkg/httpx/contract.Module`，`contract_module.go` 的 `ModuleContract`）与运行期装箱（`handlers.go` 的 `RuntimeHandlers`）。
   - 注册到 `internal/tools/contract-gen` 的 `registeredModules()`，生成 `api/openapi.yaml` 与 operation inventory。
   - `handler` 不 import `binding/**`、`internal/transport/**`，不创建 Router、不加载 OpenAPI。
2. **config binding**：模块配置 owner（`binding/config`），供 composition 连接。
3. **cli binding**：模块自有 CLI（`binding/cli`）。
4. **migration binding**：Schema/migration 完成品（如 Todo `binding/migration`）。
5. **i18n binding**：业务模块按统一方式提供自身的 i18n 语言资源与对应 binding（目标形态，见 R002）；而非仅由 `kernel/app/i18n` 统一处理。
6. **middleware**：模块拥有的 HTTP 横切策略。

## 4. 现状核对（事实）

### 4.1 Todo（最完整，参考）

- 模块顶层 `handler/`（`Operations`、`Handler`、DTO、`ActorAccess`、错误呈现）。
- `binding/http`：`contract.go`、`contract_module.go`（`ModuleContract`）、`handlers.go`（`RuntimeHandlers`）。
- 注册于 `internal/tools/contract-gen/main.go` 的 `registeredModules()`（仅 Todo 目前）。
- `binding/config`、`binding/cli`、`binding/migration` 齐全。
- i18n：`handler.go` 消费注入的 `pkg/i18n.Translator`，message ID `todo.error.*`；语言资源放 `./locales/messages.*.yaml`。**未提供业务模块自有的 i18n binding**（语言资源由 kernel/app/i18n 统一路径处理）。

结论（事实）：Todo 是 HTTP/config/cli/migration binding 的现行参考；但**尚无业务模块自有 i18n binding**。

### 4.2 Ops（旧方式）

- `internal/module/ops/binding/http/handler.go` 仍是 **030 之前的手写 `http.Handler` + 自建 `http.ServeMux`**（hardcoded `GET /startupz /livez /readyz /build /diagnostics /metrics`），没有 `Operations` 接口、没有 `ModuleContract`、不注册 contract-gen。
- 该 management HTTP 通过 `binding/http.New` 构造，再由 `module.go` 输出 `ManagementHTTP http.Handler`；composition 用 `httpx.NewServer(&generation.opsModule.Management, generation.opsModule.ManagementHTTP)` 独立监听（generation.go 第 391-399 行），与公开 API 路由（Todo-only `httptransport.NewRouteBinding`）分离。
- `generation_test.go` 断言 management 路由包含 `/startupz /livez /readyz /build /diagnostics /metrics`，`/debug/pprof/` 404。
- `ops` 还提供 `HTTPMiddleware`（telemetry）、`Access`（Auth 连端口）、config binding。

结论（事实）：Ops management HTTP 是**独立 management 监听器，不是公开 API**；但它仍在 `binding/http` 内用手写 ServeMux 实现，且无代码优先契约。Ops module 的 `HTTPMiddleware`、`Access` 也是横切/端口契约。**Ops 与 Todo 的 HTTP 形态不一致**，且 Ops 无 i18n 消费。

### 4.3 Auth（无自有 HTTP 契约）

- `auth`：`module.go`（NewHTTP/NewLocal）、`middleware/http.go`（HTTPMiddleware）、`adapter/{jwt,audit}`、`binding/config`、`service`、`model`。
- Auth 无自有 `handler/` 与 `binding/http`（它是被各方 consume 的认证/授权横切模块，自身不暴露业务 HTTP operation）。
- i18n：无消费。

结论（事实）：Auth 作为横切模块没有自有 HTTP 业务契约；其对外是 `HTTPMiddleware`、`Access`、`Authorizer`、`CredentialVerifier` 等端口。**不属于公开 API 的 `ModuleContract` 目标**，但需在文档明确其接入契约（middleware/port）。

### 4.4 Migration（cli+config）

- `migration`：`binding/cli`、`binding/config`、`service.go`。无 HTTP、无 i18n。
- 编排显式 status/up，不拥有业务表。

结论（事实）：Migration 无 HTTP contract，属于纯 CLI/binding 模块。

### 4.5 i18n 现状

- 只有 Todo handler 消费 `pkg/i18n.Translator`。Auth/Ops/Migration 无 i18n 消费。
- 语言资源统一在 `./locales/messages.zh-CN.yaml`（032），由 `kernel/app/i18n` 装配全局 Translator。
- 没有「业务模块自提供 i18n 语言资源 + binding」的现行形态。

## 5. 推断

1. 统一契约清单应中文文档固化：HTTP binding（顶层 handler + binding/http 契约 + contract-gen 注册）、config、cli、migration、i18n binding、middleware。
2. i18n 应成为业务模块正式 binding 契约：每个业务模块按统一方式提供自身语言资源与 binding（例如模块内语言文件 + `binding/i18n` 暴露自己的 message catalog / Translator 配置），再由 `kernel/app/i18n` 聚合装配，而不是业务模块完全依赖全局统一翻译。
3. Ops 的对齐方向待定：可作为独立 management HTTP 不纳入公开 contract-gen（因为不是公开 API），但应至少统一为不手写固定路由的封装，并在文档明确其适用边界；若要求纳入契约，则需为其定义 ModuleContract。
4. Auth/Migration 按形态纳入文档说明（横切 middleware/port、纯 CLI），并补齐缺失契约（如 i18n binding 若非空）。
5. 新增模块门禁：明确必须提供哪些 binding、必须接入哪些基础契约（含 i18n binding）、声明位置/接入方式/维护位置。

## 6. 适用与不适用场景

- 适用：业务模块契约清单、统一 binding 接入、i18n binding 设计、新增模块门禁。
- 不适用：改变业务模块业务逻辑、引入服务注册/动态路由、重构 Kernel 生命周期、改变公开 HTTP 语义。

## 7. 局限与剩余未知

- 是否将 Ops management HTTP 纳入公开 contract-gen 仍未定案（取决于是否视为公开 API），需设计阶段决策。
- 业务模块自有 i18n 资源的具体文件/naming/注册机制需 R002 设计。
- 当前为只读复核，未运行测试/生成。

## 8. 对 033 的影响

- 计划必须明确：统一契约清单、各模块对齐任务（尤其 Ops HTTP 形态、i18n binding 落地）、Auth/Migration 文档说明、i18n 语言资源维护位置、契约固化与门禁。
- 保留 032 的 kernel/app vs pkg 配置边界检查（非本任务核心，但需维持）。
- 研究门禁通过。
