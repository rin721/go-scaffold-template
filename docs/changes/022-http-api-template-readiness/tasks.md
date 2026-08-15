# 任务：HTTP API 脚手架成熟化 Program

## 1. 门禁状态

- 022 研究门禁：已通过。
- 022 文档计划：已完成，属于纯文档交付。
- 当前成熟度：Foundation-ready；Copy-ready 部分通过；Production HTTP API-ready 未通过。
- 非文档实施：未授权。后续 Program item 必须各自建立新变更目录、完成研究和计划，并在计划报告后的独立消息中获得确认。

## 2. 本轮任务

| ID | 工作量 | 依赖 | 内容 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| `RES-001` | L | 019/020/021 | 复核当前 HTTP、Kernel、模块、CI、复制与 identity 快照 | R001 区分已通过、部分通过、未通过和场景触发能力 | 已完成 |
| `RES-002` | S | `RES-001` | 复核 019-R002 状态、日期与刷新触发器 | 参考模型仍 active，不复制第二份标准摘要 | 已完成 |
| `VERIFY-001` | M | `RES-001` | 执行当前 Windows test/race/vet/build/tidy 与交付资产扫描 | 成功与失败均记录，不修改依赖或行尾 | 已完成 |
| `PLAN-001` | XL | `RES-001`、`RES-002` | 建立成熟标签、硬门禁、目标架构、依赖顺序和验收 | requirements/design/tasks 一致且不声称目标已实现 | 已完成 |

## 3. 后续独立变更 Program

| Program ID | 优先级 | 前置 | 目标 | 完成门禁 | 当前状态 |
| --- | --- | --- | --- | --- | --- |
| `PORTABILITY-001` | P0 | 无 | 统一 module/行尾与 Windows/Linux validation manifest | `BASE-003`，两平台 `tidy/test/race/vet/build` 同义通过 | 未立项 |
| `API-AUTHORITY-001` | P0 | 无 | 用 Todo 隔离原型比较 spec-first 与 typed code-first，ADR 单轨决策 | 选型证据覆盖生成、diff、security、DTO 边界和 DX | 未立项 |
| `API-CONTRACT-001` | P0 | `API-AUTHORITY-001` | 建立 Operation、OpenAPI、Router、inventory 与 compatibility 单一权威 | `API-001` 至 `API-004` | 未立项 |
| `PROTOCOL-001` | P0 | `API-CONTRACT-001` | 严格 decode/encode、problem、validation、404/405/panic 单轨迁移 | `PROTO-001`、`PROTO-002` | 未立项 |
| `EDGE-001` | P0 | `API-CONTRACT-001` | trusted proxy、budget、limits、CORS/CSRF、rate/overload policy | `PROTO-003`、`EDGE-001` | 未立项 |
| `MANAGEMENT-001` | P0 | 无 | 独立 management listener、startup/live/ready、diagnostics/build info | `MGMT-001`、`MGMT-002` | 未立项 |
| `OBSERVABILITY-001` | P0 | `API-CONTRACT-001`、`MANAGEMENT-001` | OTel Adapter、trace/metric/log correlation、cardinality/redaction policy | `OBS-001`、`OBS-002` | 未立项 |
| `SECURITY-001` | P0 | `API-CONTRACT-001`、真实 actor | 显式 access policy、Principal、认证 Adapter、对象授权和审计 | `SEC-001` 至 `SEC-004` | 等待真实场景 |
| `MIGRATION-001` | P0 | 生产部署模型 | versioned migration、lock、独立 command/job、expand-contract | `DATA-001`、`DATA-002` | 等待部署场景 |
| `DELIVERY-001` | P0 | `MANAGEMENT-001`、`MIGRATION-001` | build metadata、容器、部署 smoke、quality/security/supply-chain CI | `DELIVERY-001` 至 `DELIVERY-003` | 未立项 |
| `RELEASE-001` | P0 | `PORTABILITY-001`、`DELIVERY-001` | 正式复制指南、tag/version、provenance、安全公告与迁移说明 | `BASE-001`、`BASE-002`、`BASE-004` | 未立项 |
| `ACCEPTANCE-001` | P0 | 全部 baseline item | 两个复制副本、双平台、协议/安全/失败/release 场景验收 | `ACCEPT-001` 至 `ACCEPT-005` | 未立项 |

## 4. 推荐启动顺序

第一批只启动三个相互低耦合的独立研究/计划，不直接合并为一个源码任务：

1. `API-AUTHORITY-001`：决定所有协议、安全与观测 metadata 的唯一来源，是最长关键路径；
2. `PORTABILITY-001`：解决本轮暴露的 Windows `go mod tidy -diff` 行尾不一致，并建立双平台门禁；
3. `MANAGEMENT-001`：研究可消费健康语义和 listener 隔离，可在 API authority 原型期间独立推进。

后续按 `API-CONTRACT -> PROTOCOL/EDGE -> OBS/SEC` 和 `MIGRATION -> DELIVERY -> RELEASE -> ACCEPTANCE` 推进。具体认证和 production migration 在真实 actor/部署模型出现前保持研究状态。

## 5. 逐轮证据

| 轮次 | 日期 | 完成任务 | 证据 | Commit | 剩余风险 |
| --- | --- | --- | --- | --- | --- |
| 1 | 2026-08-15 | `RES-001`、`RES-002`、`VERIFY-001`、`PLAN-001` | HEAD `fa349ab`；019-R002、020-R003、021-R002；源码/CI/release 扫描；test/race/vet/build 通过；Windows tidy 因 CRLF 返回 1 | 本次纯文档提交 | 所有 Program item 尚未授权；Linux、容器、真实部署、安全与 API contract 均未验收 |

## 6. 停止条件

022 到成熟度结论和 Program 计划即完成。不得因用户最初请求包含“实现目标计划”而实施任何 Program item；源码、配置、依赖、CI、容器、服务、tag、release 或外部写入必须在各自计划报告后的后续确认中授权。
