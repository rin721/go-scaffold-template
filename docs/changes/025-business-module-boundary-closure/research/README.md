# 025 研究档案

本目录记录业务模块归属原则与当前代码之间的偏差。研究格式以 [研究档案与报告](../../../research/README.md) 为准，当前实现仍以代码和项目权威主题文档为准。

## 检索与复核范围

- 既有档案：016、017、024-R005、024-R006。
- 当前代码：`cmd/app`、`internal/composition`、`internal/module`、`internal/transport/http`、`internal/kernel/composition/architecture_test.go`。
- 当前契约：`api/openapi.yaml`、生成的 strict Chi binding、Todo/Auth/Ops/Migration 模块输出与 package graph 门禁。
- 用户决策：业务能力默认收口在模块；第三方 SDK、Client、cache 或 goroutine 本身不构成 Kernel Capability 升级理由。

## 记录

| ID | 问题 | 状态 | 结论用途 |
| --- | --- | --- | --- |
| [R001](R001-current-business-module-boundary/report.md) | 当前代码是否落实模块收口原则，哪些资产应保留在应用级，哪些必须迁回业务模块？ | active | 支撑 025 的迁移边界、依赖方向、治理门禁和验证方案 |

## 复用边界

R001 只证明 `02a1768` 快照下的当前偏差与可实施路径，不证明 025 已经实施。新增第二个 HTTP 业务模块、改变 OpenAPI authority、引入跨模块共享资源或修改 Application Generation 语义时必须刷新研究。
