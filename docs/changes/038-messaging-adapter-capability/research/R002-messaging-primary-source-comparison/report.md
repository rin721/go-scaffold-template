# R002 消息可靠性与 Provider 主源比较

## 1. 范围与来源

本记录比较公共消息适配所需的最小语义，不做“功能越多越好”的产品排名。证据来自 RabbitMQ 4.3 官方文档、
Apache Kafka 4.1 文档、NATS JetStream 官方文档、Watermill 官方文档与 RabbitMQ 官方 Go Client 源码/发布页，
验证日期为 2026-08-20。

主源链接：[RabbitMQ confirms](https://www.rabbitmq.com/docs/confirms)、
[RabbitMQ reliability](https://www.rabbitmq.com/docs/reliability)、
[RabbitMQ DLX](https://www.rabbitmq.com/docs/dlx)、
[RabbitMQ 4.3 quorum queue](https://www.rabbitmq.com/docs/next/quorum-queues)、
[RabbitMQ 4.3 release](https://www.rabbitmq.com/blog/2026/04/23/rabbitmq-4.3-release)、
[RabbitMQ Go Client v1.14.0](https://github.com/rabbitmq/amqp091-go/releases/tag/v1.14.0)、
[Kafka delivery semantics](https://kafka.apache.org/41/design/design/)、
[NATS JetStream delivery/ack](https://docs.nats.io/learn/jetstream/delivery-and-acknowledgment)、
[Watermill getting started](https://watermill.io/learn/getting-started/) 与
[Watermill middleware](https://watermill.io/docs/middlewares/)。

## 2. 主源事实

### 2.1 RabbitMQ

- Publisher confirms 与 Consumer acknowledgements 是相互独立的两个可靠性机制；confirm 只表示 Broker 已接管发布，
  ack 只表示 Consumer 已完成处理，不能把其中一个当成端到端完成证明。
- manual ack 下，连接或 channel 关闭时未确认 delivery 会自动 requeue；消费者必须按 at-least-once 设计并能处理重复。
- `basic.reject/basic.nack` 的 `requeue=true` 会重投；`false` 时只有配置了 DLX 才会死信，否则消息会被丢弃。
- bounded prefetch 限制未 ack 的 in-flight 数；无限 prefetch 会带来内存与过载风险。
- 反复立即 requeue 可能形成高成本热循环；RabbitMQ 官方明确要求跟踪 redelivery 或延迟 requeue。
- RabbitMQ 4.3 quorum queue 原生提供 linear-backoff delayed retry。`delayed-retry-type=all` 可同时延迟 counted failure
  与 non-counting return，`delayed-retry-min/max` 约束退避，不需要应用自建 TTL retry queue 或睡眠持有 delivery。
- RabbitMQ 4.3 中 AMQP 0-9-1 `basic.reject` 会增加 `delivery-count`，`basic.nack` 不增加。首版据此固定：可重试
  业务失败用 `basic.reject(requeue=true)`，计入 `delivery-limit`；Execution backend unavailable、
  `ErrAlreadyRunning` 等暂时基础设施阻塞用 `basic.nack(requeue=true)` 延后但不消耗业务重试预算；永久/无效消息用
  `basic.reject(requeue=false)` 进入 DLX。所有动作只处理当前 delivery（`multiple=false`）。
- RabbitMQ 4.3 的 quorum queue 可按 `delivery-limit` 进入 DLX；官方建议通过 Policy 配置 delayed retry、limit 与 DLX，
  而不是把可变策略硬编码为通用应用 `x-arguments`。
- 默认 DLX 转发不使用 confirms，存在丢失窗口；quorum queue 的 at-least-once dead-lettering 才能在内部转发时使用 confirms。
- 官方 Go Client `github.com/rabbitmq/amqp091-go` 由 RabbitMQ core team 维护，BSD-2-Clause，主分支 `go 1.20`，
  2026-08-18 发布 v1.14.0；近期版本包含连接/channel/topology 自动恢复和恢复状态通知。

结论：RabbitMQ 4.3 的 confirm、manual ack、prefetch、原生 delayed retry、delivery-limit 与 DLX 可以形成首个
Provider 的完整验收面，但 reject/nack、topology、quorum 与 Policy 是 RabbitMQ 专属配置，不能进入所有 Provider 的公共 API。

### 2.2 NATS JetStream

- durable Consumer + explicit ack 维护 consumer cursor；AckWait 到期未确认消息会 redeliver，形成 at-least-once。
- Go Client 的 `DoubleAck` 可等待服务端确认 ack 已记录，代价是额外往返；普通 ack 仍存在确认丢失后的重复窗口。
- JetStream publish/consume、subject、stream、consumer、AckWait/MaxDeliver 与 NATS 重连缓冲都有专属语义；Core NATS
  fire-and-forget 与 JetStream 持久化不能被同一个“publish 成功”布尔值混为一谈。

结论：JetStream 是合适的后续 Provider，但公共契约只能表达“Broker 接管确认、显式消费确认、至少一次与有界重投”等
共同要求；stream/subject/DoubleAck 必须由 NATS Adapter 配置和 capability 投影保留。

### 2.3 Apache Kafka

- Kafka 明确把发布 durability 与消费处理分成两个问题。默认消费通常是 at-least-once：先处理再提交 offset 时，
  处理成功但 commit 前崩溃会导致重复。
- Kafka 的 exactly-once 主要适用于 Kafka consume-transform-produce，把输出记录和 consumer offset 写入同一 Kafka
  transaction；写外部系统仍需要目标系统协作或把 offset 与结果放入同一事务存储。
- partition、consumer group、offset、rebalance、transactional producer 与 `read_committed` 都是无法由 RabbitMQ
  queue/DLX 语义等价替代的 Provider 能力。

结论：首版不能用统一 ack/nack API 假装已经覆盖 Kafka。未来 Kafka Provider 应通过 capability requirements
校验并暴露其专属运维配置，而业务 Contract/Route ID 可以保持稳定。

### 2.4 Watermill

- Watermill 提供通用 Publisher/Subscriber 与 Router；Router middleware 还提供 retry、poison queue、metrics、
  correlation、throttling 等能力。
- 这些运行期 middleware 与当前项目的 Execution、Telemetry、Logger、Supervisor、Generation admission 发生职责重叠；
  直接采用 Router 会产生第二套 handler 注册、生命周期、重试与 poison 运行链。
- Watermill 的 topic + payload 抽象也不能消除 Kafka offset、RabbitMQ DLX、JetStream consumer 等差异。

结论：首版不采用 Watermill Router。将来某个 Provider 可以在不取得生命周期/重试/日志 authority 的前提下评估
Watermill 低层 Adapter，但公共项目契约仍由 `pkg/messaging` 拥有。

## 3. 比较矩阵

| 维度 | RabbitMQ | NATS JetStream | Kafka | 公共层处理 |
| --- | --- | --- | --- | --- |
| 发布确认 | publisher confirm | PubAck | producer ack/idempotence | `Receipt` 只表达 Broker 接管，不表达消费完成 |
| 消费完成 | manual ack | explicit/double ack | offset commit | Adapter 内映射，业务 Handler 不拿原生句柄 |
| 重投递 | delayed reject/nack + delivery-limit | AckWait/MaxDeliver | 未 commit offset 后重读 | Binding 声明有界交付要求，Provider 验证能力 |
| 死信 | DLX/quorum at-least-once | 通常需专属 stream/subject 策略 | 通常为应用 DLT topic | 不伪造统一实现；Route 要求与 Provider 配置匹配 |
| 背压 | prefetch | pull batch/pending | fetch/partition flow | 公共层要求有界，具体参数留在 Provider |
| 顺序 | channel/queue 条件语义 | stream/consumer 语义 | partition 顺序 | 只声明需要的 scope；Provider 不满足就拒绝 |
| 恢复 | client reconnect/topology recovery | client reconnect | client/group recovery | 统一状态与健康，不统一内部算法 |
| exactly-once | 不承诺业务副作用 | 不承诺业务副作用 | 仅特定 Kafka 事务闭环 | 首版明确非目标 |

## 4. 选择与约束

首个生产 Provider 选择 RabbitMQ + 官方 `amqp091-go`，原因是：

1. 用户要求的确认、重投递、死信和暂时不可用恢复都有明确一手语义与可构造集成测试；
2. 官方 Go Client 当前活跃、许可证兼容、Go 版本兼容并已提供自动恢复状态；
3. 只实现一个真实 Provider 可避免在没有业务样本时同时维护三套不等价实现；
4. 显式 Provider Factory、逻辑 Route 与 capability validation 可让后续 NATS/Kafka 接入不改业务 Contract。

实施时必须重新检查 v1.14.0 是否仍为合适固定版本，并审计其 Security/Release；研究结论不授权现在修改依赖。

## 5. 不适用与剩余风险

- 不评估集群容量、跨区域拓扑、云托管 SLA 或成本。
- 不把 RabbitMQ DLX 等同于无损死信；只有满足 quorum/Policy/目标可用等条件才可声明 at-least-once dead-lettering。
- 不把 `basic.nack(requeue=true)` 当作有界业务失败次数；RabbitMQ 4.3 中它不增加 `delivery-count`。业务失败必须走
  counted reject，基础设施延后走 non-counting nack，并由 delayed retry Policy 防止两者热循环。
- 不把客户端自动恢复等同于消息可靠性；未 confirm 的发布仍必须被调用方视为未成功。
- 实现阶段需要真实 RabbitMQ 集成测试；无 Broker 时只能标记相应门禁未执行，不能用 fake 证明协议保证。
