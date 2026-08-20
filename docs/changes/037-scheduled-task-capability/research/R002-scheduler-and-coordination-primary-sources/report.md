# R002 调度引擎与 Redis 协调主源评估

## 1. 研究问题

本记录核对成熟 Go 调度库是否同时覆盖 `cron` 与真正的 `fixedDelay`，以及现有 Redis/go-redis 技术栈
可以怎样提供分布式单实例执行权。只采用上游官方仓库、Go package 文档、Redis 官方命令/模式文档。

## 2. 候选比较

| 候选 | cron | completion-based fixedDelay | 并发/关闭 | 分布式说明 | 结论 |
| --- | --- | --- | --- | --- | --- |
| `go-co-op/gocron/v2` | `CronJob`，支持 5/6 字段和 timezone | `DurationJob + WithIntervalFromCompletion` 明确定义从完成时间计算下一次 | job/scheduler limit、context、`ShutdownWithContext` | 提供 Locker/Elector 接口，但官方注明 Locker 不同步多 scheduler 的 run time | 选择为被封装的触发引擎 |
| `robfig/cron/v3` | 成熟 parser/schedule，timezone 与 wrapper 完整 | 没有原生 fixedDelay job lifecycle | 有 skip/delay wrapper，需自行补 fixedDelay/lifecycle | 无项目所需 distributed policy | 不单独选择；避免重复实现 gocron 已有能力 |
| 自研 timer + parser | 可实现 | 可实现 | 全部自研 | 全部自研 | 成熟方案可用，不接受无理由重造 |

`gocron/v2` 的 v2 分支仍在维护，2026-07 发布 v2.22.0；Security Policy 表示 v2 受支持、v1 停止支持。
仓库使用 MIT license。实施阶段拟固定 v2.22.0，并在修改 `go.mod` 前重新核对该 tag 的 Go 版本、许可证、
依赖树和测试；若事实实质变化则回研究门禁。

## 3. 只把 gocron 当触发引擎

项目不会暴露 `gocron.Scheduler/Job/Locker`，也不会让业务模块调用 `NewJob/Start/Shutdown`。Adapter 只映射：

- `CronTrigger{Expression, WithSeconds, Location}` -> `gocron.CronJob`；
- `FixedDelayTrigger{InitialDelay, Delay}` -> `DurationJob + WithStartAt + WithIntervalFromCompletion`；
- 项目 concurrency policy -> gocron job/scheduler limit；
- 项目 context/lifecycle -> `WithContext`、Start 与 `ShutdownWithContext`；
- gocron SchedulerMonitor -> 现有 generation failure channel 与项目 diagnostics。

项目契约先完成完整校验，再创建 gocron Job。业务闭包只接收 `context.Context`，不会获得 gocron Job、UUID、
logger 或 scheduler handle。

## 4. 为什么不用 gocron Locker 直接宣称严格单实例

gocron 官方 `Locker` 文档明确：锁在一次 job run 期间持有，且 locker/scheduler 不负责多个 scheduler 的
run time 同步；duration job 需人为对齐 start time，并仍受 clock skew 影响。这意味着：

- 两个实例的 fixedDelay 序列可能漂移；
- 一个很短的任务结束并释放锁后，另一个晚到的同周期触发仍可能获得锁；
- 只用 task name 的“运行期间互斥”不等于“同一周期只有一个实例获得执行权”。

因此 037 不把上游 Locker 作为严格保证。项目建立 task-level renewable leadership lease：只有当前 lease owner
的触发引擎获得该任务 admission；同一 task 的其他实例保持 standby。这样 `cron` 与 `fixedDelay` 都由一个
leader 产生序列，而不是让每个实例各自产生后再碰运气抢短锁。

## 5. Redis lease 的正确最小语义

Redis 官方模式给出的基础是 `SET key token NX PX ttl`。token 必须对每次获取唯一；释放时必须原子比较当前值
仍是自己的 token，再删除，不能直接 `DEL`。续期同样必须在服务端原子比较 token 后延长 TTL。

计划中的项目契约：

- `Manager.Acquire(ctx, key, ttl) -> Lease/acquired/error`；
- `Lease.Renew(ctx, ttl)` 只在 token 仍匹配时成功；
- `Lease.Release(ctx)` 只释放自己的 token；
- 获取耗时从本地有效窗口扣除，renew interval 必须显著小于 TTL；
- 失去 lease 或无法在安全窗口内续期时，立即关闭该任务新 admission 并取消运行 context；
- 所有命令经现有 Cache-owned go-redis client 和项目 Access 执行，不建立第二连接。

Redis Lua 脚本在服务端原子执行，适合兼容当前 Redis 版本完成 compare-and-renew/release。脚本的 key 和 args
固定传入，不把动态值拼进脚本文本。

## 6. 保证、假设与不能夸大的部分

### 6.1 037 可以保证

在一个满足 Redis 单主节点命令语义、TTL 时钟稳定、网络延迟小于配置安全窗口的 coordination endpoint 内，
同一 task 同时只有一个未过期、token 匹配的 lease owner 获得调度执行权。协调不可用时 strict task 不运行；
恢复后实例重新参与争抢，无需重启。

### 6.2 037 不宣称

- 不宣称任意 Redis 异步 failover、时钟跳变或跨数据中心分区下仍绝对互斥。
- 不宣称业务副作用 exactly-once。Redis 官方文档同样提醒长任务应考虑 fencing token；仅取消 context 无法撤销
  已经发生且下游不支持 fencing 的外部写入。
- 不把无限续租视为无条件安全；续期次数、TTL、超时和失权处理必须有界、可观测。

需要比上述故障域更强的任务，应切换实现同一项目 coordination contract 的共识型 backend，并让受保护资源
消费 fencing token；这不在 037 首版范围。

## 7. 故障策略

每个任务显式声明 coordination mode 与 unavailable policy：

- `skip`：跳过本次/当前不可用窗口，记录诊断，后续继续争抢；
- `pause`：任务进入 paused，影响 application readiness；后台有界探测，恢复后自动重入；
- `fail`：通过现有 generation failure channel 触发 Supervisor 失败流程；
- `local`：只允许 best-effort 模式，明确弱化为每实例本地执行；strict 模式校验时拒绝。

任何未声明/未知策略都在候选准备阶段失败，不静默回退。

## 8. 局限与刷新条件

- 当前研究未实际下载 gocron 或运行 Redis；真实依赖/集成测试属于确认后的实施。
- 若用户要求 Redis Sentinel/Cluster failover 下的强互斥、fencing 下游或多数据中心一致性，必须重新研究并确认。
- 若 gocron 在实施前发布影响 API/Go 版本/安全的更新，重新验证选定版本，不自动漂移。
