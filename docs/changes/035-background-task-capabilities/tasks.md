# 035 任务清单：后台任务能力装配（幂等 / 重试 / 执行记录）

任务 ID 稳定；状态：研究门禁通过，计划待确认。

| ID | 依赖 | 任务 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- |
| EXEC-001 | — | 新增 `pkg/execution`：Key/Result/Status/Execution/OperationExecutor/Store 契约、状态机、命名错误（ErrDuplicate/ErrBackend/ErrRetryExhausted/ErrCanceled/ErrTimeout）、执行记录模型；复用 `pkg/resilience`/`pkg/fault`/`pkg/concurrency`/`pkg/clock`/`pkg/idgen` | 契约编译；错误保留原因链；不 import internal；单测覆盖语义 | 待确认 |
| STORE-001 | EXEC-001 | 实现幂等占用表与执行记录表持久化（复用 `pkg/database`）；TTL 过期与占用/完成状态迁移；提供内存 backend 供测试 | 建表/Schema/迁移接线；backend 失败可区分；并发占用经 singleflight 合并 | 待确认 |
| COMP-001 | STORE-001 | 新增 `internal/kernel/app/execution`：配置化 + Leased 组件（`app.ManagedConfigured`/`Leased`/`KernelInstanceSwap`），集中声明应用默认配置（032），输出稳定 `OperationExecutor` facade | Definition 组装/Lease/配置默认值/backend 生命周期测试；不泄漏 backend 类型与关闭权 | 待确认 |
| WIRE-001 | COMP-001 | `internal/kernel/composition/execution.go` 新增 `composeExecution`，`app.DependencySet` 注入 Database；`composition.go` 的 `Capabilities` 增 `Execution`，`Compose` 在 database 后调用 | 装配成功产出可用 `Capabilities.Execution`；失败返回零 Capabilities；无循环/反向依赖 | 待确认 |
| TEST-001 | EXEC-001, STORE-001, COMP-001, WIRE-001 | 端到端与门禁测试：幂等重复提交、重试耗尽、可重试 vs 不可重试、超时、backend 失败、错误链、记录写入、单飞合并；composition 装配 | `go test ./... -count=1` 相关包通过；architecture validators 通过 | 待确认 |
| DOC-001 | 全部 | 同步 `pkg/README.md`（能力清单从"暂缓路线"移到"当前能力"）、`internal/kernel/app/README.md`（当前组件 + 手工装配步骤）、模块开发指南能力表、035 文档状态 | 文档与实现一致；`pkg/README` 不再把后台任务列在暂缓 | 待确认 |
| RCV-001 | EXEC-001 | 新增 `pkg/execution.RecoveringStore`：Healthy/Degraded/Recovering 状态机、主存储故障降级到本地有界实现、有界记录缓冲（discard/block/alert 溢出策略）、后台恢复循环（退避+抖动+最大频率探测、可用性验证、缓冲回放、原子切回主实现）、`Snapshot`/`OnStateChange` 可观测、goroutine 归属与 `Stop()` | 状态机/溢出/回放/生命周期单测 + executor 集成测试通过；错误保留原因链；不 import internal | 已完成 `d42e044+本轮` |
| NEXT-001 | RCV-001 | 把 `RecoveringStore` 接入 `kernel/app/execution`（恢复治理配置 + 命令式按模块策略隔离 `Config.Policies`/`Execution.PolicyName`），memory backend 装配恢复治理并接线 terminal finalizer | `Access.Execute` 依命名策略解析；恢复配置校验与默认值集中声明；组件 `ready` 不阻塞启动；本地降级边界文档化 | 已完成 `本轮` |
| NEXT-002 | NEXT-001 | 接入真正外部主存储（Cache-primary Redis 或 database backend），使 `RecoveringStore` 的 Degraded/Recovering 语义在应用内实际触发；composition 接线缓存/数据库依赖 | 装配出可用 `Capabilities.Execution`；多实例能力边界明确 | 待确认 |
| OBS-001 | NEXT-001 | 恢复治理可观测性接线：`Access.Recovery()`/`Access.Health()`、组件注入结构化 Logger 并在状态变化时输出 Warn/Info 日志、`pkg/execution.RecoveringStore.Health()` | 组件单测覆盖 Recovery/Health/日志回调；不泄漏敏感字段 | 已完成 `本轮` |
| INT-001 | OBS-001 | 组件级端到端验证：经 `Access` 注入故障主存储，覆盖 主存储故障→降级本地有限能力→后台自动恢复探测→回放→原子切回→幂等去重回到主存储，并验证 Recovery()/Health()/日志随状态变化输出 | 端到端单测（`-race`）通过；`assemble` 内部装配缝支持故障注入 | 已完成 `本轮` |
| ASYNC-001 | INT-001 | 执行记录异步持久化：`pkg/execution.AsyncRecorder`（幂等占用/完成同步、过程/失败记录异步有界队列 + 溢出策略 + 排空式 Shutdown），`kernel/app/execution` 装配 `Config.Async` 并接线 `stop()` 关闭顺序 | pkg 异步/溢出/排空/竞态单测 + 组件装配通过；不阻塞业务链路 | 已完成 `本轮` |
| GOV-001 | WIRE-001 | 门禁/反例：`pkg/execution` 不得 import internal；业务模块不得反向 import backend 具体类型；execution 组件不加隐藏第三方 | 架构 validator 覆盖并通过；无循环/反向依赖 | 待确认 |
| VER-001 | 全部 | 全量验证：`go build/vet/gofmt ./...`、`go test ./... -count=1`、`go mod tidy -diff`、`go generate ./...`、`git diff --check` | 全部通过；更新 035 状态为已完成并聚焦提交（不 push） | 待确认 |

## 确认状态

- 研究门禁：已通过（R001、R002）。
- 计划状态：已确认并进入实施（本目标授权继续调整 035，已在既有 `d42e044` 之上追加恢复治理机制增量 RCV-001）。
- 已实现并提交：`pkg/execution`（幂等/重试/执行记录契约、`OperationExecutor`、memory backend）、`kernel/app/execution` 组件、composition 装配（`d42e044`）；恢复治理能力 `pkg/execution.RecoveringStore` 及测试；`kernel/app/execution` 装配恢复治理（memory backend）与按模块策略隔离（`Config.Policies`/`Execution.PolicyName`）。
- 下一增量（NEXT-002）：Cache-primary/数据库 backend 真实外部主存储接入，使 Degraded/Recovering 语义在应用内实际触发，尚未开始。

## 剩余风险

- backend 具体 SQLite 表能力（TTL 清理、并发占用写入）以真实实现验证为准。
- 跨进程严格竞争/分布式锁/消息调度暂不引入；出现该需求时回退研究重新论证。
- 组件默认启用与否由配置决定，确认后再定。
