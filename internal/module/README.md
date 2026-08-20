# 应用模块

`internal/module` 保存由应用组合根显式选择的纵向模块。这里的 Module 是进程内业务单元，不是 Go module、Kernel Component 或动态插件。每个模块按业务名称收口 Model、Service、Repository、协议 Adapter、binding 与 contribution；底层资源仍由 Kernel 统一创建，模块对象不进入 Kernel Plan。

新增模块必须先按 [应用模块开发指南](../../docs/development/application-module-development.md) 完成真实用例、现有能力、新 Capability、资源 owner、生命周期和当前契约适配性评估，再进入目录与接口设计。

当前已有 Auth、[Ops](ops/README.md)、Migration 与 [Todo](todo/README.md) 模块。Auth 拥有认证/授权/审计，Ops 拥有 management、探针和诊断用例，Migration 编排显式 status/up，Todo 拥有业务实体、对象授权 port 与 SQL migration set；composition 只连接完成品：

```text
model <- service <- repo/binding <- module.go <- internal/composition
                    middleware ───────────────┘
```

- `model` 只表达业务状态与不变量。
- `service` 定义用例以及调用方拥有的窄 port。
- `adapter` 只封装该业务模块专属的第三方实现，并实现模块调用方定义的窄 port；第三方类型、错误、配置对象、Client 和关闭权不得越过 Adapter package，composition 不得穿透模块根导入私有 Adapter。
- `repo`、operation Handler 和各 binding 负责业务拥有的技术/协议转换；模块顶层 `handler` 包可以承载 HTTP 应用适配与 DTO 映射，但模块不创建 Router、不绑定整份应用路由，也不满足完整应用 server interface。
- `middleware` 只实现所属模块拥有的 HTTP 横切策略；不能放入其他模块的业务不变量、Service、Repository 或事务。
- `module.go` 只做纯内存局部装配。
- 模块需要定时任务时只在 `module.go` 构造项目自有 `schedule.Binding` 并通过 `Contribution.Schedules` 输出；统一调度层负责触发和运行治理，详见[定时调度能力](../../docs/development/scheduled-task-capability.md)。
- `internal/composition` 是唯一可以同时知道 Kernel Capability 与应用模块的位置。

HTTP 模块遵循固定的代码优先源头与分层（031 分责）：模块顶层 `handler/` 承载 HTTP 应用语义适配（`Operations`/`Handler`、DTO 与映射、错误呈现、`ActorAccess`），`binding/http` 只做代码优先契约声明（`pkg/httpx/contract.Module`，见 `contract_module.go`）与运行期把 typed handler 装箱为 `contract.Handler`（见 `handlers.go`）；`internal/composition` 聚合模块基础契约与运行期 handler；`internal/transport/http` 从同一份契约一次绑定 OpenAPI 校验、operation gate 与路由；生成器 `internal/tools/contract-gen` 据此渲染 `api/openapi.yaml` 与 operation inventory。`handler` 不 import `binding/**` 或 `internal/transport/**`，不创建 Router、不加载 OpenAPI。新增模块不得复制完整 Router、route binding 或 method/path 表。

各模块契约形态（033 对齐）：**Todo** 是最完整参考（`handler/` + `binding/http` 契约/装箱 + `binding/config`/`binding/cli`/`binding/migration` + `binding/i18n` 模块自有语言资源）。**Ops** 的 management HTTP 是独立 management 监听（`/startupz /livez /readyz /build /diagnostics /metrics`），不属于公开 API，不作为公开契约参与 contract-gen，但必须在 `module.go` 中以模块自有 `ManagementHTTP` 输出并经 composition 挂载，文档明确其边界。**Auth** 是横切 middleware/port 模块（`HTTPMiddleware`、`Access`、`Authorizer`、`Audit`、`CredentialVerifier`），无自有业务 HTTP operation，不要求 `ModuleContract`。**Migration** 只编排 status/up 用例与 cli/config binding，无 HTTP、无 i18n。新增业务模块按 [应用模块开发指南](../../docs/development/application-module-development.md) 的统一 binding 契约清单（HTTP/config/cli/migration/i18n/middleware）只创建真实需要的 binding。

新增业务能力先把真实存在的 Model、Repository、Service、Handler、Adapter、binding、配置、migration/运行单元与 contribution 完整收口到 `internal/module/<name>`，不为对称制造空层。只服务该模块的第三方进入完整路径 `internal/module/<name>/adapter/<technology>` 并完全封装技术影子；不存在无 owner 的全局 `internal/module/adapter`。

只有能力评估同时证明资源跨业务复用且由进程统一选择，才进入完整 `pkg -> internal/kernel/app -> internal/kernel/composition` 链。只满足跨业务复用的普通库可以评估留在 `pkg`，但不自动获得 Kernel 组件；SDK、Client、cache、连接或 goroutine 本身都不是升级理由。

当前 Auth/JWT 遵循“模块内 Adapter、项目 port 输出”；Observability 因跨业务复用且由进程统一选择，已经迁移到 `pkg/observability -> internal/kernel/app/observability -> internal/kernel/composition`。Ops 只消费项目契约，不再拥有或导出 Prometheus/OTel 具体实现，详见 [027 第三方封装与分轨装配](../../docs/changes/027-business-module-third-party-isolation/README.md)。

禁止自动扫描、`init` 注册、Service Locator、全局可变 Registry，以及让 Handler 直接访问 Repository。
