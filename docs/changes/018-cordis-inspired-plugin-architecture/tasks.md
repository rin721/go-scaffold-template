# 任务：Cordis 启发的插件架构

> **已废除。** 本任务账本中的实施任务全部取消，不得继续确认或执行。当前方向见 [019 HTTP API 成熟度缺口评估](../019-http-api-maturity-gap-assessment/README.md)。

## 1. 门禁状态

- 研究门禁：已通过，证据为 `R001`、`R002`。
- 当前计划状态：已废除。
- 当前授权：无；`RES-001`、`RES-002`、`PLAN-001` 仅作为历史文档保留，所有非文档任务失效。
- 实施原则：按 Phase 分轮确认；如果公共 API、依赖选择、边界、配置迁移或动态化范围实质变化，退回研究并重新确认。
- Git：废除记录可以随 019 纯文档任务提交；不得提交任何 018 非文档实现。

## 2. 任务清单

| ID | Phase | 工作量 | 依赖 | 任务 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- | --- |
| `RES-001` | Research | L | 无 | 核验当前 Definition/Plan/Kernel/Contribution/composition 与既有研究 | R001 区分事实、缺口、冲突和局限 | 已完成 |
| `RES-002` | Research | L | 无 | 核验 DeepSeek Harness、vendored Cordis、论文和 Go `plugin` | R002 固定快照并给出采用/改造/拒绝结论 | 已完成 |
| `PLAN-001` | Plan | L | `RES-001`、`RES-002` | 冻结需求、目标架构、阶段、风险和验收 | `requirements.md`、`design.md`、`tasks.md` 完整 | 已完成 |
| `ADR-001` | 0 | M | 用户确认 | 新建插件装配 ADR，精确调整 012 决策 | compiled-in、typed token、无 Resolver、启动期 Profile 成为单一决策 | 已废除 |
| `CONTRACT-001` | 0 | L | `ADR-001` | 定义 Plugin/Instance ID、Manifest、Capability Ref/Version/Cardinality、错误和诊断 | 契约测试覆盖零值、重复、namespace/version 和不可变性 | 已废除 |
| `GRAPH-001` | 1 | XL | `CONTRACT-001` | 实现显式 Catalog、Bundle/Profile 与纯 Graph Compiler | 副作用前拒绝未知/缺失/重复/环/版本；排序确定 | 已废除 |
| `TOKEN-001` | 1 | XL | `CONTRACT-001` | 实现 typed `Key[T]` 与 Requirements 构造注入 | 业务代码无 `any` Store/Get/Resolver；类型错配不可进入运行期 | 已废除 |
| `INSPECT-001` | 1 | M | `GRAPH-001` | 增加 validate/dump/explain 只读诊断 | 不启动资源即可输出脱敏 Profile、graph、order、owner | 已废除 |
| `KERNEL-001` | 2 | XL | `GRAPH-001`、`TOKEN-001` | 将现有 Kernel App 接入 Catalog/Profile 编译结果 | 新 Profile 生成与当前等价 Frozen Plan，现有 reload/Lease 测试通过 | 已废除 |
| `KERNEL-002` | 2 | M | `KERNEL-001` | 单轨删除旧硬编码 `Compose` 清单与失效文档 | 旧入口/符号零残留，Bootstrap/Service 边界保持 | 已废除 |
| `BROKER-001` | 3 | XL | `GRAPH-001` | 定义 Route/Command/Health/Participant/Runner owner/broker | contribution 有 typed owner、冲突校验与 cleanup | 已废除 |
| `TODO-001` | 3 | XL | `KERNEL-002`、`BROKER-001` | 将 Todo 粗粒度模块、HTTP/CLI surface 迁移为插件 | Todo 内部仍普通构造，HTTP/CLI 验收保持 | 已废除 |
| `TODO-002` | 3 | M | `TODO-001` | 删除旧 `module.Contribution` 与硬编码 Todo composition | 旧路径、测试、文档和配置引用零残留 | 已废除 |
| `EFFECT-001` | 4 | XL | `BROKER-001` | 实现实例 Effect ledger 和 LIFO cleanup | 部分失败、幂等、异步清理、错误聚合与泄漏测试通过 | 已废除 |
| `LIFE-001` | 4 | XL | `KERNEL-001`、`EFFECT-001` | 统一 Instance 状态、拓扑启动/停止与 Host/Kernel phase | dependents 先于 providers 停止；取消/超时有确定结果 | 已废除 |
| `GOV-001` | 4 | L | 全部 Phase 0-4 | 同步架构文档、开发指南、示例与自动门禁 | scan/init/Resolver/旧路径搜索通过；权威文档单轨 | 已废除 |
| `VER-001` | 4 | L | 全部 Phase 0-4 | 完成全量验证和真实 Todo 进程验收 | test/race/vet/Linux build/diff check 与 HTTP/CLI smoke 通过 | 已废除 |
| `LIVE-001` | Future | XL | 独立真实需求与新确认 | 研究并设计 live graph reconciliation | 独立变更、ADR、失败/外部副作用/等价性证据齐全 | 已废除 |
| `EXTERNAL-001` | Future | XL | 第三方插件需求与威胁模型 | 研究 Wasm 或进程外插件协议 | 权限、版本、签名、升级、sandbox、审计与协议验收齐全 | 已废除 |

## 3. 历史推荐确认粒度

以下顺序已失效，仅说明废除前的计划形态。

不要一次确认 Phase 0-4 的全部实现。推荐顺序：

1. 先确认 `ADR-001`、`CONTRACT-001`、`GRAPH-001`、`TOKEN-001`、`INSPECT-001`，只建立无副作用编译模型；
2. 提交 Graph Compiler 证据后，再确认 Phase 2 的 Kernel 单轨切换；
3. Kernel 等价验证通过后，再确认 Phase 3/4 的应用 contribution 与 Effect；
4. `LIVE-001` 与 `EXTERNAL-001` 永不从本轮概括授权推导。

## 4. 逐轮证据

| 轮次 | 日期 | 完成任务 | 证据 | Commit | 剩余风险 |
| --- | --- | --- | --- | --- | --- |
| 1 | 2026-08-15 | `RES-001`、`RES-002`、`PLAN-001` | 本地 HEAD `28fbc7a`；DeepSeek Harness `47f9438`；vendored Cordis 4.0.0-rc.7/upstream `56b3d4f`；论文 Draft 2026-08-13；Go 官方 `plugin` 文档 | 未提交（计划门禁） | Cordis/DeepSeek 处于快速变化期；目标 API 尚未由编译原型验证 |

## 5. 当前停止条件

永久停止 018 实施。任何后续消息都不得恢复或确认本任务；如需重新研究插件架构，必须建立新的变更编号、说明与 019 的关系并重新经过研究与确认门禁。
