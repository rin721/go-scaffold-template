# R006：Kernel、Coordinator 与 Supervisor 统一运行诊断

## 1. 研究问题与门禁边界

本研究回答：在 `FOUNDATION-LIFECYCLE-001` 已实施的 HEAD `7d64f8634c59375a522e66b5b989dd40b557ee9d` 上，当前运行时能否用一份脱敏快照回答“进程处于什么状态、哪些 owner 正在运行或尚未结束、它们属于哪个 generation/phase、使用何种退出策略、是否耗尽共享预算、终结与释放验证结果是什么”。

本轮只研究和规划 `FOUNDATION-DIAGNOSTICS-001`。不新增 management listener、HTTP endpoint、独立 CLI 运维命令、运行中 retry/force operation，也不替部署平台决定第二次信号或最终 kill 政策。

## 2. 方法、既有研究复用与刷新判断

研究从根 README、022 当前账本和代码出发，追踪以下真实链路：

```text
cmd/app
  -> internal/composition
  -> Kernel app component / stable Lease
  -> Coordinator
  -> Host
  -> Supervisor participant + task
  -> Health / process exit
```

既有研究的使用边界如下：

- R002 的 `Kernel Stop 或 cleanup 状态变化` 刷新条件已被生命周期提交触发，因此不复用其中关于旧 pending 缺口的当前事实；只复用“从 owner 到 Stop 的全链审计方法”。
- R004 的总体顺序仍成立，但 `P0 lifecycle 已实现` 已触发其刷新条件；本报告重新核对 diagnostics 相关结论，不把 R004 的旧实现快照当作当前代码。
- R005 的场景化终结政策仍适用，并已对当前 `WithTerminalFinalizer`、HTTP `ForceStop` 和无 finalizer 组件重新核对；不重新研究各第三方 Close 细节。
- R003 在同日验证，owner 可识别、终态真实、停止有界的外部原则没有触发刷新条件。本任务没有新的技术选型或外部语义缺口，因此不重复联网研究。

当前基线执行了：

```text
go test ./internal/kernel/... ./pkg/supervisor ./pkg/httpx
```

结果通过。该结果只证明现有测试基线，没有覆盖本报告指出的统一视图、成功终态留痕或并发快照要求。

## 3. 当前事实

### 3.1 Coordinator 已有配置与清理债务快照

`internal/kernel/coordinator.go` 的 `Diagnostics` 已表达：

- Kernel lifecycle state 与 ready；
- 配置 generation、digest 和 provenance；
- restart/cleanup required；
- failure type；
- Kernel 返回的 finalization snapshots。

快照复制 slice，不输出原始配置值和原始错误文本。reload 提交后清理失败会进入 degraded，terminal drain 超时会进入 cleanup-pending，terminal finalizer 失败会进入 failed。这些状态已经比 error string 更强。

局限是该结构只描述 Coordinator/Kernel。`Generation` 是配置提交代际，组件实例代际位于另一层；字段没有明确区分二者。`LastFailureType` 没有对应 phase/time，成功或无 finalizer 的终态也不会作为责任结果保留。

### 3.2 Kernel finalization snapshot 只覆盖仍被指针持有的槽位

`internal/kernel/app/finalization.go` 的 `FinalizationSnapshot` 已表达 component ID、instance generation、candidate/retired/current phase、waiting/pending/running/finalized/terminal-failed state、attempt 和 error type。

`managedComponent.Finalizations()` 只遍历 `candidate`、`retired`、`stopping`。当前正在 lease 中服务的 slot 不在该集合；成功 finalization 后 slot 指针被清空，no-finalization 组件也以零 attempt 成功后消失。因此当前快照能定位 unresolved debt，却不能形成完整责任清单，也不能在 clean stopped 后证明每项责任如何结束。

场景政策同样没有进入快照：有无 `terminalFinalizer` 可以由实现推断 `NoFinalization` 或 `DrainThenTerminalClose`，但 management consumer 不应通过 nil function 猜语义。运行时也没有单独记录“finalizer 返回成功”和“物理句柄释放已被额外验证”的差异。

### 3.3 当前 finalization 快照尚未证明可与 reload/stop 并发读取

`Kernel.Finalizations()` 只在复制 component slice 时持有 `Kernel.mu`；组件的 candidate/retired/stopping 指针和 slot 元数据由另一条 operation 路径修改。`lease` 对 current slot 和 active use 有自己的 mutex，但 `managedComponent.Finalizations()` 没有通过同一锁读取全部元数据。

因此“返回 slice 副本”不等于完整并发安全。现有 race 测试没有在 reload、terminal drain 或 finalizer 执行期间持续读取 diagnostics，不能据此宣布安全快照门禁通过。

### 3.4 Supervisor 已有共享预算，但诊断只保留名称分类

`pkg/supervisor` 已在一次 shutdown 中计算同一 graceful deadline 和 final deadline。Participant 反向 Stop、可选 `ForceStop` 和 Task 等待共享总预算，没有为每个 owner 重建完整 timeout。

`Snapshot` 当前只包含 process state/ready/since、一个 error type 字符串，以及：

- `PendingParticipants []string`；
- `PendingTasks []string`；
- `ForcedParticipants []string`。

它没有 owner kind、phase、attempt、exit policy、per-owner error type、首次进入时间和 budget phase/deadline。Task/Participant 在正常运行时也没有出现在责任清单中。

另一个直接控制流事实是：Participant `Stop` 返回 error 后，其 result channel 已被消费，但 owner 仍先进入 pending map；shutdown 尾部对 channel 的非阻塞读取无法再次取得已消费结果。因此“Stop 已返回错误”和“Stop goroutine 仍未返回”会被同样保留为 pending。前者应是明确 failed 终态，后者才是未完成责任。

Task 也只有最终未返回者进入 `PendingTasks`；已返回 error 的 task 只存在于聚合 error type，不能从 snapshot 定位该 task 的终态。

### 3.5 Host 仍暴露两份并列快照

`internal/kernel/host.go` 的 `Diagnostics()` 返回 `(kernel.Diagnostics, supervisor.Snapshot)` tuple。Health 再读取两者并自行组合 ready，但不存在一份 process-level authority 来统一：

- process state 与 Kernel state；
- config generation 与 component instance generation；
- capability、participant、task 三类 owner；
- graceful、force、terminal close、no-finalization 与 cancel-and-wait 政策；
- pending、failed、forced、finalized 的终态；
- shutdown budget 与 cleanup/restart requirement。

全仓生产代码没有其他 diagnostics consumer。Service 创建 Host 后直接 `Run`，CLI one-shot 直接创建 Supervisor；当前也没有 management transport。这意味着可以在不迁移外部协议的情况下，让 Host 成为唯一组合 authority，同时保留 Kernel 与 Supervisor 各自的窄 typed source。

### 3.6 当前错误记录边界基本正确

Coordinator、Kernel、Supervisor 和 Adapter 继续返回带链 error；`internal/composition` 的 reload reporter 与 `cmd/app` 顶层才决定日志/输出策略。当前 diagnostics 字段保存 `%T`，没有保存 `error.Error()`、DSN、配置值或凭据。

本任务不需要把错误对象塞进快照，也不需要让底层逐层记录日志。需要补的是 owner-local error type 与 phase，而不是复制原始 error 文本。

## 4. 字段覆盖矩阵

| 目标问题 | Coordinator | Kernel app | Supervisor | Host 组合 | 当前判断 |
| --- | --- | --- | --- | --- | --- |
| state / ready | 有 | finalization state only | 有 | tuple 后由 consumer 组合 | 部分通过 |
| config generation/digest/provenance | 有 | 无 | 不适用 | 无统一字段 | 部分通过 |
| instance generation / phase | 只嵌套 unresolved | 有 | 无 | 无统一 owner 模型 | 部分通过 |
| owner kind / stable owner name | component ID | component ID | 名称列表，无 kind | 无 | 未通过 |
| scenario / exit policy | 无 | 可从 nil finalizer 推断 | 可从 `ForceStopper` 推断 | 无 | 未通过 |
| attempt | 无 | terminal attempt 有 | 无 | 无 | 部分通过 |
| pending units | cleanup bool + unresolved finalization | unresolved only | names-only | 无统一索引 | 部分通过 |
| failed / forced terminal result | Kernel failed + terminal-failed | terminal-failed | forced names；task/participant failed 不定位 | 无 | 部分通过 |
| shutdown budget | 无 | 只消费 context | 实现有 deadline，快照无 | 无 | 未通过 |
| release verification | 无 | 未单独表达 | 不适用 | 无 | 未通过 |
| error type without text | aggregate 有 | per slot 有 | aggregate 有 | 无统一 owner 映射 | 部分通过 |
| concurrent safe snapshot | Coordinator slice copy | 未证明 | mutex copy | 依赖两源 | 未通过 |

## 5. 推断与方案取舍

以下是由当前事实推导的目标设计，不是已实现能力。

### C-001：Host 应拥有唯一 process diagnostics，不能把 tuple 当统一视图

Kernel 与 Supervisor 的生命周期对象不同，仍应保持分工；但 management consumer 不应自行拼装两份状态。Host 正好同时拥有 Coordinator 与 Supervisor，应把它们映射为一份项目自有、脱敏、不可变的 process snapshot。

该结构属于 `internal/kernel`，当前不需要放入 `pkg`，也不需要提前承诺 JSON/HTTP schema。未来 management listener 只能消费它，不能绕过 Host 分别查询底层。

### C-002：各底层应维护 typed responsibility ledger，而不是增加更多名称 slice

Kernel component 需要记录 current/unresolved/最近终态；Supervisor 需要记录 participant/task 的 phase、state、policy、attempt、error type 和 since。Host 只做确定性映射与聚合，不通过 error string 推断 owner。

Participant Stop 返回 error 应进入 failed 终态；只有调用仍未返回才是 pending。Task 返回 error 同理。显式 force 成功是 forced 终态，不等于 graceful success；process 可因此 failed，但不能继续把 owner 描述为 pending。

### C-003：政策从现有真实契约确定，不能新增万能 option

当前政策可以在 owner 建立时确定：

- Kernel 无 terminal finalizer：`no-finalization`；
- Kernel 有 terminal finalizer：`drain-then-terminal-close`；
- 普通 Participant：`graceful-shutdown`；
- 实现 `ForceStopper` 的 Participant：`graceful-then-force`；
- Task：`cancel-and-wait`。

不需要增加 `Close(force bool)`、任意字符串 policy 或允许调用方谎报语义的通用 option。HTTP 的 force 支持继续由真实接口决定。

### C-004：finalizer completion 与 release verification 必须分开

terminal finalizer 返回 nil 只证明项目定义的终结动作返回成功；并不自动证明 OS handle、连接池或文件锁已由运行时额外探测。当前快照只需要显式输出 `not-required` 或 `not-proven`：无 finalizer 的责任不需要释放验证，当前有 finalizer 但没有运行时 verifier 的资源必须诚实为 `not-proven`。没有真实 producer 时不预建 `passed/failed` 常量，更不能因为重复 Close 返回 nil 就标 passed。

确定性测试仍可用 listener rebind、文件 rename/delete、goroutine done 等外部证据证明 release。未来若真实管理需求要求运行时 verifier，再由资源 Adapter 单独研究并扩展 result vocabulary；本任务不加入无调用方的万能 callback。

### C-005：budget 必须可观察，但不在 diagnostics 中重新控制时间

Supervisor 继续是 process shutdown budget owner。快照只暴露 total/graceful/force 的起点、deadline、当前 phase 和是否耗尽；Coordinator、Participant 与 Task 只消费收到的 context，不创建第二套 policy。

部署 grace period、第二次信号和最终进程 kill 仍是独立研究项。当前 diagnostics 只能说明 owner 已 failed/forced/pending，不能执行外部 kill。

### C-006：Stopped 必须由责任终态证明

clean `StateStopped` 只允许在全部已启动 Participant 和 Task 都已确定完成、且 Coordinator 已完成 Kernel Stop 时出现。failed/forced 是明确终态，可以结束等待，但 process outcome 仍应保持 failed；未返回的 owner 保持 pending，绝不能报告 clean stopped。

## 6. 不采用的路线

| 路线 | 不采用原因 |
| --- | --- |
| 只给现有 tuple 增加几个 bool | 仍由 consumer 猜 owner、policy 和终态，不能关闭 FND-DIAGNOSTICS 门禁 |
| 把所有状态迁到一个新通用 lifecycle package | Kernel generation 与 process task 语义不同，会形成第二套万能状态机 |
| 快照保存完整 error 或配置 candidate | 会泄露敏感值并扩大锁与序列化边界 |
| 立即新增 `/diagnostics`、CLI 或 management listener | 当前任务没有鉴权、审计、transport 和产品协议授权；这属于后续 MANAGEMENT-001 |
| 把 timeout owner 直接标 failed 并遗忘 goroutine | failed terminal 必须有“调用已返回”的证据；仍运行的 goroutine只能是 pending |
| 以 finalizer nil/`ForceStopper` 由 consumer 动态推断 policy | 重复隐含规则，未来 transport 容易漂移；应在 owner ledger 建立时冻结 |

## 7. 研究门禁结果、局限与对计划的影响

研究门禁通过，理由是：当前数据源、唯一组合位置、字段缺口、终态分类、并发风险、脱敏边界和非目标均有代码证据，剩余未知不妨碍形成施工计划。

仍未验证或不在本任务回答：

- 真实 PostgreSQL/MySQL/S3 的物理释放探测；
- HTTP hijacked/WebSocket owner；
- deployment grace、第二次信号与外部 forced exit；
- management listener 的鉴权、序列化、缓存和审计；
- 运行中 retry/force/reconciliation operation。

实施计划必须：

1. 保持 Kernel、Supervisor 各自拥有 typed ledger，由 Host 形成唯一 process snapshot；
2. 单轨移除 names-only/tuple 旧 authority，不保留兼容别名；
3. 区分 pending、failed、forced、finalized 与 release verification；
4. 对 snapshot/reload/stop 并发增加 `-race` 证据；
5. 不改变配置、资源关闭政策、HTTP transport、CLI 或部署副作用。
