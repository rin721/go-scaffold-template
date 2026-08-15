# FOUNDATION-LIFECYCLE-001：生命周期闭环独立实施计划

## 1. 文档定位与门禁

本文是 022 内 `FOUNDATION-LIFECYCLE-001` 的唯一施工级计划。它把 [R005](../research/R005-resource-finalization-policy/report.md) 的逐资源研究结论冻结为可直接实施的 Go 契约、状态转换、文件影响和测试清单；022 的 `requirements.md`、`design.md`、`acceptance.md` 与 `tasks.md` 只保留上层目标和链接，不再复制另一套实现细节。

- 当前状态：**已确认并实施完成**。
- 实施授权：用户于 2026-08-15 在计划报告后的后续消息明确确认 `FOUNDATION-LIFECYCLE-001`。
- 实施结果：`LFC-001` 至 `LFC-010` 已按本文单轨完成；验证证据回写到 [`tasks.md`](../tasks.md)，未把后续 Foundation/API Program 混入本任务。
- 实施范围：仅覆盖本文 `LFC-001` 至 `LFC-010`；发现必须改变公共目标、依赖选择、模块边界、数据、外部副作用或本文冻结契约时，返回研究/计划阶段并重新确认。
- 提交边界：022 计划文档与后续实现按仓库规则在确认后作为同一任务变更提交；当前仍保持未暂存、未提交。

## 2. 要解决的确定问题

当前代码在正常路径可完成启动、reload、drain 和 stop，但失败路径存在四个责任断点：

1. terminal drain 等待超时后，`Kernel.Stop` 把状态改为 stopped；最后一个 Lease 释放只关闭 channel，不再有 owner 完成 finalization。
2. candidate 与 previous 的 finalizer 返回错误后，`managedComponent` 仍清空实例；错误链存在，实际 owner/reference 已丢失。
3. `WithStop` 把“无需释放”“排空后一次终结”“可重试终结”和“协议 graceful/force”压成同一函数，无法阻止错误策略。
4. HTTP `Stop` 内部自动从 `Shutdown` 降级到 `Close`，且等待再次超时后仍缓存 stopped；Supervisor 也无法区分 graceful、forced 和 pending Participant。

本任务不通过增加万能 Registry 或通用生命周期框架解决问题，而是在两个现有 owner 域内分别闭环：

```text
Kernel generation owner
  -> admission / Lease drain
  -> optional terminal finalizer
  -> generation-scoped result

Supervisor process owner
  -> graceful participant stop
  -> optional protocol-specific force
  -> participant/task completion result
```

## 3. 冻结决策

### 3.1 统一什么、不统一什么

统一：owner ID、instance generation、phase、state、attempt、deadline 消费、错误链、诊断快照和“不能假 stopped”的不变量。

不统一：Kernel component 与 Supervisor Participant 接口、协议 graceful shutdown、terminal resource close、consumer-owned handle、是否存在 force，以及第三方 release verification。

因此不会新增以下 API：

- `Close(force bool)`、`Stop(mode string)` 或可组合任意行为的 options bag；
- 全局 resource registry、反射扫描、后台 cleanup worker或第二套 DI/lifecycle runtime；
- 未被真实资源证明的通用 retry/backoff engine；
- 跨进程直接操作内存 instance 的 CLI command。

### 3.2 当前场景与实际落点

| 设计场景 | 本任务实际表达 | 当前对象 | 是否实施 |
| --- | --- | --- | --- |
| `NoFinalization` | Kernel definition 不声明 finalizer | Clock、ID、Validator、I18n、Todo migrator、当前 StorageManager | 是 |
| `DrainThenTerminalClose` | `WithTerminalFinalizer`；每 generation 最多一次 raw attempt | Database、Redis、configured logger；config watcher 与文件 Storage 在各自 owner 内同语义 | 是 |
| `GracefulShutdown` | `Participant.Stop(ctx)` 返回 nil 才表示完成 | Kernel Coordinator、普通 Participant、长期 Task 取消等待 | 是 |
| `GracefulThenForce` | Participant 额外实现 `ForceStopper` | HTTP Server | 是 |
| `RetryableFinalize` | 只有研究证明具体 Adapter 可安全补做后才能新增的扩展点 | 当前没有实例 | **不实施代码** |

`RetryableFinalize` 保留为分类和扩展门禁，不预建接口、配置、退避器或假测试。以后第一个真实 retryable Adapter 必须新建研究/变更，说明失败点、可重复步骤、attempt 上限、退避、deadline 与 release verification。

### 3.3 terminal failure 的责任

Database、Redis、logger 和 fsnotify 的 raw close 都按 terminal attempt 对待：

- 第一次调用返回成功：状态为 `finalized`，清空 live reference；
- 第一次调用返回错误：状态为 `terminal-failed`，缓存同一错误并继续保留 owner、generation 和 live reference；
- 后续 `Stop`/cleanup 调用只返回缓存结果，不再次调用 raw close；
- `terminal-failed` 不是 pending retry，也不是 success。当前没有安全 recovery operation，只能阻断后续 reload、由顶层返回非零，并保留诊断直到进程退出。

这样避免“盲重试”，也不把仍可能持有物理资源的对象交给 GC 冒充释放。

## 4. Kernel generation 契约

### 4.1 Definition API：单轨替换 `WithStop`

在 `internal/kernel/app/contracts.go` 删除 `WithStop`，新增以下唯一 finalizer option：

```go
// TerminalFinalizer 执行不可安全重做的一次性资源终结动作。
type TerminalFinalizer[I any] func(context.Context, I) error

// WithTerminalFinalizer 声明实例在 admission 关闭并排空后执行一次终结。
func WithTerminalFinalizer[I any](finalize TerminalFinalizer[I]) Option[I]
```

确定语义：

- 不声明 option 即 `NoFinalization`，不提供空函数占位；
- nil 和重复声明在 Definition freeze 前返回错误；
- option 不承诺 retry 或 force；
- finalizer 必须完整返回 flush/close/verification 的聚合错误，不在 Adapter 内记录后吞掉；
- 删除 `WithStop`，不保留 alias、deprecated wrapper 或兼容分支。

`lifecycle[I]` 的 `stop` 字段同步单轨改名为 `terminalFinalizer`。

### 4.2 instance generation 与 slot

在 `internal/kernel/app/finalization.go` 新增内部 instance slot。每次 `Build` 成功取得非 nil instance 时，由该 component 的单调计数器分配新的 `InstanceGeneration`；它独立于配置 `Snapshot.Generation`，即使固定配置或相同 digest 也不会复用编号。

```go
type instanceSlot[I any] struct {
	generation uint64
	instance   I
	phase      FinalizationPhase
	state      FinalizationState
	attempts   uint32
	result     error
}
```

`managedComponent` 不再使用 `candidate/hasCandidate`、`previous/hasPrevious` 两组零值布尔字段，而是持有：

```go
candidate *instanceSlot[I]
retired   *instanceSlot[I]
stopping  *instanceSlot[I]
nextGeneration uint64
```

- `candidate`：Build 后、尚未发布或正在失败补偿的实例；
- `retired`：reload commit 后的旧代；cleanup 成功后清空，terminal failure 时保留；
- `stopping`：terminal stop 从 Lease 取出的当前代；成功后清空，terminal failure 时保留；
- reload 在 `retired != nil` 时不得再 commit 新一代，防止覆盖未闭环责任。

### 4.3 冻结状态与快照

在 `internal/kernel/app/finalization.go` 定义仅供仓库内部使用的 typed string：

```go
type FinalizationPhase string

const (
	FinalizationPhaseCandidate FinalizationPhase = "candidate"
	FinalizationPhaseRetired   FinalizationPhase = "retired"
	FinalizationPhaseCurrent   FinalizationPhase = "current"
)

type FinalizationState string

const (
	FinalizationWaitingForDrain FinalizationState = "waiting-for-drain"
	FinalizationPending         FinalizationState = "pending"
	FinalizationRunning         FinalizationState = "running"
	FinalizationSucceeded       FinalizationState = "finalized"
	FinalizationTerminalFailed  FinalizationState = "terminal-failed"
)

type FinalizationSnapshot struct {
	ComponentID       ID
	InstanceGeneration uint64
	Phase             FinalizationPhase
	State             FinalizationState
	Attempts          uint32
	LastErrorType     string
}
```

快照不包含 instance、配置值、DSN、地址或原始 error text。时间、跨 owner 统一视图、外部 management API 和持久化审计留给 `FOUNDATION-DIAGNOSTICS-001`；本任务只提供不会丢责任的最小安全事实。

### 4.4 状态转换

| 入口 | 前态 | 操作 | 成功后 | 失败/超时后 |
| --- | --- | --- | --- | --- |
| Build | 无 slot | 构造 instance、分配 generation | candidate/pending | 无 instance 时不建 slot；部分构造由第 7 节处理 |
| DiscardCandidate | candidate/pending | raw finalizer 0 或 1 次 | 清空 candidate/finalized | 保留 candidate/terminal-failed，缓存错误 |
| BeginDrain | serving | 拒绝新 Use | waiting-for-drain | 无状态回退 |
| Commit | drain complete + candidate | current 变 retired，candidate 变 current | retired/pending + serving | 前置不满足即错误，不部分提交 |
| FinalizeRetired | retired/pending | raw finalizer 0 或 1 次 | 清空 retired/finalized | 保留 retired/terminal-failed，缓存错误 |
| PrepareStop | drain complete | current 从 Lease 移入 stopping | stopping/pending | 不满足 drain complete 不移动 reference |
| FinalizeCurrent | stopping/pending | deactivate 后 raw finalizer 0 或 1 次 | 清空 stopping/finalized | 保留 stopping/terminal-failed，缓存错误 |
| 重复 finalization | terminal-failed | 不调用 raw finalizer | 不适用 | 返回缓存错误，attempt 仍为 1 |

没有 finalizer 的 slot 仍经过 owner 状态转换，但 raw attempt 为 0，并直接成功；这样 `NoFinalization` 不制造空 Close，也可证明 generation 已退出 Lease。

### 4.5 Lease drain 与可继续 Stop

`lease.beginDrain` 改为幂等的 `beginOrContinueDrain`：

- `serving`：关闭 admission，进入 draining，返回本代 `drained` channel；
- `draining`：返回同一个 channel，不创建新 channel、不恢复 admission；
- `stopped`：返回已关闭 channel；
- pending/未发布：保持现有明确错误。

最后一个 active `Use` 释放时只关闭 `drained` channel，不启动无预算后台 finalizer。若调用 `Stop(ctx)` 的 caller deadline 先到：

1. Kernel 保持 `draining`，Coordinator 保持 `cleanup-pending`；
2. live instance 仍在 Lease，新增 `Use` 持续返回 draining/stopped 类错误；
3. 后续对同一 `Coordinator.Stop(newCtx)` 的调用继续等待同一 channel；
4. drain 完成后才 `PrepareStop` 并执行一次 terminal finalizer。

不创建后台 worker 的原因是当前没有独立于 process shutdown 的合法 lifetime、budget 和错误接收者。未来 management operation 如需继续 cleanup，仍调用同一 owner 的 `Stop(ctx)`，不直接拿 instance。

### 4.6 Kernel 与 Coordinator 终态

`internal/kernel/kernel.go` 新增内部 `kernelDraining`，并修改 `Kernel.Stop`：

- `running -> draining` 后开始/继续反向 drain；任一等待超时返回错误但不写 stopped；
- 所有 drain 完成后反向 `PrepareStop/FinalizeCurrent`；全部成功才写 stopped；
- terminal finalizer 失败写 failed，保留 slot；重复 Stop 返回缓存错误，不二次 raw close；
- 已 stopped 的 Stop 继续返回 nil；尚未 start 的现有语义保持确定测试。

`internal/kernel/coordinator.go` 新增：

```go
const LifecycleCleanupPending LifecycleState = "cleanup-pending"

type Diagnostics struct {
	// 现有字段保持。
	CleanupRequired bool
	Finalizations   []app.FinalizationSnapshot
}
```

- drain caller deadline -> `LifecycleCleanupPending`、`CleanupRequired=true`；允许重复 Stop；
- terminal-failed -> `LifecycleFailed`、`CleanupRequired=true`，快照保留失败 slot；
- 全部完成 -> `LifecycleStopped`、`CleanupRequired=false`；
- committed retired cleanup 失败仍为 `LifecycleDegraded` 并阻断 reload，同时暴露 retired snapshot；
- `Diagnostics()` 深拷贝 slice；不把原始 error message 或 resource address 放入快照。

`CommittedCleanupError` 保留“新代已经提交”的现有语义；底层 error 继续用 `%w`/`errors.Join`，不改写成纯字符串。

## 5. Supervisor 与 HTTP 契约

### 5.1 Supervisor 的 force 是协议 opt-in

保留现有 `Participant`，新增唯一可选接口：

```go
// ForceStopper 表示协议明确支持有损终止的 Participant。
type ForceStopper interface {
	ForceStop(context.Context) error
}
```

`Participant.Stop(ctx)` 的契约收紧为：返回 nil 表示该 Participant 拥有的运行循环和资源已经完成 graceful stop；只发出停止信号但未等待完成不能返回 nil。普通 Participant 不实现 `ForceStopper`，Supervisor 不通过类型外推 force。

### 5.2 一个总 budget，预留 force 阶段

保留兼容字段 `Config.ShutdownTimeout` 作为整个 Supervisor stop 的总预算，新增：

```go
type Config struct {
	ShutdownTimeout time.Duration
	ForceTimeout    time.Duration
}
```

冻结默认值：

```go
const (
	defaultShutdownTimeout = 10 * time.Second
	defaultForceTimeout    = 1 * time.Second
)
```

校验与计算：

- `ShutdownTimeout <= 0` 使用 10 秒默认；`ForceTimeout <= 0` 使用 1 秒默认；
- `ForceTimeout >= ShutdownTimeout` 在 `New` 时不能继续静默修正，因此 `New` 改为 `New(Config, ...Participant) (*Supervisor, error)` 并返回配置错误；
- graceful deadline = stop 起点 + `ShutdownTimeout - ForceTimeout`；final deadline = stop 起点 + `ShutdownTimeout`；
- Participant stop、force 与 Task wait 只消费这两个绝对 deadline，不按组件重建完整 timeout；
- `HostOptions` 同步新增 `ForceShutdownTimeout`，现有 `ShutdownTimeout` 仍是总预算；默认行为为总计 10 秒，其中最后 1 秒只供 force/最终 task wait。

这是一次明确公共构造签名迁移；仓库内全部调用方必须同批修改，不保留第二个 `MustNew` 或旧构造器。

### 5.3 stop/force 算法

1. 取消所有 Task context，记录 stop 起点与两个绝对 deadline。
2. Participant 按启动反序调用 `Stop(gracefulCtx)`；每个调用仍用结果 channel 隔离不合作实现，但共享同一 graceful deadline。
3. 返回错误或 deadline 时，把 participant name 和错误加入 pending，不宣称 stopped。
4. graceful 阶段结束后，只对 pending 且实现 `ForceStopper` 的 Participant 按反序调用 `ForceStop(forceCtx)`。
5. force 返回 nil 的 Participant 记录为 forced 并从 pending 移除；错误或超时继续 pending。
6. 在 final deadline 前收集 Task 与仍在运行的 Stop goroutine结果；无法证明结束的 unit 留在 snapshot。
7. 任一 graceful error、forced 结果或 pending unit 都进入最终聚合结果；forced 不等于 graceful success。

`Snapshot` 在本任务扩充为：

```go
type Snapshot struct {
	// 现有字段保持。
	PendingParticipants []string
	PendingTasks        []string
	ForcedParticipants  []string
}
```

删除含义混合的 `PendingUnits`。跨 Kernel/Supervisor 的统一 owner/generation/phase 诊断仍由后续 `FOUNDATION-DIAGNOSTICS-001` 完成。

### 5.4 HTTP Server 单轨拆分 graceful 与 force

`pkg/httpx.Server` 保留 `Participant.Stop(ctx)` 并新增 `ForceStop(ctx)`：

- `Stop` 只调用 `http.Server.Shutdown(ctx)` 并等待 `Serve` 结果；不得在内部调用 `Close`；
- graceful deadline 到期时保持 `serverStopping`，返回 error，不调用 `completeStop`；
- `ForceStop` 才调用 `http.Server.Close()`，随后在传入 context 内等待 `Serve` 结果；成功进入新终态 `serverForced`；
- graceful 完成进入 `serverStopped`；Serve 意外退出进入 `serverFailed`；
- created/started 状态不需要中断 active request：关闭未服务 listener并等待一致终态；
- 并发 Stop 共享同一次 graceful 结果；并发 ForceStop 共享同一次 force 结果；channel 只关闭一次；
- stopped/forced 的重复调用返回缓存结果，不再次关闭 listener/server；
- HTTP 不管理 hijacked connection。当前无 WebSocket/upgrade profile；出现真实需求时必须独立建立连接 registry、owner 和验收，不能把 `Server.Close` 当作证明。

`var _ supervisor.ForceStopper = (*Server)(nil)` 只在 `pkg/httpx` 无法导入 supervisor 而不形成错误依赖时添加；优先用 compile-time 局部匿名接口断言，避免 transport package 反向依赖 process supervisor。

## 6. 逐资源最终政策

| 资源 | owner | Definition/协议 | 失败后 raw retry | force | 本任务迁移 |
| --- | --- | --- | --- | --- | --- |
| Database | Kernel generation | `WithTerminalFinalizer` | 禁止 | 无 | 构造期移除 Ping；Ready 保留唯一 Ping；wrapper Close 缓存首次结果 |
| Redis | Kernel generation | `WithTerminalFinalizer` | 禁止 | 无 | 保持 go-redis client；由 slot 阻止第二次 raw Close |
| configured logger | Kernel generation | `WithTerminalFinalizer` | 禁止 | 无 | 保持 activate/deactivate；Sync+sink Close 聚合结果缓存 |
| baseline logger | `cmd/app` process | 现有具体 `Close` | 禁止 | 无 | 不进入 Kernel；顶层继续合并 execute 与 close error |
| StorageManager | Kernel generation | 不声明 finalizer | 不适用 | 不适用 | 删除 app definition 的空 `stop` 与 `WithStop`；Manager public Close 暂保 consumer API |
| HTTP Server | Supervisor Participant | `Stop` + optional `ForceStopper` | 不适用 | 显式允许 | 按第 5.4 节拆分并验收 graceful/forced |
| config watcher | Supervisor Task 内部 | defer 中 terminal close | 禁止 | 无 | 命名返回值 `errors.Join` Close error；用内部 factory 做故障测试 |
| typed cache Client | 创建它的 consumer/composition | 自有 `Close` | 禁止 | 无 | 不并入 Redis Kernel finalizer；补充 owner 文档和关闭后测试 |
| 文件 Storage watcher | 创建 `storage.New` 的 consumer | 自有 `Close` terminal attempt | 禁止 | 无 | 保留 baseline，聚合 Remove/Close，取消并等待内部 watch goroutine，缓存首次结果 |
| Workbook | 调用方 | `Workbook.Close` | 由 excelize 语义决定，当前不自动 retry | 无 | `ReadExcelSheet` 合并读取错误与 defer Close error |
| `pkg/resource.Registry` | 无生产 owner | 重复生命周期原语 | 不可靠 | 无 | 删除 package、README 和测试；全仓引用必须为零 |

### 6.1 Database 部分构造闭环

`pkg/database.NewGORM` 只负责创建 GORM/`sql.DB` pool，不执行网络 `Ping`。连接可达性只由 `internal/kernel/app/database.ready` 在 candidate 已交给 Kernel owner 后检查：

- open 失败：构造器没有可返回 instance，按 driver 自身错误返回；
- open 成功：立即返回 Resource，Kernel 建 candidate slot；
- Ready Ping 失败：`DiscardCandidate` 使用同一 slot 执行 terminal finalizer；
- finalizer 失败：candidate slot 保留 owner/reference/error chain，启动失败不能伪装已补偿。

Database wrapper 的 `Close` 使用 `sync.Once + closeErr` 缓存第一次结果；`closed` 在第一次 attempt 开始时设置，后续 query 拒绝，重复 Close 返回相同结果而非 nil。该 wrapper 仍不提供 retry。

logger 多 sink 在构造期失败仍可能有局部 handle；本任务只要求构造函数继续 best-effort 反向关闭并 `errors.Join`。因为其函数目前不能返回残余 instance，若新增故障测试证明存在无法转交的实际 handle，再退回计划修改 Builder result；本任务不先引入所有 Builder 都必须使用的部分构造泛型。

### 6.2 config watcher

在 `internal/kernel/config/watch.go` 引入包私有最小后端接口和 factory，仅用于把 fsnotify 创建/Add/Events/Errors/Close 隔离成可故障注入的内部实现；不导出第三方类型。`WatchFiles` 使用命名返回值：

```go
defer func() {
	if closeErr := watcher.Close(); closeErr != nil {
		resultErr = errors.Join(resultErr, fmt.Errorf("close config watcher: %w", closeErr))
	}
}()
```

context cancellation本身保持正常退出；若 Close 失败则最终返回 Close error。Add 失败与 Close 失败同时发生时必须都可 `errors.Is`。

### 6.3 consumer-owned 文件 Storage 与 Workbook

文件 Storage 仍是公开 baseline，但不进入 Kernel。实施时：

- `impl` 增加 `closeOnce`、`closeErr` 与 watch goroutine `WaitGroup`；
- `Watch` 拒绝 nil handler 和已 closed 实例；每个已启动 goroutine 都登记 owner；
- `Close` 在锁内标记 closed、复制 watch entries、清空 map，锁外 cancel、尽最大努力 Remove、Close watcher、等待内部 goroutine，使用 `errors.Join` 聚合并缓存结果；
- 重复 Close 返回相同 cached result，不返回伪 nil、不重复调用 fsnotify；
- `StopAllWatch` 改为返回 `error`，全部调用方/测试/文档单轨迁移；
- handler 必须及时返回且不得把 callback 当长期任务；当前不增加 handler context、force kill 或每 handler goroutine。真实阻塞 handler 场景需另立公共 API 变更。
- `ReadExcelSheet` 改为命名返回值，在 defer 中 `errors.Join` `Workbook.Close` 错误；调用者直接取得 Workbook 时仍由调用者 Close。

## 7. 文件影响清单

### 7.1 新增

| 文件 | 内容 |
| --- | --- |
| `internal/kernel/app/finalization.go` | slot、phase/state/snapshot、terminal finalizer 执行与缓存 |
| `internal/kernel/app/finalization_test.go` | 状态转换、attempt、reference 保留、snapshot 脱敏 |
| `pkg/supervisor/shutdown.go` | shutdown deadline、participant graceful/force result 的小型内部结构；若实现后不足以独立成文件则并回 `supervisor.go` |

不新增公共 `pkg/lifecycle`、`pkg/resource2` 或 cleanup daemon。

### 7.2 修改

| 文件/目录 | 必须变化 |
| --- | --- |
| `internal/kernel/app/contracts.go` | `WithStop` -> `WithTerminalFinalizer`，lifecycle 字段单轨改名 |
| `internal/kernel/app/runtime.go` | slot ownership、generation、candidate/retired/current finalization、错误后保留 reference |
| `internal/kernel/app/lease.go` | slot 入 Lease、幂等 begin/continue drain、重复 Stop 可收敛 |
| `internal/kernel/app/definition.go`、`plan*.go`、相关测试 | option 校验与 RuntimeComponent 新方法/快照 |
| `internal/kernel/kernel.go`、`errors.go`、`kernel_test.go` | draining 非终态、反向收敛、cached terminal error、committed cleanup debt |
| `internal/kernel/coordinator.go`、测试 | cleanup-pending、重复 Stop、最小 finalization snapshots |
| `internal/kernel/app/{database,cache,logger,storage}` | 逐资源 option 迁移；StorageManager 删除假 finalizer |
| `pkg/database/database.go`、测试 | 构造/Ready 分工、Close 首次结果缓存 |
| `pkg/logger/*`、测试 | 验证现有首次 Sync+Close result 缓存与文件释放；必要时只做窄修复 |
| `pkg/cache/*`、测试 | typed Client owner/Close 后行为文档与回归测试，不接入 Kernel generation |
| `pkg/httpx/server.go`、测试、README | graceful/force 拆分、并发与终态测试 |
| `pkg/supervisor/supervisor.go`、测试、README | `ForceStopper`、总/force budget、分类 pending snapshot、New error 返回 |
| `internal/kernel/host.go`、测试 | Supervisor 构造签名、ForceShutdownTimeout 透传 |
| `internal/kernel/config/watch.go`、测试 | Close error 合并、包私有 watcher fake |
| `pkg/storage/fileservice*.go`、`watch.go`、测试、README | consumer-owned watcher terminal close、goroutine 等待、Workbook close error |
| `cmd/app`、`internal/composition` 及测试 | 迁移 Supervisor.New 返回 error；保持 baseline logger owner 和 Participant 顺序 |
| 根 README、`internal/kernel{,/app}/README.md` 与受影响 `pkg/*/README.md` | 只描述实施后的当前单轨语义 |
| `docs/changes/022-http-api-template-readiness/*` | 状态、证据、验收结果；实现完成后把当前事实同步到权威主题文档 |

### 7.3 删除

| 文件 | 原因与门禁 |
| --- | --- |
| `pkg/resource/resource.go` | 无生产调用方，且失败后先 closed/丢 per-handle responsibility，不能成为第二套 lifecycle engine |
| `pkg/resource/resource_test.go` | 随唯一实现删除 |
| `pkg/resource/README.md` | 不保留失效公开承诺 |

删除前后分别运行全仓 import/symbol 搜索；若实施时出现新的真实调用方，必须停止并回到计划，不静默迁移到 Kernel。

## 8. 稳定任务清单

| ID | 依赖 | 实施内容 | 完成条件 |
| --- | --- | --- | --- |
| `LFC-001` | R005 | 建立 slot、instance generation、phase/state/snapshot 与 terminal result cache | 纯状态测试覆盖合法/非法转换；错误后 owner/reference 留存 |
| `LFC-002` | `LFC-001` | `WithStop` 单轨替换为 `WithTerminalFinalizer`，迁移现有 Definition | 旧符号零残留；nil/duplicate option 测试通过；NoFinalization 无空函数 |
| `LFC-003` | `LFC-001/002` | 重写 Lease、RuntimeComponent 和 managedComponent 的 candidate/retired/current ownership | drain timeout 不假 stopped；candidate/retired/current failure 分别可定位；attempt 至多一次 |
| `LFC-004` | `LFC-003` | Kernel/Coordinator 支持 cleanup-pending 和重复 Stop 收敛 | active Use 释放后第二次 Stop 完成；terminal failure 返回 cached error；reload debt 阻断不覆盖旧 slot |
| `LFC-005` | `LFC-002/003` | 迁移 Database、Redis、logger、StorageManager；Database Ping 归 Ready | 构造/ready/close 责任唯一；Storage 无假 Close；真实 error chain 保留 |
| `LFC-006` | R005 | Supervisor 加总/force budget、`ForceStopper` 和分类 pending snapshot | 总时限不被逐层重置；普通 Participant 永不被 force；forced 与 stopped 可区分 |
| `LFC-007` | `LFC-006` | HTTP 拆分 Stop/ForceStop，并发安全与 Serve 完成确认 | graceful 不暗中 force；force 可中断 active request；listener 可重绑定；hijacked 明确不支持 |
| `LFC-008` | R005 | config watcher、consumer file Storage、Workbook 终结错误闭环 | defer Close error 可识别；watch goroutine 可等待；重复 Close 返回首次结果；读取与 Close 双错误均保留 |
| `LFC-009` | `LFC-002/005/008` | 删除 unused `pkg/resource`，同步所有权文档和架构门禁 | 包/README/import/示例零残留；没有第二套 Registry |
| `LFC-010` | `LFC-001..009` | 全量故障注入、race/vet/build、Diff/文档验收并回写 022 | 第 9 节全部适用 gate 通过；未执行外部场景如实记录；无范围外实现 |

实施顺序固定为 `001 -> 002 -> 003 -> 004`，资源线 `005`、进程线 `006 -> 007`、consumer line `008` 可在核心 slot 契约冻结后分别推进；最后统一执行 `009 -> 010`。不得先删除旧 API 再留下仓库不可编译的中间提交。

## 9. 精确测试与验收矩阵

### 9.1 Kernel/app 单元测试

必须新增或修改以下场景：

1. 每次成功 Build 分配递增 instance generation，失败 Build 不分配可见 slot。
2. 无 finalizer 的 candidate/current 以 0 attempt 成功退出。
3. terminal finalizer 成功只执行一次；重复调用不增加计数。
4. candidate、retired、current 三个 phase 分别注入 error：`errors.Is` 原因成立、snapshot 类型正确、reference 未清空。
5. terminal-failed 后重复调用返回同一原因且 raw counter 仍为 1。
6. active Lease 时 begin drain 拒绝新 Use；caller timeout 后旧 Use 仍能正常结束，不被 force。
7. 最后 release 关闭原 drained channel；第二次 begin/Stop 继续同一 drain，不恢复 admission。
8. retired failure 后下一次 reload/commit 被拒绝且不能覆盖 generation。
9. 多 component reverse finalization 即使一项失败也继续安全尝试其余项，并 `errors.Join` 全部原因。
10. snapshot 不出现 DSN、配置值或 `error.Error()` 原文。

### 9.2 Kernel/Coordinator/Host 集成测试

1. initial candidate Ready 失败 + finalizer 成功/失败。
2. reload candidate Start/Ready 失败 + finalizer 成功/失败。
3. reload commit 后 retired finalizer 失败：新代继续 active，Coordinator degraded，reload blocked，retired generation 可见。
4. terminal current drain timeout：Coordinator cleanup-pending、ready false、新 Use 拒绝；release 后第二次 Stop 完成。
5. terminal current finalizer 失败：Coordinator failed、cached error、不是 stopped。
6. Service 和 RunOperation 都保持 Participant 正序启动、反序停止，并保留 operation/run error 与 cleanup error。

### 9.3 Supervisor/HTTP 测试

1. `ForceTimeout >= ShutdownTimeout` 构造失败；默认总 budget 仍为 10 秒且 force reserve 为 1 秒。
2. cooperative Participant 在 graceful 阶段完成，不调用 ForceStop。
3. Stop 返回 error 的普通 Participant 保持 pending，不调用不存在的 force。
4. Stop timeout 的 `ForceStopper` 在 force deadline 内只调用一次；snapshot 记录 forced。
5. Stop 与 ForceStop 都不合作时 Supervisor 在总 deadline 内返回，pending participant/task 名称分别存在。
6. HTTP graceful：active request 在 graceful deadline 内完成，Serve done，listener 可重绑，ForceStop counter 为 0。
7. HTTP force：active request 超过 graceful deadline，Supervisor 显式 ForceStop，请求被中断，终态 forced，listener 可重绑。
8. HTTP Stop/ForceStop 并发和重复调用通过 `-race`，channel 无 double close。
9. 未运行、已启动未 Run、Serve 意外错误三类状态有确定结果。

### 9.4 逐资源测试

| 资源 | 必须证明 |
| --- | --- |
| Database | `NewGORM` 不 Ping；Ready 才 Ping；SQLite 文件可关闭后重新取得所有权；fake close error 首次结果被缓存 |
| Redis | Lease drain 后才 raw Close；重复 lifecycle stop 不触发第二次 raw Close；测试 TCP server 观察连接关闭 |
| Logger | stable target 先切回 baseline；Sync 与 sink Close error 均在链中；Windows 临时日志文件可重命名/删除 |
| StorageManager | 当前 local/S3 Adapter 没有独占 goroutine/transport，Kernel definition 不声明 finalizer |
| config watcher | Add error + Close error 可同时 `errors.Is`；cancel 后 channel/任务结束；Windows watch 目录可删除 |
| typed cache Client | consumer owner Close 后 cleanup goroutine结束、后续操作拒绝；不与 Redis Kernel generation 重复关闭 |
| 文件 Storage | 多 watch 全 cancel/Remove/Close、错误聚合、首次结果缓存、内部 goroutine done；closed 后 Watch 拒绝 |
| Workbook | GetRows error 与 Close error 可同时识别；直接打开的 Workbook 文档明确由 caller Close |

真实 PostgreSQL/MySQL/外部 Redis/S3 不作为本任务完成的硬依赖；使用 fake driver、测试 TCP listener、SQLite 和本地文件系统建立确定证据。若 Docker/外部服务未执行，交付报告必须显式写“未验证”，不得声称通过。

### 9.5 验证命令

实现完成后至少执行：

```text
gofmt -w <本任务修改的 Go 文件>
go test ./internal/kernel/app/...
go test ./internal/kernel/...
go test ./pkg/supervisor ./pkg/httpx ./pkg/database ./pkg/logger ./pkg/storage ./pkg/cache/...
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/app
git diff --check
```

另执行生产 Go 源码的旧符号/重复原语搜索；历史变更与本计划可以保留旧名称作为迁移证据，不计入残留。完成条件是：

```text
rg -n "WithStop" --glob "*.go"                                      -> 0
rg -n "pkg/resource|resource.NewRegistry|AddCloser" --glob "*.go"    -> 0
rg -n "Close\(force bool\)" --glob "*.go"                            -> 0
检查 pkg/httpx.Server.Stop 实现                                      -> 不调用 Server.Close
```

Markdown 链接和 022 导航必须通过仓库现有文档检查方式；不存在对应脚本时，使用只读链接/路径检查并如实记录方法。

## 10. 验收映射

| 022 门禁 | 本计划证据 |
| --- | --- |
| `FND-ACCEPT-001` | `LFC-001/003/004/006/007` + 9.1、9.2、9.3 |
| `FND-ACCEPT-002` | `LFC-001/003/004/005/008` + candidate/retired/current/partial build 测试 |
| `FND-ACCEPT-003` 生命周期部分 | `LFC-006/007` + pending Participant/Task、总 deadline、forced 结果；完整统一诊断仍由后续任务完成 |
| `FND-ACCEPT-005` 生命周期部分 | `LFC-010` + 9.4、9.5；Foundation 全门仍需 diagnostics/config 后续任务 |

本任务完成不会单独把 022 改为 `Foundation-closed`，也不解锁新业务模块。它只使生命周期阻断项具备已实现证据；`FOUNDATION-DIAGNOSTICS-001`、`FOUNDATION-CONFIG-001` 和 `FOUNDATION-ACCEPTANCE-001` 仍需分别完成。

## 11. 明确非目标与停止线

- 不实现 retryable raw Close、自动 backoff 或 cleanup queue；当前没有已证明资源。
- 不实现第二次 signal、deployment grace period、Kubernetes hook 或外部 process kill 政策。
- 不实现 WebSocket/hijacked connection registry。
- 不新增 management listener、鉴权、审计或 `FinalizePending` CLI/API。
- 不改 EnvSource 冲突语义；它属于 `FOUNDATION-CONFIG-001`。
- 不统一完整 Kernel/Supervisor management diagnostics；它属于 `FOUNDATION-DIAGNOSTICS-001`。
- 不设计或实现新的 Handler/Service/Repo/Model。
- 不借机升级第三方依赖、改 module identity、生成代码或重构无关包。

若实施证明 `Builder[C,D,I] (I,error)` 无法在 logger 等真实部分构造失败中保留必须继续管理的 handle，必须停止并回到本计划，单独设计 ownership-transfer result；不得临时加入 `any`、隐藏全局 registry 或后台 goroutine 绕过。

## 12. 确认语句的解释

本任务实际授权语句为“确认实施 022 的 FOUNDATION-LIFECYCLE-001 计划”，符合上述门禁；该确认只覆盖 `LFC-001` 至 `LFC-010`，不授权其他 Program ID。
