# 消息系统适配能力设计

## 1. 文档状态

- 变更编号：`038`
- 当前阶段：研究门禁已通过，计划已确认
- 实施状态：实现与本地工程门禁完成；RabbitMQ 4.3 真实协议门禁受环境阻断
- 需求依据：[requirements.md](requirements.md)
- 研究依据：`R001`、`R002`、`R003`

本文记录本变更采用并已落地的设计，具体 API 仍以源码和当前主题文档为准。未执行的 Broker 协议结果不因设计描述而
自动成立；公共契约、依赖选择、模块边界、可靠性或外部副作用后续发生实质变化时必须建立新变更。

## 2. 总体结构

```text
internal/contract/message/<domain>（仅真实共享 wire contract 时创建）
                 |                         internal/module/<name>/binding/message
                 +------------------------------+
                                                v
                                      pkg/messaging Contract/Binding
                                                |
                                      module.Contribution.Messages
                                                |
                                      internal/composition aggregate
                                                |
              +---------------------------------+-----------------------------+
              |                                                               |
              v                                                               v
 generation-owned messaging Output                                  process-stable ConsumerHub
 Publisher + candidate Control                                      Commit/Retire admission
              |                                                               |
              +-------------------- named Provider/Route ---------------------+
                                      |
                         rabbitmq Adapter (amqp091-go)
                                      |
                  confirm / manual ack / redelivery / DLX
                                      |
                   Telemetry -> Execution -> module Handler
```

Provider resource、Publisher 与 Consumer candidate 属 Generation；ConsumerHub 只负责当前代准入。
业务模块只看到 `pkg/messaging` 或自身窄 port。

## 3. `pkg/messaging` 公共契约

### 3.1 Identity 与 Envelope

目标核心类型：

```go
type ContractID string
type ProducerID string
type ConsumerID string
type RouteID string
type MessageID string
type SchemaVersion uint32

type Contract struct { /* immutable descriptor */ }

type Message struct {
    ID            MessageID
    Contract      ContractRef
    OccurredAt    time.Time
    OrderingKey   string
    CorrelationID string
    CausationID   string
    Payload       []byte
}
```

字段通过构造器生成不可变副本。`Payload` 有集中上限；公共 Envelope 不接受任意 `map[string]any`。Trace propagation
由 Adapter 与 Telemetry 使用项目固定 header 名处理，不把未经审查的 Broker headers 交给业务或日志。

`Contract` 只描述 wire identity/version/content type/size；真实 Go payload、`Encode`/`Decode` 位于模块 binding 或应用
共享 contract 包。业务 Service 使用 typed command/event，不直接拼 bytes。

### 3.2 Producer/Consumer Binding

目标形态：

```go
type ProducerSpec struct {
    ID       ProducerID
    Contract ContractRef
    Route    RouteID
    Confirm  ConfirmRequirement
}

type ConsumerSpec struct {
    ID          ConsumerID
    Contract    ContractRef
    Route       RouteID
    Delivery    DeliveryPolicy
    Concurrency ConcurrencyPolicy
    Importance  Importance
}

type Handler func(context.Context, Message) error
```

构造函数 `BindProducer`/`BindConsumer` 完整校验并返回 immutable Binding。Consumer binding 内的 Handler 通常是
模块 binding Adapter：先验证 Contract/Decode，再调用 typed Service Handler。

### 3.3 Publisher 与 Receipt

```go
type Publisher interface {
    Publish(context.Context, ProducerID, Message) (Receipt, error)
}
```

Receipt 只包含项目稳定、低敏字段：Message ID、Producer ID、confirmed 时间与可选 opaque safe reference；Provider
kind/name 只进入受控 diagnostics，不返回业务调用方。Receipt 不暴露 AMQP confirmation、channel、exchange 或原始
Broker response。

稳定错误至少包括：`ErrInvalidMessage`、`ErrContractMismatch`、`ErrUnknownProducer`、`ErrUnavailable`、
`ErrUnroutable`、`ErrPublishRejected`、`ErrPublishAmbiguous`、`ErrNotActive`、`ErrRetired`、
`ErrDeliveryExhausted`、`ErrProviderCapability`。
Adapter 添加上下文时用 `%w`/`errors.Join` 保留取消、timeout 与原生原因链，但对外日志只记录稳定分类。

## 4. 模块与 Contract 边界

`module.Contribution` 目标扩展为：

```go
type Contribution struct {
    ID           ID
    Participants []supervisor.Participant
    Schedules    []schedule.Binding
    Messages     messaging.Contribution
}
```

`messaging.Contribution` 分别保存 Contracts、Producers、Consumers，避免用一个含混切片和运行时类型断言。
`module.MessageBindings` 复制、排序并验证：

- Contract ID/version 唯一；同 identity 的 descriptor/fingerprint 不得冲突；
- Producer ID、Consumer ID 全局唯一；
- Producer/Consumer 引用已声明 Contract；
- Route ID 与所有策略合法；
- nil Handler、重复 Handler owner、无 producer/consumer 的悬空声明按明确规则处理。

单模块 contract 位于该模块 `binding/message`。只有确有跨模块共同 wire authority 时才创建
`internal/contract/message/<domain>`；038 不创建空目录或示例业务消息。

## 5. Kernel App 组件

### 5.1 组件形态

新增 `internal/kernel/app/messaging`，ID=`messaging`、ConfigPath=`messaging`，采用 Configured Leased、
`KernelInstanceSwap`：新旧 Provider connection 可在 Generation 排空期间并存，借用者无 Close 权。

依赖：Logger、Clock、ID Generator、Execution、Telemetry、Generation ID、failure reporter 与显式 ProviderFactories。

组件 Output 分离：

```go
type Output struct {
    Publisher messaging.Publisher // 可向模块收窄注入
    Control   Control             // 只给 internal/composition
}
```

Control 是 candidate-local、只允许一次 Freeze/Bind 的显式接口：模块构造完成后，composition 把聚合 catalog 提交给
Control；Control 校验 Route/Provider capability 并准备 Consumer，但不启动消费。它不是全局 Registry，也不能在 Commit
后追加 Binding。

该两阶段解决真实构造顺序：Provider/Publisher 先构造供模块注入，模块返回 Contribution 后再冻结 Consumer catalog。
模块构造期间调用 Publisher 返回 `ErrNotActive`，防止构造副作用。

### 5.2 Provider Factory

Factory 与 Provider 只在 `internal/kernel/app/messaging` 内可见：

```go
type ProviderFactory interface {
    Kind() Driver
    Build(context.Context, ProviderConfig, ProviderDependencies) (Provider, error)
}
```

composition 显式提交 `rabbitmq.Factory()`；测试提交 fake。Factory kind 重复、未知 driver 或遗漏 Factory 在 Prepare 失败。
Provider 公开自身 capability descriptor，Control 用 Binding requirements 做 fail-closed 匹配。

一个配置可创建多个命名 Provider；Route 只指向一个 Provider。首版不提供 fanout/dual-write，因为跨 Provider 部分成功
需要独立事务与补偿语义。

## 6. RabbitMQ Adapter

目录目标：`internal/kernel/app/messaging/rabbitmq`。唯一第三方依赖为实施时复核后固定的
`github.com/rabbitmq/amqp091-go` v1.14.0。

### 6.1 资源所有权

- Provider resource 拥有 connection、publish channel、每个 Consumer channel、confirm/return listener、恢复状态与 goroutine。
- Publisher 借用 Provider，不关闭 Channel。
- 每个 delivery 只能在收到它的 Channel 上 ack/nack。
- Stop 先停止新 publish/consume，等待 in-flight confirm/handler，再关闭 channel/connection；清理错误与主错误合并。

### 6.2 发布路径

1. 校验 Producer/Message Contract 与 payload limit；
2. 获取 Route 与健康 Provider lease；
3. 注入受控 message/correlation/causation/trace headers；
4. 使用 persistent delivery、mandatory routing 和 context timeout 发布；
5. 等待对应 confirm，并同时处理 return、connection/channel state；
6. 只有确认且未被 mandatory return 才返回 Receipt。

Publisher confirm 追踪表有界。断连时未决发布全部以 typed ambiguous/unavailable error 完成；不自动重发，因为 Broker
可能已接管但 confirm 丢失，调用方必须基于 Message ID/业务幂等决定重试。

### 6.3 消费路径

每个 Consumer 使用 manual ack 和有界 prefetch。处理顺序：

```text
delivery -> envelope/contract validation -> Telemetry Observe
         -> Execution(single attempt, stable key) -> module Handler
         -> success/duplicate: ack
         -> retryable business failure: reject(requeue=true, multiple=false)
              -> counted quorum delayed retry -> delivery-limit
         -> Execution unavailable/active lease: nack(requeue=true, multiple=false)
              -> non-counting quorum delayed retry
         -> permanent/invalid/panic/exhausted: reject(requeue=false) -> DLX
```

RabbitMQ 4.3 quorum queue 的 `delayed-retry-type=all`、正数 `delayed-retry-min/max`、正数 `delivery-limit`、
`dead-letter-strategy=at-least-once`、`overflow=reject-publish` 与 DLX Policy 作为首版可靠 Route 的生产前提。
`basic.reject` 增加 `delivery-count`，用于真实业务失败预算；`basic.nack` 不增加计数，只用于暂时基础设施延后。
Adapter 不硬编码通用 x-arguments；Prepare/Activate 只验证 AMQP 能证明的 connection/exchange/queue/routing 前提。
AMQP 无法读取 effective Policy，因而 delayed retry/DLX/delivery-limit 由独立运维 topology verifier 和真实消息
集成门禁证明；运行期 diagnostics 必须把这类保证标为外部已配置要求，不能伪造为每次启动已动态证明，也不能只相信
YAML 就宣称无损。

## 7. Execution 协作

Consumer runtime 构造：

```go
execution.Execution{
    Key:          execution.Key(consumerID + ":" + contract + ":" + messageID),
    Policy:       resilience.RetryPolicy{MaxAttempts: 1},
    Timeout:      delivery.HandlerTimeout,
    LeaseTTL:     delivery.ProcessingLease,
    RetentionTTL: delivery.IdempotencyRetention,
    Trigger:      "message." + string(consumerID),
    Operation:    handlerOperation,
}
```

现有 `Execution.ClaimTTL` 同时承担 running claim 和 completed retention，且失败 `Record` 不释放 claim，不能支撑同一
Message ID 的及时重投。038 先做单轨公共契约替换：

```go
type Execution struct {
    // 其余字段保持当前职责。
    LeaseTTL     time.Duration // running claim 的崩溃恢复期限
    RetentionTTL time.Duration // success 后从完成时起算的去重窗口
}

type Store interface {
    Claim(context.Context, Key, time.Duration, time.Time) (bool, error)
    IsCompleted(context.Context, Key) (bool, error)
    Complete(context.Context, Key, time.Duration, Record) error
    Release(context.Context, Key) error
    Record(context.Context, Key, Record) error
}
```

`ClaimTTL` 在同一变更中删除，Schedule 等所有调用方显式迁移，不留 alias。成功时 `Complete` 从完成时建立
`RetentionTTL`；失败时先同步 `Release`，再用既有 `Record` 路径记录。Release/Record 的错误用 `errors.Join` 保留，
Consumer 不 ack。`LeaseTTL` 必须大于 Handler timeout 与停止余量，用于进程/执行器消失后最终解锁；它不承担成功去重。

`MaxAttempts: 1` 避免落入 Execution App 默认三次策略。Broker delivery count 是跨交付重试 authority。Consumer 按优先级映射：

1. Completed 或 completed duplicate：ack；
2. `ErrBackend`、`ErrAlreadyRunning`、release/record failure：`basic.nack(requeue=true)`，不增加业务失败计数；
3. 可重试业务错误且未达上限：release 成功后 `basic.reject(requeue=true)`，增加 delivery count；
4. permanent、invalid、panic 或 exhausted：`basic.reject(requeue=false)` 进入 DLX。

Execution Health 非 Ready 时 ConsumerHub 取消/暂停对应 intake；只有健康变化竞态中已取得的 delivery 才走 non-counting
nack，避免基础设施整体故障持续刷消息。

当前 memory Store 在进程重启/跨实例时失效，文档必须要求业务副作用自身使用 Message ID 幂等。038 不扩建数据库
Execution Store，因为它仍不能自动把任意业务事务与 ack 原子化；这应由后续真实 Outbox/Inbox 用例驱动。

## 8. Generation 协议

### Prepare

1. 解码 messaging config，构造命名 Provider candidate；
2. 暂时网络不可达生成 Recovering candidate，不因 reachability 直接失败；本地配置/capability 错误 fail-fast，
   Broker 认证/拓扑拒绝进入可诊断的 non-retryable candidate failure；
3. 将 candidate Publisher 通过模块窄 Adapter 注入并构造模块；
4. 聚合/校验 module Messages，调用 Control Freeze；
5. 准备 Consumer subscription 元数据但不 `Consume`；证明零业务 delivery；
6. 与 HTTP、Schedule 等其余 candidate 一起完成 Ready。

### Commit

1. ConsumerHub quiesce 旧 current；
2. 等待 handoff timeout；未收敛 delivery 取消并通过 channel close 交还 Broker；
3. 发布新 Consumer candidate 为 current；若 Provider 在 Recovering，登记待恢复激活而不让 Commit 失败；
4. 提交 HTTP/management routes 和 Logger 的顺序在实现时用失败回滚测试固定，不能留下部分 commit。

### Retire/Abort/Stop

- Abort 未 commit candidate：从未消费，直接停止恢复循环与连接。
- Retire current：撤销 Consumer admission；HTTP 旧请求仍可使用本代 Publisher，直到 route drain 后释放资源。
- Stop/ForceStop：有界等待 confirm/handler；超时取消，关闭 channel 使未 ack delivery 可重投，保留清理错误。
- Provider recovery loop 非预期终止经既有 factory failure channel 进入 Generation Monitor/Supervisor。

## 9. 暂时不可用状态机

```text
Disabled -> Connecting -> Ready
               |          |
               v          v
            Recovering <- Degraded
               |
               v
             Ready

确定性错误 -> Failed(candidate/terminal)
Stop        -> Draining -> Stopped
```

每次连接/探测有 timeout；失败后指数退避 + jitter + 最大频率，循环受 resource context 和 WaitGroup 管理。
连接恢复不自动重放不确定发布；它只恢复后续 Publish 和 Consumer。

组件的 `WithReady` 只证明恢复循环和内部资源 owner 已建立，不把 Broker reachability 当成候选构造成功条件；
外部可用性由 messaging Health 聚合：任一 required Route 未 Ready => fail；仅 optional Route 未 Ready => warn；
全部可用或 disabled => pass。
进程 liveness 不因暂时断连失败。

## 10. 配置设计

目标结构（字段名可在实现中按 typed contract 收敛）：

```yaml
messaging:
  enabled: false
  publishConfirmTimeout: 5s
  handoffTimeout: 30s
  shutdownTimeout: 30s
  recovery:
    connectTimeout: 3s
    initialBackoff: 500ms
    maxBackoff: 30s
  providers: {}
  routes: {}
```

启用 RabbitMQ 时，`providers.<name>.driver=rabbitmq`，driver block 持有 URI/TLS/heartbeat/recovery；
`routes.<id>` 持有 provider、exchange/routing key、queue、prefetch、quorum/DLX requirements 与 importance。
reliable Route 还声明 handler timeout、processing lease、idempotency retention、delivery limit 与 delayed retry min/max
要求；`processingLease > handlerTimeout + shutdownMargin`。默认示例不含真实凭据；环境变量覆盖 Secret。未知
provider/route/字段、空 URI、无限 prefetch、非正数 delivery/retry 上限、required Policy 未声明等失败。

物理 topology 变化按 RabbitMQ 能力分类：可由新旧 connection 并存并安全 handoff 的走 Generation Swap；需要删除/重建
queue 或改变不可变属性的候选拒绝并给出 Restart/运维变更要求，不由应用破坏性修改。

## 11. 可观测与错误

Telemetry Work kind 使用 `message.publish`/`message.consume`，name 使用逻辑 Producer/Consumer ID，不记录物理 destination。
Execution trace 从 Observe context 自动记录。Metrics/diagnostics 使用有界 label：provider kind/name、route ID、consumer ID、
outcome；Message ID 不作为 metric label。

日志 owner：

- Provider state transition：messaging Provider；
- publish 最终失败策略：Publisher boundary；
- delivery disposition/耗尽：Consumer runtime；
- 业务错误决定：Handler/Service 或 Consumer disposition boundary 二选一，避免重复打印。

错误文本、AMQP URI、payload、headers、exchange/queue 与 credential 不进入日志；diagnostics 只保留 stable error type。

## 12. 目标文件影响

新增：

- `pkg/messaging/**`
- `internal/kernel/app/messaging/**`
- `internal/kernel/app/messaging/rabbitmq/**`
- `docs/development/messaging-capability.md`
- `docs/operations/messaging.md`
- RabbitMQ opt-in integration verifier/test assets

修改：

- `pkg/execution` Store/Execution、Memory/Recovery/Async wrapper、测试与当前主题文档
- `internal/kernel/app/schedule` 等全部 `ClaimTTL` 调用方及测试（单轨迁移）
- `internal/module/contracts.go` 及测试
- `internal/composition/generation.go`、`generation_resources.go`、`ops.go` 及测试
- `internal/kernel/composition/bootstrap.go`、`composition.go` 与架构测试
- `internal/kernel/app/README.md`、`internal/module/README.md`、`pkg/README.md`
- 根/文档/配置/运维索引与 `config.example.yaml`
- `go.mod`、`go.sum`
- Ops runtime model 与 observability metrics/diagnostics

不修改具体业务模块消息逻辑，不创建空 shared contract 包。

## 13. 验证方案

1. Execution 契约单测：lease/retention 分离、完成时起算、失败 release、release/record 复合错误、现有调用方迁移。
2. 消息纯契约/边界单测：immutable、ID/version、冲突、capability、错误链、无第三方泄漏。
3. fake Provider 状态机：Prepare 零消费、Commit、handoff、Abort、Retire、Stop、两个 Provider 并存、恢复。
4. concurrency/race：confirm 表、prefetch、in-flight handler、Hub 切换、recovery stop。
5. 真实 RabbitMQ 4.3：confirm/unroutable、manual ack、counted delayed reject、non-counting delayed nack、
   delivery-limit/at-least-once DLX、Broker down/up 自动恢复。
6. Application Generation：候选失败保留旧代、required/optional readiness、HTTP old-generation Publisher 排空。
7. 工程门禁：format、unit、race、vet、质量/架构脚本、dependency/license/security 检查与 Diff 审阅。

## 14. 决策边界

以下任一变化必须更新研究/计划并重新确认：首个 Provider 改变、采用 Watermill Router、加入 Outbox/Inbox、
增加具体业务消息、允许跨 Provider 双写、改变 module/contract 依赖方向、改变 required/optional 语义、引入数据库迁移
或生产 topology 写操作。Execution 的 lease/retention/release 单轨替换已经写入当前待确认计划；若实现发现需要
持久 Store、fencing token、lease renewal 或跨实例保证，也必须另行研究并重新确认。
