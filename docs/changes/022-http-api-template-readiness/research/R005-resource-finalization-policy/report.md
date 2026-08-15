# R005：当前资源终结、重试与强制关闭语义

## 1. 研究结论

本轮逐项研究支持“统一治理 + 资源策略重写”，但需要把含义收窄：

- 统一的是 admission、drain、owner、generation、attempt、deadline、结果和诊断状态，不是把所有资源都改造成同一种 `Close`。
- Go 没有传统方法重载；目标应使用窄策略接口或 typed option 做显式多态，并在声明时拒绝重复、冲突或不完整策略。
- `Close` 幂等只表示重复调用安全或结果稳定，不表示失败后会重新执行底层释放。当前多数资源恰好是 terminal attempt：第一次调用已把对象置为 closed，第二次不会补做失败步骤。
- retry、force 和 release verification 都必须由具体 Adapter 给出证据后 opt-in。没有证据时，安全默认是保留 owner/reference 与失败诊断，但不盲目重复 `Close`，也不强关活跃操作。
- 当前 CLI 是另一个新进程，只能关闭自己构造的资源。运行中资源的 retry/force 若未来需要人工触发，必须先有 owner 进程内受控 operation；CLI 只能作为该 operation 的客户端，不能直接持有另一个进程的内存资源。

因此，`FOUNDATION-LIFECYCLE-001` 可以形成独立施工计划，但还不能直接实施。当前冻结后的施工权威见 [`plans/foundation-lifecycle-001.md`](../../plans/foundation-lifecycle-001.md)；本文继续只保存研究事实和推断。

## 2. 方法与范围

本研究从 `cmd/app` 和 `internal/composition` 追到 Kernel、Supervisor、具体构造器、调用方和测试，并核对仓库锁定版本的依赖源码。范围分三类：

1. 进程与 Kernel 实际拥有的资源：baseline/configured logger、Database、Redis、StorageManager、HTTP Server、config watcher；
2. 消费者可能构造的派生资源：typed cache Client、workbook、文件服务 watcher；
3. 无需终结的值：Clock、ID、Validator、I18n、Todo migrator，以及当前 local/S3 storage client。

这里只判断当前快照的语义和目标约束，不把概念契约写成已存在的公开 API，也不决定部署平台的 SIGTERM 总预算。

## 3. 术语纠正

| 术语 | 本任务采用的含义 | 不能推出的结论 |
| --- | --- | --- |
| idempotent | 重复调用不会造成新的业务副作用，或返回稳定终态 | 失败步骤一定会再次执行 |
| retryable | 同一 owner 保留足够状态，再次调用会真实重做尚未完成的安全步骤 | 只因为方法名是 `Close` 就可重试 |
| graceful drain | 先拒绝新借用，再等待已借用工作结束 | 底层连接或文件已经释放 |
| force | 在宽限失败后主动中断仍在进行的工作或连接 | 对事务、响应、文件写入必然安全 |
| finalized | owner 已完成声明的终结动作并通过所需结果检查 | 仅仅“调用过 Close” |
| failed terminal attempt | 终结动作返回错误，且底层对象已进入不可再次尝试的 closed 状态 | 可以清空 owner/reference 或伪装成功 |

最重要的区别是：**一次 Close 的执行次数**、**资源是否仍可用**、**物理句柄是否全部释放**和**错误是否可重试**是四个不同事实。

## 4. 当前统一生命周期事实

### 4.1 已有优点

- Kernel `Lease` 在 reload/stop 前关闭 admission，并等待 `activeUses == 0`；Database、Cache、Storage 的调用方不会直接取得关闭权。
- reload 在提交前失败会回滚；提交后旧代 cleanup error 使用 `CommittedCleanupError` 表达，不伪装成候选未提交。
- configured logger 在关闭旧资源前把稳定 target 恢复到 baseline，避免新日志继续写旧 sink。
- HTTP Server 已区分 `Shutdown(ctx)` 和 `Close()`，具备资源专用 graceful-to-force 雏形。

### 4.2 统一层当前缺口

- terminal drain 超时后 Kernel 直接置为 stopped；最后一个 active use 释放时没有 owner 继续 finalization。
- candidate/previous 的 stop 返回错误后仍清空实例引用；错误链留下了，清理责任没有留下。
- `WithStop` 只有一个函数，不能表达 terminal attempt、retryable、force eligibility、verification 或无资源四种不同契约。
- 构造器只有 `(instance, error)`。当构造器在分配资源后失败并且补偿也失败时，Kernel 无法接收残余 owner；Database 的构造期 Ping 和 logger 多 sink 打开属于这类风险。
- Supervisor 给每个 Participant 启动一个 Stop goroutine，但 deadline 后只返回错误；它不能证明 goroutine 或底层资源已结束，也没有记录 Participant pending owner。

## 5. 逐资源证据与策略分类

### 5.1 Kernel 与进程实际资源

| 资源与 owner | 构造/失败补偿 | graceful | 当前 Close/失败后重试 | force | 目标分类与验证 |
| --- | --- | --- | --- | --- | --- |
| Database `*sql.DB`，Kernel generation | `NewGORM` 创建 pool 后立即 Ping；Ping 失败调用 `resource.Close()` 并返回 nil instance。App `Ready` 又 Ping 一次 | Kernel Lease 先排空业务借用；[`DB.Close`](https://pkg.go.dev/database/sql@go1.25.7#DB.Close) 还会阻止新 query 并等待已在服务端开始的 query | Go 1.25.7 源码先设 `db.closed=true`，再关闭 idle connection/connector；第二次直接 nil。项目 wrapper 也先将 client 标 closed。第一次返回错误后重复调用不会重做失败步骤 | 无通用安全 force；活跃 query 应由 request context 取消，事务不能默认破坏 | `DrainThenTerminalClose`。把网络 readiness 留给 candidate `Ready`，避免构造期失败隐藏 pool owner；验证 client unavailable、`DB.Stats`/连接释放及 SQLite 文件所有权 |
| Redis `*redis.Client`，Kernel generation | `redis.NewClient` 是 lazy；store 构造失败会 Close，网络 Ping 在 `Ready` | Kernel Lease 排空经 Access 发出的命令 | [v9.22.0 `Client.Close`](https://github.com/redis/go-redis/blob/v9.22.0/redis.go) 关闭后台工作和 pool；pool 先 CAS closed，再关闭全部 idle/used connection，第二次返回 `ErrClosed`。失败后没有可靠 raw retry | 关闭 used connection 会中断命令，只能在 Lease drain 后执行；没有独立更安全 force | `DrainThenTerminalClose`。验证后续操作 closed、后台 flusher 退出、连接池句柄释放；不把 `ErrClosed` 当 retry 成功证据 |
| configured logger，Kernel replacement generation | 多 sink 逐个打开；后续打开失败时 join 已打开 sink 的 Close error 后丢失引用 | deactivate 先恢复 baseline；没有 active-use Lease，但 stable manager 切走旧 target | 项目 `resourceState.closeOnce` 固化第一次 Sync+Close 聚合结果，第二次只返回旧 error；[`zap.Sync`](https://pkg.go.dev/go.uber.org/zap@v1.28.0#Logger.Sync) 是 flush，不等于 sink Close | 无通用 force；文件 sink 使用 `os.File.Close`，重复 Close 自身会报错 | `DrainThenTerminalClose`，并分别记录 flush 与 close result；验证 stable target、文件可重新打开/改名/删除，不能只看 Sync error |
| baseline logger，`cmd/app` | `main` 创建；logging manager 创建失败时 join Close error | configured logger 停止后仍可记录顶层错误 | 同上，但当前只在进程末尾调用一次 | 无 | 与 configured logger使用同一终结结果模型，但 owner 是 process，不进入 Kernel generation |
| StorageManager，Kernel generation | local+object 构造失败会 Close local；当前 local Close 为 nil | Kernel Lease 排空 Put/Get/Delete/Exists | 当前 local 和 S3-compatible Client 的 `Close` 都是 no-op；锁定的 [S3 Client API](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3@v1.107.0#Client) 没有 Close。当前 `StorageManager.Close` 实际不释放底层句柄 | 不适用 | 当前应声明 `NoFinalization`，而不是用 `WithStop` 制造“已释放 client”的假象；验证重点是没有独占 transport/goroutine。未来若注入专用 HTTP Transport，再由 Adapter 显式拥有其 idle connection cleanup |
| HTTP Server，Supervisor Participant/Task | `Start` 预绑定 listener；`Run` 执行 Serve | [`Server.Shutdown`](https://pkg.go.dev/net/http#Server.Shutdown) 停 listener 并等待连接 idle | 当前 Shutdown 出错后立即 `Server.Close`，随后即使等待 `Run` 再次超时也把状态设 stopped 并缓存 error，不能继续完成等待 | [`Server.Close`](https://pkg.go.dev/net/http#Server.Close) 会中断 active connection，但不知道 hijacked connection | `GracefulThenForce` 只能显式 opt-in；force 结果应记 `forced`，不是 graceful success。验证 listener rebind、Serve done、active request 结果和 hijacked owner |
| config fsnotify watcher，Supervisor Task 内部 owner | `NewWatcher` 后 Add 目录；任一 Add 失败由 defer Close，但 Close error 被忽略 | Task context 取消后退出 select | [`Watcher.Close`](https://pkg.go.dev/github.com/fsnotify/fsnotify@v1.10.1#Watcher.Close) 关闭 watch/channel；各平台实现先标 closed，失败后第二次通常 no-op，不能保证重做 | 无 | `DrainThenTerminalClose`；Task 返回值必须 join watcher Close error。验证 goroutine/channel 退出和 Windows/Linux watch handle 释放 |

### 5.2 派生与次级资源

| 资源 | 当前事实 | 对计划的影响 |
| --- | --- | --- |
| typed cache Client | 每个 Client 自己启动 L1 cleanup goroutine；`Close` 用 `sync.Once`、取消并等待 goroutine、清空本地状态。当前生产业务没有消费者，未来由构造它的 composition/module owner 关闭 | 不能把它误算为 Redis Kernel resource。新增消费者时必须贡献一个明确 owner/Participant 或由组合根持有，不能只依赖 Redis Close |
| `pkg/storage.Storage` 文件工具 | 当前只在测试/公开包使用；watcher Close 忽略每个 Remove 和 Watcher.Close error，再设 `closed=true` | Foundation-closed 前需决定它是否属于正式 baseline。若保留，按 `DrainThenTerminalClose` 修复错误聚合和 Windows 文件句柄验收；若不是 baseline，应在单轨任务中移出公开承诺，不能留下第二套含 watcher 的 Storage 生命周期 |
| workbook | 调用方拥有 `excelize.File.Close`，部分读取路径使用 defer 但忽略错误 | 明确为短生命周期 consumer-owned handle；错误处理和验证在使用边界完成，不进入 Kernel finalizer |
| `pkg/resource.Registry` | 当前没有 production 调用方；`Close` 先设 registry closed，失败后不保留 per-handle pending 状态 | 不能拿它直接解决本问题。冻结计划选择删除无真实 owner 的重复原语；若实施前出现真实调用方，必须退回计划而不是静默迁移，避免第二套生命周期引擎 |

### 5.3 无终结动作的值

Clock、ID、Validator、I18n 和 Todo migrator 不拥有外部句柄或长期 goroutine；Todo migrator 只在 `Start` 执行一次 migration，`Stop` 为 nil。它们应显式落入 `NoFinalization`，不应为统一外观强造空 `Close`。

## 6. 失败后资源是否可用

统一判断不能依赖第三方 client 的内部状态，而应由 owner 的 admission 状态决定：

```text
serving
  -> drain-pending       拒绝新借用，等待 active use
  -> finalizing          资源 Adapter 执行一次已声明 attempt
       -> finalized      终结动作与必要验证成功
       -> cleanup-pending 只有 Adapter 证明 retryable 才允许再次 attempt
       -> terminal-failed attempt 已终结但结果失败；保留责任与诊断，不盲重试
       -> force-pending  只有显式 force policy 才可进入
            -> forced | force-failed
```

候选构造失败、旧代 cleanup 和当前代 stop 使用同一状态词与结果结构，但仍由各自 owner 治理。业务能力从 `drain-pending` 开始就不可再借用；“不可借用”不代表物理资源已释放。

## 7. 推荐的统一策略与组件重写边界

### 7.1 统一引擎负责

- owner、generation、phase、attempt number、开始/结束时间和最后 error type；
- admission close、Lease drain 和状态转换；
- 保留 candidate/previous/current 引用直到 `finalized`、`forced` 或经政策确认的 `terminal-failed`；
- 一个 process-level 总 shutdown deadline，并向下传递剩余 budget；子层不得偷偷创建更长无限 context；
- 当前 terminal attempt 禁止自动 retry；未来只有第一个真实 Adapter 证明 retryable 后，才能另立变更新增有界 backoff/attempt；
- 输出可供 Coordinator/未来 management plane 使用的安全 snapshot。

### 7.2 资源 Adapter 负责

- 声明 `NoFinalization`、`DrainThenTerminalClose`、`GracefulShutdown`、`GracefulThenForce` 或 `RetryableFinalize` 中的一种；
- 实现真正的 Finalize，并说明 error 后对象是 retryable 还是 terminal；
- 只有存在协议证据时实现 Force；
- 提供必要的 release verification，且验证失败不得被改写成成功；
- 构造部分成功时在 ownership transfer 前完成补偿，或把仍需清理的 handle 显式转交 owner。

### 7.3 Go 中的多态方式

目标是策略多态，不是方法重载。冻结计划选择在现有 `Option[I]` 上用 `WithTerminalFinalizer` 表达 Kernel 一次 terminal attempt，并以 Supervisor 可选 `ForceStopper` 表达 HTTP force；当前不创建 retryable strategy API。具体契约以 [`FOUNDATION-LIFECYCLE-001`](../../plans/foundation-lifecycle-001.md) 为准，并满足：

- 无 `Stop` 的值默认 `NoFinalization`；
- 现有 `WithStop` 单轨删除；拥有真实 terminal action 的 Definition 迁移为 `WithTerminalFinalizer`，不能默认“失败自动重试”；
- retry、force、verify 必须显式 opt-in，且声明时校验依赖关系，例如有 Force 才能选择 `GracefulThenForce`；
- Kernel component 与 Supervisor Participant 可共享状态词和诊断结果，但不合并为一个万能接口。

本文作为研究档案不单独宣称接口权威；具体名称、迁移与测试已经在 `FOUNDATION-LIFECYCLE-001` 施工计划中冻结。

## 8. retry operation 与 CLI 边界

推荐次序是：

1. 最后一个 active Lease 释放后关闭同一 drained channel，仍存活的 owner 保留 generation；不启动脱离总预算的后台 finalizer；
2. caller timeout 后，后续 owner `Stop(ctx)` 继续同一 drain/finalization；当前 terminal attempt 不自动重试 raw Close；
3. 嵌入式 Host 或未来 management plane 可暴露 owner 进程内的受控 cleanup operation，但仍只能调用 owner，而不能直接取得 instance；
4. CLI 若需要人工触发，只能调用该 management operation，并携带 owner/generation/幂等 request identity；不能启动一套新 Kernel 后声称清理了旧进程资源；
5. terminal attempt、force 或 process forced exit 必须要求更高权限、审计和明确结果，不与普通业务 CLI 混用。

当前项目没有 management listener、跨进程控制协议或认证边界，因此本轮只记录目标，不新增 CLI command。

## 9. 超时、force 与进程退出

- Supervisor/composition 拥有总 shutdown budget；Kernel/HTTP/watcher 使用剩余 budget，不能每层重新获得一个完整 timeout。
- graceful deadline 到期后，状态保持 pending 或进入显式 force decision；不得先报 stopped 再后台碰运气。
- HTTP 可以在政策允许时 force，但会中断 active request，且 hijacked connection 需要独立 owner。
- Database、Redis、logger、fsnotify 没有第二种更安全 force；它们的 Close 本身就是 terminal attempt。Redis/DB 只有在 Lease 排空后调用才是默认安全路径。
- 总 budget 用尽仍 unresolved 时，顶层返回非零并报告 owner/generation/phase/policy；外部 process supervisor 可以结束进程，但这不是“资源已优雅释放”的证明。
- 第二次信号是否立即退出、总 budget 数值和 deployment grace period 仍由后续部署研究决定。

## 10. 释放验证矩阵

| 分类 | 最小验证 |
| --- | --- |
| Database | 新借用拒绝；Close attempt 数；pool stats/driver handle；SQLite 文件可重新取得所有权；带 error driver 验证 terminal failure |
| Redis | 新命令返回 closed；连接池后台 flusher 退出；测试 TCP listener 观察连接关闭；Close error 后不误重试 |
| Logger | stable target 已切走；Sync 与 sink Close error 分列；Windows 文件可重命名/删除或重新打开；每个 sink attempt 一次 |
| StorageManager | 证明当前 adapter 没有独占 transport/goroutine；未来专用 transport 必须有独立验证 |
| HTTP | listener 可重绑定；Serve goroutine done；graceful request 完成；force request 被中断；hijacked owner 单独列出 |
| fsnotify | Events/Errors channel 与 reader goroutine结束；watch handle 释放；Close error 被返回而非 defer 丢失 |
| typed cache Client | cleanup goroutine done；Close 后操作拒绝；构造 owner 明确 |

只检查“第二次 Close 没报错”不属于释放验证。

## 11. 对当前计划的修正

1. 把 `FND-LIFECYCLE-004` 的“每个资源 Close 恰好一次”改为“每个 attempt 至多一次；成功终结一次；只有被证明 retryable 的策略允许多个有编号 attempt”。
2. 新增构造部分成功的 ownership transfer/补偿要求，优先消除 Database 构造期重复 Ping 形成的隐藏清理窗口。
3. `FOUNDATION-LIFECYCLE-001` 先实现共享状态与 terminal-attempt 默认，再逐资源适配；不能先做通用自动 retry。
4. HTTP force、运行中 retry operation 和 CLI transport 分开立项；当前 CLI 不能作为进程内 owner。
5. 当前 StorageManager 归类为 `NoFinalization`；公共文件 Storage 和 unused Registry 必须在 Foundation baseline 中明确保留或单轨移除。

## 12. 局限与刷新触发器

- 没有故障注入驱动真实 PostgreSQL/MySQL/Redis/S3；“关闭错误后的物理句柄结果”仍需后续 fake driver、测试 listener 或隔离外部资源验证。
- HTTP hijacked/WebSocket、数据库驱动 connector、专用 AWS HTTP Transport 当前没有真实业务场景，不提前设计。
- 本研究没有决定部署平台 grace period、第二次信号或 management 认证协议。
- Go、go-redis、fsnotify、zap、AWS SDK 升级，或任一 Adapter 改变资源所有权时必须刷新本记录。

这些未知不妨碍形成生命周期实现计划，但会阻止把某个资源从 `DrainThenTerminalClose` 擅自升级为 retryable/force-safe。
