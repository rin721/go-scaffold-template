# R002：底层能力、装配与生命周期闭环审计

## 1. 研究问题与复用判断

本报告不再问“缺哪些常见 HTTP 功能”，而是追踪一条真实进程链：配置如何进入，依赖如何装配，资源由谁创建，何时启动并 ready，运行失败如何上抛，配置如何重载，流量如何排空，资源如何停止，以及这些语义是否有诊断和测试证明。

复用检查结论：

- [012-R021](../../../012-business-module-architecture/research/R021-foundation-closure-implementation/report.md) 提供了 Kernel/Host 旧快照的正向闭环证据，但其刷新条件已经被后续 Todo 真实业务接入命中，不能直接作为当前结论；
- [012-R019](../../../012-business-module-architecture/research/R019-config-contracts/report.md) 的外部配置原则仍可复用，但其中关于弱解码的本地缺口已经被当前严格 Decode 实现关闭；
- [017-R001](../../../017-module-capability-assessment-gate/research/R001-current-module-capability-assessment/report.md) 的“先做能力评估、局部纯内存装配”原则仍有效，但当前 Contribution 的实际表达能力必须重新核验。

因此本轮新增当前快照审计，而不修改旧研究历史。

## 2. 当前事实链

### 2.1 配置输入与初始化

```text
cmd/app
  -> internal/composition.Application
     -> FileSource -> EnvSource -> Loader.Load
     -> Coordinator.Prepare
     -> ValidateCandidate(all owner bindings)
     -> same immutable Snapshot
```

- Service 与 CLI 都从 `internal/composition` 进入，业务包不读取环境变量或配置文件。
- `Loader` 按声明顺序合并 Source；Snapshot 深拷贝、计算摘要并提供脱敏视图。
- JSON/YAML 重复键、未知字段、跨类型弱转换、未知顶层 section 已有拒绝语义。
- `Coordinator` 是 Loader 唯一进程级调用者，初始应用配置和 Kernel 配置消费同一候选。
- 未覆盖点：`EnvSource` 的 `APP_A=x` 与 `APP_A__B=y` 形状冲突由 `setNested` 覆盖，结果依赖环境枚举顺序；该路径没有复用 `mergeMap` 的 scalar/object 冲突检查。

### 2.2 依赖装配与资源创建

```text
Bootstrap command: ComposeBootstrap -> config bindings/default manager/frozen CLI
Service/CLI:       explicit application composition
                    -> Kernel Plan (forward-only typed bindings)
                    -> Freeze -> Install once
                    -> stable capability Access/Lease
                    -> local Todo module composition
```

- `cmd/app` 不创建业务依赖；`internal/composition` 是唯一应用组合根。
- Kernel Plan 只允许依赖同一 Plan 中更早的 typed Binding，因此循环依赖在结构上不可表达。
- 资源构造留在 Kernel component，业务依赖项目自有窄契约；Todo `Service` 不导入 Kernel、HTTP 或 GORM。
- Plan 冻结后一次安装，不存在运行期 service locator、扫描或第二个容器。
- Bootstrap command 明确不创建 Kernel、listener 或长期 goroutine；它和 Service/CLI 的生命周期不是一条混合路径。

### 2.3 启动、ready 与运行

- `Kernel.startCandidate` 依序执行 `Stage -> Build -> Start -> Ready -> PublishInitial`；失败时反序补偿已经启动的组件。
- `Host` 固定先启动 Coordinator/Kernel，再启动 module participant、application participant 和 HTTP server；停止反序执行。
- `Supervisor.Task` 拥有长期 `Run(ctx)`，可用 Ready channel 确认已经承担运行责任；长期任务提前返回被视为失败并取消同级任务。
- HTTP listener 的 Ready 与 config watcher 都进入 Supervisor；CLI 使用 `RunOperation`，不会把一次性操作误判为长期任务提前退出。
- Host 健康目前只汇总 process liveness/readiness；业务模块无法贡献 health check，Contribution 也不能贡献长期 Runner/Ready。

### 2.4 Reload、一致性与排空

```text
load one candidate
  -> validate every owner
  -> stage/build/start/ready every changed Kernel component
  -> reverse drain old leases
  -> commit all changed components
  -> publish one Snapshot generation
  -> resume admission
  -> reverse cleanup previous generation
```

- 候选构造或 drain 失败时，旧代恢复服务且 Snapshot 不提交。
- `RestartRequired` 在提交前返回，不产生部分切换。
- 资源消费者只能经 `Lease.Use` 取得短期引用；drain 先停止新借用，再等待活跃借用释放。
- 提交后的旧代清理失败返回 `CommittedCleanupError`，Coordinator 进入 degraded、撤销 ready 并阻断后续 reload，避免假装回滚。

这一设计的事务边界清晰，应该保留；但“阻断后续动作”不等于“清理责任已完成”。

## 3. 关键闭环缺口

### 3.1 终止排空超时后，Kernel 过早进入 stopped

`Kernel.Stop` 只对 `terminalDrainReverse` 已经成功排空的组件调用 `PrepareStop/StopCurrent`。若某个活跃 Lease 超时：

1. 该组件及更早的 provider 不进入待关闭集合；
2. Kernel 仍把内部状态改为 `kernelStopped`；
3. 后续 `Stop` 直接返回 nil；
4. 活跃 Use 之后即使释放，也没有 owner 再完成 Close。

`TestTerminalDrainTimeoutDoesNotResumeOrForceCloseActiveLease` 证明不会在活跃使用期间强制关闭，这是正确安全属性；但测试在释放 Lease 后没有证明最终关闭或可重试关闭。当前语义因此是“拒绝新使用但可能永久保留资源”，不能称为终止闭环。

### 3.2 清理失败后丢失资源引用与清理责任

`managedComponent.StopPrevious` 和 `DiscardCandidate` 都在 stop 返回错误后清空实例引用。结果是：

- 错误链仍在，但 owner、generation、实例清理状态不可重试；
- `CommittedCleanupError` 能说明新代已提交，却不能指出哪些旧代仍待处理；
- Coordinator degraded 后只允许进程重启，但当前进程也没有可执行的最后清理路径；
- 同一模式还影响启动失败补偿和候选丢弃失败。

错误向上导出通过了，但资源所有权没有随错误一起保留，生命周期与诊断门禁未通过。

### 3.3 Supervisor 对不合作 Stop 的诊断不完整

长期 Task 超时会把 task name 写入 `PendingUnits`。Participant Stop 则由 goroutine 调用并受同一 shutdown context 约束，超时错误带 participant name，但最终 diagnostics 的 `PendingUnits` 只接收长期 Task 清单。若 Stop 忽略 context，Supervisor 返回后仍可能残留 goroutine，无法从结构化快照区分责任方。

Go 无法安全强杀任意 goroutine，问题不在“缺少强杀 API”，而在没有明确声明：调用方何时可以退出、哪些 owner 尚未结束、是否允许重试或只能由进程退出收口。

### 3.4 模块扩展只证明了有限 profile

`module.Contribution` 当前只有 Routes 与 Participants。Todo 证明了：

- 同步 HTTP request/response；
- 一次性 CLI operation；
- 启动阶段同步 migration participant；
- 共享 Database/Clock/ID/I18n 能力消费。

它没有证明后台 consumer、scheduler、长轮询、异步 warm-up、模块健康检查或独立 drain。未来业务若需要这些能力，不能在 module `Start` 内私自起 goroutine，也不能绕过 composition 创建第二套资源；必须先回到底层能力评估。这里应采用场景门禁，不应现在扩成万能插件协议。

## 4. 十一门判断

| 门禁 | 当前结论 | 证据与缺口 |
| --- | --- | --- |
| 事实门 | 通过 | 已从入口、组合根、运行实现、测试和边界规则交叉验证；旧研究仅作历史线索 |
| 目标门 | 通过 | 目标是 copy-owned HTTP API server baseline 与业务设计解锁，不是通用插件 Runtime |
| 边界门 | 部分通过 | Kernel/业务/HTTP/第三方边界清晰；Runner/Health 模块扩展边界尚未表达 |
| 装配门 | 当前 profile 通过 | 手工组合根与 forward-only Plan 闭环；后台业务 profile 未证明 |
| 生命周期门 | **未通过** | terminal drain timeout 和 cleanup failure 会丢失最终清理责任 |
| 一致性门 | 部分通过 | 提交前原子、提交后诚实 degraded；失败清理状态没有持续拥有 |
| 错误门 | 部分通过 | `%w`/`errors.Join`/typed error 良好；诊断缺 owner/generation/pending cleanup |
| 治理门 | 部分通过 | import graph、binding、route/participant 唯一性有门禁；关键失败路径缺反向验收 |
| 演进门 | 部分通过 | immutable generation 与 RestartRequired 明确；sticky restart 和清理债务恢复政策未闭环 |
| 复杂度门 | 通过 | 当前显式架构足够小；替换为容器或通用 DAG 不能直接修复上述问题 |
| 业务扩展门 | **未通过** | 现有 Todo profile 可复用，但按本任务规则，底层生命周期未闭环前不解锁新模块详细设计 |

## 5. 结论记录

### C-001：保留显式双层生命周期，而不是合并或换容器

- 本地事实：Kernel 管资源代际与 Lease，Supervisor 管进程 participant/task；职责不同。
- 原始意图：业务不感知容器和资源换代，进程 owner 统一收口长期工作。
- 选项：保留并加固；用 Fx 替换；改成通用运行期 DAG；删除 reload。
- 取舍：Fx/通用 DAG 会扩大依赖、反射或注册复杂度，却不能自动定义原子 reload 与清理失败责任；删除 reload 会丢弃已经真实验证的能力。
- 建议：**保留并局部优化**。
- 证据强度：高（当前源码、测试和既有真实 SQLite reload 集成测试）。

### C-002：清理责任必须成为状态，而不只是 error

- 本地事实：当前 stop error 向上返回后实例引用被清空，terminal timeout 后 Kernel 仍 stopped。
- 原始意图：不强制关闭活跃资源，同时保证资源 owner 最终释放。
- 选项：超时强关；超时即遗忘；保留 pending cleanup 并定义重试/最终退出政策。
- 风险：强关可能破坏正在处理的请求；遗忘会泄漏；无限等待破坏有界退出。
- 建议：**补齐两阶段终止与 pending cleanup 状态**。停止接入应立即且不可回滚；宽限期超时后保留 owner/generation/原因和清理责任，具体资源只有在安全条件满足时关闭。是否允许 retry、何时 force close 或直接以非零退出，必须由后续设计按资源类型明确。
- 证据强度：高（可由当前控制流直接证明）。

### C-003：业务扩展采用 profile gate，不预建万能贡献协议

- 本地事实：Routes + Participants 已满足 Todo；Runner/Ready/Health 未进入 Contribution。
- 原始意图：模块只贡献完成品，goroutine 和共享资源有唯一 owner。
- 选项：立即把所有能力塞入 Contribution；允许模块私启 goroutine；按真实需求先做能力评估。
- 建议：**保持最小契约，增加业务设计解锁清单**。同步 HTTP/CLI profile 在底层 P0 修复并验收后可解锁；需要后台任务、健康贡献或新外部资源时，先建立独立 foundation capability 研究，不得绕过组合根。
- 证据强度：高（当前契约与 Todo 调用方）；后台场景需求仍未知。

## 6. 验证、未知与刷新

本轮只做静态代码、测试意图和文档证据审计，没有重新运行 Go test、启动服务或制造故障。022-R001 已记录相同生产源码快照上的 test/race/vet/build 结果，但该结果不覆盖这里发现的缺失断言。

仍未知：

- Database、Cache、Logger、Storage 各自 Close 失败后是否允许安全重试；
- 目标部署平台的 SIGTERM 总预算和强制终止策略；
- 第一个新业务模块是否需要后台 Runner、动态健康或新共享资源；
- sticky RestartRequired 是否应允许配置恢复后自动清除。

这些未知不妨碍形成加固计划，但会阻塞具体清理 API、force policy 和后台模块契约的源码设计。

## 7. 研究结论

当前不是“没有底座”，而是“正向主链已经形成、少数最难失败路径尚未闭环”。应保留显式 composition、严格候选、forward-only Plan、stable Access/Lease、Coordinator 与 Supervisor 分工；优先修复终止与清理所有权、结构化诊断和反向验收。完成这些 P0 前，不按本任务规则继续细化新的 Handler/Service/Repo/Model。
