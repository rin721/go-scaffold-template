# Messaging Kernel App

本组件把 `pkg/messaging` Catalog 装配到命名 Provider，并拥有 Provider connection、Publisher confirm、Consumer
channel、恢复循环、Execution/Telemetry 协作和 Generation handoff。业务模块不得导入本目录。

- `Config`：定义命名 Provider、逻辑 Route、confirm/handoff/shutdown timeout 与恢复退避；默认 disabled。
- `Output.Publisher`：只向 composition 提供 generation-local 业务发布出口，不暴露关闭权。
- `Output.Control`：只供 composition Freeze Catalog、开放 Publisher、切换 Consumer admission、读取 Health/Diagnostics。
- `Provider`：内部 SPI；当前生产实现位于 `rabbitmq`，`fake` 只用于确定性状态机测试和显式测试 composition。
- `Hub`：进程稳定的 Consumer admission owner；先排空旧代再激活候选，候选失败时恢复旧代。

RabbitMQ Adapter 只 passive probe topology，不创建生产资源。公共开发入口见
[消息系统适配能力](../../../../docs/development/messaging-capability.md)，运维入口见
[消息系统运维](../../../../docs/operations/messaging.md)。
