# 需求：仓库身份迁移

## 1. 目标

将当前 checkout 从旧身份 `github.com/rin721/go-scaffold2` 单轨迁移到 GitHub canonical 身份 `github.com/rin721/go-scaffold-template`，使 Git remote、Go module、源码 imports、运行品牌、架构门禁和当前使用文档一致。研究依据见 [R001](research/R001-current-repository-identity/report.md)。

## 2. 功能需求

| ID | 需求 |
| --- | --- |
| `REMOTE-001` | 本地 `origin` 的 fetch/push URL 必须显式改为 `git@github.com:rin721/go-scaffold-template.git`，不得长期依赖 GitHub 旧地址重定向。 |
| `MODULE-001` | `go.mod`、所有当前 Go imports、module/boundary/architecture test 常量和当前包示例统一使用 `github.com/rin721/go-scaffold-template`。 |
| `BRAND-001` | 根 README、当前主题文档、配置示例标题、`cmd/app` 应用名及对应测试统一使用 `go-scaffold-template`。 |
| `HISTORY-001` | 已完成变更中的 commit、旧 module、旧应用名和研究 snapshot 保留历史事实；020 已完成的隔离验证保留改名前证据，不伪称验证了新 canonical baseline。 |
| `RESIDUAL-001` | 对旧 module path 执行归零检查；对纯 `go-scaffold2` 执行分类检查，只允许有明确历史语境的残留。 |
| `DELIVERY-001` | `.github/**`、`.agents/**` 和交付脚本必须复核 repository identity；没有旧引用时保持不变，不制造无意义 Diff。 |
| `VERIFY-001` | 新 module 下的 formatting、module consistency、build、test、race test、vet、文档链接和 Diff 检查通过。 |

## 3. 约束

- `github.com/rin721/go-scaffold` 是另一个独立仓库，不在本任务范围内，任何 remote、源码或文档迁移都不得指向它。
- 不使用无边界的全仓字符串替换；按 module path、运行品牌、当前文档、历史记录四类语义分别迁移。
- 不改变 `APP_` 环境变量前缀、`config.yaml` 默认路径、Todo 行为、公开 API、依赖选择、模块边界或生命周期语义。
- 不重命名当前本地父目录 `go-scaffold2`；路径调整不影响 Git/Go 正确性，且需要独立工作区协调。
- 保留用户已有 `AGENTS.md` 行尾状态差异，不纳入任务 Diff、暂存或提交。
- 保留 `tmp/scaffold-copy-validation/` 中已有 020 隔离证据，不在 021 中删除、重写或宣称其已验证新 canonical baseline。
- 实施前必须验证新 remote 可访问并更新远端快照；失败时停止，不继续代码修改。
- 本任务不授权 push、tag、release 或修改 GitHub 仓库设置。

## 4. 非目标

- 不迁移或修改另一个 `go-scaffold` 仓库。
- 不重复执行 020 已完成的隔离复制、Todo 移除或 release baseline 验证。
- 不新增 generator、初始化器、兼容 module、`replace` 或旧 import alias。
- 不保留旧 module path 的兼容层，也不让 GitHub redirect 成为长期配置。

## 5. 验收标准

1. `git remote -v` 只显示 `go-scaffold-template` canonical URL。
2. `go list -m` 返回 `github.com/rin721/go-scaffold-template`。
3. 当前 Go 源码、测试、架构门禁和包使用示例不存在 `github.com/rin721/go-scaffold2`。
4. 入口应用名、根 README 与当前说明使用 `go-scaffold-template`；相关测试同步通过。
5. `go-scaffold2` 只在明确的历史 snapshot、旧 commit、020 已有隔离证据或迁移说明中保留，残留清单逐项可解释。
6. `.github/**`、`.agents/**` 和交付脚本的复核结果有证据；无旧引用的文件保持不变。
7. `go mod tidy -diff`、`go build ./...`、`go test ./...`、`go test -race ./...`、`go vet ./...` 和 `git diff --check` 通过。
8. 完整 Diff 不包含 `AGENTS.md` 或任务范围外文件，且没有 push、tag、release 等外部写入。
