# 034 研究档案

本目录保存 034 任务的研究记录。研究先于计划；任何结论必须能被源码、composition、contract-gen、权威文档与已确认决策中的证据支持，事实与推断分开标记。

## 索引

- [R001 业务模块装配流程与职责审计](R001-module-wiring-and-responsibility/report.md)：核查模块完成后的接入是否只需在 `internal/composition` 一处装配/注入，是否存在跨层多点修改、反向依赖或职责越界。
- [R002 文档与实现一致性核对](R002-doc-implementation-consistency/report.md)：核对权威文档（架构、模块接入、binding 契约、装配流程、配置、扩展方式、门禁）与当前代码/实际能力是否一致。

## 记录要求

每份研究记录固定包含 `metadata.yaml` 与 `report.md`；检索从 metadata 进入，证据以 report 为准。
