# 开发设计：Todo 路由中间件示例

## 1. 设计结论

新增模块级 HTTP Adapter `internal/business/todo/middleware`，只实现 `RequireJSONContentType`。`binding/http.Routes` 在创建路由的 `Middlewares` 中显式绑定它；全局 middleware 和 Router 安装机制不变。

选择 Content-Type 校验，是因为它有真实协议语义、可以安全展示 middleware 的“放行与短路”两条路径，并且不需要认证主体、限流状态、配置或外部依赖。`NoStore` 响应头虽然更简单，但不能展示拒绝路径；认证、限流和 CORS 会引入尚未确认的安全或配置决策。

## 2. API 与实现

目标 API：

```go
package middleware

func RequireJSONContentType() httpx.Middleware
```

实现使用标准库 `mime.ParseMediaType` 解析 Header，并用大小写不敏感比较接受 `application/json`。稳定值归当前 Adapter 所有：

- Header：`Content-Type`。
- media type：`application/json`。
- reason：`todo_unsupported_media_type`。
- safe message：`Todo 创建请求必须使用 application/json`。

拒绝时返回：

```go
&httpx.StatusError{
    StatusCode: http.StatusUnsupportedMediaType,
    Code:       "todo_unsupported_media_type",
    Message:    "Todo 创建请求必须使用 application/json",
    Err:        cause,
}
```

缺失 Header 使用模块自有 sentinel cause；解析错误保留标准库 cause；非 JSON 使用包含实际 media type、但只留在内部 error chain 的 cause。middleware 不记录日志，外层 AccessLog 是唯一 HTTP 记录边界。

## 3. 路由绑定

`binding/http.Routes` 只修改创建路由：

```go
{
    Method:      httpx.MethodPost,
    Path:        "/api/v1/todos",
    Handler:     todoHandler.Create,
    Middlewares: []httpx.Middleware{middleware.RequireJSONContentType()},
}
```

其他三条 route 保持空 middleware。`business.ValidateContributions` 继续在 listener 启动前拒绝 nil middleware；`applicationRouter` 已把 route middlewares 传给 `Router.Handle`，不新增注册路径。

## 4. 数据流与失败语义

```text
request
  -> global Recovery/RequestID/AccessLog/SecureHeaders
  -> route RequireJSONContentType
       -> invalid: StatusError(415) -> global ErrorHandler
       -> valid: Todo Handler -> Service -> Repository
```

middleware 不读取 body，合法请求仍由 Handler 的 `BindJSON` 负责 JSON 语法与 DTO 绑定：Content-Type 错误是 415，JSON 内容错误仍是既有 400 `invalid_json`，两类失败不混淆。

## 5. 文件影响

- 新增 `internal/business/todo/middleware/json.go` 与单元测试。
- 修改 `internal/business/todo/binding/http/routes.go` 与测试。
- 扩展 `cmd/app/main_test.go` 的进程 HTTP 验收。
- 更新根 README、`internal/business/todo/README.md`、变更索引和 015 状态/证据。
- 不修改 `go.mod/go.sum`、Service/Repository/Schema/Config/CLI、Kernel 或 `pkg/httpx` 公共 API。

## 6. 验证设计

- 直接调用 middleware 包装的 stub Handler，断言放行次数、短路次数、415 code/message、cause 和下游 error identity。
- route binding 测试断言 POST create 有且仅有一个 middleware，其他 route 为零，并执行 middleware 证明不是占位函数。
- 进程测试发送缺失 Content-Type 的 POST，断言 415 与稳定 reason；随后发送合法 JSON 证明服务仍可创建。
- 复用现有 `pkg/httpx` middleware order 测试，并在 Todo 路由测试补充 route 级执行证据；不重复实现 Router 测试框架。
