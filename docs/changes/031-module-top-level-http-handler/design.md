# 设计：模块顶层 HTTP Handler 分责（031）

## 1. 设计结论

采用单轨「模块顶层 handler 层 + binding 只做绑定」：

```text
internal/module/todo/
├── model/
├── service/
├── repo/
├── handler/                  # 模块顶层 HTTP handler 层
│   ├── dto.go                # HTTP DTO 与 model↔DTO 映射
│   ├── handler.go            # Operations 接口 + Handler + NewHandler + ActorAccess + 错误呈现
│   └── handler_test.go
├── binding/
│   ├── config/  cli/  migration/
│   └── http/                 # 只做契约 + 装箱
│       ├── contract.go
│       ├── contract_module.go
│       └── handlers.go
└── module.go
```

## 2. 为什么不是现有结构

现有 `binding/http` 把业务 handler（`handler.go`、`dto.go`）与绑定职责（`contract.go`、`contract_module.go`、`handlers.go`）放在同一目录。`handlers.go` 甚至要 import 同包 `Operations` 类型，导致「绑定」与「业务 handler」语义混叠，读代码无法从目录名判断各文件职责层次。

## 3. handler 层（internal/module/todo/handler）

- `Operations` 接口：模块 HTTP operation 的窄契约，签名使用模块自有 DTO。
- `Handler` 结构 + `NewHandler`：校验依赖后持有 `service.UseCases`、`i18n.Translator`、`ActorAccess`。
- `ActorAccess` 窄端口：从 context 读当前业务主体。
- DTO（`dto.go`）：`Todo`、`TodoList`、`CreateTodoRequest`、`ListTodosParams`、`TodoStatus`，以及 `todoDTO` 映射。
- 错误呈现：`present`/`errorContract` 把 `fault.Code` 映射为 `httpx.StatusError`。

边界：不创建 Router、不加载 OpenAPI、不 import `binding/**`、`internal/transport/**` 或 chi/kin-openapi/nethttp-middleware；只依赖 `model`、`service`、`pkg/httpx`、`pkg/i18n`、`pkg/fault`。

## 4. binding 层（internal/module/todo/binding/http）

- `contract.go`：模块 contract.Schema 描述（`CreateTodoRequest`/`Todo`/`TodoList`/`TodoStatus`/`TodoID`）。
- `contract_module.go`：`ModuleContract()` 返回 `contract.Module`（routes/policies/security/schemas），供 contract-gen 与 composition 使用。
- `handlers.go`：`RuntimeHandlers(handler.Operations) map[contract.OperationID]contract.Handler`，把 typed handler 装箱为 transport 所需 `contract.Handler`。

边界：只依赖 `handler`（.Operations 类型）、`pkg/httpx/contract`、`net/http`；不承载任何业务 handler 实现、DTO 映射或错误呈现。

> 实现细节：`contract.go` 的 schema 描述与 `dto.go` 的 Go 类型是两套不同关注点——schema 供契约/校验，DTO 供 handler 编解码。两者保持各自归属：schema 留在 binding/http，DTO 随 handler 上移。若实施发现 schema 文件同时被 handler 引用，则明确 handler 只依赖自身 DTO 类型，不依赖 schema。

## 5. 装配层（module.go 与 composition）

- `internal/module/todo/module.go`：`NewHTTP` 构造 `handler.NewHandler(...).Operations`；`HTTPModule` 输出 `Operations` 指向新 handler 包。
- `internal/composition/http_api.go`：`newContractDispatcher(handler.Operations)` 用 `todohttp.ModuleContract()` 聚合契约 + `todohttp.RuntimeHandlers(handler.Operations)` 装箱，生成 `contract.Dispatcher`，连接 Auth/Ops。
- `composition/service.go`、`ops.go`：继续用 `todohttp.ModuleContract()` 生成 Auth policy 与 Observability operations。
- `composition/todo_authorization.go`：`ActorAccess` 断言改为指向新 handler 包的 `ActorAccess`。

## 6. 依赖方向（单向）

```text
model <- service <- repo
           ^          ^
     handler     module.go <- composition
        |
     binding/http (只依赖 handler.Operations + contract)
```

- handler 不 import binding/transport/第三方 HTTP 框架。
- binding/http 可 import handler（仅类型），不反向实现业务逻辑。
- module.go/composition 是唯一同时连接两者之处。

## 7. 文件影响

预计实施范围：

- 新增 `internal/module/todo/handler/{dto.go,handler.go,handler_test.go}`（迁移自 binding/http）。
- 修改 `internal/module/todo/binding/http`：删除已迁走的 handler/dto 文件，保留 `contract.go`、`contract_module.go`、`handlers.go`；`handlers.go` 改依赖新 `handler.Operations`。
- 修改 `internal/module/todo/module.go`（导入与 `Operations`/`ActorAccess`/`NewHandler` 引用）。
- 修改 `internal/composition/http_api.go`、`composition/todo_authorization.go`（导入与引用）。
- 修改 `internal/transport/http/routes_test.go`、`internal/tools/contract-gen/main.go`（如导入别名变化）。
- 修改 `internal/kernel/composition/architecture_test.go`（门禁与 fixture）。
- 同步 `api/README.md`、`internal/module/README.md`、Todo README、模块开发指南。

预计不修改 `api/openapi.yaml` 语义、公开契约、`go.mod`/`go.sum`、配置、migration、Kernel、Host 或 listener。若实施发现必须修改这些语义，触发重新确认。

## 8. 失败语义

- handler 依赖 nil：`NewHandler` 返回错误，候选 generation abort，旧代保留。
- binding `RuntimeHandlers` 缺少某个 operation：`newContractDispatcher` 构造失败，不安装路由，不静默返回 501。
- 迁移后未同步的 import：编译失败，CI 阻断，阻止提交。
- 行为/契约回归：tests 与 oasdiff 失败，阻止提交。

## 9. 架构门禁

新增/修改结构检查（表达通用规则，不只搜 Todo）：

1. 模块顶层 handler 包（`internal/module/<name>/handler`）不得 import `binding/**`、`internal/transport/**` 或 chi/kin-openapi/nethttp-middleware。
2. 模块 `binding/http` 不得包含业务 handler（Operations/Handler/NewHandler/ActorAccess/present/errorContract）实现；只允许 contract + RuntimeHandlers。
3. `handler` 只依赖 model/service/pkg；`binding/http` 只依赖 handler + contract。
4. composition 仍是唯一聚合位置；transport 仍是唯一 route binding 位置。
5. 既有 module owner、cmd、Kernel 与 pkg 依赖方向继续有效。

结构测试应包含正反 fixture；禁止用别名、路径白名单或注释约定代替可执行门禁。

## 10. 验证矩阵

- 定向结构：handler 包不 import binding/transport/第三方；binding/http 无业务 handler；门禁正向 fixture 通过。
- 协议行为：Todo/Auth/I18n/Ops/composition/transport 现有 tests 全部通过；`rtest` process 行为不变。
- 完整门禁：`gofmt -l .`、`go generate ./...`、`go mod tidy -diff`、`go test ./...`、`go test -race`（受影响）、`go vet ./...`、`go build ./cmd/app`、`git diff --check`、oasdiff breaking（对 030 基线无 ERR）。

`go generate` 会写工作区，只能在用户确认后的实施阶段执行。验证后审阅完整 diff，只提交 031 范围。

## 11. 单轨迁移顺序

1. 新建 `internal/module/todo/handler`，把 handler/dto 迁入并改包名。
2. 更新 `module.go`、`composition/http_api.go`、`composition/todo_authorization.go` 导入与引用。
3. `binding/http` 保留契约 + 装箱，`handlers.go` 改依赖新 `handler.Operations`。
4. 删除旧 binding/http 中的业务 handler 文件。
5. 更新架构门禁与权威文档。
6. 执行全量验证，审阅完整 diff 后提交（只提交 031 范围；不 push）。

不保留旧包 alias、双 handler 或兼容 wrapper。

## 12. 重新确认触发器

出现以下任一事实时退回研究和待确认：

- 必须改变公开 HTTP 行为、契约、版本或兼容性；
- handler 包与 binding 包产生循环依赖或反向依赖；
- 必须新增第三方依赖或引入动态注册；
- 必须修改 Kernel/Host、listener、配置、migration 或 module Contribution；
- 必须保留旧包 alias/双轨。
