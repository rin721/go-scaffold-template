# 需求：Handler-first HTTP 路由绑定

## 1. 依据

- `R001`：当前单模块行为正确，但 Handler、validator、Router 与整份 route binding 耦合，新增第二模块不通用。
- `R002`：当前规模优先保留单一生成 binding，以静态 aggregate 连接模块；暂不承担按 tag/spec 多生成配置的复杂度。

## 2. 目标

重构当前 Todo HTTP 装配，使代码结构明确表达以下顺序：

```text
先构造模块 operation Handler
  -> 再静态聚合完整 API
  -> 再绑定一次生成路由
  -> 最后安装全局 Router 与 Server
```

重构后，新增一个编译期内置业务 HTTP 模块时，既有模块不需要实现新 operation，不复制 OpenAPI 路径，不创建第二份完整 Router/validator，也不改变 Kernel 生命周期。

## 3. 术语

- **operation Handler**：模块拥有的入站 Adapter，只实现本模块 operation 的生成 request/response 方法、DTO 映射和业务错误呈现；不是 `net/http.Handler`。
- **strict API aggregate**：application composition 拥有的静态完成品，唯一满足整份 `api.StrictServerInterface`，只做方法转发。
- **route binding**：application HTTP 协议拥有的一次性装配，把 aggregate、validator、strict middleware 和生成 Chi routes 组合成 `net/http.Handler`。
- **application Router**：安装进程级 middleware，并挂载唯一 route binding 的最外层 Router。

## 4. 功能要求

### `REQ-001` 模块 Handler 独立

Todo HTTP binding 必须先构造只属于 Todo 的 operation Handler。模块不得创建 Chi Router、读取 OpenAPI、调用生成 `Handler*` 路由函数或直接满足整份应用 `StrictServerInterface`。

### `REQ-002` 静态完整性

application composition 必须有且只有一个静态 aggregate 满足完整 `api.StrictServerInterface`。每个方法只转发给对应模块的窄 operation Handler；缺少新 operation 时必须在编译期失败。

### `REQ-003` 唯一路由绑定

整份 OpenAPI specification、request validator、strict request/response error boundary、operation identity 和 operation authorization 必须只装配一次。公开 method/path 继续只由 `api/openapi.yaml` 和生成代码定义。

### `REQ-004` Router 简洁

application Router 只负责全局 middleware 和一次语义明确的 API routes 挂载。不得接收 Todo Service、生成 DTO 或模块构造参数，不得手写业务 method/path。

### `REQ-005` 请求边界拆分

operation 级认证授权属于应用 route binding；Todo actor 转换属于 Todo Handler 使用的窄端口；对象授权继续属于 Todo Service 的 `Authorizer`。三者不得合并成万能依赖。

### `REQ-006` 行为兼容

以下行为必须保持：

- 四个 Todo 路径、method、DTO、operationId 和 OpenAPI hash；
- bearer 认证、401/403、对象隐藏、I18n 和审计语义；
- invalid JSON、未知字段、Content-Type、404/405 与 RFC 9457 Problem Details；
- request ID、日志、trace、metrics、rate/overload、timeout、body limit 与 CORS；
- Application Generation reload、listener、admission、drain 和资源释放。

### `REQ-007` 新模块扩展面

新增业务 HTTP 模块的 HTTP 接入只允许触及：

- 唯一 OpenAPI authority 与生成物；
- 新模块自己的 Handler/binding；
- application strict aggregate 的字段与转发方法；
- composition 的显式构造与注入；
- 对应测试、文档和 operation policy inventory。

不得要求修改既有 Todo Handler、复制 route binding、扩展 Kernel Plan 或增加动态 Registry。

## 5. 质量要求

| 标准 | 可验收定义 |
| --- | --- |
| 简单 | composition 中能按顺序读出 Handler -> aggregate -> routes -> Router；同一职责只有一个构造位置 |
| 高效 | 每个 Application Generation 只加载/校验一份 OpenAPI、创建一个 generated route tree，不按模块重复 |
| 通用 | 完整 API 接口扩张只影响 aggregate 和新模块，不迫使既有模块实现无关 operation |
| 明确 | `http.Handler`、operation Handler、route binding 和 Router 命名不混用；owner 可从 package 和类型看出 |
| 可验证 | 结构门禁、协议测试、进程测试和生成 clean diff 同时通过 |

## 6. 范围

### 包含

- Todo operation Handler 与 route binding 解耦；
- 应用级静态 strict aggregate；
- 单一 OpenAPI route binding；
- operation middleware 与 Todo actor 窄端口的职责调整；
- composition/Router 命名与构造顺序收口；
- 架构门禁、测试和当前权威文档同步。

### 不包含

- 新增第二个真实业务模块或假业务示例；
- 修改公开 OpenAPI 行为、版本或路径；
- 按 tag/spec 拆分生成包；
- 动态 route registry、扫描、插件或 Service Locator；
- 修改 Database、migration、配置 schema、Kernel、Host、listener 或 module Contribution；
- 新增或升级第三方依赖；
- push、tag、Release、部署或数据库操作。

## 7. 验收标准

1. `internal/module/todo/binding/http` 不再导入 Chi、OpenAPI filter 或 `nethttp-middleware`，不调用 `api.GetSwagger`、`api.NewStrictHandler*`、`api.Handler*`。
2. Todo operation Handler 不再声明满足完整 `api.StrictServerInterface`，只满足 Todo-owned 窄接口。
3. 完整 `api.StrictServerInterface` 只有一个 application-owned 实现与编译期断言。
4. 生成 route binding 在 production 代码中只有一个调用位置，每代只构造一次。
5. `applicationRouter` 只接收完成的 API routes `http.Handler`，并使用语义明确名称挂载一次。
6. Todo 当前所有 HTTP、Auth、I18n、Ops 和 process tests 保持通过。
7. `go generate ./...` 后 OpenAPI 和生成文件无意外 diff；公开契约不变。
8. package graph 与结构测试阻止模块重新自建完整 Router/binding，且不依赖 Todo 路径白名单。
9. 完整 Go test/race/vet/build/tidy、Markdown 链接和 `git diff --check` 通过。

## 8. 确认要求

这是非文档重构计划。只有用户在本计划报告之后的后续消息明确确认 026 当前方案，才能修改源码、生成物或测试实现。若确认后发现必须改变 OpenAPI、依赖、Kernel/Host、module Contribution、公共行为或 route generator 策略，必须退回研究并重新确认。
