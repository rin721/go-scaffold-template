id: R001
name: business-module-execution-adoption
status: active
created: 2026-08-20
supersedes: []
superseded_by: []
verification_date: 2026-08-20
trigger: 业务模块接入 execution 能力时复核
---

# R001 Todo 模块接入 execution：现状与接入点

## 研究问题

需要幂等 / 失败重试 / 执行记录的业务操作如何让真实业务模块（以 Todo 为例）经 composition 注入并使用
`execution` 能力，同时不破坏模块「自有窄 port」架构（AGENTS 3.1 / 3.2）。

## 证据定位

- Todo 模块装配：`internal/composition/todo.go`（`todo.New(todo.Dependencies{...})`）。
- Todo 模块 Dependencies：`internal/module/todo/module.go`（Database / Clock / IDGenerator / Config / Authorizer）。
- Todo Service 窄 port：`internal/module/todo/service/service.go`（Repository / Clock / IDs / Authorizer，`New(...)`）。
- `Complete` 用例：`service.go#Complete` —— 已做业务级幂等（已完成对象返回不变），`repository.Save` 是一次真实写操作。
- 底层能力出口：`internal/kernel/composition.Capabilities.Execution`（`executionapp.Access`）。

## 事实（可复核）

1. Todo 通过 `internal/composition/todo.go#prepareTodo` 从 `Capabilities` 取 Clock/IDGenerator，经
   `adaptDatabaseAccess(capabilities.Database)` 得到 repo.Access；`todo.New` 把它们收进 `todo.Dependencies`。
2. `service.New` 只接受窄 port（Repository/Clock/IDs/Authorizer），不感知具体底层实现（AGENTS 3.1：抽象由使用方定义）。
3. `Complete` 是写操作（`Get`→`enforce`→`todo.Complete`→`repository.Save`），适合作为「执行治理」载体：
   用幂等键合并重复提交、对可重试仓库失败按策略重试、留下执行记录。
4. `pkg/execution.OperationExecutor.Execute(ctx, Execution{Key, PolicyName, Operation})` 提供幂等/重试/记录；
   `Capabilities.Execution`（`executionapp.Access`）实现该契约，并带 `PolicyName` 解析、`Recovery()`/`Health()` 与
   `WithTrace` 支持（见 035）。

## 推断

- 接入应沿用「Service 定义自有窄 port，composition 提供 Adapter」：在 `service` 层新增一个执行 port（如
  `Executor func(context.Context, ...) error` 或窄接口），由 composition 用 `capabilities.Execution` + 命名策略
  （如 `todo`）实现 Adapter 并注入；模块与适配层均不反向依赖 backend 具体类型。
- 候选落地：让 `Complete`（或 `Create`）的关键写操作经该 port 执行，幂等键取自业务 ID（如 `todo:complete:<id>`）。
- 不动其它用例；`Complete` 已有的业务幂等保留，执行治理在其外层叠加（去重 / 重试 / 记录）。

## 适用 / 不适用

- 适用：演示「统一、声明式」接入 execution；验证窄 port + composition 注入路径。
- 不适用：不引入跨进程分布式锁 / 消息；不实现第二套执行治理体系；不改变其它业务模块。

## 局限 / 触发

- 真实外部主存储（Cache=Redis/DB）仍未接入（035 NEXT-002）；本接入沿用 memory 主存储。
- 若未来要接入支付/库存等多实例严格一次语义，需回到 035 研究外部主存储接入。

## 对本任务影响

确认「Service 窄 port + composition Adapter + 命名策略」为 Todo 接入设计；`Complete` 为候选用例。
