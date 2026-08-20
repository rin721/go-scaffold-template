# Messaging 公共契约

`pkg/messaging` 只定义业务模块可见的 Message Contract、Producer/Consumer Binding、Publisher、策略、错误和低敏诊断。
RabbitMQ、Kafka、NATS 等 Client、destination、确认句柄、连接和关闭权不得进入本包公开 API。

模块把不可变 `Contribution` 放入 `module.Contribution.Messages`；`internal/composition` 聚合 `Catalog`、解析逻辑 Route
并选择 Provider。业务 Handler 只处理项目 `Message` 或模块 Adapter 解码后的 typed command/event。

发布成功只证明 Broker 接管；消费默认 at-least-once。业务副作用仍需按 Message ID 保持幂等，Outbox/Inbox 与
exactly-once 不属于当前契约。

业务模块声明、配置、消费处置和运行边界见[消息系统适配能力](../../docs/development/messaging-capability.md)；Broker
故障与真实 RabbitMQ 4.3 协议门禁见[消息系统运维](../../docs/operations/messaging.md)。
