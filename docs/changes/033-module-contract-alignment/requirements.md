# 需求：业务模块统一契约与 binding 对齐（033）

## 1. 依据

- `R001`：现有模块契约不一致——只有 Todo 走 030/031 分层；Ops management HTTP 仍是手写 ServeMux 旧方式；Auth/Migration 无自有 HTTP 契约；i18n 只有 Todo 消费且无业务模块自有 i18n binding。
- `R002`：给出一致契约清单、i18n binding 设计、Ops/Auth/Migration 对齐路径、新增模块门禁，并保留 032 的 pkg/kernel-app 配置边界。

## 2. 目标

把现有业务模块（Todo/Auth/Ops/Migration）完整对齐到同一套模块规范，并把该规范固化到业务模块接入文档与项目门禁中，明确新增模块必须提供的 binding、必须接入的基础契约（含 i18n binding）、每类 binding 的声明位置/接入方式/维护位置。核心是「现有业务模块契约补齐 + 统一 binding 接入 + 新增模块门禁规范落地」，不只是改 kernel/app 或文档。

## 3. 术语

- **业务模块统一契约**：业务模块应遵循的 binding/接入集合（HTTP、config、cli、migration、i18n、middleware）。
- **i18n binding**：业务模块按统一方式提供自身语言资源与对应 binding，而非仅依赖全局 `./locales` + `kernel/app/i18n` 统一处理。
- **kernel/app vs pkg 配置边界**：`pkg/*` 只提供通用能力+基础默认；`kernel/app/*` 负责应用层默认与装配，不隐式依赖 `pkg/*.DefaultConfig()`（032 已建立并保留）。

## 4. 功能要求

### `REQ-001` 定义并固化统一 binding 契约清单

在业务模块接入文档与门禁中明确：HTTP binding（模块顶层 `handler/` + `binding/http` 契约/装箱 + contract-gen 注册）、config binding、cli binding、migration binding、i18n binding、middleware，以及每类的声明位置/接入方式/维护位置。仅按真实需要创建（沿用 031/032：不为空造对称层）。

### `REQ-002` i18n 作为业务模块正式 binding 契约接入

业务模块按统一方式提供自身的 i18n 语言资源与对应 binding（例如模块内 `binding/i18n/locales/` + 暴露静态 catalog / `[]MessageFile` / `fs.FS`），由 kernel/app/i18n 或 composition 显式聚合为全局 Translator 后按模块注入。业务模块不得再只用全局 `./locales` 散落声明，也不得绕过注入直接读 `pkg/i18n` 默认配置。仍由 kernel/app/i18n 负责整体装配与默认配置（保留 032 边界）。

### `REQ-003` 补齐现有模块对齐

- **Ops**：management HTTP 不再保留「binding/http 内手写固定 ServeMux」直接形态——统一为模块契约描述 + composition 挂载（或明确为独立 management 监听且不纳入公开 contract-gen 的适用边界），并在文档说明其 `HTTPMiddleware`/`Access`/config 端口。若决定纳入公开契约则提供 `ModuleContract` 并注册 contract-gen。
- **Auth**：文档明确其横切 middleware/port 契约（HTTPMiddleware、Access、Authorizer、Audit、CredentialVerifier），无自有业务 HTTP operation，不要求 `ModuleContract`；若 Auth 有用户可见翻译则接入 i18n binding。
- **Migration**：文档明确其纯 CLI/binding 形态（cli+config），无 HTTP、无 i18n。
- **Todo**：保持为参考，补齐 i18n binding（若采用模块自有语言资源形态）。
- 各模块缺失/不一致的 binding 统一补齐；删除旧式路径残留。

### `REQ-004` 新增模块门禁规范落地

在业务模块接入文档 + 项目门禁中明确：新增业务模块必须提供哪些 binding、必须接入哪些基础契约（含 i18n binding 若适用）、每类 binding 的声明位置/接入方式/维护位置。架构门禁可扩展：模块若声明 HTTP binding 必须注册 ModuleContract；若声明 i18n binding 必须提供语言资源来源；防止旧式手写路由重新引入。

### `REQ-005` 保留 kernel/app vs pkg 配置边界

继续沿用 032：`pkg/*` 只提供通用能力 + 基础默认；`kernel/app/*` 负责应用层默认与装配，不隐式依赖 `pkg/*.DefaultConfig()`；基础默认常量（如 `redisstore.DefaultTagPrefix`）可作为未声明时的回退保留。本任务不得破坏该边界。

## 5. 质量要求

| 标准 | 可验收定义 |
| --- | --- |
| 统一 | 现有业务模块对统一契约清单的 gap 全部补齐，无旧式手写路由残留 |
| 明确 | 每类 binding 的声明位置/接入方式/维护位置在权威文档单一声明 |
| i18n | 业务模块以自有资源 + binding 提供语言内容，不再散落全局路径 |
| 可验证 | 门禁（含 ModuleContract 注册检查）可执行，防回退 |

## 6. 范围

### 包含

- 建立统一 binding 契约清单与文档；
- 补齐 Ops HTTP 形态（对齐契约路径或明确独立 management 边界）、Auth/Migration 文档说明；
- i18n binding 接入设计并落地（业务模块自有语言资源 + binding + 聚合）；
- 新增模块门禁规范 + 门禁（含 contract-gen 注册检查）；
- 同步模块开发指南、模块 README、配置说明、变更索引；
- 保留 032 的 pkg/kernel-app 配置边界与架构门禁。

### 不包含

- 改变业务模块业务逻辑、版本或公开 HTTP 语义；
- 引入服务注册 / 动态路由 / Service Locator；
- 更换翻译引擎第三方或引入 i18n 平台；
- 重构 Kernel 生命周期或 config Scheduler；
- push、tag、Release、部署或数据库操作。

## 7. 验收标准

1. 统一 binding 契约清单在模块开发指南中单一声明，含每类 binding 的声明位置/接入方式/维护位置。
2. 现有业务模块全部对齐（Ops 不再手写 ServeMux 直接形态或文档明确独立 management 边界；Auth/Migration 按形态说明；Todo 保持参考并补齐 i18n binding 若采用）。
3. i18n binding 落地：业务模块提供自有语言资源 + binding，经 kernel/app/i18n 或 composition 聚合后注入 Translator；无散落全局路径依赖；保留 032 默认数。
4. 新增模块门禁规范与可执行门禁落地（HTTP binding → ModuleContract 注册；i18n binding → 语言资源来源），fixture 通过。
5. 架构门禁 `validateKernelAppConfigOwnership` 与既有测试全部通过；无旧式手写路由残留。
6. `go build ./...`、`go vet ./...`、`gofmt -l .`、`go test ./... -count=1`、`go test -race`（受影响）、`go mod tidy -diff`、`go generate ./...` 幂等、`git diff --check` 通过。
7. 权威文档（模块开发指南、模块 README、配置说明）与实现一致；变更索引更新。

## 8. 确认要求

这是非纯文档实施计划：将修改业务模块源码、binding、测试、门禁与权威文档。只有用户在本计划报告后的后续消息中明确确认 033 当前方案，才能开始实施。若实施中发现必须改变公开 HTTP 契约、i18n 第三方、Kernel 生命周期/配置框架或需要引入服务注册/动态路由，必须退回研究并重新确认。
