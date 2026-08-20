# R003 消息系统适配单轨综合结论

## 1. 结论摘要

单轨目标结构为：

```text
应用共享或模块自有 Message Contract
  -> module Producer/Consumer Binding
  -> module.Contribution.Messages
  -> internal/composition aggregate + validate
  -> logical Route -> named Provider capability match
  -> generation-owned messaging resource + Publisher
  -> process-stable ConsumerHub Commit/Retire admission
  -> RabbitMQ confirm / manual ack / delayed reject-or-nack / delivery-limit / DLX
  -> Telemetry Observe
  -> Execution lease + single-delivery idempotency + release/complete/record
  -> module Handler
```

`GenerationCoordinator` 继续是代际 owner，`Supervisor` 继续是进程 owner，Ops 继续是 health/diagnostics 出口。
RabbitMQ Client、connection、channel、Delivery 与 topology 类型只存在于 `internal/kernel/app/messaging/rabbitmq`。

## 2. 公共语义与 Provider 差异

公共层只统一业务实际需要且各 Provider 可验证的语义：

- 稳定 Contract ID + schema version + content type；
- 稳定 Producer ID、Consumer ID 与逻辑 Route ID；
- Message ID、发生时间、partition/ordering key、correlation/causation/trace identity 和 payload；
- 发布是否需要 Broker 接管确认、无法路由是否必须失败；
- 消费至少一次、最大 delivery、非重试错误、死信要求、并发与重要性；
- typed success/retry/dead-letter/unavailable/cancelled 结果。

Provider 保留并显式校验自己的差异：RabbitMQ exchange/queue/routing key/quorum/DLX，Kafka topic/partition/group/offset，
NATS stream/subject/durable consumer/AckWait。不能通过空字段或 `map[string]any` 抹平差异；Route 配置使用封闭 driver union。

## 3. Contract 与 Binding authority

- `pkg/messaging` 只定义通用类型、构造器、校验与稳定 Publisher/Handler 契约，不放具体业务消息。
- 单模块私有消息 schema 放在 `internal/module/<owner>/binding/message`。
- 被多个模块共同消费的 wire contract 放在应用级稳定 contract 包，具体目录作为本计划新增边界在实施时按
  `internal/contract/message/<domain>` 落地；它只能依赖标准库和 `pkg/messaging`，模块可依赖它，contract 不依赖模块。
- `module.Contribution.Messages` 是唯一聚合入口；Contract identity 冲突、Producer/Consumer ID 冲突、未知 Contract、
  未绑定 Route 或 Provider 能力不足都在候选 Prepare 失败。

## 4. Producer 与 Consumer 分离

### 4.1 Producer

每个 Generation 拥有自己的 Provider resource 和 Publisher facade。HTTP/CLI 业务模块依赖自身定义的窄发布 port，
模块顶层 Adapter 调用项目 Publisher；业务从不拿原生 Client 或 Close。

`Publish` 成功只表示所选 Provider 已确认接管。断连、timeout、negative confirm、unroutable 或 generation retire 返回
可识别错误；不使用易失内存队列返回假成功。Outbox 不在首版，业务数据库提交与 Publish 不具有原子性。

### 4.2 Consumer

Consumer Binding 只包含 Contract 引用、Handler、逻辑 Route、并发/交付/重要性策略。候选可以构造连接和校验配置，
但在 Commit 前不得开始拉取/投递。

进程稳定 `ConsumerHub` 在 Commit 时：

1. quiesce 旧 Consumer，停止取得新 delivery；
2. 在有界 handoff 内等待旧 in-flight 收敛；超时则取消并关闭旧 channel，让 Broker 重投；
3. 激活新候选 Consumer；
4. Retire/Abort 只处理属于对应候选的资源，不影响新代。

Consumer Hub 不拥有业务逻辑、Broker 重试或 Execution Store，只拥有代际 admission/handoff。

## 5. 可靠性职责矩阵

| 责任 | 唯一 owner | 首版行为 |
| --- | --- | --- |
| Publisher 接管确认 | RabbitMQ Provider | persistent publish + mandatory + confirm；失败向上返回 |
| delivery 分发/背压 | RabbitMQ Provider | manual ack + bounded prefetch/concurrency |
| 跨 delivery 重投 | Broker/Route Policy | quorum delayed retry + delivery-limit/DLX；不在应用复制持久重试队列 |
| 重试错误分类 | 项目 fault/Binding Policy | retryable 才允许 redelivery，permanent/invalid 进入 dead-letter |
| 单 delivery 幂等/记录 | Execution | key=`consumerID:contract:messageID`，独立 lease/retention，一次业务尝试，失败释放后记录 |
| ack | Consumer runtime | Execution 成功或 duplicate-completed 后 ack；ack 失败保留歧义并允许重投 |
| dead-letter | Broker topology | 达上限或 permanent error 后 reject without requeue；必须验证 DLX |
| 业务副作用 exactly-once | 不在首版 | 需要业务资源幂等或后续 Outbox/Inbox/事务协作 |

默认禁止把 Execution 的进程内多次 retry 与 Broker redelivery 叠加。038 先把 Execution 当前共用的 `ClaimTTL` 单轨
替换为 running `LeaseTTL` 与 completed `RetentionTTL`，并增加同步 `Release`；失败 release 后仍用既有 `Record`。
Consumer runtime 给 Execution 提交显式单次策略；RabbitMQ 4.3 对 retryable business failure 使用 counted
`basic.reject(requeue=true)`，对 Execution backend unavailable/active lease 使用 non-counting
`basic.nack(requeue=true)`，两者都由 quorum delayed retry Policy 延后。未来若需要进程内立即 retry，必须形成可计算
总尝试上限的新计划。

## 6. 暂时不可用与恢复

每条 Route 声明 `required` 或 `optional`：

- `required`：进程保持 live，消息 readiness 为 fail；Publisher 返回 `ErrUnavailable`，Consumer 停止取新消息。
- `optional`：进程与总体 readiness 可保持，消息 health 为 warn/degraded；Publisher仍返回 `ErrUnavailable`，
  是否忽略由业务决策边界显式处理，底层不静默丢弃。

Provider 对网络/节点暂时故障进入 Recovering，以有超时、退避、抖动、最大探测频率和 context 停止的循环自动恢复。
配置、认证、Contract 冲突、unsupported capability 或 topology mismatch 等确定性错误使候选失败；恢复循环异常终止才进入
现有 Generation monitor/Supervisor terminal path。

## 7. 配置与多 Provider

`messaging` 配置包含：enabled、shutdown/handoff/publish confirm timeout、命名 providers、逻辑 routes 与 driver-specific
封闭配置。RabbitMQ reliable Route 要求 quorum、`delayed-retry-type=all`、正数 min/max、正数 delivery-limit、DLX 与
at-least-once dead-letter strategy 的运维验收。业务 Binding 只引用逻辑 Route ID；Route 指向一个 Provider，不自动多写。

`internal/composition` 显式提交 `[]ProviderFactory`。首版只有 `rabbitmq` 与测试 fake；配置允许多个 RabbitMQ Provider
同时存在并由不同 Route 使用，以证明并存。未来新增 Kafka/NATS Factory 时不改业务 Contract，但必须扩展 driver union、
capability validation、运维文档和真实集成测试。

## 8. 研究门禁判定

关键边界、首个依赖、确认/重投/死信语义、暂时故障、代际准入、现有能力复用和 exactly-once 限制均已有可复核证据。
剩余未知可由确认后的实现/集成测试验证，不阻塞形成计划，因此研究门禁通过。

037 已形成可追溯的本地提交 `00083be`，但实施门禁仍未通过：用户尚未在本计划报告后的后续消息中
确认，实施开始时也仍须复核 HEAD、工作区与 037 验证证据。
