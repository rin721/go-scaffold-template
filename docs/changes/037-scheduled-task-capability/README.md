# 037 定时调度能力

状态：已确认、已实施并完成本地验证；已创建本地提交，不推送。

## 范围

本变更为业务模块提供统一的 `cron` / `fixedDelay` 定时任务 Binding、进程内触发与并发治理、
分布式单实例执行权协调，以及与现有 Application Generation、Supervisor、Execution、Cache、
Observability、Ops health/diagnostics 的单轨装配。

研究与计划轮没有修改源码、配置、依赖、测试或运行状态。用户在计划报告后的后续消息中明确
回复“确认按 037 计划实施”；实现仅覆盖 `tasks.md` 中已确认的任务，并已把当前行为同步到主题文档。

## 阅读顺序

1. [研究索引](research/README.md)：检索范围、证据快照和研究结论。
2. [需求规格](requirements.md)：目标、范围、约束、非目标和验收标准。
3. [设计方案](design.md)：契约、装配、生命周期、协调、失败和验证设计。
4. [任务清单](tasks.md)：稳定任务 ID、依赖、完成条件和确认状态。

## 当前结论

- 复用 `gocron/v2` 的 `cron`、completion-based interval、并发限制与有界关闭能力，但不把其类型暴露给业务模块。
- 不使用 `gocron` 内置 distributed Locker 作为严格语义依据；其官方文档明确不负责多实例触发时间同步。
- 模块只通过 `module.Contribution.Schedules` 贡献项目自有 Binding；`internal/composition` 是唯一聚合与绑定位置。
- 调度 admission/handoff 并入现有 Application Generation，运行失败上送既有 `GenerationCoordinator.Monitor -> Supervisor`，不新建第二套 Runtime/Supervisor。
- 真实任务运行复用现有 Execution；分布式执行权使用项目自有窄协调契约，并由现有 Cache Redis resource 复用同一连接。
- 严格单实例任务禁止自动本地降级；协调不可用时按任务策略选择 skip、pause 或 fail，恢复后自动重新争抢。
- 当前仓库没有新增具体业务定时任务；能力保持默认关闭，业务模块按当前权威接入文档显式贡献 Binding 后才运行。
