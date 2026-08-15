# R004：底层闭环综合结论与业务解锁条件

## 1. 总结论

当前底层状态是 **partial closure**：配置、显式装配、资源创建、启动、ready、运行失败、提交前 reload 回滚和正常 Stop 已形成一条真实主链；但终止排空超时、候选/旧代清理失败时，资源清理责任不能被持续拥有和再次完成，因此生命周期、一致性、诊断和业务扩展门未全部通过。

这不是推倒重来的证据。推荐方案是保留当前架构，仅在失败状态、诊断和验收上做最小加固。

## 2. 保留、补齐、优化、重设计

| 类别 | 决策 | 理由 |
| --- | --- | --- |
| 保留 | `cmd/app -> internal/composition` 唯一应用组合根 | 入口薄、命令模式清楚、无隐式全局装配 |
| 保留 | 手工业务装配 + forward-only typed Kernel Plan | 当前规模可读，循环依赖不可表达，不需要 Runtime DI |
| 保留 | owner-bound strict config 与同一 immutable Snapshot | 已关闭未知字段、弱转换和跨 owner 漂移的主要风险 |
| 保留 | stable Access/Lease + prepare/drain/commit/cleanup 代际 | 已用真实 Database/Logger reload 证明价值，和 Caddy 原子代际原则一致 |
| 保留 | Kernel 管资源、Supervisor 管进程任务的双层分工 | 两者生命周期对象和失败语义不同，合并会丢失边界 |
| 补齐 P0 | terminal drain timeout 后的 pending cleanup/finalization | 当前会过早 stopped 且无法再次 Close |
| 补齐 P0 | cleanup failure 保留 owner、generation、状态和最终政策 | 当前只保留 error，清空实例引用 |
| 补齐 P0 | Supervisor pending participant 与统一退出诊断 | 当前只结构化列出未结束 Task |
| 补齐 P0 | `EnvSource` scalar/object 形状冲突拒绝 | 当前同源覆盖顺序不确定，破坏严格配置承诺 |
| 补齐 P0 | start/reload/stop/CLI/service 的故障注入验收矩阵 | 现有测试没有证明超时释放后的最终清理 |
| 局部优化 P1 | RestartRequired 配置恢复后的 reconciliation policy | 当前 sticky 阻断安全但运维体验保守，不是资源完整性 P0 |
| 局部优化 P1 | health/runner 的场景化 module contribution | 真实后台业务出现前保持窄契约，避免万能模块协议 |
| 不重设计 | 不引入 Fx/Wire/通用 DAG，不删除 reload | 迁移成本高且不直接解决清理责任；当前无必要性证据 |

## 3. 推荐目标状态

目标不是新增一个大接口，而是让每条责任有可判定结果：

```text
configuration candidate
  -> validate all owners
  -> build/start/ready resources
  -> publish stable capabilities
  -> run supervised participants/tasks
  -> reload: prepare -> drain -> commit -> cleanup
  -> stop admission -> graceful drain
       -> finalized
       -> cleanup-pending/failed (owner + generation + policy)
  -> diagnostics + acceptance evidence
```

后续 [R005](../R005-resource-finalization-policy/report.md) 已逐资源确认 Close/retry/force 语义。源码设计必须满足以下不变量，但本研究不预先虚构具体公开 API：

1. Stop 超时后不能把“仍有清理责任”伪装成 stopped。
2. stop/close 返回错误时不能丢失实例引用；Close 幂等不等于 retryable，terminal attempt 与显式 retryable strategy 必须分开。
3. diagnostics 至少能定位 phase、owner、generation、pending unit 和 error type，且不泄露配置值。
4. 通用层只治理 owner/state/budget；no-finalization、drain 后 terminal close、graceful、force 和 retry 使用场景化策略。不能安全强关或重试的资源必须明确失败终态。
5. 正常、超时、清理失败、调用方最终释放四条路径都有确定性测试。

## 4. 为什么推荐方案更合适

- **比换 Fx 更直接**：Fx 擅长装配和 hook 顺序，不表达项目已有的资源代际事务。
- **比通用 DAG 更简单**：当前 forward-only Plan 已满足真实依赖，没有未表达的循环或动态发现需求。
- **比删除 reload 更保守**：保留已经验证的稳定 Access 和原子换代，不用破坏现有能力掩盖 bug。
- **比强制 Close 更安全**：不在活跃 request/use 上破坏资源，同时不遗忘最终责任。
- **比一次补齐所有 module hook 更克制**：只把已出现的同步 HTTP/CLI profile 纳入 baseline；新 Runner/Health/资源按真实业务能力评估扩展。

## 5. 业务模块详细设计解锁门

在本任务约束下，新的 Handler/Service/Repo/Model 详细设计必须同时满足：

1. `FOUNDATION-LIFECYCLE-001`、`FOUNDATION-DIAGNOSTICS-001`、`FOUNDATION-CONFIG-001` 已实现并通过 [acceptance.md](../../acceptance.md) 的底层验收；
2. 当前仓库在目标平台完成 scope-appropriate test/race/vet/build，且没有以超时或进程退出掩盖资源泄漏；
3. 新模块已先记录 actor、use case、错误、数据 owner、事务、外部资源、配置、HTTP/CLI、后台任务、ready/health 和停止需求；
4. 需求只落在已证明 profile 内；若需要 Runner/Health/new resource/reload policy，先建立独立 foundation capability 研究与计划；
5. 模块不得访问 Kernel/Container/第三方具体 client，不得在 Start 内私自拥有无法等待的 goroutine。

这不是要求模板预装消息、调度或所有数据库；它要求出现新场景时先验证底层契约能真实表达，再进入业务细节。

## 6. 实施顺序、风险与未知

推荐顺序：

1. 先设计并实现 pending cleanup/finalization 状态与诊断；
2. 补 EnvSource 冲突和 Supervisor pending owner；
3. 建立跨 start/reload/stop、Service/CLI 的故障验收；
4. 复核底层十一门并决定是否解锁同步 HTTP 模块设计；
5. 再推进 API authority、management、protocol/security/observability 和交付成熟化。

主要风险是错误地把“最终清理”实现成无限等待，或把“有界退出”实现成强关活跃资源。R005 已确认当前 Database、Redis、logger、fsnotify 不能靠重复 Close 重试，HTTP force 有损，当前 StorageManager 无底层 close；仍未知的是部署终止总预算、第二次信号、HTTP hijacked owner、真实外部资源 close error 和首个后台业务需求。

## 7. 最终判断

- 底层能力闭环：**部分闭环，未通过总门禁**。
- 已通过：配置 owner、显式装配、依赖方向、正常启动/ready/run、提交前 reload 原子性、正常反向停止、基本边界治理。
- 缺失：终止与失败清理责任、owner/generation 诊断、EnvSource 形状冲突、关键反向验收，以及超出 Todo profile 的业务扩展证明。
- 架构动作：**保留主架构，局部补齐与优化，不做容器级重设计**。
- 业务动作：完成 P0 并复核验收前，停止新的业务模块详细设计；完成后按 profile 解锁，不把未知场景预装进模板。
