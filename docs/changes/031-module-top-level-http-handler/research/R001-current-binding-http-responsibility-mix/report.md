# R001 当前 binding/http 职责混叠复核

## 1. 研究问题

Todo 模块的 `internal/module/todo/binding/http` 当前实际承载了哪些职责？handler 实现是否确实与「绑定」混在同一包？哪些消费方使用该包，重构 package 会影响哪些文件？

## 2. 方法与范围

- 只读取仓库内源码、测试与权威文档作为证据，不编写实现。
- 复核对象：`internal/module/todo/binding/http/` 下的五个源文件与一个测试文件，及其全部生产消费方。

## 3. 证据（事实）

### 3.1 binding/http 内部职责

`internal/module/todo/binding/http` 现有文件（HEAD b7fca48）：

- `handler.go`：定义 `ActorAccess` 端口、`Operations` 接口、`Handler` 结构、`NewHandler`、`actor()`、`present()`、`errorContract()`，并 `var _ Operations = (*Handler)(nil)`。这是业务 HTTP 语义适配：读 actor → 组装 UseCases 命令/查询 → 返回模块自有 DTO → 错误呈现为 `httpx.StatusError`。
- `dto.go`：定义模块自有 HTTP DTO（`Todo`、`TodoList`、`CreateTodoRequest`、`ListTodosParams`、`TodoStatus`）与 `todoDTO` 映射，`import model`。
- `contract.go`：定义 `CreateTodoRequest`/`TodoStatus`/`Todo`/`TodoList`/`TodoID` 等 `contract.Schema`（供代码优先契约）。
- `contract_module.go`：定义 `ModuleContract() contract.Module`——方法可能随实施微调，职责是声明 route/policy/security/schema。
- `handlers.go`：定义 `RuntimeHandlers(Operations) map[contract.OperationID]contract.Handler`，把 typed handler 装箱为 transport 所需 `contract.Handler`。

结论（事实）：**同一个 package 承载了「运行期业务 HTTP handler」「DTO 与映射」「代码优先契约声明」「运行期装箱」四类不同职责**；其中 handler 实现与 binding（装箱/契约）混在同目录，与「binding/http 只负责协议绑定」的单一职责目标冲突。

### 3.2 消费方

grep 显示以下生产/测试位置 import `binding/http`（别名 `httpbinding`/`todohttp`）并使用其符号：

- `internal/module/todo/module.go`：`httpbinding.ActorAccess`、`httpbinding.Operations`、`httpbinding.NewHandler`。
- `internal/composition/http_api.go`：`todohttp.Operations`、`todohttp.ModuleContract()`、`todohttp.RuntimeHandlers`；并 `var _ httptransport.Dispatcher`。
- `internal/composition/service.go`：`todohttp.ModuleContract()`（生成 Auth policy）。
- `internal/composition/ops.go`：`todohttp.ModuleContract()`（生成 Observability operations）。
- `internal/composition/todo_authorization.go`：`var _ httpbinding.ActorAccess = todoActorAccessAdapter{}`。
- `internal/transport/http/routes_test.go`：`httpbinding.ModuleContract()`。
- `internal/tools/contract-gen/main.go`：`todohttp.ModuleContract()`。

结论（事实）：迁移 handler 后改动面清晰——`module.go`、`composition/http_api.go`、`composition/todo_authorization.go` 需改用新 handler 包的 `Operations`/`ActorAccess`/`NewHandler`；`composition/service.go`、`ops.go`、`transport/routes_test.go`、`contract-gen/main.go` 继续使用 `ModuleContract()`（留在 binding/http 或随职责归属调整导入路径）。

### 3.3 依赖方向现状

- `handler.go`/`dto.go` import `model`、`service`、`pkg/httpx`、`pkg/i18n`、`pkg/fault`（不 import 第三方 HTTP 框架）。
- `handlers.go` import `pkg/httpx/contract` 与 `net/http`。
- `contract.go`/`contract_module.go` import `pkg/httpx/contract`。
- 生产代码无 import 循环；架构门禁已禁止模块 HTTP binding 导入 chi/kin-openapi/nethttp-middleware 与生成包。

## 4. 推断

1. 当前把 handler 放在 `binding/http`，使「binding=绑定」与「handler=应用适配」语义混叠；读代码时无法从一个目录名判断各文件职责层次。
2. 由于 `RuntimeHandlers` 依赖同一包 `Operations`/DTO，只要 handler 与装箱仍同包，就难以独立演进或理解；把 handler 上移到模块顶层可让 `binding/http` 收敛为「只描述与适配 transport 的契约/装箱」。
3. 迁移是纯目录/包级重构，不改公开 HTTP 行为、DTO 形态或契约语义；消费方签名保持，仅改 import 路径与包别名。

## 5. 适用与不适用场景

- 适用：单一模块 HTTP handler 分层；binding 只做绑定；职责可读性提升。
- 不适用：不改变公开协议/路由；不做动态注册；不新增第二个业务模块；不改 Kernel/Host/listener。

## 6. 局限与剩余未知

- 新 handler 包名（`handler`/`http` 等）与是否继续保留 `binding/http` 子目录，需在设计阶段定名；本报告只确立「handler 在模块顶层、binding 只管绑定」这一边界。
- `contract.go`/`contract_module.go` 归属（绑定层或随 handler 迁移）在设计阶段决定；本报告只确认它当前承载的是契约声明职责。

## 7. 对 031 的影响

- 计划必须给出明确目标分层、职责表、迁移顺序与消费方同步清单。
- 架构门禁与 `api/README.md`、`internal/module/README.md`、Todo README、模块开发指南需随实施同步。
- 研究门禁通过：混叠与消费方事实已确认，足以形成计划。
