# 设计：业务模块边界收口

## 1. 设计结论

采用“应用级协议 authority + 模块内手写 binding + 唯一 composition 连接”的单轨结构：

```text
api/openapi.yaml
  -> internal/transport/http/api       只保存生成 DTO/server/routes/inventory
       -> internal/module/todo/binding/http
            -> Todo UseCases + Todo-owned RequestAccess

Kernel capabilities -> internal/composition <- Auth module
                              |
                              +-> Todo HTTP profile completed Handler
                              +-> global middleware + Router + Server/listener
```

OpenAPI 是整个可部署应用的公开契约，不属于单个 module core；所有手写 Todo 协议语义属于 Todo。composition 可以看到两侧契约并做窄映射，但不实现任何 Todo 请求、响应或错误规则。

## 2. 目标目录

```text
api/
├── openapi.yaml
└── oapi-codegen.yaml

internal/transport/http/api/
├── api.gen.go
├── operation_inventory.gen.go
└── generate.go

internal/module/todo/
├── model/
├── service/
├── repo/
├── binding/
│   ├── config/
│   ├── cli/
│   ├── migration/
│   └── http/
│       ├── handler.go
│       └── handler_test.go
└── module.go
```

迁移后删除 `internal/transport/http/todo.go` 与 `todo_test.go`。顶层 transport 目录不保留 Todo wrapper、alias 或转发入口。

## 3. Todo HTTP 窄端口

`internal/module/todo/binding/http` 定义请求边界所需的最小接口，名称在实施时可按 Go 语义微调，但职责固定：

```go
type RequestAccess interface {
    Actor(context.Context) (service.Actor, bool)
    EnforceOperation(context.Context, service.Actor, string) error
}
```

- `Actor` 只返回 Todo Service 已定义的 Actor，不暴露 Auth Principal、JWT claims 或第三方类型。
- `EnforceOperation` 接收稳定 operation ID；未认证由 `Actor` 的缺失语义表达，拒绝使用 Todo/项目可识别的 permission error，其他错误保留原因链。
- Todo HTTP binding 继续负责 strict request metadata、DTO 映射、I18n error presenter 和 RFC 9457 状态选择。
- Auth middleware 继续在全局 Router 上建立认证 context；Todo binding 不解析 bearer token。

composition 中的 `todoRequestAccessAdapter` 持有 Auth Service：

1. 从 context 取 Auth Principal；
2. 转成 Todo Actor；
3. 调用 Auth operation policy；
4. 把 Auth 未认证/拒绝映射为 Todo binding 能识别的稳定语义；
5. 对未知依赖错误保留错误链。

对象级授权继续使用现有 `service.Authorizer` 和 `todoAuthorizerAdapter`，不与 route operation authorization 合并成万能接口。

## 4. Todo profile 与完成品输出

保留局部纯内存构造，但区分没有 HTTP 入口的 core/local profile 与长期 Service 使用的 HTTP profile：

```text
todo.New(...)          -> Module{Service, Contribution}
todo.NewHTTP(...)      -> HTTPModule{Module, Handler}
```

精确 Go 名称由实施编译结果校准，语义必须满足：

- `New` 不创建 listener、goroutine、资源探测或 HTTP 占位值，继续供 one-shot CLI 使用；
- `NewHTTP` 复用同一 core 构造，再调用模块内 `binding/http` 创建非 nil Handler；
- `HTTPModule` 明确包含 core Module 与 Handler，不使用可混淆的 nil 字段表示 profile；
- Contribution 继续只承载真实 Participants，不扩展为动态 Resolver 或路由 Registry。

Application Generation 构造顺序保持：Capability -> Auth -> Todo -> Ops -> Participants -> Router -> listener/server。区别仅在 Todo 阶段已经得到完成的 Handler。

## 5. Router 与 composition

`applicationRouter` 调整为接收完成的 `http.Handler`，不再接收 Todo Service 并调用业务 transport 构造器：

```text
applicationRouter(capabilities, httpConfig, authMiddleware, todoHTTP)
  -> install global middleware
  -> install Auth middleware
  -> mount completed Todo HTTP handler
```

根挂载仍由当前单一 OpenAPI-generated Handler 使用，因此公开路径和生成 route 不变。本任务不新增第二个根 Handler，也不建立 route Registry。未来第二个 HTTP 业务模块必须先研究 OpenAPI tag/生成 binding 的分区与多模块安装，不能复制当前根挂载。

composition 中允许保留：

- Kernel Database Access -> Todo repo.Access；
- Auth Service -> Todo service.Authorizer；
- Auth Service/context -> Todo HTTP RequestAccess；
- OpenAPI inventory -> Auth policy 与 Ops operation metadata；
- Config Snapshot -> 各模块 owned binding。

composition 中禁止保留：Todo DTO、Handler、presenter、业务状态判断和 Repository 实现。

## 6. 入口边界

把构建信息的入口 DTO 收口到 `internal/composition`：`cmd/app` 构造 composition-owned `BuildInfo` 或等价固定输入，composition 再映射为 Ops Model。这样 `cmd/app` 只导入 application composition root，Ops Model 不向进程入口外溢。

该调整不改变 ldflags 变量、输出字段或 management `/build` 行为。

## 7. Package graph 门禁

扩展现有 architecture test，以实际 import graph 和定向 fixture 强制以下规则：

1. `cmd/app` 只能导入标准库和 `internal/composition`；
2. `internal/composition` 是唯一允许跨 module owner、连接 Kernel composition 与模块的包；
3. `internal/module/<owner>` 不能导入另一个 `internal/module/<other>`；
4. `internal/module/*` 不能导入 application/kernel composition；
5. `internal/transport/http` 的非生成包不能导入 `internal/module/*`；
6. `pkg` 与 Kernel App 的既有方向规则继续有效；
7. module core 的 HTTP/CLI/Database 禁止规则继续有效，不扩大到合法 binding/Adapter。

测试必须避免只针对 Todo 路径硬编码；owner 从 import path 的第一个 module segment 计算。生成 API package 作为应用级协议资产不导入 module，因此不需要例外业务依赖。

## 8. 单轨迁移步骤

1. 先扩展 package graph fixture，使当前外溢以预期方式暴露，但在同一实施任务中立即迁移，不能提交长期红灯。
2. 移动 Todo HTTP 源码和测试，改为模块内 package。
3. 引入 RequestAccess 窄端口，删除 Todo binding 对 Auth Model 的导入。
4. 增加 Todo HTTP profile，更新 Generation 与 Router 使用完成品 Handler。
5. 收口 `cmd/app` BuildInfo 输入。
6. 删除旧顶层文件、imports、package 名和文档说明。
7. 运行生成 clean diff、完整测试、race、vet、build、tidy 和残留搜索。

不保留旧 import path compatibility package。Git 历史承担迁移追踪。

## 9. 文件影响

预计非文档实施范围：

- 移动并修改：`internal/transport/http/todo.go`、`todo_test.go` -> `internal/module/todo/binding/http/`；
- 修改：`internal/module/todo/module.go` 与相关测试；
- 修改：`internal/composition/generation.go`、`service.go`、`todo_authorization.go` 或新增同职责窄 Adapter 文件；
- 修改：`internal/composition/application.go`、`cmd/app/main.go` 与相关测试，用 composition-owned BuildInfo 隔离 Ops Model；
- 修改：`internal/kernel/composition/architecture_test.go`；
- 同步：根 `README.md`、`api/README.md`、`internal/module/README.md`、`internal/module/todo/README.md`、`docs/development/application-module-development.md`；
- 更新本 025 任务状态与实施证据。

`api/openapi.yaml`、生成文件、go.mod/go.sum 和配置预计无语义改动；若实施发现必须改变它们，先回到研究/计划并重新确认。

## 10. 失败语义

- HTTP profile 任一依赖为 nil：构造候选失败，旧 generation 保持服务；不安装部分 Handler。
- context 无 Auth Principal：返回现有 401 Problem 语义。
- operation policy 拒绝：返回现有 403 Problem 语义。
- Auth/依赖未知错误：保留错误链，由现有统一 Problem/日志边界处理，不泄露内部文本。
- DTO、验证、I18n、Todo fault：保持现有状态码、code、message 与取消/超时语义。
- package graph 发现新外溢：测试失败，不通过白名单、alias 或路径字符串特判绕过。

## 11. 验证方案

### 11.1 静态边界

- package graph production 与正反 fixture；
- 搜索旧 `internal/transport/http` Todo package/import；
- 搜索 module 之间的直接 import；
- 搜索 `cmd/app` 对 module/kernel/第三方业务实现的 import；
- 搜索旧 Handler 构造器、旧 package 名和兼容 wrapper。

### 11.2 协议与行为

- 原 Todo HTTP protocol tests 在新模块路径全部通过；
- 现有 `cmd/app` HTTP/CLI process tests 通过；
- 认证缺失、拒绝、对象隐藏、invalid JSON、Content-Type、404/405、I18n 与 dependency failure 行为不变；
- `go generate ./...` 后生成文件无 diff；OpenAPI hash/operation inventory 不变。

### 11.3 完整门禁

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

生成命令会写工作区，只能在用户确认后的实施阶段执行。验证后必须审阅完整 diff，并只暂存 025 文件。

## 12. 重新确认触发器

出现以下任一事实时退回研究并重新确认：

- 必须改变公开 OpenAPI、生成器策略或 API 兼容性；
- 必须支持第二个 HTTP 业务模块或新增 route registry；
- 必须扩展 module Contribution、Kernel API 或 Application Generation 生命周期；
- 必须新增/升级第三方依赖；
- 发现 Todo HTTP 无法在不导入 Auth 具体类型的情况下保留现有语义；
- 需要改变配置、migration、数据或外部副作用。
