# 任务：底层闭环与 HTTP API 脚手架成熟化 Program

## 1. 门禁状态

- 022 研究门禁：已通过。
- 022 文档计划：Program 总计划已完成；`FOUNDATION-DIAGNOSTICS-001` 的独立研究与施工计划已完成，属于纯文档交付。
- 当前底层：Foundation-partial；生命周期 P0 与统一 diagnostics 已闭环，EnvSource 冲突与完整 Foundation acceptance 仍未通过。
- 业务模块详细设计：阻断，等待 Foundation P0 验收和具体 profile assessment。
- Production HTTP API-ready：未通过。
- 非文档实施：`FOUNDATION-LIFECYCLE-001` 与 `FOUNDATION-DIAGNOSTICS-001` 均已由用户在各自计划报告后的后续消息明确确认并完成；其他 Program item 仍需形成各自计划并单独确认。
- 本轮边界：只实施并提交 `DGN-001` 至 `DGN-009`；不启动服务、不部署、不推送，不实施 EnvSource、management transport 或 HTTP API 产品治理。

## 2. 本轮研究与计划任务

| ID | 工作量 | 依赖 | 内容 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| `RES-001` | M | 现有 022 | 检查 022 metadata 与 012/017/019/020/021 研究刷新条件 | 明确复用、当前复核和新增记录原因 | 已完成 |
| `RES-002` | XL | `RES-001` | 从配置输入追踪装配、资源、运行、reload、drain、stop、diagnostics、tests | R002 给出十一门结论和代码证据 | 已完成 |
| `RES-003` | L | `RES-002` | 定向研究 Go/Fx/controller-runtime/dskit/Caddy/Wire 官方主源 | R003 给出相同点、差异、方案取舍与证据强度 | 已完成 |
| `SYN-001` | L | `RES-002`、`RES-003` | 综合保留、补齐、优化、不重设计项和业务解锁条件 | R004 形成推荐与未知清单 | 已完成 |
| `PLAN-001` | XL | `SYN-001` | 修订 requirements/design/acceptance/tasks | Foundation 与 HTTP 两层门禁、依赖顺序和停止线一致 | 已完成 |
| `RES-004` | XL | `RES-002`、用户策略输入 | 逐项追踪 Database、Redis、logger、Storage、HTTP、fsnotify 和派生资源的 partial build、drain、Close/retry/force/verification | R005 明确事实、场景分类、CLI 边界与剩余未知 | 已完成 |
| `PLAN-002` | L | `RES-004` | 按“统一状态治理 + 场景化关闭策略”纠正 lifecycle requirements/design/acceptance/tasks | 不再假设 Close 幂等即可 retry，不再设计万能 Close | 已完成 |
| `PLAN-003` | XL | `PLAN-002` | 在 022 内建立 `FOUNDATION-LIFECYCLE-001` 唯一施工计划 | 冻结 Go 契约、状态机、逐资源政策、文件影响、迁移、任务 ID 和精确测试，状态为待确认 | 已完成 |
| `RES-005` | L | `FOUNDATION-LIFECYCLE-001` 已实施 | 在当前 HEAD 追踪 Kernel app、Coordinator、Supervisor、Host、Service/CLI 的 diagnostics 数据源、终态、预算、脱敏和并发边界 | R006 给出字段矩阵、代码证据、复用/刷新判断与方案取舍 | 已完成 |
| `PLAN-004` | XL | `RES-005` | 建立 `FOUNDATION-DIAGNOSTICS-001` 唯一施工计划并同步 022 authority | 冻结 typed ledger、Host 单一快照、文件影响、`DGN-001..009`、精确测试和重新确认线，状态为待确认 | 已完成 |

## 3. Foundation 后续实施项

| Program ID | 优先级 | 前置 | 研究/计划目标 | 完成门禁 | 当前状态 |
| --- | --- | --- | --- | --- | --- |
| `FOUNDATION-LIFECYCLE-001` | P0 | 022-R002/R003/R004/R005 | 实现统一 owner/state/budget 引擎与场景化 finalization Adapter，修复 terminal drain、部分构造和 cleanup debt | `FND-LIFECYCLE-001` 至 `008`，`FND-ACCEPT-001/002` | [已确认并实施完成](plans/foundation-lifecycle-001.md) |
| `FOUNDATION-DIAGNOSTICS-001` | P0 | `FOUNDATION-LIFECYCLE-001` 已实施、022-R006 | 统一 Kernel/Coordinator/Supervisor responsibility、generation、phase、policy、budget、verification 和 terminal diagnostics | `FND-RUNTIME-002`、`FND-DIAGNOSTICS-001/002`、`FND-ACCEPT-003` | [已确认并实施完成](plans/foundation-diagnostics-001.md) |
| `FOUNDATION-CONFIG-001` | P0 | 022-R002 | 统一 EnvSource 与 Loader 的结构冲突语义，拒绝顺序相关路径 | `FND-CONFIG-001` 至 `003`、`FND-ACCEPT-004` | 未立项 |
| `FOUNDATION-ACCEPTANCE-001` | P0 | 上述三项 | 故障注入、真实资源、Service/CLI、race/vet/build 和十一门复核 | `FND-GOV-001` 至 `004`、`FND-ACCEPT-005` | 未立项 |
| `FOUNDATION-RECONCILIATION-001` | P1 | Foundation P0 | 研究 sticky RestartRequired 的恢复与人工干预语义 | 不破坏提交前原子和 degraded 诚实性 | 未立项 |
| `MODULE-RUNTIME-PROFILE-001` | 场景触发 | 首个后台/health 需求 | 为真实 Runner/Ready/Health 场景设计最小贡献契约 | `BIZ-UNLOCK-002` 至 `004`，无万能 hook/locator | 等待真实场景 |

### 3.1 `FOUNDATION-LIFECYCLE-001` 当前计划

唯一施工级正文为 [`plans/foundation-lifecycle-001.md`](plans/foundation-lifecycle-001.md)。`LFC-001` 至 `LFC-010` 已完成：Kernel instance slot 与 terminal result cache、可继续 Stop、Supervisor 总/force budget、HTTP graceful/force 拆分、逐资源终结修复、unused `pkg/resource` 删除和权威文档同步均已落地；本文件不复制第二套实现细节。

### 3.2 `FOUNDATION-DIAGNOSTICS-001` 当前计划

实施前事实与取舍由 [R006](research/R006-unified-runtime-diagnostics/report.md) 支撑，唯一施工级正文与实施证据为 [`plans/foundation-diagnostics-001.md`](plans/foundation-diagnostics-001.md)。`DGN-001` 至 `DGN-009` 已完成：Kernel/Supervisor 各自维护 typed responsibility ledger，Host 提供单一 process diagnostics，并覆盖共享 shutdown budget、pending/failed/forced/finalized 分类、release verification、并发脱敏快照和 Service/CLI/Host 验收。

## 4. HTTP API 成熟化后续 Program

| Program ID | 优先级 | 前置 | 目标 | 完成门禁 | 当前状态 |
| --- | --- | --- | --- | --- | --- |
| `PORTABILITY-001` | P0 | 无 | 统一 module/行尾与 Windows/Linux validation manifest | `BASE-003`，两平台同义通过 | 未立项 |
| `API-AUTHORITY-001` | P0 | `FOUNDATION-ACCEPTANCE-001` | Todo 隔离原型比较 spec-first/typed code-first，ADR 单轨决策 | 证据覆盖 schema/security/diff/DX | 未立项 |
| `API-CONTRACT-001` | P0 | `API-AUTHORITY-001` | 建立 Operation/OpenAPI/Router/inventory/compatibility 单一权威 | `API-001` 至 `003` | 未立项 |
| `PROTOCOL-001` | P0 | `API-CONTRACT-001` | strict decode/encode、problem、validation、404/405/panic 单轨迁移 | `PROTO-001/002` | 未立项 |
| `EDGE-001` | P0 | `API-CONTRACT-001` | trusted proxy、budget、limits、CORS/CSRF、rate/overload | `EDGE-001` | 未立项 |
| `MANAGEMENT-001` | P0 | `FOUNDATION-ACCEPTANCE-001` | 独立 management listener、startup/live/ready、dependency contribution、diagnostics/build info | `MGMT-001/002` | 未立项 |
| `OBSERVABILITY-001` | P0 | `API-CONTRACT-001`、`MANAGEMENT-001` | OTel Adapter、trace/metric/log correlation、cardinality/redaction | `OBS-001/002` | 未立项 |
| `SECURITY-001` | P0 | `API-CONTRACT-001`、真实 actor | access policy、Principal、认证 Adapter、对象授权、审计 | `SEC-001` 至 `003` | 等待真实场景 |
| `MIGRATION-001` | P0 | 生产部署模型 | versioned migration、lock、独立 command/job、expand-contract | `DATA-001/002` | 等待部署场景 |
| `DELIVERY-001` | P0 | `MANAGEMENT-001`、`MIGRATION-001` | build metadata、容器、部署 smoke、quality/security/supply-chain CI | `DELIVERY-001` 至 `003` | 未立项 |
| `RELEASE-001` | P0 | `PORTABILITY-001`、`DELIVERY-001` | 复制指南、tag/version、provenance、安全公告与迁移说明 | `BASE-001/002/004` | 未立项 |
| `ACCEPTANCE-001` | P0 | 全部 baseline item | 两副本、双平台、生命周期/协议/安全/release 失败场景验收 | acceptance 第 5 节 | 未立项 |

## 5. 推荐启动顺序

第一条关键路径先于业务设计和 API product work：

1. `FOUNDATION-LIFECYCLE-001`：已完成；后续以当前代码、权威文档和本轮验证证据为准；
2. `FOUNDATION-DIAGNOSTICS-001`：让所有未结束责任可定位，不以 error string 代替状态；
3. `FOUNDATION-CONFIG-001`：可与前两项研究并行，但合入前统一验收；
4. `FOUNDATION-ACCEPTANCE-001`：故障注入后复核十一门，决定是否解锁同步业务 profile；
5. Foundation 通过后，`API-AUTHORITY-001`、`MANAGEMENT-001` 与 `PORTABILITY-001` 可独立推进；
6. 再按 `API-CONTRACT -> PROTOCOL/EDGE -> OBS/SEC` 和 `MIGRATION -> DELIVERY -> RELEASE -> ACCEPTANCE` 推进。

不得在 Foundation P0 期间并行细化新的 Handler/Service/Repo/Model。现有 Todo 只作为证据和后续故障验收对象，不据此扩张业务范围。

## 6. 逐轮证据

| 轮次 | 日期 | 完成任务 | 证据 | Commit | 剩余风险 |
| --- | --- | --- | --- | --- | --- |
| 1 | 2026-08-15 | 原 `RES-001/002`、`VERIFY-001`、`PLAN-001` | source `fa349ab`；019-R002、020-R003、021-R002；test/race/vet/build 通过；Windows tidy 因 CRLF 返回 1 | `b868893` | 当时把底层正向路径误判为完整 Foundation-ready |
| 2 | 2026-08-15 | `RES-001/002/003`、`SYN-001`、`PLAN-001` | HEAD `b868893` 静态全链审计；012/017 研究刷新判断；Go/Fx/controller-runtime/dskit/Caddy/Wire 官方主源；十一门矩阵 | 无，按用户要求保持未提交 | 未执行运行测试；资源 retry/force、部署预算、首个后台业务仍未知 |
| 3 | 2026-08-15 | `RES-004`、`PLAN-002` | HEAD `b868893` 逐资源调用链；Go 1.25.7、go-redis v9.22.0、fsnotify v1.10.1、zap v1.28.0、AWS S3 v1.107.0 官方源码/文档；R005 场景矩阵 | 无，按用户要求保持未提交 | 未执行故障注入；部署总预算/第二次信号、HTTP hijacked、外部资源真实 close error 待后续验证 |
| 4 | 2026-08-15 | `PLAN-003` | `plans/foundation-lifecycle-001.md`；当前 HEAD/调用方/公共接口复核；022 导航与门禁单轨引用 | 无，按用户要求保持未提交 | 源码未实施；需用户后续明确确认当前计划；外部真实资源与部署信号仍待实施/后续研究 |
| 5 | 2026-08-15 | `LFC-001..010` | instance generation/terminal cache、可继续 drain、classified Supervisor snapshot、HTTP explicit force、Database Ready Ping、watcher/workbook error join、Redis/Logger/SQLite/文件释放测试；`go test ./...`、`go test -race ./...`、`go vet ./...`、`go build ./cmd/app` 通过 | `7d64f86` | 真实 PostgreSQL/MySQL/S3、HTTP hijacked/WebSocket、部署 grace/第二次信号未验证；统一 diagnostics、EnvSource 和 Foundation 总验收仍待后续 Program |
| 6 | 2026-08-15 | `RES-005`、`PLAN-004` | HEAD `7d64f8634c59375a522e66b5b989dd40b557ee9d`；Coordinator/Kernel app/Supervisor/Host/Service/CLI 调用链；`go test ./internal/kernel/... ./pkg/supervisor ./pkg/httpx` 通过；R006 字段矩阵与 diagnostics 施工计划 | 无，计划阶段按门禁保持未提交 | `DGN-001..009` 待用户后续明确确认；EnvSource、Foundation 总验收、management transport 和部署信号仍不在范围 |
| 7 | 2026-08-15 | `DGN-001..009` | Kernel ownership ledger、Supervisor unit/budget ledger、Host `ProcessDiagnostics`、Health authority、Service/CLI 与并发/脱敏/终态测试；完整验证命令见施工计划第 14 节 | 本任务提交 | EnvSource、Foundation 总验收、management transport、外部真实资源和部署信号仍不在范围 |

## 7. 停止条件

本轮在 `DGN-001..009`、完整验证、Diff 审计和本任务提交完成后停止。不得顺带实施 `FOUNDATION-CONFIG-001`、HTTP API 产品治理、服务启动、部署或推送；后续非文档实施仍必须等待对应计划报告后的明确确认。
