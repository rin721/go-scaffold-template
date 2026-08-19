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
