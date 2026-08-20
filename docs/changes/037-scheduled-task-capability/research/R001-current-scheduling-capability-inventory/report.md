# R001 当前定时调度与运行治理能力清单

## 1. 研究问题与范围

本记录回答：在当前 `a675f33` 代码快照中，定时调度可以复用哪些真实能力，哪些只是文档目标，
缺口应落在哪一层。范围覆盖模块 Contribution、Application Generation、Supervisor、Execution、Cache、
Observability、Ops health/diagnostics、配置与资源 owner；不设计具体业务任务。

## 2. 方法与证据

- 从根 README、`docs/README.md`、`docs/changes/README.md` 进入当前主题文档。
- 检索全部既有 research metadata；打开 023、035、036 与模块开发指南中的相关记录。
- 追踪 `cmd/app -> internal/composition -> GenerationCoordinator/Supervisor` 的长期 Service 路径。
- 逐项核对 `module.Contribution`、`pkg/execution`、`internal/kernel/app/execution`、Cache Redis resource、
  Telemetry 和 Ops runtime snapshot 的定义、构造、消费者、停止和测试。

## 3. 当前事实

### 3.1 当前没有调度或分布式锁实现

- `pkg/README.md` 仍把 `cron/interval` 调度与分布式锁列在暂缓路线。
- `go.mod` 没有调度库；源码没有 schedule/cron/fixedDelay/lease/fencing/Redis lock 实现。
- `internal/module.Contribution` 只有 `ID` 与 `Participants`，无法声明任务 ID、触发方式、并发或协调策略。

因此定时调度是新底层能力，不存在可以“打开开关”的现有 scheduler。

### 3.2 生命周期与运行失败已有唯一 owner

- 长期 Service 只有一个进程级 `pkg/supervisor.Supervisor`。它拥有 `GenerationCoordinator`、application lifecycle、
  config watcher 与 generation monitor；关键 runner 意外退出会触发进程失败。
- `GenerationCoordinator` 已定义 `Prepare -> Commit -> Retire/Abort/Stop`。候选 generation 在 commit 前不能获得
  生产 admission，旧 generation 在新代 commit 后排空。
- `applicationGenerationFactory.failures` 已由 `GenerationCoordinator.Monitor` 上送 Supervisor，适合承接调度引擎的
  terminal failure，不需要第二个 Runtime/Supervisor。
- 当前 generation 在 `Prepare` 中直接 `Start` 模块 Participant。若把会触发业务任务的 scheduler 直接作为
  Participant，候选可能在 commit 前执行任务，违反 generation admission；当前 Contribution 形态不能直接套用。

### 3.3 Execution 可复用，但不提供跨实例执行权

- `pkg/execution.OperationExecutor` 已组合幂等占用、`pkg/resilience` 重试/超时、执行记录、Trace 字段与同进程 singleflight。
- `internal/kernel/app/execution` 已提供命名策略、恢复状态、异步记录和稳定 Access；Todo 已通过 composition 窄 Adapter 真实消费。
- 当前 primary/local 都是 memory Store。035/036 权威文档明确它不保证多实例去重，也不是分布式锁。
- `Execution` 很适合治理“获得触发权之后如何运行并记录”，但不能决定哪个实例获得调度执行权。
- 当前 `WrapBackend`/`WrapRetryExhausted` 使用 `%v` 而非 `%w`，与注释中的“保留原因链”不一致；调度需要识别
  cancel/timeout/backend，实施时必须在既有 Execution 单轨修复，而不是在 scheduler 再造错误分类。

### 3.4 Cache 可复用同一 Redis 资源，但当前接口不是锁

- `internal/kernel/app/cache` 唯一拥有 `redis.Client` 的构造、Ping 和 Close；业务只拿稳定 Access。
- `pkg/cache.RemoteStore` 只有普通 `Get/Set/Delete/InvalidateTags`。普通 `Set` 会覆盖值，没有 `NX`、owner token、
  compare-and-renew 或 compare-and-delete，不能拿来冒充分布式锁。
- Cache 默认 disabled；启用 Redis 时当前 `Ready` 会 Ping 并阻断候选。要让每个任务选择 skip/pause/fail，
  Redis 的运行期可用性判断必须从“Cache 全局启动必需”收敛为消费方策略，并提供 Health/diagnostics。
- 正确的最小增强是在同一 Cache resource 内增加项目自有 coordination facade，共用 client/连接池；不能由
  scheduler 自行创建第二套 Redis client，也不能把 `redis.UniversalClient` 暴露出去。

### 3.5 Tracing、health 和 diagnostics 需要扩展现有出口

- `pkg/observability.Telemetry` 目前只提供 HTTP middleware 与 exporter diagnostics，没有后台工作 span 入口。
- `pkg/execution.Record.Trace` 与 `WithTrace` 可以接收 trace ID，但不会创建 span。
- Ops 当前 snapshot/ready 只聚合 process/generation、Auth、Database 与 Telemetry，没有 scheduler task 状态。
- `pkg/health` 已有 pass/warn/fail 结果，Ops 已有唯一 management endpoints；调度应把快照投影进去，不能新增
  `/scheduler/health` 或第二个健康 registry。

## 4. 推断

1. 调度触发、并发与 task-level coordination 是跨业务复用且由进程统一选择的底层能力，满足完整能力链条件。
2. 模块需要新增项目自有 Schedule Binding，并由 Contribution 显式交给唯一 composition root；不能扫描包、`init` 注册或让模块拿 scheduler handle。
3. 调度运行 admission 必须成为 Application Generation 的一部分，形态应与 ListenerHub 的候选/commit/retire 类似；
   复用 generation 和 Supervisor 状态机，不新建平行生命周期。
4. Execution 继续拥有重试、幂等和实际执行记录；scheduler 只生成稳定 occurrence identity、选择执行权并调用 Execution。
5. 分布式 lease 是独立项目契约，但 Redis Adapter 应由现有 Cache resource 提供，以复用连接和关闭 owner。
6. Telemetry 只需增加项目自有 background-work observation，不应把 OTel Tracer/Span 暴露给 scheduler 或模块。

## 5. 适用、不适用与限制

- 适用：当前单进程 Service profile、多实例共享同一受支持 Redis coordination endpoint、Application Generation reload。
- 不适用：CLI one-shot、消息消费、跨数据中心共识、任意业务副作用的 exactly-once。
- 限制：Redis lease 能约束“谁持有执行权”，不能自动回滚忽略 context 的业务副作用；严格任务的 Run 必须遵守取消。
- 真实 Redis 故障/恢复、时钟偏移和 lease 续期需要确认后的集成测试；当前只完成静态研究。

## 6. 对 037 的影响

- 037 不能只新增 `pkg/schedule`；必须同时扩展 module Contribution、Application Generation admission、Cache coordination、
  background tracing 与 Ops 投影。
- 037 不实现具体业务任务；使用测试 fixture 证明多个模块 Binding 聚合与策略隔离。
- Cache readiness 语义变化、Observability 契约扩展和 Execution 错误链修复都是当前目标的必要合并增强，必须在计划中显式列出并接受确认。
