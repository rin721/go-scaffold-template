# 设计：Handler-first HTTP 路由绑定

## 1. 设计结论

采用“模块 operation Handler + 应用静态 aggregate + 单一生成 route binding + 外层 Router”的单轨结构：

```text
internal/module/todo
  Service
    -> binding/http.Handler        只实现 Todo operations

internal/composition
  Todo Handler
    -> strictAPIServer             唯一满足完整 StrictServerInterface

internal/transport/http
  strictAPIServer
    -> OpenAPI validator
    -> strict operation middleware
    -> generated Chi route binding
    -> apiRoutes http.Handler

internal/composition
  apiRoutes
    -> application Router          全局 middleware + 单次 Mount
    -> HTTP Server / Listener
```

静态 aggregate 是显式 composition glue，不是业务实现、路由注册表或万能容器。根 `Mount("/", apiRoutes)` 可以保留，因为它只挂载唯一完整 API route tree；不再允许模块各自隐藏一份完整 route tree。

## 2. 为什么不是现有结构

当前 `httpbinding.New` 的名字看似只构造 Handler，实际同时完成模块 Handler、规范加载、validator、strict middleware、Chi Router 和整份路由绑定。`TodoHandler` 直接实现完整 `api.StrictServerInterface`，把应用规模耦合进模块。

这不影响当前四个 Todo operation 的运行正确性，但使第二模块只能选择“让 Todo 实现别人的方法”或“重复绑定整份路由”。026 删除这一单模块假设。

## 3. 模块 operation Handler

Todo binding 定义只包含 Todo operation 的窄接口。精确名称在实施时可按 Go 语义微调，职责不可扩张：

```go
type Operations interface {
    ListTodos(context.Context, api.ListTodosRequestObject) (api.ListTodosResponseObject, error)
    CreateTodo(context.Context, api.CreateTodoRequestObject) (api.CreateTodoResponseObject, error)
    GetTodo(context.Context, api.GetTodoRequestObject) (api.GetTodoResponseObject, error)
    CompleteTodo(context.Context, api.CompleteTodoRequestObject) (api.CompleteTodoResponseObject, error)
}

type Handler struct {
    service    service.UseCases
    translator i18n.Translator
    actors     ActorAccess
}
```

`NewHandler` 只校验依赖并返回 `*Handler`。它不创建 `chi.Router`，不读 embedded spec，不安装 request validator，不返回 `net/http.Handler`。

Todo `HTTPModule` 输出 `Operations` 或等价明确字段，不再把已绑定路由称作 `Handler`：

```text
todo.NewHTTP(...) -> HTTPModule{Module, Operations}
```

模块继续拥有 DTO 映射、Todo fault -> Problem 映射、I18n 文本和 Todo actor 需求。

## 4. 请求身份与授权

把当前组合在 `RequestAccess` 中的两类职责拆开：

### 4.1 应用 operation gate

route binding 使用 application-owned 窄接口，例如：

```go
type OperationGate interface {
    Enforce(context.Context, string) error
}
```

composition Adapter 从 Auth context 读取 Principal，并按生成 inventory 的 operationId 执行认证与 policy。未认证、拒绝和未知依赖错误继续保留可识别语义与原因链。

### 4.2 Todo actor access

Todo Handler 使用自己拥有的窄端口：

```go
type ActorAccess interface {
    Actor(context.Context) (service.Actor, bool)
}
```

composition Adapter 把 Auth Principal 转换为 Todo Actor。每个 Todo operation 在调用 UseCases 前通过同一个私有 helper 取得 actor；缺失时 fail closed。对象授权继续由 `service.Authorizer` 完成，不与 operation gate 合并。

### 4.3 通用请求 metadata

operationId 和 `Accept-Language` 等协议 metadata 在唯一 strict middleware 中建立。实现可以使用 `httpx` 的专用 typed context helper，或由应用协议层提供窄只读 accessor；不得使用公开字符串 key、`map[string]any` 或 Auth 具体类型泄漏到 Todo。

## 5. 应用静态 aggregate

`internal/composition` 增加一个小型静态 aggregate，例如：

```go
type strictAPIServer struct {
    todo todohttp.Operations
}

func (s strictAPIServer) ListTodos(...) (...) {
    return s.todo.ListTodos(...)
}
```

所有方法只做一行类型安全转发，不添加业务规则、DTO 转换、授权或日志。这里是唯一的：

```go
var _ api.StrictServerInterface = (*strictAPIServer)(nil)
```

未来 OpenAPI 新增 operation 时，生成接口会让这个 aggregate 编译失败，迫使开发者在唯一 composition root 连接新模块；Todo Handler 不受无关方法影响。

不使用匿名嵌入多个接口来掩盖同名冲突，也不使用 `Unimplemented` fallback 返回 501 冒充完整接入。

## 6. 单一 route binding

在 `internal/transport/http` 的非生成、非业务 package 中建立一次性 binding 构造。它可以导入生成 `api`、Chi、OpenAPI validator、`pkg/httpx` 和自己定义的 `OperationGate`，但不得导入 `internal/module/*`。

职责固定为：

1. 校验 `api.StrictServerInterface`、OperationGate 等依赖非 nil；
2. 加载一次 embedded OpenAPI 并清除 Servers 验证约束；
3. 创建一个内部 Chi Router，设置统一 404/405；
4. 安装单 JSON document 与 OpenAPI request validator；
5. 安装统一 strict request/response error boundary；
6. 从生成 inventory 建立 operationId、语言和 operation authorization；
7. 调用一次生成 `api.HandlerWithOptions`，返回命名为 `apiRoutes` 的 `http.Handler`。

它不包含 Todo Service、Todo DTO 映射、模块判断或手写 method/path。`api/openapi.yaml` 与生成 route 仍是唯一公开事实。

## 7. Router 与构造顺序

`applicationRouter` 继续拥有全局中间件：request ID、recovery、access log、trusted proxy、secure headers、upgrade rejection、timeout、body limit、JSON accept、CORS、rate limit、overload 和 Auth bearer middleware。

函数签名把 `businessHandler` 改为语义明确的 `apiRoutes`：

```text
applicationRouter(capabilities, httpConfig, authMiddleware, apiRoutes)
  -> install global middleware
  -> install Auth context middleware
  -> Mount("/", apiRoutes)
```

Generation 中的可读顺序必须是：

```text
todoModule := todo.NewHTTP(...)
strictAPI := newStrictAPIServer(todoModule.Operations)
apiRoutes := transporthttp.NewRouteBinding(strictAPI, operationGate)
router := applicationRouter(..., apiRoutes)
server := httpx.NewServer(..., router)
```

不扩展 `pkg/httpx.Router` 暴露原生 Chi，不为消除一个根 Mount 引入第三方类型泄漏。一次嵌套路由层的运行成本可忽略，且换来明确的协议边界。

## 8. 新增业务 HTTP 模块的标准路径

未来真实模块 `<name>` 必须先通过应用模块能力评估，再按以下 HTTP 路径接入：

1. 在 `api/openapi.yaml` 增加稳定 tag、operationId、security、policy、DTO 与 Problem response。
2. 运行生成并审阅 breaking diff；不手写 method/path。
3. 在 `internal/module/<name>/binding/http` 构造 `<name>` operation Handler，只实现本模块方法。
4. 模块 HTTP profile 输出窄 `Operations`。
5. composition 为跨模块身份/能力建立最小 Adapter。
6. `strictAPIServer` 增加字段和生成接口要求的转发方法。
7. 现有单一 route binding、application Router 和 Server 不复制。

如果多个模块出现相同 Go 方法名、API 需要独立版本/发布或生成物已显著阻碍维护，返回研究阶段评估按 tag/spec 分包；不得临时增加 route registry。

## 9. 文件影响

预计实施范围：

- 修改 `internal/module/todo/binding/http/handler.go` 及测试；
- 修改 `internal/module/todo/module.go` 及相关测试；
- 新增或修改 `internal/transport/http` 的应用级 route binding 与测试；
- 新增或修改 `internal/composition` 的 strict aggregate、Auth Adapter、generation 与 Router 测试；
- 修改 architecture/package graph tests，阻止模块自建完整 binding；
- 同步根 `README.md`、`api/README.md`、`internal/module/README.md`、Todo README 和应用模块开发指南；
- 更新 026 实施证据。

预计不修改 `api/openapi.yaml`、生成文件、`go.mod`、`go.sum`、配置、migration、Kernel、Host 或 listener。若实施发现必须修改这些语义，触发重新确认。

## 10. 失败语义

- 模块 Handler 依赖 nil：该模块构造失败，候选 generation abort，旧代保留。
- aggregate 缺少模块 Handler：构造失败；不安装返回 501 的 fallback。
- embedded spec/validator/binding 构造失败：候选 generation abort，旧代保留。
- 无 Principal：保持 401；policy 拒绝：保持 403；未知 Auth 错误保留原因链并由统一边界脱敏。
- Todo actor 缺失：fail closed，不以零值 Actor 调用 Service。
- request/response/DTO/I18n/业务 fault：保持当前状态码、Problem code 和取消/超时语义。
- 多个错误同时发生时继续遵循现有 generation abort 与 cleanup error join 规则。

## 11. 架构门禁

新增结构检查应表达通用规则，而不是只搜索 Todo 文件名：

1. `internal/module/*/binding/http` 可以导入生成 DTO/strict request/response 类型，但不得导入 Chi、OpenAPI filter、`nethttp-middleware`，不得调用生成 route binder 或 `GetSwagger`。
2. `internal/transport/http` 的手写 binding 不得导入 `internal/module/*`。
3. 完整 `api.StrictServerInterface` 的生产实现与断言只允许在 application composition aggregate。
4. 生成 `HandlerWithOptions` 的生产调用只有一个。
5. application Router 不得导入模块 Service/Model/binding，也不得手写业务 method/path。
6. 既有 module owner、cmd、Kernel 和 pkg 依赖方向继续有效。

结构测试应包含正反 fixture；禁止用 alias、路径白名单或注释约定代替可执行门禁。

## 12. 验证矩阵

### 12.1 定向结构与构造

- Todo Handler 只满足 Todo-owned `Operations`；
- aggregate 满足完整 strict interface，nil dependency 确定失败；
- route binding 只构造一次 validator/router/generated routes；
- composition 构造顺序和参数命名可读；
- architecture 正反 fixture 阻止旧模式回归。

### 12.2 协议行为

- 当前 Todo contract tests 全部迁移并通过；
- 401、403、对象隐藏、I18n、unexpected error 脱敏不变；
- invalid JSON、尾随 JSON、未知字段、Content-Type、404/405 不变；
- operation ID、policy、日志、trace、metrics 与审计不变；
- `cmd/app` HTTP/CLI process tests 通过。

### 12.3 完整门禁

```powershell
gofmt -l .
go generate ./...
git diff --exit-code -- api/openapi.yaml internal/transport/http/api
go mod tidy -diff
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/app
git diff --check
```

`go generate` 会写工作区，只能在用户确认后的实施阶段执行。验证后审阅完整 diff、staged diff 和 staged file list，只提交 026 范围。

## 13. 单轨迁移顺序

1. 先建立 route binding 和 aggregate 的测试/结构门禁，并在同一实施轮完成迁移，不能提交长期红灯。
2. 把 Todo `NewTodoHandler` 收敛为唯一 operation Handler 构造，删除模块内 Router/validator/generated binding。
3. 拆分 operation gate 与 Todo actor access，保持 Auth/业务错误语义。
4. composition 先聚合 Handler，再创建 API routes，再创建 Router/Server。
5. 删除旧 `httpbinding.New`、完整接口断言、旧 `RequestAccess` 合并职责和失效测试。
6. 同步权威文档并执行全部验证。

不保留旧构造器 wrapper、兼容 alias、双 route binding 或 feature flag。

## 14. 重新确认触发器

出现以下任一事实时退回研究和待确认：

- 必须改变公开 OpenAPI、生成器配置/版本或 API 兼容性；
- 必须按 tag/spec 分包或引入多份 generated server；
- 必须扩展 `pkg/httpx.Router`、module Contribution、Kernel、Host 或生命周期；
- 必须新增/升级第三方依赖；
- 必须改变 Auth policy、Todo actor、配置、migration、数据或外部副作用；
- 静态 aggregate 无法在不引入业务规则的情况下表达真实第二模块。
