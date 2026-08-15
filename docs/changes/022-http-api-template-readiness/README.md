# 022：HTTP API 脚手架成熟就绪度

## 当前状态

- 任务类型：研究、规划与已确认的生命周期 P0 实施。
- 研究门禁：已通过。R001 复核整体 HTTP API 成熟度，R002 审计当前底层链路，R003 对照成熟 Go 项目，R004 给出综合结论，R005 逐项确认资源终结、重试、force 和验证语义。
- 生命周期计划：[`FOUNDATION-LIFECYCLE-001`](plans/foundation-lifecycle-001.md) 已确认并实施完成；`LFC-001` 至 `LFC-010` 的代码、测试和权威文档已同步。
- 当前判断：**底层主链已形成但未完全闭环；Production HTTP API-ready 未通过；新的业务模块详细设计尚未解锁。**
- 实施基线：HEAD `b868893`；本轮在该基线上完成 lifecycle 变更与全量 test/race/vet/build，不启动服务、不部署、不推送。
- 与既有任务的关系：019 是旧身份下的 HTTP 差距快照；020 已确定 copy-owned 产品形态；021 完成 canonical identity；022 现在同时是底层闭环和 HTTP 成熟化的当前计划入口。

## 一句话结论

当前项目**还不能称为成熟的 Go Server HTTP API 后端脚手架模板**。生命周期 P0 已修复终止排空超时、清理失败责任丢失和 HTTP 暗中 force；但统一 management diagnostics、EnvSource 冲突和完整 Foundation acceptance 尚未完成，因此整体仍只能称为 **Foundation-partial**。

推荐不是推倒重来：

> 保留手工组合根、forward-only Kernel Plan、stable Access/Lease、immutable config generation 和 Kernel/Supervisor 分工；先补齐终止与失败清理所有权、结构化诊断、EnvSource 冲突和故障验收，再按已证明的业务 profile 解锁模块设计。

生命周期加固采用“统一 owner/state/budget 治理 + 场景化关闭策略”，不建立万能 `Close()`：Database/Redis/logger/fsnotify 是 drain 后 terminal close，HTTP 是显式 graceful-to-force，当前 StorageManager 和纯内存值是 no-finalization；retry 只允许资源 Adapter 证明再次调用会真实补做释放时启用。

## 当前闭环状态

| 平面 | 当前判断 | 说明 |
| --- | --- | --- |
| 配置输入与 owner 校验 | 部分通过 | 同一 Snapshot、严格 Decode、owner binding 已闭环；EnvSource scalar/object 冲突仍可能顺序覆盖 |
| DI 与装配 | 通过（当前 profile） | 唯一 composition root、手工业务装配、forward-only typed Plan，无 locator/扫描/第二容器 |
| 资源创建与正常启动 | 通过 | Stage/Build/Start/Ready/Publish 和启动失败补偿已有实现 |
| 运行、ready 与错误传播 | 通过（当前 profile） | Supervisor 拥有长期任务，HTTP/watcher ready 和错误链已接入 |
| Reload 一致性 | 通过（生命周期范围） | 提交前原子、旧代可恢复；提交后 cleanup debt 保留 retired owner/generation 并阻断后续 reload |
| Drain 与 Stop | 通过（当前 profile） | terminal drain 超时保持 cleanup-pending；后续 Stop 继续同一 drain，terminal attempt 失败保留责任 |
| 诊断与治理 | 部分通过 | Kernel finalization 与 Supervisor pending/forced 已分类；跨 owner management view、policy/verification 和 EnvSource 门禁仍缺 |
| 业务扩展 | **未通过** | Todo 只证明同步 HTTP/CLI/migration profile；Runner/Health/new resource 需先做能力评估 |
| HTTP API 产品治理 | 未通过 | API authority、统一协议、安全、管理面、遥测、迁移和 release 仍缺 |

## 成熟标签与业务停止线

- `Foundation-closed`：配置、装配、资源、运行、reload、drain、stop、诊断和故障验收全部闭环。当前未达到。
- `Copy-ready`：正式 release 可在支持平台复制、迁移 identity、保留/移除示例并独立演进。当前部分达到。
- `Production HTTP API-ready`：协议、安全、管理、遥测、数据演进、交付和兼容保证均已实现并验收。当前未达到。

新的 Handler/Service/Repo/Model 详细设计只有在 [底层验收](acceptance.md) 的阻断项通过、且新模块能力评估确认需求落在已证明 profile 内后才解锁。需要后台 Runner、动态 Health、新共享资源或新 reload policy 时，先新增独立 foundation 研究，不绕过组合根。

## 阅读顺序

1. [R002：当前底层闭环审计](research/R002-foundation-closure-audit/report.md)
2. [R003：成熟 Go 项目实践对照](research/R003-go-runtime-practices/report.md)
3. [R004：综合结论与业务解锁条件](research/R004-foundation-closure-synthesis/report.md)
4. [R005：逐资源终结、重试与强制关闭策略](research/R005-resource-finalization-policy/report.md)
5. [R001：整体 HTTP API 成熟度复核](research/R001-current-readiness-reassessment/report.md)
6. [需求：闭环与成熟标签门禁](requirements.md)
7. [设计：保留架构下的加固路线](design.md)
8. [FOUNDATION-LIFECYCLE-001：生命周期闭环独立实施计划](plans/foundation-lifecycle-001.md)
9. [验收：十一门与业务解锁矩阵](acceptance.md)
10. [任务：实施 Program 与确认边界](tasks.md)

## 当前授权边界

用户已在计划报告后的后续消息确认 `FOUNDATION-LIFECYCLE-001`，因此本轮只实施并提交 `LFC-001` 至 `LFC-010`。服务启动、部署、推送、tag、release、外部写入和其他 Program item 均不在授权范围；后续事项仍须各自研究、计划并确认。
