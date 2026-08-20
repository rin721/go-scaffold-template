# 037 研究档案

本目录记录定时调度能力在 `a675f33` 快照下的当前实现事实、外部主源语义和接入综合结论。
研究记录是形成 037 计划的证据，不代表源码已经实现。

## 检索与复用

研究开始前已检索 `docs/**/research/**/metadata.yaml` 以及 035/036 变更记录：

- 035 的 R001/R002 覆盖幂等、重试、执行记录，但其刷新触发器明确包含“引入任务调度”；
  因此只复用 Execution 事实，不能直接复用其“无跨进程调度”的边界结论。
- 036 已证明业务模块经自有窄 port 与 composition 注入 Execution 的路径，但没有调度 Binding、
  distributed coordination 或 Application Generation admission。
- 023 的 Application Generation / ListenerHub 证据仍适用于候选准备、commit、retire 与排空模式。

## 记录

- [R001 当前调度与治理能力清单](R001-current-scheduling-capability-inventory/report.md)：当前代码的可复用能力、真实缺口和契约不适配点。
- [R002 调度引擎与 Redis 协调主源](R002-scheduler-and-coordination-primary-sources/report.md)：候选库、官方分布式限制和 Redis lease 风险边界。
- [R003 单轨接入综合结论](R003-single-track-integration-synthesis/report.md)：能力归属、Application Generation 接入与计划约束。

## 研究门禁

R001-R003 已覆盖形成计划所需的关键问题：现有装配和 lifecycle、模块 Binding、Execution/Cache/
Tracing/health 的复用边界、第三方选择、严格单实例语义及其限制。剩余未知只涉及确认后的实现细节和
真实 Redis 运行验收，不妨碍形成范围与验收，因此研究门禁标记为通过。
