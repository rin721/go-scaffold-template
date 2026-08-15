# httpx

`pkg/httpx` 是项目内通用 HTTP 封装，包含出站客户端和入站服务端路由能力。客户端基于 Go 标准库 `net/http`，服务端路由基于 `net/http` 和 `github.com/go-chi/chi/v5`；业务代码通过本包的 `Client`、`Router`、`Context` 和 `Server` API 使用常见 HTTP 能力。

## 技术选型

- 客户端优先使用 `net/http`：标准库长期稳定，原生支持连接池、`context.Context` 取消、超时、`Transport` 替换和 `httptest` 验证，能覆盖脚手架的通用出站请求场景。
- 服务端继续使用 `net/http` 作为基础协议，并使用 `chi/v5` 负责路由匹配和标准中间件组合。chi 轻量、兼容标准库、维护活跃，适合长期 REST API 项目。
- 首版不引入 `resty`、`fasthttp` 或完整 Web 框架。通用脚手架更需要稳定边界和低依赖面；只有出现明确场景时，才在包内部重新评估第三方客户端或框架。
- chi 只作为本包内部路由实现，业务代码不直接依赖 chi 类型。

## 设计目标

- 简单：封装常见 JSON 请求、响应解码、路由注册和响应写入。
- 通用：覆盖客户端请求、服务端路由、中间件、统一错误处理和优雅关闭。
- 可维护：配置、默认值、请求、响应、路由、上下文和服务生命周期分文件维护。
- 边界清晰：不提供全局 client/router/server，推荐在应用入口创建后显式注入。

## 目录结构

```text
pkg/httpx/
├── builder.go      # 配置补全、校验和内部辅助逻辑
├── client.go       # Client 接口和 HTTP 请求执行
├── config.go       # ClientConfig、RouterConfig、ServerConfig
├── constants.go    # 方法、Header、Content-Type、错误码等固定定义
├── context.go      # 服务端 Context 和响应/绑定辅助方法
├── defaults.go     # 默认配置和默认值
├── errors.go       # StatusError 等错误类型
├── middleware.go   # Handler、Middleware、ErrorHandler
├── request.go      # 客户端 Request
├── response.go     # 客户端 Response
├── router.go       # Router 和默认错误处理
├── server.go       # Server 生命周期封装
└── README.md       # 使用文档
```

## 配置项说明

### ClientConfig

| 字段 | 说明 | 默认值 |
| --- | --- | --- |
| `BaseURL` | 客户端基础地址，配置后 `Request.URL` 可使用相对路径 | 空 |
| `Timeout` | 单次请求总超时 | `10s` |
| `MaxResponseBodyBytes` | 最大响应体读取大小 | `10MiB` |
| `Transport` | 自定义 `http.RoundTripper` | `nil`，使用标准库默认 Transport |
| `RetryCount` | 请求重试次数 | `0`，默认不重试 |
| `RetryWaitTime` | 启用重试后的初始等待时间 | `100ms` |
| `RetryMaxWaitTime` | 启用重试后的最大等待时间 | `2s` |

`RetryCount` 只对网络错误、`429` 和 `5xx` 状态码生效。默认不重试，避免对非幂等请求产生隐式副作用；业务确实需要重试时，应显式配置并确认接口幂等性。

### RouterConfig

| 字段 | 说明 | 默认值 |
| --- | --- | --- |
| `ErrorHandler` | 统一错误处理函数 | `DefaultErrorHandler` |

`Router.Use` 接收项目自己的 `Middleware`，`Router.UseHTTP` 接收标准库形态的 `func(http.Handler) http.Handler`，用于接入 chi 生态或其他标准 HTTP 中间件。

### ServerConfig

| 字段 | 说明 | 默认值 |
| --- | --- | --- |
| `Addr` | 服务监听地址 | `:8080` |
| `ReadHeaderTimeout` | 请求头读取超时 | `5s` |
| `ReadTimeout` | 请求读取超时 | `15s` |
| `WriteTimeout` | 响应写入超时 | `30s` |
| `IdleTimeout` | 空闲连接超时 | `60s` |
| `MaxHeaderBytes` | 最大请求头大小 | `http.DefaultMaxHeaderBytes` |

## 客户端基础示例

```go
package main

import (
	"context"
	"fmt"

	"github.com/rin721/go-scaffold-template/pkg/httpx"
)

func main() {
	client, err := httpx.NewClient(nil)
	if err != nil {
		panic(err)
	}
	defer client.CloseIdleConnections()

	var out struct {
		Name string `json:"name"`
	}
	_, err = client.JSON(context.Background(), httpx.Request{
		Method: httpx.MethodGet,
		URL:    "https://example.com/api/user",
		Query:  map[string]string{"id": "1"},
	}, &out)
	if err != nil {
		panic(err)
	}

	fmt.Println(out.Name)
}
```

## 客户端自定义示例

```go
package main

import (
	"context"
	"net/http"
	"time"

	"github.com/rin721/go-scaffold-template/pkg/httpx"
)

func main() {
	client, err := httpx.NewClient(&httpx.ClientConfig{
		BaseURL:              "https://example.com",
		Timeout:              3 * time.Second,
		MaxResponseBodyBytes: 2 << 20,
	})
	if err != nil {
		panic(err)
	}

	_, err = client.Do(context.Background(), httpx.Request{
		Method:       httpx.MethodPost,
		URL:          "/api/users",
		Headers:      map[string]string{"X-Trace-ID": "trace-1"},
		Body:         map[string]string{"name": "rin"},
		AcceptStatus: []int{http.StatusCreated},
	})
	if err != nil {
		panic(err)
	}
}
```

## 服务端基础示例

```go
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"

	"github.com/rin721/go-scaffold-template/pkg/httpx"
)

func main() {
	router := httpx.NewRouter(nil)
	router.Handle(httpx.MethodGet, "/users/{id}", func(ctx *httpx.Context) error {
		return ctx.JSON(http.StatusOK, map[string]string{
			"id": ctx.Param("id"),
		})
	})

	server, err := httpx.NewServer(nil, router)
	if err != nil {
		panic(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := server.Start(ctx); err != nil {
		panic(err)
	}
	runDone := make(chan error, 1)
	go func() { runDone <- server.Run(context.Background()) }()
	<-server.Running()

	<-ctx.Done()
	if err := server.Stop(context.Background()); err != nil {
		panic(err)
	}
	if err := <-runDone; err != nil {
		panic(err)
	}
}
```

## 在业务代码中的推荐使用方式

推荐在应用入口创建 `Client`、`Router` 和 `Server`，再通过构造函数注入业务组件。业务组件不要在函数内部重复创建客户端，也不要绕过 `httpx` 重新散写 JSON 请求、状态码判断和错误响应。

```go
package user

import (
	"context"

	"github.com/rin721/go-scaffold-template/pkg/httpx"
)

type Gateway struct {
	client  httpx.Client
	baseURL string
}

func NewGateway(client httpx.Client, baseURL string) *Gateway {
	return &Gateway{client: client, baseURL: baseURL}
}

func (g *Gateway) Find(ctx context.Context, id string, out any) error {
	_, err := g.client.JSON(ctx, httpx.Request{
		Method: httpx.MethodGet,
		URL:    g.baseURL + "/users/" + id,
	}, out)
	return err
}
```

`Server.Stop(ctx)` 只执行 `http.Server.Shutdown` 并等待 Serve 完成，不会在超时后暗中强关连接。进程 owner 只有在 graceful 失败且预算仍允许时，才通过 `Server.ForceStop(ctx)` 显式调用 `Close`；forced 会作为有损结果上报。当前 Server 不治理 hijacked/WebSocket 连接，出现真实升级协议时必须增加独立 owner。

长期 Service 使用 `ListenerHub` 独占物理 TCP listener。每个不可变 Application Generation 创建自己的 `Server` 和虚拟 listener route；候选先通过 `StartWithListener` 进入 Serve-ready，commit 再把新连接 dispatch 到新 route。旧 route 的 pending connection 先交付旧 Server，然后 `Shutdown` 排空 active connection/request。`ListenerHub` 是 composition owner 的基础设施，不应进入业务模块或作为运行时查询入口。

首版不实现熔断、链路追踪、OpenAPI、TLS 证书管理、HTTP/2 特化配置或服务发现。需要这些能力时，应在明确场景后单独扩展，并继续保持业务侧只依赖 `httpx` 的项目契约。
