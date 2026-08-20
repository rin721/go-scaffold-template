# 业务模块接入消息系统适配能力

> 权威文档：本文是业务模块声明、生产和消费消息的唯一现行开发入口。公共契约以
> `pkg/messaging` 为准，运行装配以 `internal/kernel/app/messaging` 与 `internal/composition` 为准。

## 1. 接入顺序

业务模块接入消息能力时固定完成四件事：

1. 定义稳定的 wire Contract，并在模块边界完成 typed payload 与 `[]byte` 的转换；
2. 声明 Producer Binding、Consumer Binding 和各自需要的治理策略；
3. 通过 `module.Contribution.Messages` 显式输出声明；
4. 由 `internal/composition` 聚合 Catalog、解析逻辑 Route、选择 Provider，并只向模块注入需要的窄 Publisher port。

模块不得导入 `amqp091-go`、创建 Broker connection/channel、读取物理 exchange/queue、调用 `Consume`，也不得通过
`init`、扫描或全局 Registry 注册消息关系。当前默认没有业务消息声明，`messaging.enabled` 为 `false`，因此本地启动
不依赖 Broker。

## 2. 声明 Contract 与 Binding

以下代码只展示公共契约形态；Contract ID、schema、payload 和 Handler 必须由真实业务模块拥有：

```go
contract, err := messaging.DefineContract(messaging.ContractSpec{
    ID:              "orders.created",
    Version:         1,
    ContentType:     "application/json",
    MaxPayloadBytes: 64 << 10,
    Fingerprint:     "sha256:<canonical-schema-fingerprint>",
})

producer, err := messaging.BindProducer(messaging.ProducerSpec{
    ID:       "orders.writer",
    Contract: contract.Ref(),
    Route:    "orders.events",
    Confirm:  messaging.ConfirmBroker,
})

delivery, err := messaging.NewDeliveryPolicy(
    3,              // 包含首次投递在内，最多三次 delivery
    5*time.Second,  // 单次 Handler timeout
    15*time.Second, // Execution running lease，必须大于 Handler timeout
    24*time.Hour,   // 成功后的 Message ID 去重窗口
    messaging.DeadLetterRequired,
)
concurrency, err := messaging.NewConcurrencyPolicy(4, 8)
consumer, err := messaging.BindConsumer(messaging.ConsumerSpec{
    ID:          "orders.projector",
    Contract:    contract.Ref(),
    Route:       "orders.events",
    Delivery:    delivery,
    Concurrency: concurrency,
    Importance:  messaging.ImportanceRequired,
}, handleOrderCreated)

contribution := module.Contribution{
    ID: moduleID,
    Messages: messaging.Contribute(
        []messaging.Contract{contract},
        []messaging.ProducerBinding{producer},
        []messaging.ConsumerBinding{consumer},
    ),
}
```

构造器会复制可变输入并验证 identity、版本、大小、超时、并发和 Handler。Catalog 在 composition 中统一检查跨模块
Contract 冲突、Binding 重复、未知引用和未使用 Contract；声明不会自行打开网络资源。

模块 Handler 负责 decode、schema/业务输入校验和调用自身 Service，不负责 ack、Broker 重投、Execution claim、
Telemetry 或连接恢复。可重试业务错误沿用 `pkg/fault` 分类；永久错误直接进入终止处置。

## 3. 生产消息

业务模块只获得 `pkg/messaging.Publisher` 或自己定义的更窄 port。`internal/composition` 是唯一可以把 Generation 的
Publisher 适配并注入模块的位置。构造消息时保持稳定 Message ID；同一逻辑消息的重发不得生成新 ID：

```go
message, err := messaging.NewMessage(messaging.MessageSpec{
    ID:         messaging.MessageID(messageID),
    Contract:   contract.Ref(),
    OccurredAt: clock.Now(),
    Payload:    encodedPayload,
})
receipt, err := publisher.Publish(ctx, "orders.writer", message)
```

`Publish` 成功只证明所选 Broker 已确认接管，不证明 Consumer 已完成，更不证明业务副作用 exactly-once。RabbitMQ
Provider 使用 persistent message、mandatory routing 和 publisher confirm：unroutable、negative confirm、超时或断连
分别返回稳定错误；confirm 结果不确定时返回 `ErrPublishAmbiguous`，不会自动重发、缓存后伪造成功或静默切换 Provider。

## 4. 消费可靠性闭环

统一 Consumer runtime 把 Broker、Execution 和业务 Handler 的职责分开：

| 结果 | Execution / Broker 行为 |
| --- | --- |
| 成功或 completed duplicate | `ack`；相同 Message ID 不重复执行 Handler |
| Execution backend 不可用、已有 running lease、上游 delivery 被取消 | `nack(requeue=true)`，基础设施延后不消耗业务投递预算 |
| 可重试业务错误或 Handler 自身超时，且预算未耗尽 | `reject(requeue=true)`，计入 RabbitMQ delivery count |
| 永久错误、panic、无效 Envelope 或预算耗尽 | `reject(requeue=false)`，交给已验证的 DLX |

每次 delivery 以 `message:<ConsumerID>:<ContractRef>:<MessageID>` 调用 Execution，保证不同 Consumer 各自幂等；
运行时显式使用单次执行策略，避免一次 delivery 内 N 次重试、Broker
再重投 M 次形成 N×M。`LeaseTTL` 只负责 running claim 的崩溃恢复，`RetentionTTL` 从成功完成时建立去重窗口；失败会
同步释放 claim 并保留执行记录错误链。

RabbitMQ `delivery-limit` 表达重投次数，公共 `MaxDeliveries` 表达包含首次投递在内的总次数，因此可靠 Route 必须满足：

```text
route.deliveryLimit + 1 == consumer.delivery.MaxDeliveries
```

当前 Execution memory backend 只提供单进程去重，进程重启或多实例间不共享状态。涉及扣款、库存、外部调用等不可重复
副作用时，业务目标仍须以 Message ID 实现自身幂等。Outbox/Inbox、数据库事务与消息发布原子性、端到端 exactly-once
不属于当前能力，必须由真实事务用例另行设计。

## 5. Route 与 Provider 配置

逻辑 Route ID 属业务声明，物理 topology 只存在于应用配置。当前生产 composition 提供 RabbitMQ Factory；同一进程
可以配置多个命名 Provider，并让不同 Route 显式选择其中一个，但不会隐式双写或自动降级到另一 Provider。

```yaml
messaging:
  enabled: true
  publishConfirmTimeout: 5s
  handoffTimeout: 30s
  shutdownTimeout: 30s
  recovery:
    connectTimeout: 3s
    initialBackoff: 500ms
    maxBackoff: 30s
  providers:
    primary:
      driver: rabbitmq
      rabbitmq:
        uri: ""
        heartbeat: 10s
        tls:
          enabled: false
  routes:
    orders.events:
      provider: primary
      exchange: orders.events
      exchangeType: topic
      routingKey: orders.created
      queue: orders.created.v1
      queueType: quorum
      importance: required
      reliable: true
      deliveryLimit: 2
      delayedRetryMin: 1s
      delayedRetryMax: 1m
      deadLetterExchange: orders.dlx
      deadLetterRoutingKey: orders.created.dead
      atLeastOnceDeadLetter: true
```

URI 和凭据使用 `APP_MESSAGING__PROVIDERS__PRIMARY__RABBITMQ__URI` 等部署环境变量注入，不写入仓库。可靠 Route
要求 quorum queue、正数 delayed retry 范围、正数 delivery limit、at-least-once DLX 和 Provider 对应能力；缺失或与
Binding 策略冲突会在 Candidate Freeze 时失败。运行时只做 passive topology probe，不创建或修改生产 topology；Policy
必须由运维侧预置并验证。

## 6. 暂时不可用、健康与代际

- Broker 启动时不可达不会直接使进程 terminal；Provider 进入 `connecting/recovering`，按有界退避自动恢复。
- required Route 不可用时 messaging readiness 为 `fail`；只有 optional Route 不可用时为 `warn`。两者的发布失败都
  显式返回，optional 不代表丢消息降级。
- Execution health 失败时暂停 Consumer admission，恢复后自动重开；健康变化竞态中的 delivery 使用不计数延后。
- Candidate Freeze 和 Prepare 不消费消息。Commit 先撤销旧代 Consumer admission、排空后激活新代；候选激活失败会
  尝试恢复旧代。旧代 Publisher 保持到其 HTTP 请求排空，之后才随 Generation 关闭。
- Provider connection、恢复循环、Consumer channel、handler 和监控 goroutine 均由 Messaging Component 拥有并等待；
  模块没有关闭权。

运行诊断与故障处置见[消息系统运维](../operations/messaging.md)。

## 7. 边界清单

- 当前只实现 RabbitMQ 4.3 生产 Provider；Kafka、NATS JetStream 需要新增显式 Provider 和能力验证，不能从公共接口
  推断已经支持。
- 公共语义只统一 Broker 接管确认、至少一次交付、显式确认、有界重投、死信、重要性和并发要求；不会抹平各中间件
  的事务、顺序、分区、Consumer Group、Stream 等专属能力。
- Adapter 不承载 payload 业务含义、补偿、Saga、业务重试决策或数据库事务。
- 原始 payload、Broker URI、凭据、完整 destination 和原始错误文本不进入日志或公共诊断。
