# R003 定时调度单轨接入综合结论

## 1. 结论摘要

定时调度满足“跨业务复用 + 进程统一选择”双条件，但业务 Task 本身属于模块。最终单轨结构为：

```text
module service/task closure
  -> module-owned schedule Binding
  -> module.Contribution.Schedules
  -> internal/composition aggregate + validate
  -> Application Generation ScheduleHub Prepare/Commit/Retire
  -> gocron trigger adapter
  -> task concurrency + coordination admission
  -> existing Execution Access
  -> business closure
```

`GenerationCoordinator` 继续是 generation 状态 owner，`Supervisor` 继续是进程运行 owner，Ops 继续是 health/diagnostics
出口。ScheduleHub 只是 Application Generation 的 admission resource，与 ListenerHub 同级，不建立第二套 Runtime。

## 2. 必答能力评估

| 维度 | 结论 |
| --- | --- |
| 用例 | 模块声明“执行什么、何时执行、采用何策略”；调度层触发并治理，不拥有业务逻辑 |
| 现有能力 | 复用 Generation/Supervisor、Execution、Cache Redis resource、Logger、Telemetry、Ops、health、clock |
| 新能力 | 新增项目 Schedule Binding/Hub、gocron Adapter、coordination lease contract；扩展 background tracing/diagnostics |
| 归属 | Trigger/coordination 跨模块且进程统一选择，属底层；具体 Task closure/周期参数属模块 Binding |
| 资源 | Cache 继续拥有 Redis client；ScheduleHub 拥有 scheduler goroutine/timer、task admission 与 wait；generation 拥有 candidate |
| 运行 | candidate 只 Prepare；Commit 才开放触发；Retire/Stop 先关 admission，再等 in-flight；terminal failure 上送既有 monitor |
| 配置 | `scheduler` 只拥有引擎/lease 全局边界；任务周期和策略由模块 typed Binding 声明，可由模块 config 解码后生成 |
| 出口 | 业务不取得 scheduler；Contribution 只输出项目自有 immutable Binding；composition 消费 Control/diagnostics |
| Reload | ScheduleHub 原子切换 current candidate，旧代停止新触发并排空；同 task ID 的进程内 admission 跨代共享 |
| 契约适配 | 当前 Participant 会过早启动，不适用；需要 Application Generation admission 扩展，不新增 Supervisor |
| 失败 | typed validation、overlap/limit、Execution error、coordination skip/pause/fail/local、lease loss cancel、errors.Join |
| 日志 | scheduler 是执行/协调策略的唯一日志 owner；稳定 task/phase/generation/outcome 字段，不记录 raw payload/secret/error text |
| 影响 | module contract、pkg/kernel app/composition、generation、Cache、Execution、Observability、Ops、config/docs/tests |

## 3. 职责拆分

### 3.1 模块 Binding

不可变 `schedule.Binding` 至少声明：稳定 Task ID、`Run(context.Context) error`、Trigger、Execution PolicyName、
Concurrency Policy、Coordination Mode/Unavailable Policy。构造与校验不启动 goroutine、不访问 Redis。

`module.Contribution` 增加 `Schedules []schedule.Binding`，统一校验跨模块 Task ID 唯一。业务模块不得调用 gocron、
coordination Manager、Execution Access 或 ScheduleHub；composition 把现有能力适配进运行器。

### 3.2 触发与并发

gocron 只计算/触发 cron 与 fixedDelay，项目 Adapter 拦截所有 run：先检查 generation/current admission，再检查
task-level/process-level 并发，再检查 distributed leadership，最后调用 Execution。队列必须有界；首版不提供无限 wait queue。

### 3.3 执行治理

每次实际执行生成稳定 occurrence key，调用现有 `Execution.Access.Execute`。Scheduler 不复制重试循环、幂等 Store
或执行记录。coordination skip/pause 不伪装成业务执行成功；它们进入 scheduler diagnostics/log/metrics。

### 3.4 分布式协调

Cache App 在不泄漏 go-redis 的前提下输出稳定 coordination facade。ScheduleHub 对 strict task 维持 task-level
renewable leadership；lease 有效时才允许 gocron trigger 进入 Execution。失权先关闭 admission 并取消 task context。

### 3.5 Tracing、健康与诊断

扩展现有 Telemetry 项目契约以包围 background work，生成 scheduled-task span，并把低敏 trace ID 注入 Execution record。
ScheduleHub 生成并发安全 typed snapshot；composition 投影到 Ops `RuntimeSnapshot`，readiness 按 skip/pause/fail/local
语义计算，不新增 endpoint/registry/state machine。

## 4. Application Generation 协议

1. `Prepare`：解码 scheduler 配置；聚合/校验全部 module Schedule Binding；创建未启动 gocron candidate；探测按策略
   允许的 coordination 状态；不执行业务 Task。
2. `Commit`：ScheduleHub 在无失败 commit 区关闭 previous 新 admission、发布 candidate、开始当前触发；与 ListenerHub
   的 current generation 一致。
3. `Retire`：旧代不再产生 trigger，等待其 in-flight Task；同 Task ID 的进程级 admission 避免新旧代本地重叠。
4. `Abort`：未 commit candidate 直接 Shutdown/释放，证明零业务执行。
5. `Stop/ForceStop`：先撤销调度 admission，再按总 Supervisor/generation deadline 排空；deadline 到达取消 task context，
   保留未完成 owner 错误，不能谎报优雅成功。
6. engine terminal failure：写入 factory failure channel，沿 `GenerationCoordinator.Monitor -> Supervisor` 处理。

## 5. 配置与策略边界

`scheduler` 应用配置只声明底层默认与安全边界：enabled、global concurrency、shutdown/drain、coordination key prefix、
lease TTL/renew interval/acquire timeout/backoff。模块任务的 expression/delay、timeout、overlap、coordination mode 与 unavailable
policy 在模块 Binding 内声明；模块如果允许环境配置，应由自己的 `binding/config` 解码后构造 Binding，不能形成第二份全局任务表。

strict validation 至少拒绝：空/重复 ID、无 Run、cron/fixedDelay 同时或均未声明、非法 timezone/duration、fixedDelay
允许 overlap、strict+local fallback、renew interval 不小于 TTL 安全比例、未知 Execution policy、无界 concurrency/queue。

## 6. 单轨演进

- 不保留 old/legacy scheduler 或第二种隐式注册路径。
- `pkg/README` 的调度/分布式锁暂缓项在实现完成后移入当前能力，并链接唯一接入文档。
- Cache Redis readiness 的变化必须同步 Cache 文档和 Ops 健康语义；不能一边允许 task pause、一边由 Cache 先阻断整个 candidate。
- Observability 只扩展现有 facade；不创建独立 tracer/provider。
- 035 的 Execution memory 多实例边界继续如实保留；strict uniqueness 由 coordination lease 提供，不伪装成 Execution 已分布式化。

## 7. 研究门禁判定

关键边界、外部选择、失败语义和生命周期均有可复核证据，冲突点（gocron Locker 不同步、多实例 Redis lease 限制、
Participant 过早启动、Cache Ready 冲突）已解释。剩余未知可由确认后的单元/集成测试验证，不阻塞计划，因此研究门禁通过。
