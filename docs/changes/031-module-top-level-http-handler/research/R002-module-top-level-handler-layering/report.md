# R002 模块顶层 handler 分层目标与边界

## 1. 研究问题

把 handler 上移到模块顶层后，handler 层与 binding 层各自的目标职责如何划分，包名/目录如何组织，依赖方向如何保持单向，迁移顺序与门禁更新是什么？

## 2. 方法与范围

- 只使用仓库内当前源码、测试与架构门禁作为证据，不编写实现。
- 目标是确定分层与迁移边界，为 requirements/design/tasks 提供依据。

## 3. 目标分层（推断）

### 3.1 模块顶层 handler 层：`internal/module/todo/handler`

承载「HTTP 应用语义适配」，即模块对 HTTP 请求/响应的业务适配：

- `Operations` 接口（调用方拥有的窄 HTTP 契约）；
- `Handler` 实现 + `NewHandler`（read actor → 组装 UseCases 命令/查询 → 返回 DTO → 错误呈现）；
- `ActorAccess` 窄端口；
- HTTP DTO 类型与 `model ↔ DTO` 映射（`dto.go`）；
- `present`/`errorContract` 错误呈现。

边界：不创建 Router、不加载 OpenAPI、不 import `binding/**`、不 import `internal/transport/**`、不使用第三方 HTTP 框架（chi/kin-openapi/nethttp-middleware）。它只依赖 `model`、`service`、`pkg/httpx`、`pkg/i18n`、`pkg/fault`。

### 3.2 binding 层：`internal/module/todo/binding/http`

收敛为「只描述与适配 transport」的协议绑定：

- `contract.go` / `contract_module.go`：代码优先契约声明（schemas、`ModuleContract()`，routes/policies/security）。
- `handlers.go`：`RuntimeHandlers(handler.Operations) map[contract.OperationID]contract.Handler` 装箱。

边界：只依赖 `handler`（.Operations 类型）、`pkg/httpx/contract`、`net/http`；不承载任何业务 handler 实现、DTO 映射或错误呈现。

### 3.3 装配层

- `internal/module/todo/module.go`：构造 `handler.NewHandler(...)` 得到 `Operations`，并（如需要）把 `Operations` 与 `ModuleContract`/`RuntimeHandlers` 交给 composition 或作为 HTTPModule 输出。
- `internal/composition`：聚合 handler `Operations` + binding `ModuleContract`/`RuntimeHandlers` 生成 `contractDispatcher`，连接 Auth/Ops。

### 3.4 依赖方向

```text
model <- service <- repo
           ^          ^
     handler     module.go <- composition
        |
     binding/http (只依赖 handler.Operations + contract)
```

关键单向约束：
- `handler` 不得 import `binding/http`；
- `binding/http` 可 import `handler`（仅取 `Operations` 等类型），不得反向依赖业务实现新增逻辑；
- `module.go` / `composition` 是唯一同时看 handler 与 binding 的位置。

## 4. 目录/包组织建议

```text
internal/module/todo/
├── model/  service/  repo/  module.go
├── handler/
│   ├── dto.go
│   ├── handler.go
│   └── handler_test.go
└── binding/
    ├── config/  cli/  migration/
    └── http/
        ├── contract.go
        ├── contract_module.go
        └── handlers.go
```

包名与 `contract.go`/`contract_module.go` 的最终归属（留在 binding/http 或随 handler）在设计阶段定名；本报告确立边界语义即可。

## 5. 迁移顺序（单轨）

1. 新建 `internal/module/todo/handler`，把 `handler.go`（Operations/Handler/ActorAccess/错误呈现）、`dto.go` 及其测试迁入并改包名。
2. 更新 `module.go`、`composition/http_api.go`、`composition/todo_authorization.go` 导入指向新 handler 包。
3. `binding/http` 保留 `contract.go`/`contract_module.go`/`handlers.go`，`handlers.go` 改为依赖新 `handler.Operations`；必要时把契约声明与装箱拆分文件说明职责。
4. 删除旧 `binding/http` 中已迁走的业务 handler 文件，保留 binding 职责文件。
5. 更新架构门禁、权威文档同步，执行全量验证。
6. 不保留旧包 alias、双 handler 实现或兼容 wrapper。

## 6. 适用与不适用场景

- 适用：模块顶层 handler 分层、binding 职责收口、改善阅读。
- 不适用：不改公开 HTTP 行为、不改契约、不新增第二个业务模块、不做动态注册、不改 Kernel/Host/listener。

## 7. 局限与剩余未知

- handler 包名、契约文件归属、`HTTPModule` 输出形状是否变化，在设计阶段定名/定案。
- 本次为只读研究，未运行测试或生成命令。

## 8. 对 031 的影响

- 计划的 requirements/design/tasks 必须按本报告分层与顺序编写。
- 架构门禁需新增/修改：模块顶层 handler 位置与依赖方向、binding 只依赖 handler + contract、禁止 handler import binding/transport 与第三方 HTTP 框架。
- 研究门禁通过：分层与边界已确定。
