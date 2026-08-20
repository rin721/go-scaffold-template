# 业务模块接入定时调度能力

本文是业务模块声明 `cron` / `fixedDelay` 任务的当前权威入口。模块只描述“执行什么、什么时候执行、采用什么策略”；`internal/composition` 统一完成触发、并发、分布式执行权、Execution、Tracing、诊断和 Application Generation 生命周期装配。

## 1. 声明入口

模块使用 `pkg/schedule` 构造不可变 `Binding`，再通过唯一的 `module.Contribution.Schedules` 输出。构造过程不访问网络、不启动 goroutine，也不持有底层 scheduler：

```go
trigger, err := schedule.Cron("0 */5 * * * *", "Asia/Shanghai", true)
if err != nil {
    return Module{}, err
}
coordination, err := schedule.DistributedSingleton(true, schedule.UnavailablePause)
if err != nil {
    return Module{}, err
}
binding, err := schedule.Bind(schedule.Spec{
    ID:              "billing.reconcile",
    Trigger:         trigger,
    Concurrency:     schedule.SerialSkip(),
    Coordination:    coordination,
    ExecutionPolicy: "billing",
}, service.Reconcile)
if err != nil {
    return Module{}, err
}

contribution := module.Contribution{
    ID:        "billing",
    Schedules: []schedule.Binding{binding},
}
```

Task ID 在整个应用内唯一，格式为小写字母开头、以字母数字及 `.` / `-` 分段的稳定标识。composition 会统一复制、排序并校验所有模块贡献；重复 ID、无效策略和未知配置覆盖都会让候选 Generation 在 Commit 前失败。

禁止以下接入方式：

- 业务模块导入 `gocron`、`go-redis` 或 `internal/kernel/app/schedule`；
- 通过 `init`、扫描、全局 Registry 或运行时查找隐式注册；
- 业务模块自行创建 scheduler、Redis client、健康端点或后台 Supervisor；
- 把具体业务逻辑放入调度层。

## 2. 触发方式

### cron

`schedule.Cron(expression, timezone, withSeconds)` 支持五字段或显式六字段表达式。`timezone` 为空时使用 `scheduler.timezone`；非空时必须是可加载的 IANA 时区。项目契约先校验字段数，内部 gocron Adapter 在 Generation Prepare 阶段完成语法校验。

### fixedDelay

`schedule.FixedDelay(delay, initialDelay)` 从上一次运行完成后再等待 `delay`，不是按开始时间固定频率触发。fixedDelay 任务必须使用 `SerialSkip()`；完成包括成功、最终失败、超时或取消后的运行器收敛。

## 3. 并发与协调策略

`Concurrency(maxConcurrent, congestion, queueLimit)` 声明 Task ID 级并发。`skip` 不排队；`wait` 必须声明正数且有界的队列。调度组件还使用 `scheduler.maxConcurrency` 作为当前 Generation 的全局上限，不同任务仍保留独立状态和策略。

协调策略：

| 声明 | 协调不可用时 | Readiness | 恢复 |
| --- | --- | --- | --- |
| `Local()` | 每个实例本地运行 | pass | 不依赖协调 |
| strict + `skip` | 关闭任务准入，标记 degraded | pass / warn | 自动重新争抢 |
| strict + `pause` | 关闭任务准入 | fail | 自动重新争抢 |
| strict + `fail` | 关闭准入并上送现有 Generation monitor | fail | 由既有 Supervisor 策略处理 |
| best-effort + `local` | 显式开放本地弱化执行 | pass / warn | 恢复后切回 leader |

严格 `distributedSingleton` 不允许 `UnavailableLocal`，因此协调依赖故障不会静默退化为每实例本地执行。未获得租约是正常 standby，不记录为故障；只有取得执行权的实例才安装该分布式任务的触发器。续租失败或失权会取消任务 context、关闭后续准入并按策略重新参与。

## 4. 运行治理与边界

每次获准的触发都进入同一条现有治理链：

```text
Schedule trigger
  -> Task / global admission
  -> Telemetry.Observe
  -> execution.OperationExecutor.Execute
  -> module task function
```

调度器按 Task ID 和本次 occurrence 生成 Execution key，写入 `schedule.cron` / `schedule.fixedDelay` 触发来源，并使用 `scheduler.occurrenceRetention` 作为占用与完成去重窗口。命名重试/超时策略仍由 `execution.policies.<name>` 负责；模块通过 `ExecutionPolicy` 引用，不在调度声明中复制重试参数。

严格执行权表示 Redis 租约在同一协调键上只授予一个 token owner；token 比较续租和释放防止非 owner 修改租约。它不等价于业务副作用 exactly-once：Redis failover、长时间进程停顿以及不识别 fencing token 的目标系统仍需要业务幂等或目标资源自身的一致性协议。当前 Execution Store 为进程内 memory，不能把它解释为跨进程持久化去重。

## 5. 配置与启用

全局调度默认关闭。示例：

```yaml
scheduler:
  enabled: true
  timezone: Asia/Shanghai
  maxConcurrency: 32
  shutdownTimeout: 30s
  occurrenceRetention: 24h
  coordination:
    namespace: go-scaffold-template:scheduler
    leaseTTL: 30s
    renewInterval: 10s
    acquireTimeout: 2s
    retryMin: 500ms
    retryMax: 10s
  tasks:
    billing.reconcile:
      enabled: true
      unavailablePolicy: pause
```

`tasks` 只能覆盖模块已经声明的 Task ID，并且只覆盖启用状态与协调不可用策略；不能从配置注入函数或另写触发定义。启用分布式任务时应把 `cache.driver` 配成 `redis` 并使用部署环境提供的凭据。Cache disabled 或 Redis 临时不可用会进入任务自身策略，不会自动改成本地执行。

配置变化通过完整 Application Generation 重建。Prepare 只校验并构建暂停候选，Commit 后才开放触发和执行权争抢；新代 Commit 会先关闭旧代准入，Abort/Retire/Stop 会取消、排空并释放资源。

## 6. 诊断与验证

现有 management diagnostics 的 `scheduler` 字段提供 `enabled`、`ready`、`degraded`、Generation 和逐任务状态；`schedulerHealth` 给出 `pass` / `warn` / `fail`。日志只记录 Task ID、phase、generation、state 和稳定错误类型，不记录 Redis token、业务参数或原始错误文本。

生命周期 start/drain/stop 是 Info，单次任务失败和协调依赖导致的可恢复状态变化是 Warn，进入 `failed` fatal state 是 Error。
正常 standby、未取得租约、健康完成和优雅取消不得打印 Warn/Error。

模块测试至少验证 Binding 构造、任务取消、业务错误和自有依赖；底层测试已覆盖 cron/fixedDelay、并发、两逻辑实例竞争、失权、策略隔离、自动恢复、Generation 切换以及任务失败/fatal 日志门禁。真实部署还必须演练 Redis 中断、恢复、failover 和长暂停，并按目标系统能力评估 fencing / 业务幂等。

相关入口：

- 项目声明契约：`pkg/schedule`
- 分布式租约契约：`pkg/coordination`
- 统一运行实现：`internal/kernel/app/schedule`
- 配置： [配置说明](../configuration/README.md)
- 运维恢复： [定时任务运维](../operations/scheduled-tasks.md)
