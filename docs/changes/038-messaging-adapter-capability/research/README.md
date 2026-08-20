# 038 研究索引

## 研究问题

1. 当前五层能力链中哪些生命周期、执行、健康与装配机制可直接复用，消息能力真实缺口在哪里；现有 Execution
   claim 生命周期是否能直接承接 Broker redelivery？
2. RabbitMQ、NATS JetStream、Kafka 与 Watermill 对确认、重投递、死信、恢复和事务分别提供什么，
   哪些语义可以进入公共契约，哪些必须留在 Provider？
3. 如何让消息生产与消费接入 Application Generation，同时避免候选代际提前消费、重复重试、
   静默丢消息和第二套运行治理？

## 检索与快照

- 已检索 `docs/**/research/**/metadata.yaml`；没有当前消息系统适配研究可直接复用。
- 复核 035、036、037 的 Execution、Module Binding、Application Generation、ScheduleHub、
  Cache coordination、Telemetry 与 Ops 结论，并以当前工作区代码校验。
- 研究开始时基线为 `a675f33` 加 037 工作区实现；复核时 037 已收口为本地提交 `00083be`，
  当前 `origin/main` 仍为 `a675f33`。研究结论已按 `00083be` 复核，不把本地提交描述为已推送事实。
- 外部事实只使用 RabbitMQ、Apache Kafka、NATS、Watermill 与官方 Go Client 主源，验证日期为 2026-08-20。

## 记录

- [R001 当前消息相邻能力与缺口](R001-current-messaging-capability-inventory/report.md)
- [R002 消息可靠性与 Provider 主源比较](R002-messaging-primary-source-comparison/report.md)
- [R003 单轨消息适配综合结论](R003-single-track-messaging-synthesis/report.md)

R003 判定研究门禁通过，只表示证据足以形成计划；非文档实施仍需用户在计划报告后的后续消息中明确确认。
