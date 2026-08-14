# ADR-001：采用 copy-owned source scaffold

- 状态：已接受
- 日期：2026-08-15
- 依据：[R001](research/R001-current-distribution-boundary/report.md)、[R002](research/R002-go-distribution-versioning/report.md)、[R003](research/R003-isolated-copy-validation/report.md)

## 决策

本项目只采用 **copy-owned source scaffold**：发布一个经过验证的完整 tracked source baseline，使用者复制该快照、迁移身份并建立独立 Git 历史。复制后的 `cmd`、`internal`、`pkg`、配置、测试、文档和 CI 全部归新项目所有。

不提供 generator、模板 DSL、公共 Kernel Runtime module、generator-owned 文件、运行期脚手架依赖或自动覆盖/merge 升级机制。Todo 是默认保留的学习示例，但不是底座依赖；用户可以按完整清单删除它并得到只返回 404 的最小 HTTP 服务基线。

## 底层装配不变量

复制不会改变当前显式装配模型：

```text
pkg capability contract/adapter
  <- internal/kernel/app component definition
  <- internal/kernel/composition plan and capabilities
  <- internal/composition application root
  <- cmd/app process entry
```

`pkg` 不导入 Kernel。`internal/kernel/app` 依赖并封装 `pkg` 能力；`internal/kernel/composition` 明确选择 Component 并输出 typed `Capabilities`；`internal/composition` 是唯一同时知道底层能力和应用模块的地方。业务模块不得接收万能容器、Kernel、Plan 或 Resolver，只接收使用方所需的最小契约。

## Baseline 与 provenance

可复制 baseline 必须由 tag/release 或完整 commit 标识。新项目至少记录：

- source repository；
- source tag/release（存在时）与完整 commit；
- copied-at 日期；
- 产品模型为 copy-owned；
- 已选择保留还是删除示例。

这些字段只用于追溯和判断公告适用性，不建立 Go dependency、Git remote 或上游文件所有权。

## 升级与安全修复

普通改进、缺陷修复和破坏性变化都通过 release notes 与按版本编写的人工迁移指南传播。安全公告必须至少包含 advisory ID、严重级别、受影响 baseline/tag/commit、修复 release/commit、受影响文件和行为、迁移步骤、兼容风险与验证命令。已复制项目自行审阅和移植，不承诺无冲突升级。

## 验证结论与限制

`main@bba1802` 的保留示例和删除示例两条路径均在 Windows 通过 build、test、vet、config init、身份残留和独立 Git 边界检查。WSL 没有可运行发行版，因此 Linux 未验证。021 改变 canonical repository identity 后，正式复制指南和 release baseline 仍需用新身份复核；该缺口不改变本 ADR 的产品形态与装配结论。

## 后续独立工作

1. 021 完成 repository/module/产品身份迁移。
2. 建立正式 copy checklist、identity migration matrix 与 Linux CI。
3. 定义 release baseline、provenance 文件模板、安全公告和人工迁移指南模板。
4. 在 019 的后续变更中研究单一 API authority；不以 generator 模板或外部 Runtime 为前提。
