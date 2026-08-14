# R001：GitHub 改名后的当前仓库身份核验

> 本报告记录迁移前 `main@bba1802` 快照。020 随后先完成于 `cc20b62`，021 的迁移后当前证据见 [R002](../R002-post-migration-verification/report.md)。

## 1. 研究问题

用户说明当前 Git 仓库已改名。需要核验 GitHub canonical 地址与本地 checkout 是否一致，并确定 repository URL、Go module、源码 imports、运行品牌、当前文档、020 已开始的隔离验证和历史记录的迁移边界。

## 2. 方法与范围

- 从 `git status -> git remote -v -> go.mod -> cmd/app -> boundary/architecture tests -> 当前 README` 检查本地事实。
- 对跟踪文件分别检索 `github.com/rin721/go-scaffold2`、`go-scaffold2` 和精确的 `github.com/rin721/go-scaffold`。
- 打开 GitHub 官方仓库页面，核验新地址、旧地址重定向和相邻同名仓库。
- 复核 [020-R001](../../../020-scaffold-product-form/research/R001-current-distribution-boundary/report.md) 的 snapshot、状态和刷新触发器，并检查 020 当前任务状态与隔离目录。

本研究快照为本地 `main@bba1802`，日期为 2026-08-15。没有执行 `fetch`、修改 remote、改 module、启动服务、暂存或提交。

## 3. 可复核事实

### 3.1 GitHub canonical 身份已经改变

- [新仓库页面](https://github.com/rin721/go-scaffold-template) 当前显示 `rin721/go-scaffold-template`，内容与本 checkout 的 `go-scaffold2` README 和目录一致。
- [本地旧 remote 对应页面](https://github.com/rin721/go-scaffold2) 当前重定向到 `go-scaffold-template`。
- 用户消息写的是 `go-scaffold -> go-scaffold-template`，但 [go-scaffold 仓库](https://github.com/rin721/go-scaffold) 仍独立存在，具有不同目录、README 和提交历史。因此当前 checkout 的可验证迁移源是 `go-scaffold2`，不能修改或覆盖另一个 `go-scaffold` 仓库。

### 3.2 本地 Git 身份尚未同步

- 本地 `origin` 的 fetch/push URL 仍是 `git@github.com:rin721/go-scaffold2.git`。
- 当前分支为 `main`，相对已缓存的 `origin/main` 显示 ahead 1；该缓存状态不是 rename 后远端最新状态的证明。
- 工作区已有 `AGENTS.md` 的行尾状态差异，内容 Diff 为空。本任务必须保留该用户状态，不把它纳入迁移修改或提交。

### 3.3 Go module 与运行品牌尚未同步

- `go.mod` 仍声明 `module github.com/rin721/go-scaffold2`。
- 排除 `docs/changes/**` 后，有 105 个跟踪文件包含旧 module path，有 109 个跟踪文件包含 `go-scaffold2`。
- 旧 module path 分布在生产代码、测试、boundary/architecture test 常量和 `pkg/**/README.md` 示例中；只改 `go.mod` 会导致内部 import 与模块声明不一致。
- `cmd/app/main.go` 的 `applicationName`、入口测试、根 README 和 `config.example.yaml` 仍使用 `go-scaffold2` 产品名。
- 当前跟踪文件没有精确引用 `github.com/rin721/go-scaffold` 作为本 module；不能用模糊替换把相邻仓库名称卷入迁移。

### 3.4 020 已确认验证遇到 baseline 变化

- `docs/changes/020-scaffold-product-form/tasks.md` 已记录用户确认，`COPY-001` 为“实施中”。
- 忽略目录 `tmp/scaffold-copy-validation/` 已存在 `baseline-bba1802.tar`、`todo-service/` 与 `minimal-service/`，这些是改名前 module identity 的隔离证据，不属于 021 的生产源码修改范围。
- 新 canonical repository identity 会改变 020 的 source baseline 命名与后续残留判定。已有隔离证据应保留，但不得继续把旧 identity 的结果直接当作新 canonical baseline 验收结论。

### 3.5 历史记录与当前说明需要分治

- `docs/changes/**` 中已有 20 个文件记录 `go-scaffold2` 的历史设计或快照。它们不是全部都应重写的当前使用说明。
- `020` 是正在实施的产品形态验证，GitHub rename 已触发其 baseline 身份刷新；它应暂停并依赖 021 完成，而不是抹除已经获得确认和已生成证据的历史。
- 根 README、包 README、入口、测试和未完成计划属于当前身份范围；已完成任务中的旧 commit、旧 module、旧应用名和研究快照应保留并标明历史语境。

### 3.6 当前迁移清单可以按所有者精确划分

- 精确旧 module path 当前命中 88 个 `.go` 文件、17 个 `.md` 文件和 1 个 `go.mod`。17 个 Markdown 中，16 个是当前 `pkg/**/README.md` import 示例，另 1 个是 020 的旧 snapshot 报告。
- `pkg/boundary_test.go` 与 `internal/kernel/app/boundary_test.go` 没有连续写出完整 module path，而是通过字符串片段构造 `go-scaffold2` 前缀；残留扫描必须覆盖这种语义构造，不能只搜索完整 URL。
- 排除 `docs/changes/**` 后，纯品牌且不包含连续旧 module path 的文件只有根 `README.md`、`config.example.yaml` 和上述两个 boundary test。`cmd/app` 的应用名与测试同时位于已命中旧 import 的 Go 文件中。
- `.github/**`、`.agents/**` 和当前交付脚本未发现 `go-scaffold2` 或旧 repository URL。实施时仍应复核，但没有证据支持为了“看起来完整”而修改这些文件。
- 历史 metadata 中的 `go-scaffold2` 是研究问题、标题和适用场景快照；除 020 与 021 的当前关联说明外，不属于生产 identity 迁移清单。

## 4. 推断与计划影响

1. **仓库 URL、Go module 和产品名需要同一轮单轨迁移。** 只依赖 GitHub redirect 会让 `go.mod`、文档和复制 baseline 长期保留旧身份。
2. **不能全局替换所有 `go-scaffold2`。** 当前代码与使用文档需要迁移，历史证据需要保留；两类文件的所有权不同。
3. **`APP_`、`config.yaml` 和本地目录名不属于本次 rename 的必然变化。** 它们是下游项目一次性 identity 初始化项，除非另有产品决策，不应随 repository rename 扩大范围。
4. **先验证 canonical 远端，再修改本地状态。** 实施时应先确认新 URL 可访问、更新 `origin` 并 fetch；失败时停止，不能在远端事实不明时继续修改或提交。
5. **020 的既有确认不能跨 baseline 复用。** 021 应暂停而非删除 020 证据；完成后需重新提交基于新 canonical identity 的剩余计划报告。
6. **实施文件集合可由语义扫描闭包确定。** 旧完整 module path、拆分构造的 boundary prefix、当前产品名和当前文档四组扫描共同覆盖迁移；CI/脚本仅验证无命中，不预设修改。

## 5. 适用与不适用

本研究适用于当前 checkout 的 021 迁移和 020 baseline 刷新。不适用于另一个 `go-scaffold` 仓库，也不定义下游复制项目的最终 module、应用名或环境变量前缀。

## 6. 局限与剩余未知

- 计划阶段没有执行 `fetch`，因此没有把已缓存 `origin/main` 当作 rename 后的远端最新证明。
- 尚未验证 module/import 替换后的 build、test、vet、race test 和文档残留扫描。
- 020 隔离副本的验证进度只核对了目录与任务状态，未在本任务中复跑或解释全部结果。
- 尚未确认用户是否希望重命名本地目录；本计划将其排除，因为它不是 module 或 remote 正确性的必要条件，且会改变工作区外层路径。

## 7. 研究门禁

研究门禁通过。关键身份、冲突证据、迁移边界和验证路径均可复核；剩余未知不妨碍形成计划，但所有 Git/source 状态变更仍需后续明确确认。
