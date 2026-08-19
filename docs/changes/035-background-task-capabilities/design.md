# 035 设计方案：后台任务能力装配（幂等 / 重试 / 执行记录）

引用支撑研究：`R001`（当前能力清单）、`R002`（语义与装配决策）。

## 1. 总览

新增一个由进程统一装配的底层能力 `execution`：

- `pkg/execution`：项目自有能力契约、状态机、可重试包装、执行记录模型与错误。不拥有 backend 具体类型、不感知第三方。
- `internal/kernel/app/execution`：Kernel App 组件，配置化 + Leased，选择并治理 backend（复用 `pkg/database`），向 composition 输出稳定 facade。
- `internal/kernel/composition/execution.go`：把组件 `Added.Output` 放入 `Capabilities.Execution`，业务模块经 composition 注入 `OperationExecutor` 最小契约。
- 复用 `pkg/resilience`（重试/超时/熔断）、`pkg/fault`（可重试分类）、`pkg/concurrency.SingleFlight`（同进程同 key 并发合并）、`pkg/clock`、`pkg/idgen`。

不重建重试引擎、不引入消息/任务库、不实现订单/支付/库存模块。

## 2. 能力契约（pkg/execution）

### 2.1 核心接口

```go
// Key 标识一次业务操作的幂等单元（如订单创建 key = 业务域 + 业务ID/请求ID）。
type Key string

// Result 描述一次执行的结果，供幂等与调用方判断。
type Result struct {
    Status    Status // PendingRunning|Completed|DuplicateCompleted|Failed|FinalFailed
    Completed bool
    // 执行记录引用（持久化后）。
    RecordID string
}

// Status 不变量：Completed 只有恰好一次；DuplicateCompleted 表示重复成功（不重跑）。
// Failed 表示本次可重试失败；FinalFailed 表示重试耗尽/永久失败。

// Operation 是业务执行体：返回结果与可重试错误。
type Operation func(ctx context.Context) (any, error)

// Execution 描述一次受托管执行。
type Execution struct {
    Key         Key
    Policy      resilience.RetryPolicy // 复用 pkg/resilience；Retryable 用 fault.Retryable
    WithTimeout time.Duration          // 0 = 不额外超时
    Operation   Operation
    Trigger     string                 // 触发者（模块/来源），低敏
}

// OperationExecutor 是业务模块消费的稳定契约。
type OperationExecutor interface {
    Execute(ctx context.Context, exec Execution) (Result, error)
}
```

### 2.2 状态机（幂等）

- `Running`：占用已建立（幂等表中 k/状态=进行中/TTL 内）。同 key 并发经 singleflight 合并，避免重复执行。
- `Success`：操作成功 → 记录执行记录（Completed），占用标记 Completed（含 TTL）。
- `DuplicateCompleted`：再次提交同一 key 且已 Completed → 直接返回，不执行 Operation，不重试。
- `Retryable fail`：可重试失败且未耗尽 → 用 `resilience.Do` 退避重试；每次尝试记录执行记录（失败原因保留链）。
- `FinalFailure`：不可重试错误，或重试耗尽 → 记录 FinalFailed，返回可区分错误。
- 占用 TTL 过期后允许重新执行（乐观占用，不做跨进程分布式锁）。

### 2.3 错误语义（AGENTS 3.3）

- `ErrDuplicate`（可识别已完成，非错误路径）、`ErrBackend`、`ErrRetryExhausted`、`ErrCanceled/ErrTimeout` 分别可区分且保留原因链。
- 记录写入失败不得吞掉原操作结果：返回 `errors.Join(opErr, recordErr)`。
- 幂等键/状态/错误码用命名常量或专用类型，不散写魔法字符串。

## 3. 组件形态（internal/kernel/app/execution）

参考 i18n 的 `app.ManagedConfigured + app.Leased + KernelInstanceSwap` 模板：

- `ID = "execution"`、`ConfigPath = "execution"`。
- `Config`：`backend`（database 默认 / 或 in-memory 测试）、`idempotencyKeyTTL`、`recordRetention`、`maxAttempts`、`initialWait`、`maxWait`、`defaultTrigger`。
- `Definition()` 声明 `app.ManagedConfigured`，输出所谓 Leased facade；`build` 打开 backend 相关存储（复用 `pkg/database` 的 Access/Schema），`newLeaseAccess` 收窄为 `execution.Store`/`OperationExecutor`。
- 组件集中声明应用默认值（对齐 032），不隐式依赖 `pkg/*/DefaultConfig()`。
- 无第三方类型越过 `pkg` 契约；不泄漏 backend 关闭权给消费者。

## 4. 装配（internal/kernel/composition）

- 新增 `execution.go`：`composeExecution(plan)` 使用 `app.Add(plan, executionapp.Definition())`，依赖 Database（复用既有 `databaseOutput`），`app.DependencySet` 注入。
- `composition.Compose` 在 database 之后调用 `composeExecution`，把 `Added.Output` 放入 `Capabilities.Execution`。
- `Capabilities` 增 `Execution execution.OperationExecutor`（可选能力，业务模块按需取用）。
- 顺序确保 backend 依赖已就绪；失败则返回零 Capabilities，Kernel 在最终 Install 前保持为空（AGENTS / app README 验收）。

## 5. 持久化模型（backend）

- 幂等占用表：`idempotency_claims(key, status, expires_at, created_at, updated_at)`。
- 执行记录表：`execution_records(id, key, status, result, error_reason, trigger, duration_ms, created_at)`。
- 表结构/Schema 通过 `pkg/database` 的 Repository 能力归 capability 自己拥有；迁移经既有 migration binding/composition 接线。
- backend 选择（database vs 内存测试实现）由组件配置与 composition 决定；业务模块不感知。

## 6. 业务模块接入（示例）

订单/支付/库存（本任务不实现）通过 composition 注入 `Capabilities.Execution`，例如支付扣款：

```go
res, err := executionExecutor.Execute(ctx, Execution{
    Key:   Key("pay:" + payID),
    Policy: retryPolicy,
    Operation: func(ctx context.Context) (any, error) {
        return ledger.Debit(ctx, account, amount)
    },
})
```

重复提交同一 `payID` 返回 `DuplicateCompleted`，不重复扣款，并留下执行记录。

## 7. 文件影响

| 文件 | 动作 |
| --- | --- |
| `pkg/execution/*.go` | 新增：Key/Result/Status/Execution/OperationExecutor/状态机/错误/Store 契约/记录模型 |
| `internal/kernel/app/execution/*.go` | 新增：Definition、Config、defaults、build、Lease access、backend 接线 |
| `internal/kernel/composition/execution.go` | 新增：`composeExecution`，加入 `Capabilities` |
| `internal/kernel/composition/composition.go` | 修改：`Capabilities` 增 `Execution`，`Compose` 增调用 |
| `pkg/README.md`、`internal/kernel/app/README.md`、`docs/development/application-module-development.md` | 修改：能力清单、当前组件、能力评估表与权威文档同步 |
| 测试 | `pkg/execution/*_test.go`、`internal/kernel/app/execution/*_test.go`、composition 测试 + 门禁 |

## 8. 失败与边界语义

- 幂等占用冲突（同 key 已 Running 且未过期，非并发合并范围内）返回可识别状态，不无限等待。
- 重试仅在 `fault.Retryable` 且未超时/未耗尽时进行；`MaxAttempts` 有上限。
- `WithTimeout` 走 `pkg/resilience.WithTimeout`；取消/超时与业务失败可分。
- backend 不可用：`Execute` 返回 `ErrBackend`，不静默成功；调用方决定降级。
- 记录写入失败：`errors.Join` 保留原结果与记录失败。

## 9. 验证方案

- 单元：幂等判重与重复提交、重试耗尽、可重试 vs 不可重试、超时、backend 失败、错误链、记录写入、singleflight 并发合并。
- 组件：Definition 组装、Lease 切换、配置默认值、内存 backend 构建。
- 装配：`Compose` 产生可用 `Capabilities.Execution`，不泄漏 backend 类型。
- 门禁：`go build/vet/gofmt/test ./... -count=1`、`go mod tidy -diff`、`go generate ./...`、`git diff --check`、architecture validators 无循环/反向依赖。
- 文档一致性：pkg/app/模块指南与实现一致。

## 10. 余留风险/未决项

- backend 具体 SQLite 表能力（TTL 清理、并发占用写入）以真实实现验证为准。
- 是否允许业务模块直接使用单飞/占用语义，需在实现时给出可测试语义；若出现跨进程严格竞争需求，回退研究论证是否引入分布式锁/消息调度。
- 组件是否默认启用：计划 Nodefault 为 enabled（database backend），由配置决定；确认后再定默认开关。

## 11. 吸收恢复治理机制（对既有 035 的增量裁剪）

吸收本目标提出的「降级不能是最终状态，必须形成完整故障恢复机制」要求，在 `pkg/execution` 新增由进程统一装配可用的 `RecoveringStore`（本项目自有契约，不强加平行治理体系，复用既有 `pkg/execution.Store` 契约与 `pkg/resilience` 语义）。

### 11.1 状态机

- `Healthy`：主存储可用，操作直接走主实现。
- `Degraded`：主存储不可用，本次操作降级到本地有限实现；后台恢复循环接管探测。
- `Recovering`：探测与基本可用性验证通过，正在回放降级期间的记录缓冲，回放完毕后原子切回 `Healthy`。
- 恢复中再次失败（验证或回放失败）→ 回到 `Degraded`，按恢复策略继续探测，不影响整个应用。

### 11.2 恢复策略

- 探测不依赖下一次业务请求触发：`RecoveringStore.Start()` 起后台循环，仅在 `Degraded` 状态探测（goroutine 归属本 Store，`Stop()` 等待退出，符合 AGENTS 3.4）。
- 探测等待采用指数退避（`InitialBackoff` 起、`*2` 增长、`MaxBackoff` 封顶）并叠加 ±20% 随机抖动，同时以 `ProbeInterval` 作为最大探测频率下限，避免依赖故障时重连风暴。
- 探测成功不等于恢复：`VerifyAttempts` 次连续基本可用性验证（主 Store 实现 `Verifier` 则调用之；否则用保留 key 做占用+完成往返），验证通过才进入 `Recovering`。

### 11.3 降级期间数据恢复

- 降级期间的执行记录写入本地，并进入有界缓冲（`BufferCapacity`），主存储恢复后逐条回放（`Complete`/`Record`）。
- 缓冲不允许无限积压：到达上限时按 `OverflowPolicy` 处理——`discard`（丢弃并计数）、`block`（阻塞直到腾出空间或上下文结束）、`alert`（丢弃并返回可区分 `ErrBufferOverflow` 供告警）。
- 回放全部成功后才原子切回 `Healthy` 并清空缓冲；任一回放失败即退回 `Degraded` 保留剩余缓冲。

### 11.4 可观测性

- `RecoveringStore.Snapshot()` 输出 `State` / `Buffered` / `Dropped` / `Transitions`，供健康状态与指标上报。
- `OnStateChange(from, to)` 在锁外触发，供应用层输出日志 / 指标 / 告警，且不造成自死锁（回调不得反向阻塞调用）。

### 11.5 明确不引入

- 不实现跨进程分布式锁 / 消息调度 / 主备选举；跨进程严格一次语义属后续独立论证。
- 本增量先把恢复治理机制作为 `pkg` 能力落地并单测（对缓存的 Cache-primary、数据库 backend 与按模块策略隔离的剩余装配为下一增量，见 `tasks.md`）。

## 12. Kernel App 装配恢复治理与按模块策略隔离（对既有 035 的增量裁剪）

把第 11 节的恢复治理机制接入 `kernel/app/execution`，并落地「不同业务模块通过独立配置声明执行策略、实现模块间策略隔离」：

### 12.1 组件装配恢复治理

- `build()` 把 backend Store 包进 `pkg/execution.RecoveringStore`（主存储 + 本地降级兜底均为内存），`Start()` 启动恢复循环；`WithTerminalFinalizer(stop)` 在实例最终化时 `Stop()` 并等待退出（符合 AGENTS 3.4 资源所有权）。
- 组件 `ready()` 不依赖外部基础设施就绪：主存储缺位时以 Healthy 启动，真正的外部主存储不可用时由恢复治理（Degraded + 后台探测）接管，不阻止整个服务启动。
- 恢复治理配置（`probeIntervalMs` / `initialBackoffMs` / `maxBackoffMs` / `verifyAttempts` / `bufferCapacity` / `overflow`）为应用层集中声明并校验，不隐式依赖 `pkg/execution` 默认值（对齐 032）；溢出策略仅允许 `discard` / `block` / `alert`。

### 12.2 按模块策略隔离

- `Config.Policies` 允许业务模块按名独立声明 `retryMaxAttempts` / `retryInitialWaitMs` / `retryMaxWaitMs` / `timeoutMs`。
- `pkg/execution.Execution` 新增可选 `PolicyName` 字段（装配方元数据，pkg 执行器不感知）。组件 `Access.Execute` 依此解析命名策略填充 `Policy`/`Timeout`；未知策略名返回可识别错误，不静默回退；未声明时回退到组件默认策略。
- 示例：业务模块（支付）`Execution{Key, PolicyName: "payment", Operation: ...}` 即按 `policies.payment` 策略执行，与其他模块互不绑定。

### 12.3 能力边界（本地降级语义）

- 本地 memory 降级只是**单实例容错手段**，不提供与分布式 Cache 相同强度的幂等/跨进程一次语义。多实例环境下，本地降级缓冲与占用仅在当前进程内可见，不保证跨实例去重一致；该边界在实现与文档中明确，不默认声称等价。

### 12.4 剩余增量

- 真正的外部主存储接入（Cache-primary Redis 或 database backend）作为下一增量；届时 RecoveringStore 的 Degraded/Recovering 语义在应用内实际触发。
