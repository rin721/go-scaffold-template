# 038 消息系统适配能力

状态：研究门禁已通过，计划已确认；实现与本地工程门禁已完成，RabbitMQ 4.3 真实协议门禁受本机无 Docker/WSL 环境阻断。

## 范围

本变更在现有幂等、失败重试、执行记录、定时调度之后，补齐第五项通用技术能力：
消息系统适配。业务模块只声明项目自有 Message Contract、Producer Binding、Consumer Binding
及治理要求；`internal/composition` 统一聚合，每条逻辑 Route 绑定一个显式选择的 Provider，不同 Route 可让多个
Provider 在同一进程并存。

消息能力复用现有 Application Generation、Execution、Telemetry、Logger、Health、Ops 和
Supervisor，不创建平行 Runtime、隐式 Registry、第二套重试/执行记录或业务可见的中间件 Client。

## 阅读顺序

1. [研究索引](research/README.md)：当前事实、主源比较和单轨综合结论。
2. [需求规格](requirements.md)：目标、范围、失败语义与验收标准。
3. [设计方案](design.md)：Contract/Binding、Provider、可靠性、代际和运维设计。
4. [任务清单](tasks.md)：稳定任务 ID、依赖、确认状态和验证证据要求。

## 当前实现结论

- 已新增 `pkg/messaging` 项目自有契约与 `internal/kernel/app/messaging` 受管组件。
- `module.Contribution` 显式贡献消息 Contract、Producer 和 Consumer Binding；不扫描、不 `init` 注册。
- 逻辑 Route 与物理 exchange/queue/topic 解耦；配置可显式装配多个命名 Provider，同进程并存但不隐式双写。
- 首个生产 Provider 采用 RabbitMQ AMQP 0-9-1，固定官方 `amqp091-go` v1.14.0；Kafka、NATS
  JetStream 作为后续 Provider，不被公共契约伪装成已经支持。
- Consumer 以 RabbitMQ 4.3 quorum delayed retry、manual ack、delivery limit/DLX 为交付真相；业务失败使用
  counted reject，基础设施暂时阻塞使用 non-counting nack。Execution 先单轨拆分 running lease、completed retention
  与失败 release，再负责每次交付的幂等占用、单次业务执行和执行记录，默认不叠加进程内重试。
- Broker 暂时不可用时保持进程 live：必需 Route 使 readiness 失败，可选 Route 进入 degraded；发布始终返回
  可识别错误，不用内存缓冲或静默成功伪造可靠降级，恢复后自动重连并恢复生产/消费。
- 本计划不承诺业务副作用 exactly-once，也不把数据库事务与消息发布原子化；Outbox/Inbox 事务协作需真实
  业务事务边界后单独研究。

## 验证说明

`scripts/Verify-Quality.ps1`、消息范围 gosec、gitleaks、Markdown links 与 RabbitMQ integration build-tag 编译均已通过；
未设置 Broker URI 时 integration test 明确跳过。由于 Windows 没有 `docker` 命令且 WSL 无可运行发行版，隔离
RabbitMQ 4.3 容器未创建，真实 confirm/retry/DLX/断连恢复门禁仍未执行，不能视为通过。逐项证据与既有基线风险见
[任务清单](tasks.md)。
