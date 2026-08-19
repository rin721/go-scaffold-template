# 033 研究档案

本目录保存 033 任务的研究记录。研究先于计划；任何结论必须能被源码、配置、测试、composition 与已确认决策中的证据支持，事实与推断分开标记。

## 索引

- [R001 业务模块统一契约清单与现状缺口](R001-module-contract-inventory/report.md)：梳理业务模块应遵循的统一契约（HTTP binding、config、cli、migration、i18n binding、middleware），并逐模块核对现状缺口。
- [R002 统一 binding 契约与 i18n 接入设计](R002-unified-binding-contract-and-i18n/report.md)：给出统一 binding 契约清单、业务模块自提供 i18n 语言资源 + binding 的接入设计、Ops/Auth/Migration 对齐路径，以及 kernel/app vs pkg 边界保留。

## 记录要求

每份研究记录固定包含 `metadata.yaml` 与 `report.md`；检索从 metadata 进入，证据以 report 为准。
