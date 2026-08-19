# 设计：业务模块统一契约与 binding 对齐（033）

## 1. 设计结论

建立并落地统一的业务模块契约规范，让现有模块（Todo/Auth/Ops/Migration）全部对齐，并把契约清单固化到模块开发指南与门禁。核心是「现有业务模块契约补齐 + 统一 binding 接入 + 新增模块门禁规范落地」。

## 2. 统一 binding 契约清单（目标权威）

| Binding / 契约 | 声明位置 | 接入方式 | 维护位置 |
| --- | --- | --- | --- |
| HTTP binding | 模块顶层 `handler/` + `binding/http`（`ModuleContract`/`RuntimeHandlers`） | 注册 contract-gen；composition 聚合；transport 绑定 | `internal/module/<name>/handler` 与 `binding/http` |
| config binding | `binding/config` | composition 连接 Config | `binding/config` |
| cli binding | `binding/cli` | cmd 装配命令 | `binding/cli` |
| migration binding | `binding/migration` | composition/migrate 使用 | `binding/migration` |
| i18n binding | 模块内语言资源 + binding（见 §3） | 聚合进 kernel/app/i18n 或 composition，注入 Translator | 模块内语言资源 |
| middleware | `middleware/`（横切） | composition 挂载 | `middleware/` |

- 仅真实需要才创建（HTTP operation → HTTP binding；用户可见翻译 → i18n binding），不为对称造空层。
- 契约单一权威：模块开发指南 + 模块 README，不复制到多处。

## 3. i18n binding 设计

目标：业务模块按统一方式提供自身 i18n 语言资源 + binding，而非仅依赖全局 `./locales` + kernel/app/i18n 统一处理。

预选方案（实施时定名与细化）：
- 每个需要 i18n 的业务模块在模块内提供语言资源，例如 `internal/module/<name>/binding/i18n/locales/messages.<lang>.yaml`，并由 `binding/i18n` 暴露一个窄契约（如 `MessageFiles() []string` 或 `MessagesFS() fs.FS` / 静态 catalog）。
- kernel/app/i18n 或 composition 显式聚合各业务模块的 i18n binding，装配为统一全局 Translator；再按模块注入 `pkg/i18n.Translator`。
- 保留 032：`./locales` 仍是默认语言路径与聚合源；kernel/app/i18n 集中声明默认配置（默认语言、缺失行为、`LocalesDir`）；业务模块通过注入的 Translator 消费，不直接读 `pkg/i18n` 默认配置。
- 不引入动态注册 / Service Locator；聚合由 composition 显式完成，保持依赖清晰。

## 4. 现有模块对齐

### 4.1 Ops
- management HTTP 是独立 management 监听（`managementRoute`/`managementServer`），**不是公开 API**（composition/generation.go 独立监听，`/startupz /livez /readyz /build /diagnostics /metrics`）。
- 对齐方向：将 `binding/http/handler.go` 的手写固定 ServeMux 收敛为模块自有管理路由封装，并在文档明确「Ops management 是独立管理监听，不作为公开 API 契约，不参与 contract-gen」，避免旧式手写路由蔓延语义；保留 `HTTPMiddleware`、`Access`、config、probe/build/diagnostics operation model。
- 若用户决策要求 Ops 也可纳入公开契约，则为其定义 `ModuleContract` 并注册 contract-gen；默认不纳入公开 API（因为它是 management 而非业务 API）。

### 4.2 Auth
- 横切 middleware/port 模块：无自有业务 HTTP operation。文档明确其接入契约（`HTTPMiddleware`、`Access`、`Authorizer`、`Audit`、`CredentialVerifier`），不要求 `ModuleContract`。若 Auth 引入用户可见翻译则接入 i18n binding（当前无，记录豁免）。

### 4.3 Migration
- 纯 CLI/binding 模块（cli+config），无 HTTP、无 i18n。文档明确其形态。

### 4.4 Todo（参考）
- 保持 030/031 分层与 contract-gen 注册；补齐 i18n binding（若采用模块自有资源形态），把 `todo.error.*` 消息从全局 `./locales` 迁移/声明到模块 i18n binding 并提供给聚合。

## 5. 文件影响（预计划，实施时定名）

- 新增/修改：各业务模块 i18n binding（`binding/i18n` + 语言资源）；Ops 管理路由封装；模块 README 与模块开发指南（契约清单、i18n 接入、Auth/Migration 形态、新增模块门禁）。
- 修改：`internal/tools/contract-gen` 若维护注册清单（保持 HTTP 注册）；`internal/composition`（聚合 i18n binding / 资源）。
- 门禁：`internal/kernel/composition/architecture_test.go` 扩展「HTTP binding → ModuleContract 注册」「i18n binding → 语言资源来源」；防旧式手写路由。
- 保留：`validateKernelAppConfigOwnership`（032）+ `./locales` 默认源 + `contract-gen` 生成物语义。

## 6. 失败语义

- i18n binding 资源缺失或聚合失败：kernel/app/i18n 或 composition 装配失败 → 候选 generation abort，旧代保留。
- HTTP bound 模块未注册 ModuleContract：门禁测试失败，阻止提交。
- 旧式手写路由重新引入：门禁 fixture 失败。
- 行为/契约回归：tests 与 contract-gen 幂等失败。

## 7. 验证矩阵

- 定向单元：各模块 i18n binding / Ops 管理封装 / 契约注册。
- 集成/composition：Todo/Auth/Ops/Migration tests、composition/generation 集成测试、management 接受测试（`/startupz /livez /readyz /build /diagnostics /metrics`、`/debug/pprof/` 404）通过。
- 门禁：MODULE ModuleContract 注册、i18n binding 资源来源、防旧式手写路由、`validateKernelAppConfigOwnership`、完整架构 fixture。
- 完整：`gofmt -l .`、`go generate ./...` 幂等、`go mod tidy -diff`、`go test ./...`、`go test -race`（受影响）、`go vet ./...`、`go build ./cmd/app`、`git diff --check`。

## 8. 单轨迁移顺序

1. 先固化统一契约清单到模块开发指南（文档先行，作为实现依据）。
2. 落地 i18n binding：先 Todo（参考），再按需扩展 Auth/Ops/Migration；聚合进 composition/kernel/app/i18n。
3. 对齐 Ops 管理路由封装与文档边界；Auth/Migration 文档说明。
4. 扩展门禁（ModuleContract 注册、i18n binding 资源、防旧式路由）与 fixture。
5. 同步模块 README、配置说明、变更索引；全量验证后提交（只提交 033 范围；不 push）。

不保留旧式手写路由、双套 i18n 路径或兼容 wrapper。

## 9. 重新确认触发器

- 必须改变公开 HTTP 契约、版本或兼容性；
- 必须引入服务注册 / 动态路由 / Service Locator；
- 必须更换 i18n 第三方或引入 i18n 平台；
- 必须重构 Kernel 生命周期 / config 框架 / `pkg/i18n.Translator` 契约；
- 必须新增第三方依赖；
- Ops 是否纳入公开契约与用户期望冲突需重新决策。
