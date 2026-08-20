# 040 项目文档体系系统重构设计

## 文档体系

正式文档采用一条连续项目路径：

```text
README.md
  -> docs/README.md
      -> 本地启动
      -> 配置与能力使用
      -> 应用开发
      -> 基础设施接入
      -> 架构与生命周期
      -> 扩展底层能力
      -> 调试排障
      -> 运行维护
      -> 底层设计与历史证据
```

根 README 只保留项目定位、五分钟启动、项目手册入口、架构摘要和文档权威边界。`docs/README.md` 是完整项目手册总目录，负责把主题文档串成真实使用顺序。

## Authority 收敛

| 知识类型 | 当前 authority | 其他位置职责 |
| --- | --- | --- |
| 启动、配置、迁移、readiness | `README.md`、`docs/getting-started/local-development.md`、`docs/configuration/README.md`、`docs/operations/migration-and-rollback.md` | 任务记录只保存证据，包 README 不复制流程。 |
| 业务模块和 Binding 契约 | `docs/development/application-module-development.md`、`internal/module/README.md` | 模块 README 只说明本模块局部目录和已实现能力。 |
| 日志、execution、schedule、messaging | `docs/development/*.md` | `pkg/**/README.md` 只说明公共类型、边界和到主题文档的链接。 |
| composition、Application Generation、Kernel App | `docs/architecture/README.md`、`internal/kernel/README.md`、`internal/kernel/app/README.md` | 研究快照只说明当时证据和目标设计边界。 |
| 构建、发布、复制、安全、运行维护 | `docs/operations/README.md` 与其子页 | 根 README 不扩展 runbook。 |
| 研究与变更历史 | `docs/research/**`、`docs/changes/**` | 不替代当前主题文档，不作为操作手册。 |

## 文件影响

新增：

- `docs/changes/040-documentation-system-rebuild/**`
- `pkg/execution/README.md`

修改：

- `README.md`
- `docs/README.md`
- `docs/architecture/README.md`
- `docs/development/README.md`
- `docs/operations/README.md`
- `docs/changes/README.md`
- `pkg/README.md`
- `internal/kernel/README.md`
- `internal/kernel/app/README.md`

## 审计策略

040 不把 353 个 Markdown 文件逐个迁移。审计按文件职责分组：

- 正式入口：根 README、`docs/README.md`、configuration、getting-started、architecture、development、operations、api。
- 局部 README：`api/README.md`、`internal/**/README.md`、`pkg/**/README.md`。
- 历史证据：`docs/changes/**`、`docs/research/**`。

矩阵记录每组文件的当前职责、发现、处理动作和验证证据。对于历史文件中的目标设计、待确认、旧方案等词，不改写历史事实，只确认正式文档没有把它们当作当前规范。

## 维护约束

- 新增或修改能力时，先判断它属于正式主题文档、局部 README、研究快照还是任务证据。
- 项目级契约、Binding、生命周期、配置、装配方式、业务模块接入、基础能力使用、架构原则和门禁规范只能写入对应权威主题；其他文档用链接引用。
- 包级 README 只说明本包公开类型、资源所有权、错误/配置边界和主题链接；不得复制完整业务开发流程。
- 模块级 README 只说明本模块局部目录、binding、运行单元和 contribution；不得成为全项目模块开发规范。
- 历史变更完成后，当前有效结论必须同步回正式主题文档；`docs/changes/**` 不成为第二套现行规范。

## 验证方案

- 用本地脚本检查本轮新增与修改 Markdown 的相对链接，并尽量覆盖全仓 Markdown。
- 搜索一致性关键词，人工区分正式文档与历史证据。
- 静态搜索代码事实，抽样核对 composition、配置 owner、module Contribution、HTTP contract、execution、schedule、messaging 与 pkg 能力入口。
- 执行 `git diff --check`。
- 审阅 `git diff --stat` 和 staged diff，确认没有非文档变更。
