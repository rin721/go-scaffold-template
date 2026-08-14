# 020：复制型脚手架产品形态

## 当前状态

- 任务类型：用户决策后的产品形态固化与隔离复制验证。
- 研究门禁：已通过，证据为 [R001 当前分发边界](research/R001-current-distribution-boundary/report.md)、[R002 Go 分发和版本语义](research/R002-go-distribution-versioning/report.md) 与 [R003 隔离复制验证](research/R003-isolated-copy-validation/report.md)。
- 产品形态决策：用户于 2026-08-15 明确选择“复制脚手架源码”，不由脚手架创建项目，不采用 generator、library-only 或 Runtime + generator 形态。
- 源仓库变化：GitHub canonical 仓库已改名为 `go-scaffold-template`；020 完成后，[021](../021-repository-identity-migration/README.md) 已独立迁移本地 remote、module 与产品身份。
- 计划状态：已确认并完成隔离复制验证；[ADR-001](decision.md) 已接受 copy-owned 单轨产品形态。
- 验证快照：`main@bba180266cba99ec84e2da0296df7fca373764b4`。
- 平台范围：Windows 已通过；当前机器没有可运行 WSL 发行版，Linux 未验证。
- 来源：[019 的 `FORM-001`](../019-http-api-maturity-gap-assessment/tasks.md)。

## 一句话结论

当前仓库本身就是一份版本化、可运行、可复制的完整服务源码基线。使用者复制某个发布快照后，一次性替换项目身份并建立自己的 Git 历史；`pkg`、`internal/kernel/app`、底层 composition、业务模块和交付文件全部归新项目所有，不再依赖源脚手架，也不接受后续 generator 覆盖。

```text
go-scaffold-template release/tag
          │ 复制源码快照，不运行 scaffold create
          ▼
新项目工作区
  ├─ 一次性迁移 module / app / env prefix 等身份
  ├─ 保留或删除 Todo 示例
  ├─ 完整验证
  └─ 建立独立 Git 历史，之后自行演进
```

## 明确排除

- 不开发 `scaffold new`、`init`、模板渲染器或项目生成 DSL。
- 不把 Kernel/Plan/Host 搬成供外部项目依赖的公共 Runtime module。
- 不把上游 Git merge、自动重生成或文件覆盖作为升级机制。
- 不维护“复制模式”和“generator 模式”两套当前入口。

## 仍需验证的问题

1. 一个干净源码快照能否排除 `.git`、运行数据和本机状态后完整复制？
2. module path、应用名、可执行名、环境变量前缀和文档身份是否能一次性迁移且无残留？
3. `pkg -> internal/kernel/app -> internal/kernel/composition -> internal/composition` 是否能在新 module 中原样编译和运行？
4. Todo 示例能否被明确保留，或按清单完整移除而不破坏底座？
5. 如何记录复制来源，并诚实发布后续安全修复和人工迁移说明？

## 阅读顺序

1. [需求与验收标准](requirements.md)
2. [复制、身份迁移与版本设计](design.md)
3. [ADR-001 决策结果](decision.md)
4. [验证任务与证据](tasks.md)
5. [研究档案](research/README.md)

## 当前行为边界

020 只在忽略目录验证了改名前固定 baseline，没有修改当前仓库的 `cmd`、`internal`、`pkg`、配置、依赖、脚本或 CI。正式复制指南、release baseline、安全公告模板、Linux CI 和 canonical identity 迁移仍需独立任务实施。
