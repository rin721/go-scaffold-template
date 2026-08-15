# R001 当前业务模块边界与外溢实现复核

## 1. 研究问题

本报告回答：当前仓库是否真正落实“业务能力默认收口在 `internal/module/<name>`”这一边界；Todo HTTP 为什么在模块外；Auth、Ops、Migration 缺少部分目录是否属于缺失；全局 OpenAPI、Kernel Capability、composition 与模块 contribution 各自应保留什么职责。

研究目标是形成可施工计划，不改变公开 HTTP API，不提前实现代码。

## 2. 方法与范围

1. 从 HEAD `02a176831b0071c1cddb3c615360b2e428304aa5` 检查 Git 状态、入口、路由构造、模块构造和 package graph。
2. 复用并复核 016 的模块命名、017 的能力评估、024-R005 的 Auth 归属和 024-R006 的 Ops/Migration 归属。
3. 区分模块专属业务/协议语义、应用级 API authority、跨模块连接代码和进程级资源 owner。
4. 以用户本轮给出的边界作为已确认架构原则，不把过去的目录选择当成继续外溢的理由。

本研究没有运行服务、生成代码、移动文件或修改依赖；当前结论来自源码、测试结构、权威文档和现有研究快照。

## 3. 当前事实

### 3.1 已经成立的模块边界

- `internal/module/README.md` 把应用模块定义为进程内纵向业务单元，要求 Model、Service、Repository/协议 Adapter、binding 与局部装配按业务名收口。
- `docs/development/application-module-development.md` 明确：模块专属 SDK、Client、消费者、cache、goroutine 或 migration 不会仅因存在资源和生命周期就自动成为 Kernel Component。
- Auth 的 JWT/JWKS、audit、policy 与 HTTP middleware 位于 `internal/module/auth`；JWKS participant 由 generation 启停，但没有进入 Kernel Plan。
- Ops 的 management Handler、OTel/Prometheus Adapter、middleware 与配置位于 `internal/module/ops`；物理 listener 和稳定 registry identity 留给进程 owner。
- Migration 只编排显式 CLI 用例；Todo SQL、compatibility 和 completion 位于 `internal/module/todo/binding/migration`，通用 migration engine 位于既有 Database Capability 子边界。

这些模块没有统一的目录形状：Auth、Ops 和 Migration 没有业务持久化需求，因此不存在 repo；Migration 没有 HTTP 入口，因此不存在 HTTP Handler。这是按真实职责裁剪，不是缺失。

### 3.2 当前 Todo HTTP 外溢

当前公开路径由 `api/openapi.yaml` 定义，`internal/transport/http/api` 保存生成的 DTO、strict server interface、Chi route binding、embedded spec 和 operation inventory。该部分不实现 Todo 用例，属于应用级公开协议 authority 的生成产物。

但 `internal/transport/http/todo.go` 不是纯生成或跨业务 HTTP 基础设施。它直接导入 Todo Model/Service 和 Auth Principal，负责：

- 构造 Todo strict Handler；
- 把 OpenAPI DTO 映射为 Todo command/query；
- 从 Auth context 转换 Todo Actor；
- 把 Todo fault 映射为 HTTP Problem；
- 执行 Todo operation authorization；
- 安装 Todo 特有 request validation。

这些都是 Todo-owned 入站 Adapter/binding。`internal/composition/service.go` 又直接调用 `NewTodoHTTPHandler` 并把它挂载到根 Router，使 composition 同时承担“连接依赖”和“构造业务 transport”两种职责。

`internal/module/todo/README.md` 明确宣称手写 HTTP Adapter 位于模块外，而通用开发指南又把真实 Handler/binding 放在模块目录。这不是读者漏看文件，而是当前权威说明之间存在边界冲突。

### 3.3 其他外溢与门禁缺口

- `cmd/app/main.go` 直接导入 `internal/module/ops/model` 构造 `BuildInfo`，绕过了 `cmd/app -> internal/composition` 的单一入口语义。
- `internal/kernel/composition/architecture_test.go` 只保护 module core 不导入 Kernel/HTTP/Database，并限制 composition root 的上层入口；它没有禁止顶层 transport、cmd 或其他包直接导入业务模块内部包。
- `module.Contribution` 当前只承载模块 ID 和 lifecycle Participants。该契约本身没有错误，但它不能被描述成包含所有入口绑定的统一 Registry；Todo HTTP 完成品必须由 Todo HTTP profile 明确输出，composition 只消费输出。
- `internal/composition/database.go` 和 `todo_authorization.go` 分别在 Kernel-owned Access 与 Todo-owned port、Auth-owned contract 与 Todo-owned port 之间做窄映射。这是唯一 composition root 的合法职责，不是业务实现外溢。
- `internal/composition/todo.go` 为 one-shot CLI 准备 Kernel 能力、配置候选、跨模块 Principal 与 Supervisor owner。它依赖 Kernel 和多个模块，因此不能迁入 Todo；应继续作为 application orchestration，但不得实现 Todo 业务规则。

## 4. 用户决策

本任务采用以下单轨边界：

> 新增业务能力必须先按 `internal/module/<name>` 收口 model、service、Adapter、binding 与 contribution；只有经能力评估证明是跨业务复用且由进程统一选择的底层资源，才进入 `pkg/<name> -> internal/kernel/app/<name> -> internal/kernel/composition`。拥有第三方 SDK、Client、cache 或 goroutine 本身不是升级为 Kernel Capability 的理由。

因此，过去 024 把 Todo 手写 transport 放到顶层目录的实现选择不能继续作为当前例外；历史记录保留事实，但当前代码和权威文档必须单轨收口。

## 5. 推断与目标方向

### 5.1 必须迁回模块

- Todo HTTP Handler、request/response DTO 映射、Todo error presenter、Todo 请求身份端口和相关测试迁至 `internal/module/todo/binding/http`。
- Todo HTTP binding 不得导入 Auth module。它定义自己需要的 `RequestAccess` 窄端口；composition Adapter 从 Auth context 提取 Principal、转换 Todo Actor 并执行 operation policy。
- Todo HTTP profile 返回已经完成的 `http.Handler`。application Router 只安装 Handler，不再导入或调用顶层 Todo transport 构造器。

### 5.2 可以保留在应用级

- `api/openapi.yaml` 继续作为整个可部署应用的唯一公开 HTTP authority。
- `internal/transport/http/api` 继续只保存固定版本生成的 DTO、strict interface、Chi route binding、embedded spec 和 operation inventory；其中不得出现手写 Todo 用例映射。
- 全局 request ID、recovery、access log、trusted proxy、CORS、rate/overload 与 Server/listener 生命周期继续由 application/process owner 组合。
- Auth/Todo、Kernel/Todo、Ops/process 之间的窄契约 Adapter 继续由 composition root 持有。

### 5.3 不应实施的伪对称

- 不为 Auth、Ops、Migration 新建空 repo、handler、model 或 binding。
- 不把 Todo HTTP 迁入 `pkg/httpx`、Kernel App 或新的顶层通用 Adapter。
- 不新增自动扫描、运行时 Registry、Service Locator 或通用 DI 容器。
- 不为了目录移动改变 OpenAPI 路径、DTO、状态码、鉴权策略、配置或数据库语义。

## 6. 当前契约适配性

当前同步 HTTP/Application Generation 契约能够表达这次收口：Handler 是纯内存构造的 generation-owned 对象，物理 listener、Server、Auth participant 和底层租约不改变。Todo binding 只需项目现有 I18n、Service 和由 composition 提供的请求访问窄端口，不需要新增 Kernel Capability、第三方依赖、资源生命周期或 Reload 策略。

`module.Contribution` 可继续拥有 Participant；HTTP 完成品由 Todo HTTP profile 显式返回。当前只有一个业务 HTTP 模块，本任务不为未来第二个模块虚构动态 route Registry。新增第二个 HTTP 业务模块时，必须基于同一 OpenAPI authority 重新评估生成 route 的分区和安装方式，不能复制根挂载或建立第二份路径表。

## 7. 适用、不适用与局限

适用于：当前 Todo HTTP 归属修复、模块文档单轨、package import governance，以及后续新增业务能力的默认归属判断。

不适用于：改变公开 API、增加第二个 HTTP 模块、拆分独立进程、引入新共享底层资源、扩展 Kernel 生命周期或调整数据库 migration。

局限：本轮未执行移动后的编译和动态测试；这些属于确认后的实施证据。当前只有一个业务 HTTP 模块，因此不声称已经证明多模块 OpenAPI route 聚合方案。

## 8. 剩余未知

- 第二个 HTTP 模块如何从单一 OpenAPI authority 获得独立生成 binding，需要在真实模块需求出现时研究，当前不预建抽象。
- `module.Contribution` 未来是否需要承载 Health、HTTP 或 background task 以外的完成品，取决于真实运行单元，不在本任务扩展。

这些未知不妨碍当前 Todo 外溢收口和 package graph 加固，因此研究门禁通过。

## 9. 对 025 的影响

025 可以形成非文档实施计划，但必须等待用户在计划报告后的后续消息确认。实施范围应包含 Todo HTTP 移动、Auth/Todo 窄端口、HTTP profile 输出、composition 简化、cmd 边界修复、architecture test 与权威文档同步；不包含新 Capability、API 行为变化或未来多模块 Router 框架。
