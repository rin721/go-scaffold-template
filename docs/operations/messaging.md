# 消息系统运维

消息能力默认关闭。启用前必须确认应用模块已经声明对应 Contract/Binding，RabbitMQ 4.3 topology 与 Policy 已由部署侧
创建，配置中的逻辑 Route 能唯一映射到这些资源。应用只做 passive probe，不替运维创建或修正生产 exchange、queue、
binding 或 Policy。

## 上线前检查

可靠 RabbitMQ Route 至少满足：

- durable exchange 与 quorum queue 已存在，routing binding 正确；
- Policy 启用 `delayed-retry-type=all`、正数 `delayed-retry-min/max` 和 `delivery-limit`；
- dead-letter exchange/routing key 存在，quorum queue 使用 `dead-letter-strategy=at-least-once`；
- overflow 策略与 at-least-once dead-letter 要求兼容；
- `delivery-limit + 1` 等于 Consumer Binding 的 `MaxDeliveries`；
- URI/用户名/密码和 TLS 文件通过部署 Secret 注入，证书验证未被关闭。

配置示例和 Binding 对照见[消息系统适配能力](../development/messaging-capability.md)。

## 观测状态

读取 management `/diagnostics`，关注 `messagingHealth`、`messaging.providers[]` 与
`messaging.consumers[]`：

| 状态/字段 | 含义 | 运维动作 |
| --- | --- | --- |
| `connecting` | 首次建立连接 | 检查地址、网络、TLS 与认证；等待有界自动恢复 |
| `ready` | publish session 可用，已提交代可开放 Consumer | 正常观察 confirm、failure 与 redelivery 变化 |
| `recovering` | 连接、channel 或 topology probe 失败，正在退避恢复 | 恢复 Broker/网络/topology；不要重启应用掩盖根因 |
| `draining` / `stopped` | Generation 正在排空或已释放 Provider | reload/停止期间的正常状态 |
| `confirmed` / `failed` | 已确认与失败发布计数 | 失败增长时结合稳定错误类型排查，不依据单一计数自动重发 |
| Consumer `inFlight` | 正在处理且尚未确认的 delivery | handoff/停止时观察是否在预算内归零 |
| `redelivered/rejected/deadLettered` | 重投、计数重试和终止死信计数 | 按 Consumer ID 检查业务错误分类和 DLQ |

required Route 不可用会令 `messagingHealth=fail` 并使 readiness 失败；optional Route 不可用为 `warn`，应用仍可 ready。
这只决定流量治理，不改变发布错误：任何 Route 都不会静默成功、写内存兜底或隐式切换 Provider。Execution health
失败时 Consumer intake 会暂停，恢复后自动开放。

同时关注结构化日志：

- `messaging provider state changed`：按 `provider`、`driver`、`state`、`generation` 和 `error_type` 判断是连接恢复、
  拓扑失败还是正常停止排空；
- `messaging delivery rejected`：RabbitMQ Envelope/Contract 在 Provider 边界被拒绝，先核对 Contract header、
  content type、message type 和 route 配置；
- `messaging consumer deferred delivery`：Execution backend、active lease 或上游取消导致不计数延后；
- `messaging consumer scheduled retry`：业务可重试错误或 Handler timeout 已消耗一次投递预算；
- `messaging consumer dead-lettered delivery`：预算耗尽、永久错误或 panic 已进入 DLX，应按 Consumer 和 Message ID 查业务补偿。

## 发布故障处置

- `ErrUnavailable`：Broker/session 尚未 ready；由调用方按业务入口策略呈现失败，不在 Adapter 内缓存。
- `ErrUnroutable`：mandatory publish 被退回；核对 exchange、routing key 和 binding。
- `ErrPublishRejected`：Broker negative confirm；核对资源限制、权限和 Broker 状态。
- `ErrPublishAmbiguous`：发送后 confirm 未能确定；消息可能已被接管。只允许按同一 Message ID 经过业务幂等策略重试，
  不得生成新 ID 或把未知结果记成成功。

日志和工单只记录 Provider 名、状态、Generation、Consumer/Producer/Route ID、Message ID 或 trace ID 等受控字段；不要
粘贴 URI、密码、Authorization、原始 payload、完整 headers 或未经审查的错误文本。

## RabbitMQ 4.3 协议门禁

仓库提供隔离验证脚本：

```powershell
./scripts/verify-messaging-rabbitmq.ps1
```

脚本要求本机 Docker，创建固定且经过校验的临时容器 `go-scaffold-messaging-rabbitmq-43-test`，使用随机临时凭据，安装
测试专用 quorum delayed-retry/DLX Policy，然后真实验证 confirm、unroutable、counted reject、non-counting nack、
delivery-limit/DLX、断连恢复和 Consumer quiesce 后 Publisher 保留。脚本最终只清理自己创建且名称核验通过的容器。

该脚本是测试门禁，不是生产 topology 工具。升级 RabbitMQ 版本、修改 Policy、切换 Provider、改变投递预算或调整
ack/retry 映射时必须重新执行；没有真实成功日志时只能声明集成测试未验证。
