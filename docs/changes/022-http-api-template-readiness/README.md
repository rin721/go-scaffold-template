# 022：HTTP API 脚手架成熟就绪度

## 当前状态

- 任务类型：研究、规划，以及已确认并实施的生命周期、统一诊断、配置确定性与剩余 Foundation 单轨闭环。
- 研究门禁：已通过。R008 基于当前 HEAD 复核三项 P0 实施结果并取代旧 R002/R004 的当前结论；R001/R003/R005 继续提供 HTTP、外部实践和逐资源语义，R006/R007 保留对应实施前快照。
- 生命周期计划：[`FOUNDATION-LIFECYCLE-001`](plans/foundation-lifecycle-001.md) 已确认并实施完成；`LFC-001` 至 `LFC-010` 的代码、测试和权威文档已同步。
- 诊断计划：[`FOUNDATION-DIAGNOSTICS-001`](plans/foundation-diagnostics-001.md) 已确认并实施完成；`DGN-001` 至 `DGN-009` 的 typed ledger、Host 单一快照、测试和权威文档已同步。
- 配置计划：[`FOUNDATION-CONFIG-001`](plans/foundation-config-001.md) 已确认并实施完成；`CFG-001` 至 `CFG-008` 的确定性 Env path、object/non-object merge、测试和权威文档已同步。
- 剩余闭环计划：[`FOUNDATION-CLOSURE-001`](plans/foundation-closure-001.md) 已把未启动的 acceptance 与 reconciliation 单轨合并并完成 `FCL-001..008`。
- 当前判断：**`Foundation-closed(current synchronous HTTP/CLI profile)` 已通过；Production HTTP API-ready 仍未通过。**
- 实施基线：`3a936a5`；restart recovery、实际 Host diagnostics、Service/CLI 物理释放、完整 test/race/vet/build/cross-build 与十一门证据已完成。
- 与既有任务的关系：019 是旧身份下的 HTTP 差距快照；020 已确定 copy-owned 产品形态；021 完成 canonical identity；022 现在同时是底层闭环和 HTTP 成熟化的当前计划入口。

## 一句话结论

当前项目**还不能称为成熟的 Go Server HTTP API 后端脚手架模板**，但底层已在明确范围内闭环：生命周期、统一 diagnostics、配置确定性、preflight restart recovery 和 current-profile 跨层验收均已通过。当前标签是 **`Foundation-closed(current synchronous HTTP/CLI profile)`**；HTTP 产品 authority、协议、安全、管理、遥测、交付和 release 仍未完成。

本次闭环没有推倒重来：

> 保留手工组合根、forward-only Kernel Plan、stable Access/Lease、immutable config generation 和 Kernel/Supervisor 分工；只补安全 restart reconciliation 与缺失跨层证据，并按已证明 profile 关闭 Foundation。

生命周期加固采用“统一 owner/state/budget 治理 + 场景化关闭策略”，不建立万能 `Close()`：Database/Redis/logger/fsnotify 是 drain 后 terminal close，HTTP 是显式 graceful-to-force，当前 StorageManager 和纯内存值是 no-finalization；retry 只允许资源 Adapter 证明再次调用会真实补做释放时启用。

## 当前闭环状态

| 平面 | 当前判断 | 说明 |
| --- | --- | --- |
| 配置输入与 owner 校验 | 通过（配置范围） | 同一 Snapshot、严格 Decode、owner binding、Env path 与跨 Source object/non-object 冲突已闭环 |
| DI 与装配 | 通过（当前 profile） | 唯一 composition root、手工业务装配、forward-only typed Plan，无 locator/扫描/第二容器 |
| 资源创建与正常启动 | 通过 | Stage/Build/Start/Ready/Publish 和启动失败补偿已有实现 |
| 运行、ready 与错误传播 | 通过（当前 profile） | Supervisor 拥有长期任务，HTTP/watcher ready 和错误链已接入 |
| Reload 一致性 | 通过（当前 profile） | 提交前原子；preflight restart 可由完整恢复候选解除；cleanup debt 仍 fail-closed |
| Drain 与 Stop | 通过（当前 profile） | terminal drain 超时保持 cleanup-pending；后续 Stop 继续同一 drain，terminal attempt 失败保留责任 |
| 诊断与治理 | 通过（diagnostics 范围） | Host 提供单一 `ProcessDiagnostics`；owner kind/generation/phase/policy/attempt/budget/pending/terminal/verification 已结构化且脱敏；EnvSource 属于独立配置门禁 |
| 业务扩展 | 通过（current profile） | 同步 HTTP/CLI/startup Participant 可在逐模块能力评估后进入设计；Runner/Health/new resource 仍阻断 |
| HTTP API 产品治理 | 未通过 | API authority、统一协议、安全、管理面、遥测、迁移和 release 仍缺 |

## 成熟标签与业务停止线

- `Foundation-closed`：配置、装配、资源、运行、reload、drain、stop、诊断和故障验收全部闭环。当前只在 synchronous HTTP/CLI profile 达到。
- `Copy-ready`：正式 release 可在支持平台复制、迁移 identity、保留/移除示例并独立演进。当前部分达到。
- `Production HTTP API-ready`：协议、安全、管理、遥测、数据演进、交付和兼容保证均已实现并验收。当前未达到。

新的 Handler/Service/Repo/Model 仍须先做逐模块能力评估；需求完全落在已证明 synchronous HTTP/CLI profile 时可进入详细设计。需要后台 Runner、动态 Health、新共享资源、长连接或新 reload policy 时，仍先新增独立 foundation 研究，不绕过组合根。

## 阅读顺序

1. [R008：剩余 Foundation 闭环复核](research/R008-remaining-foundation-closure/report.md)
2. [FOUNDATION-CLOSURE-001：剩余 Foundation 单轨闭环计划](plans/foundation-closure-001.md)
3. [需求：闭环与成熟标签门禁](requirements.md)
4. [设计：保留架构下的加固路线](design.md)
5. [验收：十一门与业务解锁矩阵](acceptance.md)
6. [任务：实施 Program 与确认边界](tasks.md)
7. [FOUNDATION-CONFIG-001：配置 Source 路径确定性实施计划与证据](plans/foundation-config-001.md)
8. [FOUNDATION-DIAGNOSTICS-001：统一运行责任诊断实施计划](plans/foundation-diagnostics-001.md)
9. [FOUNDATION-LIFECYCLE-001：生命周期闭环独立实施计划](plans/foundation-lifecycle-001.md)
10. [R005：逐资源终结、重试与强制关闭策略](research/R005-resource-finalization-policy/report.md)
11. [R003：成熟 Go 项目实践对照](research/R003-go-runtime-practices/report.md)
12. [R001：整体 HTTP API 成熟度复核](research/R001-current-readiness-reassessment/report.md)
13. [R002/R004：实施前历史结论](research/README.md)

## 当前授权边界

四项 Foundation 计划均已完成。本 goal 的临时全流程授权只覆盖 `FCL-001..008`、本地验证和本任务提交；推送、部署、真实配置、外部系统、tag、release 和 HTTP 产品治理始终不在范围。
