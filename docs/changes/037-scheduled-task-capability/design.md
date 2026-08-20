# 定时调度能力设计

## 1. 文档状态

- 变更编号：`037`
- 当前阶段：已确认并实施完成
- 实施状态：代码、配置、测试与当前权威文档已同步
- 需求依据：[requirements.md](requirements.md)
- 研究依据：`R001`、`R002`、`R003`

本设计记录 037 的已实施方案；当前使用方式以 `docs/development/scheduled-task-capability.md` 与实际代码为准，本文件保留任务级设计证据。

## 2. 设计摘要

本变更选择以下单轨方案：

1. 以项目自有 `pkg/schedule` 定义业务声明契约，扩展 `module.Contribution.Schedules`；
2. 以 `gocron/v2` 作为内部触发引擎，只负责 cron/fixedDelay 计算和本地触发；
3. 以 Application Generation 内的不可变 Schedule Component 构建候选，以进程稳定的 `ScheduleHub` 负责 Commit/Retire 准入切换；
4. 以项目自有 `pkg/coordination` 定义执行权契约，由 Cache 内部 Redis Adapter 复用现有 go-redis 连接实现可续租领导权；
5. 所有业务运行继续进入现有 Execution，Tracing、日志、健康、诊断、Generation monitor 和 Supervisor 也沿用现有通道。

```text
业务模块
  -> module.Contribution.Schedules
  -> internal/composition 聚合与校验
  -> Application Generation Schedule Component（gocron Adapter）
  -> ScheduleHub Commit/Retire 准入
  -> coordination 执行权（仅分布式任务）
  -> Telemetry span
  -> execution.Access / OperationExecutor
  -> 业务任务函数
```

这条链路不引入第二个 Runtime、Supervisor、Health Registry、Execution Store 或隐式 Registry。

## 3. 依赖选择

### 3.1 触发引擎

选用 `github.com/go-co-op/gocron/v2`，固定到实施时复核的 v2 小版本。选择依据见 `R002`：

- 原生支持 cron；
- `DurationJob` 与 `WithIntervalFromCompletion` 可准确表达 fixedDelay；
- 支持 context、Scheduler Shutdown 和本地并发限制；
- 活跃维护、MIT 许可证、Go API 可被内部 Adapter 收敛。

不把 gocron 的 `Locker` 作为严格分布式单实例保证。其锁语义位于各实例独立计算的运行点，无法为漂移的 fixedDelay 序列提供统一领导者，也无法独立解决租约续租、失权后的准入关闭和 Generation 切换。

`robfig/cron` 不选为本次主引擎：它适合 cron 解析与触发，但 fixedDelay、统一任务治理和生命周期需要更多自研循环；并行引入两个引擎会形成双轨。

### 3.2 协调实现

继续使用现有 `github.com/redis/go-redis/v9` 连接，不新增 Redis 客户端依赖。Redis Adapter 按官方建议使用：

- `SET key token NX PX ttl` 获取执行权；
- 原子脚本比较 token 后续租；
- 原子脚本比较 token 后释放；
- 所有调用带 context 和明确超时。

严格语义的边界是：单 Redis 主节点/受支持托管端点、TTL 有效且只有当前 token owner 能续租或释放。Redis 故障转移窗口、长时间进程停顿和目标业务资源不识别 fencing token 时，不能据此宣称业务副作用 exactly-once。

## 4. 契约设计

### 4.1 `pkg/schedule`

新增项目自有调度声明与运行状态类型。建议核心形态如下，实施时可在不改变语义的前提下收敛命名：

```go
type TaskID string

type Binding struct {
    ID           TaskID
    Trigger      Trigger
    Concurrency  ConcurrencyPolicy
    Coordination CoordinationPolicy
    Execution    execution.PolicyID
    Run          Task
}

type Task func(context.Context) error
```

`Trigger` 使用封闭的专用 Kind 和对应值对象表达 `CronTrigger`/`FixedDelayTrigger`，由构造函数完成基本校验，避免以 `map[string]any` 或多个含混指针字段表达联合类型。

`ConcurrencyPolicy` 至少表达：

- 单任务最大并发；
- 上次未完成时 `skip` 或受控排队；
- 可接受的队列上限。

本次默认同一 Task ID 不重叠，且不提供无界排队。

`CoordinationPolicy` 至少表达：

- `local` 或 `distributedSingleton`；
- 协调不可用时的 `skip`/`pause`/`fail`/`local`；
- 严格模式和弱化模式的合法组合校验。

公开契约不暴露 gocron、Redis、Generation 或内部 Hub 类型。

### 4.2 `internal/module.Contribution`

扩展为：

```go
type Contribution struct {
    ID           string
    Participants []supervisor.Participant
    Schedules    []schedule.Binding
}
```

各模块的 `Module.Bind` 仍是唯一接入入口。composition 在模块装配完成后一次性复制、排序并验证声明，Task ID 冲突时返回带模块上下文且保留原因链的错误。

### 4.3 `pkg/coordination`

新增专用协调契约，避免污染 `cache.RemoteStore`：

```go
type Manager interface {
    Acquire(ctx context.Context, key Key, options LeaseOptions) (Lease, error)
}

type Lease interface {
    Token() Token
    Renew(ctx context.Context) error
    Release(ctx context.Context) error
}
```

实际接口会区分“未获得”“协调不可用”“已失权”“调用取消”等稳定错误，并支持 `errors.Is`/`errors.As`。Token 只用于内部所有权验证和低敏关联，不进入日志或公共诊断。

协调键由固定 namespace、应用身份和规范化 Task ID 派生；原始键不进入日志。Generation ID 不进入键，避免热重载期间旧、新候选变成两个独立 owner。

## 5. 调度 Component 与 Generation 生命周期

### 5.1 组件位置

新增 `internal/kernel/app/schedule`，遵循现有应用能力的 Definition/Dependencies/Config/Output/Access/Adapter 结构：

- Definition 构建不可变任务集合和暂停状态的 gocron Scheduler；
- Dependencies 只接收项目自有 Logger、Clock、Execution、Telemetry、Coordination 和进程稳定 Hub；
- Output 仅向 composition 提供受控 Candidate/Diagnostics，不向业务模块暴露 scheduler；
- Stop 关闭调度器、取消在途任务、等待有界收敛并汇总释放错误。

底层 gocron 类型只留在该包内部 Adapter 文件。

### 5.2 `ScheduleHub`

`ScheduleHub` 由 `applicationGenerationFactory` 创建并在进程生命周期内稳定存在，其职责只包括：

- 保存当前已提交候选；
- 在 Commit 时先关闭旧候选的新准入，再原子切换并开放新候选；
- 维护跨 Generation 的 Task ID admission，防止同一进程旧、新任务重叠；
- 为诊断提供当前 Generation 的只读快照；
- 在 factory Stop 时终止当前候选。

Hub 不计算 cron、不执行业务函数、不负责 Redis 续租，也不复制 Supervisor。

### 5.3 Prepare/Commit/Retire

```text
Prepare
  1. Module.Bind 完成，聚合 Schedule Binding
  2. 校验 Task ID、Trigger、策略和配置覆盖
  3. 构建暂停状态的 gocron Scheduler
  4. 建立任务运行器、诊断状态和 monitor
  5. 禁止触发业务任务

Commit
  1. Hub 关闭旧 Candidate 新准入
  2. Hub 切换 current Generation
  3. 新 Candidate 开始争取必要执行权
  4. 只有取得资格的任务开放调度触发

Retire/Abort/Stop
  1. 关闭新触发准入
  2. 取消任务与争抢/续租循环
  3. 有界等待在途任务
  4. token 校验后释放租约
  5. 汇总主要错误与清理错误
```

普通 `supervisor.Participant` 会在当前 Prepare 中被启动，因此 Schedule Component 不能直接作为普通 Participant 注册；否则候选 Commit 前可能执行业务。它由 Application Generation 显式持有，并把终态故障发送到现有 generation monitor。GenerationCoordinator.Monitor 和 process Supervisor 继续负责最终失败策略。

## 6. 触发语义

### 6.1 cron

- Adapter 使用 gocron `CronJob`；支持秒字段与标准字段的选择由项目配置明确。
- 时区在 Prepare 时解析为 `time.Location`。
- 每次回调接收计划触发时刻，并以 `TaskID + scheduledAt.UTC()` 生成 occurrence key。
- 本地跳过或协调不可用时仍保留该 occurrence 的诊断结果，但不伪造业务 Execution 成功记录。

### 6.2 fixedDelay

- Adapter 使用 `DurationJob` 和 `WithIntervalFromCompletion`。
- 初始延迟由项目 Trigger 值对象明确表达。
- 完成包括成功、最终失败、超时或取消后的运行器收敛；下一次只在完成后计算。
- `distributedSingleton` 的调度器只有当前领导者激活。非领导实例不各自推进 fixedDelay 序列，避免实例时钟漂移产生不一致“周期”。

### 6.3 本地并发

- Task ID 级 admission 和全局有界 semaphore 位于项目运行器，而非业务函数。
- 默认同 Task ID 单并发，拥塞时跳过并记录稳定原因。
- 若后续确有排队需求，只允许显式有界队列；本次不设计持久化积压。
- 不同 Task ID 使用独立状态和策略，不共享无语义全局互斥锁。

## 7. 分布式执行权状态机

每个分布式 Task ID 独立运行以下状态机：

```text
contending -> leader -> renewing
     |          |          |
     |          +--失权----+
     |                     v
     +----不可用------> policy(skip/pause/fail/local)
                              |
                              +--依赖恢复--> contending
```

### 7.1 取得与续租

- 取得成功后才激活该任务的触发器。
- 续租循环由 Candidate 拥有，使用有界 jitter backoff，且可被 context 取消。
- 单次续租失败先依据剩余 TTL 判断是否仍能安全确认所有权；无法确认时立即失权，不能乐观继续触发。
- Release 失败不允许旧实例继续运行；错误进入低敏诊断和清理错误汇总，由 TTL 最终回收所有权。

### 7.2 不可用策略

| 策略 | 新执行准入 | Readiness | 自动恢复 | 适用范围 |
|---|---|---|---|---|
| `skip` | 关闭并跳过受影响周期 | 保持 ready，诊断 degraded | 是 | 可容忍漏跑 |
| `pause` | 关闭 | not ready | 是 | 需要人工流量隔离但不退出 |
| `fail` | 关闭并进入 fatal path | 由现有生命周期决定 | 否，由 Supervisor 策略处理 | 无法继续安全服务 |
| `local` | 按本地策略开放 | ready，明确 weakened | 是 | 仅显式 best-effort |

严格 `distributedSingleton` 只允许 `skip`、`pause`、`fail`。配置成 `local` 必须在 Prepare 失败。

## 8. 现有能力的合并与最小增强

### 8.1 Cache 与 Coordination

当前 Cache `RemoteStore` 只表达普通缓存操作，不能安全承载租约。本次：

- 保留 `pkg/cache` 的现有业务缓存契约；
- 新增 `pkg/coordination` 专用契约；
- 在 `internal/kernel/app/cache` 的 Redis 资源 owner 内创建 coordination Adapter，共享同一 go-redis Client、配置、关闭和低层健康探测；
- 禁止 scheduler 自行 `redis.NewClient`。

当前 Cache `Ready` 在 Redis 暂时不可用时会让整个 Generation 构建失败，这与每任务 `skip`/`pause` 恢复策略冲突。计划把 Ready 收敛为结构/配置就绪，把运行时连通性作为受控 Health/Coordination 状态暴露；Scheduler 在 Prepare 做策略感知的初始探测，运行期持续恢复。该调整只改变运行时可用性判定，不允许缓存调用静默成功或回退内存。

### 8.2 Execution

调度运行器构建：

- 稳定 `execution.Key`：由 Task ID、occurrence key 和明确 operation name 生成；
- task timeout/retry/record policy：引用现有 Execution Policy ID，不复制配置；
- trace identity：从 Telemetry 创建的后台 span context 注入；
- 最终结果：由 Execution Store 记录。

实施时同步修复当前 Execution `WrapBackend`/`WrapRetryExhausted` 使用 `%v` 导致错误链断裂的问题，并补全 Execution 记录 TTL 的真实策略来源；不保留旧包装或隐藏默认值。

### 8.3 Telemetry

扩展项目自有 Telemetry facade，增加后台 work span 的最小能力。SDK 类型和 provider 继续留在现有 Adapter，scheduler 只依赖项目自有 span/trace 契约。HTTP middleware 与后台任务共享同一 provider 和 shutdown owner。

### 8.4 Health、Diagnostics 与 Logging

- 扩展现有项目诊断快照和 ops 输出，不新增端点或 Registry；
- ScheduleHub 暴露只读聚合状态，任务内部状态通过同步快照复制；
- 使用现有 Logger，字段限定为 `owner`、`phase`、`generation`、Task ID、trigger kind、coordination state、result code、trace ID 等稳定低敏值；
- 下层只返回带类型错误，最终策略边界记录一次日志，避免重复打印。

## 9. 配置设计

配置仍走现有 schema、sources、decode、validate 和 Generation reload。目标结构示意：

```yaml
scheduler:
  enabled: true
  timezone: Asia/Shanghai
  max_concurrency: 32
  shutdown_timeout: 30s
  coordination:
    lease_ttl: 30s
    renew_interval: 10s
    retry_min: 500ms
    retry_max: 10s
  tasks:
    module.task-id:
      enabled: true
      unavailable_policy: pause
```

约束：

- `renew_interval < lease_ttl`，并为网络延迟和进程调度保留安全余量；
- retry min/max、并发、TTL、超时均要求正值和合理上限；
- Task ID 配置必须能匹配已声明 Binding，未知配置键在校验阶段失败；
- 业务模块声明的触发语义是代码契约，配置只做已设计的环境覆盖，不能动态注入任意业务函数；
- scheduler disabled 时所有任务保持显式 disabled 诊断，不进行部分启动。

具体配置键名在实施时与现有 schema 命名规则对齐；一旦确定只保留一组当前键，不添加兼容别名。

## 10. 错误与失败语义

| 失败点 | 行为 | 现有机制 |
|---|---|---|
| Binding/配置/cron 校验失败 | Candidate Prepare 失败，保留旧 Generation | GenerationCoordinator |
| gocron 构建失败 | Candidate Prepare 失败 | Application Generation |
| 任务业务最终失败 | Execution 记录最终失败，按任务策略继续后续周期 | Execution + Logger/Tracing |
| 无法取得执行权 | 不视为错误；保持 contender | Coordination state |
| 协调不可用 | 按每任务 skip/pause/fail/local | Health + monitor |
| 租约丢失 | 关闭准入、取消在途、重新争抢或 fail | Candidate owner |
| Scheduler 意外终止 | Generation fatal channel | monitor + Supervisor |
| Retire 清理失败 | 保留主要错误和清理错误 | errors.Join |

取消、正常关停和未取得执行权不记录为 Error。错误字符串进入日志前必须转换为稳定分类，原始原因链只向上返回。

## 11. 预计文件影响

### 新增

- `pkg/schedule/**`：项目自有声明、策略、状态与错误契约。
- `pkg/coordination/**`：项目自有租约契约与稳定错误。
- `internal/kernel/app/schedule/**`：gocron Adapter、Definition、Candidate、运行器、状态和测试。
- `internal/kernel/app/cache/*coordination*`：共享 Redis Client 的租约实现与测试。

### 修改

- `internal/module/contracts.go` 及模块装配测试：增加 Schedule Binding。
- `internal/composition/generation.go`、`generation_coordinator.go`、`service.go` 及测试：ScheduleHub 与 Generation 生命周期。
- `internal/kernel/composition/**`、`internal/kernel/config/**`：调度强类型配置、schema、默认值与校验。
- `pkg/execution/**`、`internal/kernel/app/execution/**`：调度调用接入、错误链和记录 TTL。
- `pkg/observability/**`、`internal/kernel/app/observability/**`：后台 span facade。
- `internal/kernel/ops/**`、`pkg/health/**` 的现有接入点：调度状态映射；不新增平行体系。
- `README.md`、`docs/README.md`、相关 architecture/development/config 文档和 `pkg/README.md`：同步当前能力和使用路径。
- `go.mod`、`go.sum`：只增加经确认的 gocron 依赖及必要间接依赖。

实施前会按实际引用再次核对清单；若发现需要改变公共接口、依赖选择、模块边界或外部副作用，将退回研究/计划并重新确认。

## 12. 验证设计

### 12.1 单元测试

- Trigger、Task ID、策略组合、配置和 occurrence key 校验；
- fixedDelay 完成后计时、cron 时区与表达式；
- 单任务/全局并发和有界拥塞；
- Redis token acquire/renew/release、错误分类、TTL 安全窗口；
- skip/pause/fail/local 状态机和自动恢复；
- Execution error chain、policy、trace identity 和 record TTL；
- Diagnostics 快照并发安全和低敏输出。

时间相关测试优先使用 gocron Clock 注入或项目 fake clock，协调测试使用可控 Redis 测试端点和故障注入，禁止依赖长 sleep。

### 12.2 集成测试

- 两个模块的显式 Binding 聚合与冲突拒绝；
- 两个逻辑实例争抢同一 Task ID，单周期只有一个执行者；
- 协调中断、失权、恢复后无需重启继续参与；
- Candidate Prepare 不触发，Commit 后触发，reload 时旧、新 Generation 不重叠；
- Abort/Retire/Stop 取消、等待、释放和错误汇总；
- fatal scheduler failure 进入现有 monitor/Supervisor。

### 12.3 仓库级检查

- `gofmt`、`go test ./...`、`go vet ./...`；
- 对涉及并发的包执行 `go test -race`；
- 执行仓库已有架构、文档、生成一致性和治理脚本；
- 搜索底层类型泄漏、旧符号/配置残留、直接 Redis client 创建和隐式注册；
- `git diff --check` 并审阅完整 staged diff。

## 13. 发布与迁移

这是新增能力，没有业务任务和数据迁移。默认配置关闭调度能力，只有模块存在 Binding 且部署配置显式启用后才运行。启用严格分布式任务前，部署环境必须提供受支持的 Redis 端点和完成依赖恢复演练。

本次只实现一套当前契约，不保留实验别名或旧配置。若上线后需要多主一致性或 fencing-aware 业务协议，应新建研究和 ADR，不在本次实现中伪装为已经解决。
