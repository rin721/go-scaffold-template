# 产品需求：Todo 路由中间件示例

## 1. 目标与依据

依据 [R001](research/R001-current-route-middleware-gap/report.md)，当前应用已有全局技术 middleware，业务 route contract 也能携带 route middleware，但 Todo 四条 route 的 `Middlewares` 都为空。因此 014 能运行，却没有展示“业务模块如何声明并绑定路由级 middleware”。

本任务增加一个真实、最小且无第三方依赖的示例：创建 Todo 必须显式声明 JSON Content-Type。它只负责 HTTP 协议校验，不复制标题、状态等业务规则，也不接触 Service、Repository 或数据库事务。

## 2. 行为契约

`POST /api/v1/todos` 在进入 Handler 前执行 `RequireJSONContentType`：

- `Content-Type: application/json`：放行。
- `Content-Type: application/json; charset=utf-8`：放行。
- Header 缺失、media type 格式非法或不是 `application/json`：不调用后续 Handler，返回 HTTP 415。
- 错误 envelope 继续使用 `pkg/httpx`：`{"error":"todo_unsupported_media_type","message":"Todo 创建请求必须使用 application/json"}`。
- middleware 返回的内部解析原因保留在 error chain，但不得回显给客户端。

`GET`、`PATCH .../complete` 没有 JSON 请求体，不绑定该 middleware，现有行为不变。

## 3. 目录要求

新增实际有代码的模块目录：

```text
internal/module/todo/
└── middleware/
    ├── json.go
    └── json_test.go
```

`middleware` 是 Todo 的 HTTP 入站 Adapter：只依赖标准库与 `pkg/httpx`，不导入 Kernel、Service、Repository、Database、CLI 或第三方框架。`binding/http` 负责把 middleware 显式绑定到 route，middleware 不通过 `init` 或全局 Registry 自注册。

## 4. 顺序与所有权

创建请求的实际执行顺序固定为：

```text
Recovery
-> RequestID
-> AccessLog
-> SecureHeaders
-> RequireJSONContentType
-> Todo Handler
```

全局 middleware 仍由 application composition root 安装；Todo middleware 由 Todo HTTP binding 拥有并绑定。错误继续穿过外层 AccessLog 和统一 ErrorHandler。

## 5. 非目标

- 不新增认证、授权、租户、CORS、限流或请求体大小配置。
- 不把业务不变量、Service、Repository 或事务放入 middleware/context。
- 不改变 Todo Service、CLI、Schema、配置节或数据库数据。
- 不抽象万能 middleware registry，不引入 chi 类型或新第三方依赖。
- 不给没有请求体的路由机械绑定 JSON 校验。

## 6. 兼容影响

这是一个公开 HTTP 行为收紧：此前未带 `Content-Type` 的创建请求可能仍进入 JSON binder，实施后将返回 415。根 README、Todo README 和进程测试必须同步说明并使用合法 Header。CLI 不受影响。

## 7. 验收标准

- middleware 单元测试覆盖普通 JSON、带参数 JSON、缺失、非法、非 JSON、短路、错误链和下游错误原样传播。
- route binding 测试证明只有创建路由绑定一个非 nil middleware。
- Router/进程测试证明全局 -> route -> Handler 顺序，以及 415 envelope 不泄露内部原因。
- 合法 HTTP 创建、CLI/HTTP SQLite 跨入口和全部既有测试保持通过。
- `go test ./...`、`go test -race ./...`、`go vet ./...`、`go build -o NUL ./cmd/app`、`go mod tidy -diff`、文档链接和 `git diff --check` 通过。
