# R001 业务模块装配流程与职责审计

## 1. 研究问题

业务模块完成后，接入是否只需在 `internal/composition` 一处装配/依赖注入即可完成？是否存在为接入某模块而必须跨层修改多个位置、反向依赖或职责越界的情况？

## 2. 方法与范围

- 只读取仓库内 composition、contract-gen、模块 README、开发指南作为证据，不编写实现。
- 复核对象：`internal/composition/*.go`、`internal/tools/contract-gen/main.go`、`internal/module/README.md`、模块开发指南。

## 3. 现状（事实）

### 3.1 接入一个 HTTP 业务模块需触碰的路径（以 Todo 为实例）

若新增/接入一个 HTTP 业务模块，除了模块本身，至少需要手改：

1. `internal/composition/http_api.go`：`newContractDispatcher(todoOperations todohandler.Operations)`（Todo 命名、Todo 类型参数）、`newOperationGate`（Todo 用 Auth）——若第二个 HTTP 模块出现，需要新的 dispatcher/gate 或泛化。
2. `internal/composition/service.go`：`operationPolicies()` 硬编码 `module := todohttp.ModuleContract()`，把 Todo 契约的 `operation.Policy` 复制成 `authmodel.Policy` 列表。
3. `internal/composition/ops.go`：`module := todohttp.ModuleContract()`（第 112 行）——composition 为给 Observability 生成 operations，反向读取 Todo 的 HTTP 契约包。这是 composition 对具体模块契约包的直接 import。
4. `internal/composition/todo.go`：Todo 的 CLI/本地执行器（`prepareTodo`/`executeTodo`）——Todo 专属装配。
5. `internal/composition/generation.go`：长驻 Service 装配（auth/ops/todo module、contractDispatcher、NewRouteBinding、management）。
6. `internal/tools/contract-gen/main.go`：`registeredModules()` 需追加一行 `todohttp.ModuleContract()`——这是 **composition 之外的注册点**（030 计划的既定允许，但对“仅 composition 接入”而言是额外注册位置）。

### 3.2 契约知识扩散与跨模块反向读取

- `service.go` 的 `operationPolicies()` 与 `http_api.go` 的 `contractDispatcher` 都直接 import `internal/module/todo/binding/http` 并调用 `todohttp.ModuleContract()`。即 **Todo 的契约包被 composition 多个文件消费**，Todo 契约知识不局限于 Todo 与 transport。
- `ops.go` 为完善 Observability 的 operation inventory，也 import `todohttp.ModuleContract()` —— 一个与 Todo 无关的横切模块（Ops）读取 Todo 的 HTTP 契约，属于较明显的“为取数而跨模块反向依赖”。

### 3.3 反向依赖 / 职责衡量

- `internal/composition` 本来就是唯一同时知道 Kernel Capability 与应用模块的位置，这一点是合理的（模块开发指南也声明）。所以 composition 指向模块本身不算偏移。
- 但 **composition 内部把“Todo 具体实现”散落在多个文件**（http_api.go/service.go/ops.go/todo.go/generation.go），以及 **ops.go/service.go 直接 import Todo 契约包**，说明“契约知识”和“装配逻辑”没有收敛到单一装配处，而是被多文件复制/反向读取。

## 4. 推断

1. 现有接入流程“基本由 composition 承担”，方向上符合“先通用再使用”。但**并未收敛为单一装配入口**：Todo 绑定类型、契约 policy 生成、dispatcher 都在 composition 内四处出现，新增第二个 HTTP 模块会立刻触发重复/复制模式。
2. `ops.go` 读取 `todohttp.ModuleContract()` 是**反向依赖/耦合**——Ops 与 Todo 无业务关系，却为生成 observability operations 依赖 Todo 契约包；这偏离了“各层只承担自身职责”。
3. `contract-gen` 的 `registeredModules()` 是 composition 之外的注册点。它属于 build-time 生成器（与运行时组合分离），030 计划已明确允许；但对“接入只需改 composition”而言，它仍是额外注册，需要在设计阶段决定是保留（并写文档为“生成器注册点”）还是收敛。
4. Todo 专属的 `contractDispatcher`/`operationPolicies` 说明 composition 没有为“模块 N 的 HTTP 契约 → 运行时 handler → policy”提供统一接收点。

## 5. 适用与不适用场景

- 适用：模块装配纯度、composition 职责收口、消除跨模块反向读取、统一 HTTP 契约接入点。
- 不适用：改变公开 HTTP 契约、引入动态注册/Service Locator、重构 Kernel 生命周期、改动 `pkg/*` 通用契约。

## 6. 局限与剩余未知

- 尚未运行测试/生成；只读复核。
- “是否把 `registeredModules()` 并入 composition 或保留”需设计阶段定案。
- 若引入第二个 HTTP 模块，可实证现有模式是否必然复制；当前仅 Todo 一个，倾向推断。

## 7. 对 034 的影响

- 计划需明确：将 HTTP 契约接入（dispatcher/gate/policy/itinerary）收敛为 composition 内统一接收点；消除 `ops.go`/`service.go` 对 Todo 契约包的重复 import 或反向读取；界定 `registeredModules()` 的注册归属与文档说明；同步权威文档。
- 研究门禁通过（事实与推断已分离，剩余未知不妨碍形成计划）。
