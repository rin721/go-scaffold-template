# FOUNDATION-DIAGNOSTICS-001：统一运行责任诊断实施计划

## 1. 状态与授权

- Program ID：`FOUNDATION-DIAGNOSTICS-001`
- 当前状态：**已确认并实施完成**
- 研究门禁：已通过，依据 [R006](../research/R006-unified-runtime-diagnostics/report.md)
- 实施基线：HEAD `7d64f8634c59375a522e66b5b989dd40b557ee9d`
- 前置：[`FOUNDATION-LIFECYCLE-001`](foundation-lifecycle-001.md) 已实施
- 覆盖门禁：`FND-RUNTIME-002`、`FND-DIAGNOSTICS-001`、`FND-DIAGNOSTICS-002`、`FND-ACCEPT-003`

本文件是该 Program 的唯一施工级计划与实施证据入口。用户已在计划报告后的后续消息明确确认 `FOUNDATION-DIAGNOSTICS-001`；本轮据此只执行 `DGN-001` 至 `DGN-009`，未启动服务、部署或推送。

## 2. 目标与完成定义

目标是在不合并 Kernel 与 Supervisor 生命周期语义的前提下，形成一份由 Host 拥有的 process-level 安全诊断快照，使调用方无需解析 error string 或拼接两份 authority，即可回答：

- process 与 Kernel 当前 state/ready；
- config generation/digest/provenance 与 component instance generation；
- capability、participant、task 三类稳定 owner；
- owner 当前 phase/state、scenario/exit policy、attempt、error type 和 since；
- restart/cleanup required；
- pending responsibility 与 failed/forced/finalized 终态；
- process shutdown 总/graceful/force budget；
- finalizer completion 与 release verification 的区别。

完成必须同时满足：

1. `Host.Diagnostics()` 返回一份项目自有 `ProcessDiagnostics`，不再返回 Kernel/Supervisor tuple；
2. Kernel app 与 Supervisor 各自维护 typed、并发安全、脱敏的责任 ledger，Host 只做映射和聚合；
3. clean stopped 由全部已启动责任的 clean terminal state 证明，已返回错误与仍未返回不再同为 pending；
4. diagnostics 与 reload/terminal drain/shutdown 并发读取通过 race 测试；
5. 原始配置、实例、error text、DSN、密码、Token 和凭据不进入快照；
6. 旧 names-only/tuple authority 单轨删除，不保留 alias 或兼容分支；
7. `FND-ACCEPT-003` 的 Participant/Task 不合作、force 和 Service/CLI 不变量有确定性测试；
8. 全量 test/race/vet/build 和文档/架构门禁通过。

本任务完成后仍保持 `Foundation-partial`。`FOUNDATION-CONFIG-001` 和 `FOUNDATION-ACCEPTANCE-001` 未完成前，不解锁新业务模块详细设计。

## 3. 冻结边界与非目标

### 3.1 保持不变

- `cmd/app -> internal/composition` 仍是唯一 application composition root。
- Kernel 继续拥有配置 candidate、component generation、Lease 与 finalization；Supervisor 继续拥有 Participant、Task 和 shutdown budget。
- 配置 Loader/EnvSource、reload 原子性、现有 finalizer、HTTP graceful/force、SignalContext 和退出码语义不改变。
- error 继续完整向上返回；日志只在 Coordinator/Supervisor 的策略消费者、composition reload reporter 或顶层 process 边界决定。
- `pkg/health` 继续执行进程内 check，不新增端口或协议。

### 3.2 明确不做

- 不新增 management listener、`/diagnostics`、metrics、build info、pprof 或管理 CLI。
- 不实现运行中 retry、force、FinalizePending 或 reconciliation operation。
- 不实现第二次信号、deployment grace、Kubernetes hook 或外部进程 kill。
- 不为 hijacked/WebSocket 增加连接 registry。
- 不新增 retryable finalizer、release verifier 通用 callback 或万能 lifecycle package。
- 不改 EnvSource 冲突；它属于 `FOUNDATION-CONFIG-001`。
- 不升级依赖、不改 module identity、不重构业务模块。

## 4. 目标数据模型

### 4.1 Kernel app：单轨 ownership snapshot

`internal/kernel/app` 以 `OwnershipSnapshot` 取代只描述 unresolved slot 的 `FinalizationSnapshot`：

```go
type FinalizationPolicy string

const (
	NoFinalization          FinalizationPolicy = "no-finalization"
	DrainThenTerminalClose FinalizationPolicy = "drain-then-terminal-close"
)

type ReleaseVerification string

const (
	VerificationNotRequired ReleaseVerification = "not-required"
	VerificationNotProven   ReleaseVerification = "not-proven"
)

type OwnershipSnapshot struct {
	ComponentID        ID
	InstanceGeneration uint64
	Phase              FinalizationPhase
	State              OwnershipState
	Policy             FinalizationPolicy
	Attempt            uint32
	Verification       ReleaseVerification
	LastErrorType      string
	Since               time.Time
}
```

约束：

- `WithTerminalFinalizer` 在 Definition 冻结时确定 `DrainThenTerminalClose`；没有 finalizer 的 Managed component 自动确定 `NoFinalization`。调用方不能通过任意字符串或 bool 自报 policy。
- `OwnershipState` 单轨表达 serving、waiting-for-drain、finalization-pending、finalizing、finalized、terminal-failed；不复用一个含义模糊的 pending 表达全部状态。
- `RuntimeComponent.Finalizations()` 改为 `Ownerships()`；`Kernel.Finalizations()` 改为内部 `Kernel.ownerships()`，所有 Go 调用方和测试一次迁移，旧符号零残留。
- 每个 managed component 返回当前 active/unresolved responsibility，并最多保留最近一个已经释放实例的安全 terminal tombstone；不得保留实例、配置或无界历史。
- terminal finalizer 返回 nil 只改变 completion state。当前没有运行时资源专用 verifier，因此有 finalizer 的责任一律为 `not-proven`，no-finalization 为 `not-required`。本任务不预建没有 producer 的 passed/failed 常量，也不把 Close nil 或重复 Close 当作 release passed。
- slot、lease 和 managed component 的诊断元数据必须有确定锁顺序。长时间 build/finalizer/drain 不得在持有 diagnostics 锁时执行；快照读取也不得等待整个 operation 完成。

### 4.2 Supervisor：typed unit ledger 与 budget snapshot

`pkg/supervisor` 以统一 unit ledger 取代三个 names-only slice：

```go
type UnitKind string       // participant | task
type UnitPhase string      // start | ready | run | stop | force
type UnitState string      // pending | running | ready | stopped | forced | failed
type ExitPolicy string     // graceful-shutdown | graceful-then-force | cancel-and-wait

type UnitSnapshot struct {
	Owner         string
	Kind          UnitKind
	Phase         UnitPhase
	State         UnitState
	ExitPolicy    ExitPolicy
	Attempt       uint32
	LastErrorType string
	Since         time.Time
}

type ShutdownBudgetSnapshot struct {
	Phase            ShutdownPhase
	StartedAt        time.Time
	GracefulDeadline time.Time
	FinalDeadline    time.Time
	Exhausted        bool
}

type Snapshot struct {
	State         ProcessState
	Ready         bool
	Since         time.Time
	LastErrorType string
	Budget        ShutdownBudgetSnapshot
	Units         []UnitSnapshot
}
```

`PendingParticipants`、`PendingTasks`、`ForcedParticipants` 和含义不准的 `LastError` 删除，不保留 deprecated alias。Owner ledger 的稳定顺序固定为 Participant 注册顺序后接 Task 注册顺序；快照不因 map 枚举而抖动。

Policy 由真实契约确定：普通 Participant 为 graceful-shutdown，实现 `ForceStopper` 的 Participant 为 graceful-then-force，Task 为 cancel-and-wait。`Attempt` 表示当前 phase 的实际调用次数；本任务每个 phase 最多一次。

### 4.3 Supervisor 终态分类

shutdown 算法保持同一总 deadline，但结果分类单轨调整：

1. Participant `Stop` 返回 nil：`stopped`；
2. `Stop` 返回 error：普通 Participant 为 `failed`；支持 force 的 Participant 进入一次 `force`；
3. `Stop` 到 graceful deadline 仍未返回：`pending`；支持 force 的 Participant 进入一次 `force`；
4. `ForceStop` 返回 nil：`forced`，不再 pending，但 process 保持 failed outcome；
5. `ForceStop` 返回 error：`failed`；到 final deadline 仍未返回：`pending`；
6. Task 结果已收集：按 nil/cancellation 或真实 error 进入 stopped/failed；final deadline 仍未返回才是 pending；
7. 只有全部 responsibility clean stopped 且聚合 error 为 nil，process 才是 `StateStopped`。failed/forced/pending 都让 process 为 `StateFailed`，但三者在 unit ledger 中保持可区分。

已经从 result channel 收到的值只能消费一次；实现不得再把已消费 channel 放入 pending map 后尝试二次读取。

### 4.4 Host：唯一 process management snapshot

在 `internal/kernel/diagnostics.go` 定义项目自有聚合结构：

```go
type OwnerKind string // capability | participant | task

type ResponsibilitySnapshot struct {
	Owner               string
	Kind                OwnerKind
	Generation          uint64
	Phase               ResponsibilityPhase
	State               ResponsibilityState
	ExitPolicy          ExitPolicy
	Attempt             uint32
	ReleaseVerification ReleaseVerification
	LastErrorType       string
	Since               time.Time
}

type ResponsibilityRef struct {
	Owner      string
	Kind       OwnerKind
	Generation uint64
}

type ProcessDiagnostics struct {
	ProcessState        supervisor.ProcessState
	KernelState         LifecycleState
	Ready               bool
	ConfigGeneration    uint64
	ConfigDigest        string
	ConfigProvenance    []string
	RestartRequired     bool
	CleanupRequired     bool
	KernelErrorType     string
	ProcessErrorType    string
	ShutdownBudget      supervisor.ShutdownBudgetSnapshot
	Responsibilities    []ResponsibilitySnapshot
	PendingUnits        []ResponsibilityRef
	Since               time.Time
}
```

最终命名可按 Go Doc 清晰度微调，但语义字段不得缩减或改成 `map[string]any`/任意字符串。`ResponsibilityPhase/State/ExitPolicy/ReleaseVerification` 必须是专用类型和有限常量。

聚合规则：

- `Ready = coordinator.Ready && supervisor.Ready`；
- config generation 明确命名，不与 component instance generation 混用；
- capability 来源于 Kernel ownership ledger，participant/task 来源于 Supervisor unit ledger；
- `PendingUnits` 只从 responsibility state 确定性派生，包含 owner kind 与 generation，不复制 error text；
- `Since` 取 Coordinator 与 Supervisor aggregate transition time 的较晚者，owner 自己保留各自 since；
- 返回值深拷贝所有 slice；nil Host 返回 typed failed snapshot，不写入伪错误文本。

`Host.Diagnostics()` 单轨改为返回 `ProcessDiagnostics`。`Health` 只消费这份聚合快照，不再自行组合 tuple。Coordinator 与 Supervisor 的局部快照仍作为 Host 的内部数据源，不成为第二个 management authority。

## 5. 并发、脱敏与所有权不变量

### 5.1 并发

- Diagnostics 可以在 running、reload、terminal drain、finalizer running 和 Supervisor shutdown 中读取。
- app snapshot 不能只依赖 `Kernel.mu`；component/slot 元数据需独立同步，且规定固定锁顺序，禁止 lease/slot/component 反向取锁。
- Supervisor ledger 和 budget 只在 `Supervisor.mu` 下读写；执行 Participant/Task 用户代码时不持锁。
- Host 先分别取得两个独立副本，再在无底层锁状态下映射，避免跨 owner 嵌套锁。
- race 测试必须让 diagnostics reader 与 reload、active Lease drain、blocking Participant Stop、blocking Task 同时运行。

### 5.2 脱敏

允许：稳定 owner ID、有限状态、generation、digest、provenance source 名、时间、attempt、Go error type。

禁止：原始 config map/section/value、实例/指针、完整 error string、DSN、URL query credential、密码、Token、secret/access key、请求 payload、任意第三方 client state。

测试用高辨识秘密字符串构造 config/error，序列化或格式化完整 snapshot 后必须零命中。

### 5.3 责任终态

- `pending` 只表示 owner operation 尚未返回或仍等待 drain；已返回 error 必须是 failed。
- `forced` 是明确有损终态，不是 graceful stopped。
- `finalized` 表示 finalizer completion；`ReleaseVerification` 单独说明物理释放证明。
- `terminal-failed` 保留 owner/generation/reference 与 error chain；快照只保留 error type。
- clean stopped 时不得存在 pending/failed/forced capability、participant 或 task。

## 6. 文件影响清单

### 6.1 新增

| 文件 | 内容 |
| --- | --- |
| `internal/kernel/diagnostics.go` | process-level typed snapshot、owner 映射、pending 派生和深拷贝 |
| `internal/kernel/diagnostics_test.go` | Host 统一视图、脱敏、终态和并发读取测试；若与 host_test 重复则合并到现有文件 |
| `pkg/supervisor/diagnostics.go` | unit/budget typed model 与内部 transition helper；若不足以独立成文件则并回 supervisor.go |

不新增公共 `pkg/diagnostics`、transport DTO 或 management server。

### 6.2 修改

| 文件/目录 | 必须变化 |
| --- | --- |
| `internal/kernel/app/finalization.go` | ownership state/policy/verification/since、terminal tombstone、并发安全元数据 |
| `internal/kernel/app/contracts.go`、`definition.go` | 从真实 finalizer 契约冻结 policy，不增加万能 option |
| `internal/kernel/app/runtime.go`、`lease.go` | active/unresolved/terminal ownership snapshot 与锁顺序；`Ownerships()` 单轨 API |
| `internal/kernel/app/*_test.go` | generation/phase/policy/attempt/verification、terminal tombstone、脱敏与 race |
| `internal/kernel/kernel.go`、`kernel_test.go` | ownership 聚合、cleanup-pending/failed/stopped 责任证据 |
| `internal/kernel/coordinator.go`、测试 | 提供深拷贝的 Kernel source snapshot，明确 config generation 和 error type |
| `pkg/supervisor/supervisor.go`、测试 | unit ledger、budget snapshot、pending/failed/forced 分类、stopped invariant |
| `internal/kernel/host.go`、测试 | tuple 改唯一 `ProcessDiagnostics`；Health 消费统一 authority |
| `internal/composition` 相关测试 | Service Host 与 CLI RunOperation 的 owner 顺序、错误聚合和终态不变量 |
| `pkg/supervisor/README.md`、`internal/kernel/README.md` | 当前 typed diagnostics、budget、终态和 transport 边界 |
| 根 `README.md` | 只同步实施后的当前诊断事实，不宣称 management listener |
| `docs/changes/022-http-api-template-readiness/*` | 实施证据、门禁状态和剩余 Config/Acceptance 阻断项 |

### 6.3 删除/单轨迁移

| 旧资产 | 处理 |
| --- | --- |
| `app.FinalizationSnapshot` / `Finalizations()` | 迁移到 ownership snapshot / `Ownerships()`，Go 旧符号零残留 |
| `supervisor.Snapshot.PendingParticipants` | 由 typed units 派生，不保留字段 |
| `supervisor.Snapshot.PendingTasks` | 由 typed units 派生，不保留字段 |
| `supervisor.Snapshot.ForcedParticipants` | 由 typed units state 表达，不保留字段 |
| `supervisor.Snapshot.LastError` | 单轨改为明确 `LastErrorType` |
| `Host.Diagnostics()` tuple | 单轨改为一个 `ProcessDiagnostics` |

历史变更文档可以保留旧名称作为证据，不计入生产残留；权威使用文档只能描述新语义。

## 7. 稳定任务清单

| ID | 依赖 | 实施内容 | 完成条件 |
| --- | --- | --- | --- |
| `DGN-001` | R006 | 冻结 app/supervisor/Host 的 typed vocabulary、映射和脱敏规则 | 专用类型、有限常量、Go Doc 和非法/零值测试齐全；没有 `map[string]any` 或 error text |
| `DGN-002` | `DGN-001` | 把 Kernel finalization snapshot 单轨升级为并发安全 ownership ledger | running/current、candidate、retired、draining、finalized、terminal-failed 均可定位；最近 terminal tombstone 有界 |
| `DGN-003` | `DGN-002` | 让 Kernel/Coordinator 输出 config generation 与 component generation 明确分离的 source snapshot | cleanup/restart/failure 与 ownership 深拷贝一致；成功与失败路径无数据竞争 |
| `DGN-004` | `DGN-001` | 建立 Supervisor participant/task unit ledger 与 shutdown budget snapshot | owner kind/phase/state/policy/attempt/error type/since 和 deadline 可观察，注册顺序稳定 |
| `DGN-005` | `DGN-004` | 修正 Supervisor pending/failed/forced 分类和 clean stopped invariant | 已返回 error 不再 pending；timeout 才 pending；force 与 graceful 分离；Task 同轨 |
| `DGN-006` | `DGN-003/005` | 建立 Host 唯一 `ProcessDiagnostics` 并迁移 Health | tuple 零残留；ready/pending/cleanup/budget 映射确定；nil/脱敏/deep-copy 测试通过 |
| `DGN-007` | `DGN-005/006` | 补 Service、CLI、Host 与不合作 owner 的集成证据 | Service/RunOperation 顺序和错误聚合一致；Participant/Task pending、failed、forced 可复核 |
| `DGN-008` | `DGN-002..007` | 单轨清理旧字段/符号并同步权威文档与 022 | Go 旧引用零残留；README 不声称 transport/recovery；Foundation 仍 partial |
| `DGN-009` | `DGN-001..008` | 执行全量 validation、race、build、文档链接和 Diff 审计 | 第 8/9 节全部适用门禁通过；未执行外部场景如实记录；只提交本任务文件 |

实施固定先冻结 `DGN-001`。Kernel 线 `DGN-002 -> DGN-003` 与 Supervisor 线 `DGN-004 -> DGN-005` 可分别推进；合流后执行 `DGN-006 -> DGN-007 -> DGN-008 -> DGN-009`。不得先删除旧结构再留下仓库不可编译的中间提交。

## 8. 精确测试矩阵

### 8.1 Kernel app / Coordinator

1. Managed component 无 finalizer时 policy 为 no-finalization，停止后保留 finalized + not-required tombstone，attempt 为 0。
2. 有 finalizer 的 current generation 在 serving、waiting-for-drain、finalizing、finalized 间转换，policy 为 drain-then-terminal-close，attempt 只增一次。
3. candidate/retired/current error 分别保留 generation、phase、terminal-failed、error type 与 instance responsibility；原始 error chain 仍可 `errors.Is`。
4. clean finalizer 不伪装 release verification passed；当前无 runtime verifier 时明确 not-proven。
5. 连续 reload 不产生无界 terminal history；当前 active responsibility 与最近 terminal tombstone 顺序确定。
6. active Lease + terminal Stop timeout 时，新 Use 拒绝、responsibility waiting/pending 可见；release 后后续 Stop 收敛。
7. diagnostics 与 reload/Stop/finalizer 并发循环读取通过 `go test -race`，snapshot 不读取半更新 pointer/state。
8. snapshot 格式化结果不包含注入的 DSN、secret、配置字段值或 error 文本。

### 8.2 Supervisor

1. 正常 Run：Participant/Task owner、kind、policy、start/ready/run phase 和 stable order 正确。
2. 普通 Participant Stop nil -> stopped；返回 error -> failed 且不在 pending。
3. 忽略 context 的 Participant 到 final deadline 仍未返回 -> pending，Supervisor 在总 budget 内返回。
4. ForceStopper graceful error/timeout 后只 force 一次；force nil -> forced，force error -> failed，force timeout -> pending。
5. Task cancellation 正常返回 -> stopped；真实 error -> failed；不返回 -> pending。
6. budget snapshot 的 `StartedAt <= GracefulDeadline < FinalDeadline`，Participant 与 Task 共享同一 final deadline；耗尽状态正确。
7. 任一 failed/forced/pending responsibility 时 process 不得 clean stopped；全 clean 才 StateStopped。
8. snapshot slice 深拷贝、稳定排序，外部修改返回值不污染 owner ledger。

### 8.3 Host / Service / CLI

1. running Host 返回一份 snapshot，同时包含 Kernel capability、application/http participant 和 HTTP/watch task；config 与 instance generation 不混名。
2. Host ready 只在 Coordinator 与 Supervisor 均 ready 时成立；Health 使用同一 snapshot。
3. Kernel cleanup-pending + blocking Participant/Task 可以同时进入统一 responsibilities/pending units，owner kind 和 generation 正确。
4. HTTP graceful 与 explicit force 映射为不同 unit state/policy；不把 forced 报告为 graceful stopped。
5. nil Host 返回 typed failed snapshot，不包含伪 raw error。
6. Service cancellation、runner failure、watcher failure和 CLI operation failure 都保持 participant 正序启动、反序停止、错误聚合和可判定终态。
7. 快照 consumer 无需访问 Kernel instance、Supervisor 内部 map 或解析 error string。

## 9. 验证命令与静态门禁

实施完成后至少运行：

```text
gofmt -w <本任务修改的 Go 文件>
go test ./internal/kernel/app/...
go test ./internal/kernel/...
go test ./pkg/supervisor ./pkg/httpx
go test ./internal/composition/...
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/app
git diff --check
```

生产 Go 源码旧 authority 门禁：

```text
rg -n "PendingParticipants|PendingTasks|ForcedParticipants" --glob "*.go" -> 0
rg -n "FinalizationSnapshot|Finalizations\(" --glob "*.go"                -> 0
rg -n "LastError\s+string" --glob "*.go"                                -> 0
检查 Host.Diagnostics 调用方                                                -> 只消费单一 ProcessDiagnostics
检查 diagnostics struct                                                     -> 无 error/config/instance/map[string]any
```

Markdown 不存在仓库专用 verifier 时，使用排除 fenced/inline code 的本地相对链接检查；方法和结果写回 022。`git diff --check`、完整 Diff、敏感信息和 staged scope 必须在提交前复核。

## 10. 验收映射

| 022 门禁 | 本计划证据 |
| --- | --- |
| `FND-RUNTIME-002` | `DGN-004/005/007` + shared budget、Participant/Task pending 与总 deadline 测试 |
| `FND-DIAGNOSTICS-001` | `DGN-001..006` + state/ready/generation/digest/phase/owner/policy/attempt/pending/restart/cleanup/verification/error type 全字段 |
| `FND-DIAGNOSTICS-002` | `DGN-002/005/006` + finalized/failed/forced/pending 与 clean stopped 不变量 |
| `FND-ACCEPT-003` | `DGN-004..007` + uncooperative Participant/Task、force、Service/CLI ownership 测试 |
| `FND-ACCEPT-005` 诊断部分 | `DGN-009` + race/vet/build、Host 集成、文档/静态 authority gate |

## 11. 风险与控制

| 风险 | 后果 | 控制 |
| --- | --- | --- |
| 为读取快照持有 operation 锁 | diagnostics 在最需要时阻塞，无法观察 drain/finalizer | 独立短临界区元数据；不在锁内调用用户代码 |
| ledger 与真实状态双写漂移 | 快照看似完整但不可信 | transition helper 与同一状态切换更新；表驱动非法转换测试 |
| 把 Close nil 当 release passed | 运维误判物理资源已释放 | 默认 not-proven；只有资源专用证据才可 passed |
| 把 Stop error 和 timeout 都叫 pending | 无法区分已失败和仍运行 | channel 结果单次消费，failed/pending 明确分支 |
| Host 继续暴露多个 authority | management consumer 形成不同判断 | tuple 单轨删除；Health 只消费 ProcessDiagnostics |
| 为未来 transport 提前稳定 schema | 内部模型被不成熟协议锁死 | 结构留在 internal/kernel；MANAGEMENT-001 再设计 DTO/鉴权 |
| unit/history 无界增长 | 长期 reload 增加内存 | active/unresolved + 每组件最近 terminal tombstone；不做事件审计日志 |

## 12. 停止线与重新确认条件

本计划获得确认后只实施 `DGN-001` 至 `DGN-009`。出现以下任一情况必须停止、更新研究/计划并重新确认：

- 需要新增外部 management endpoint、CLI command、网络 listener 或鉴权/审计协议；
- 需要改变 finalizer/HTTP force/SignalContext/exit code 语义；
- 需要运行时 release verifier、retryable cleanup 或 background reconciliation；
- 发现现有 component/participant/task 无法用 owner ledger 表达，必须引入新的公共生命周期抽象；
- 需要改 EnvSource、依赖版本、业务 API 或数据库迁移。

普通内部命名微调、文件拆分和已冻结语义内的测试 seam 不重复确认。实施完成后同步代码、测试、权威文档与 022 证据为一个任务提交；不启动服务、不部署、不推送。

## 13. 建议确认语句

```text
确认实施 022 的 FOUNDATION-DIAGNOSTICS-001 计划
```

该语句只授权本文件的 `DGN-001` 至 `DGN-009`，不授权其他 Foundation/API Program。

## 14. 实施结果与证据

`DGN-001` 至 `DGN-009` 已按冻结范围完成：

| ID | 结果 | 主要证据 |
| --- | --- | --- |
| `DGN-001` | 已完成 | app、Supervisor 与 Host 使用项目自有 typed vocabulary；快照不携带 error text、配置值、实例或任意 map |
| `DGN-002` | 已完成 | Kernel app 记录 active/unresolved ownership 与有界 terminal tombstone；finalization 状态并发安全 |
| `DGN-003` | 已完成 | Coordinator 明确区分 config generation 与 component instance generation，ownership 快照深拷贝 |
| `DGN-004` | 已完成 | Supervisor 记录 Participant/Task 稳定顺序、phase/state/policy/error type/since 与共享 shutdown budget |
| `DGN-005` | 已完成 | returned error 为 failed，未返回为 pending，显式 force 为 forced；非 clean unit 不得报告 clean stopped |
| `DGN-006` | 已完成 | `Host.Diagnostics()` 单轨返回 `ProcessDiagnostics`；Health 与 ready 消费同一 authority |
| `DGN-007` | 已完成 | Host、Service 与 CLI 测试覆盖 owner 顺序、错误聚合、pending/failed/forced 和终态 |
| `DGN-008` | 已完成 | 旧 names-only/finalization authority 从 Go 代码删除；Kernel、app、Supervisor 与根 README 同步 |
| `DGN-009` | 已完成 | Go format/test/race/vet/build、旧符号、Markdown 链接、metadata、Diff 与 staged scope 门禁通过 |

验证环境为 Windows、Go 1.25.7。未执行服务启动、外部真实 PostgreSQL/MySQL/S3、HTTP hijacked/WebSocket、部署信号、management transport 或 Linux 验收；这些结果不得从本轮单元/集成测试外推。R006 保留为实施前快照，当前事实以代码、测试和本节证据为准。
