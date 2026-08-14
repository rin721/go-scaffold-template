# 021：仓库身份迁移

## 当前状态

- 任务类型：GitHub 仓库改名后的 repository、Go module 与当前产品身份迁移。
- 研究门禁：已通过，证据见 [R001 当前仓库身份核验](research/R001-current-repository-identity/report.md)。
- GitHub 当前事实：`github.com/rin721/go-scaffold2` 已重定向到 `github.com/rin721/go-scaffold-template`；`github.com/rin721/go-scaffold` 是另一个仍独立存在的仓库。
- 本地实现事实：`origin`、`go.mod`、Go imports、应用名和当前说明已统一为 `go-scaffold-template`。
- 020 关系：隔离复制验证已先行完成于 `cc20b62`；021 保留其改名前 `bba1802` 快照，不重写隔离证据。
- 计划状态：用户已明确确认，迁移与 Go 门禁已完成，提交由本变更收口。
- 外部副作用：只更新本地 `origin` 并 fetch；未 push、tag、release 或修改 GitHub 设置。

## 目标

让本地仓库的 canonical remote、Go module、源码 imports、架构门禁、当前产品名称和使用文档统一迁移到 `github.com/rin721/go-scaffold-template`，同时保留历史变更记录和 020 隔离副本中的真实旧快照，不把另一个 `go-scaffold` 仓库误纳入迁移范围。

## 阅读顺序

1. [研究档案](research/README.md)
2. [需求与验收标准](requirements.md)
3. [迁移设计](design.md)
4. [实施任务与证据](tasks.md)

## 完成边界

021 只迁移源仓库 identity，不改 `APP_`、`config.yaml`、Todo 行为、依赖选择、本地父目录或历史 snapshot。正式复制指南、release、tag、push 与 Linux baseline 刷新仍需独立授权。
