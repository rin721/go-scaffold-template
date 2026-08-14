# R002：仓库身份迁移后验证

## 1. 验证问题

在用户明确确认 021 后，当前 checkout 是否已经把 Git remote、Go module/import、运行品牌与当前使用文档统一到 `github.com/rin721/go-scaffold-template`，同时保留必要历史证据并通过完整门禁？

## 2. 实施基线与范围

- 020 已先完成并提交为 `cc20b62`；021 以该 HEAD 为实施基线，没有回滚或覆盖 020 结果。
- 实施前 `origin` 为 `git@github.com:rin721/go-scaffold2.git`，Go module 和运行品牌仍为 `go-scaffold2`。
- 021 只迁移 repository identity；不修改 `APP_`、`config.yaml`、Todo 行为、依赖版本、本地父目录或另一个 `go-scaffold` 仓库。

## 3. 可复核结果

### 3.1 Git remote

- `origin` 的 fetch/push URL 已显式改为 `git@github.com:rin721/go-scaffold-template.git`。
- `git fetch --prune origin` 成功；迁移前核对 `origin/main...HEAD` 为 `0 2`，没有远端未知提交。
- 没有执行 push、tag、release、rebase、amend 或 force 操作。

### 3.2 Module、imports 与品牌

- `go.mod` 声明 `module github.com/rin721/go-scaffold-template`，`go list -m` 返回同一值。
- 88 个 Go 文件、16 个当前 `pkg/**/README.md` import 示例与两个拆分构造的 boundary prefix 已迁移。
- `cmd/app` 的 `applicationName`、入口测试、根 README 和 `config.example.yaml` 已统一为 `go-scaffold-template`。
- 当前源码和当前使用文档不存在旧 module/品牌；旧值只保留在 `docs/changes/**` 的历史快照、迁移前事实和来源说明中。
- `.github/**`、`.agents/**` 和交付脚本复核无旧 repository identity，因此保持不变。

### 3.3 验证门禁

以下命令在 Windows 上通过：

```text
go list -m
go mod tidy -diff
go build ./...
go test ./...
go test -race ./...
go vet ./...
```

测试输出的 package path 全部使用 `github.com/rin721/go-scaffold-template`。文档相对链接、尾随空白、身份残留与 `git diff --check` 在最终文档写回后复核。

## 4. 历史与当前边界

- 020-R001 的 `github.com/rin721/go-scaffold2` 是 `af7fdadc` 时的真实 module 快照，不应篡改。
- 020-R003 验证的是改名前 `bba1802` 隔离副本；021 不把它伪称为新 canonical baseline 验证。
- R001 记录迁移前状态，由本记录单轨取代为当前实施证据。

## 5. 局限与后续触发器

- 本次没有发布 release/tag，也没有 push；远端正式发布状态不能由本地提交推断。
- 020 的 Linux baseline 仍未验证，正式复制指南仍需在新 identity 上建立独立任务。
- repository、module 或应用名再次变化时必须刷新本记录和残留门禁。

## 6. 结论

021 的 repository identity 已在本地 checkout 中完成单轨迁移。当前实现、测试和使用文档以 `go-scaffold-template` 为唯一现行身份；旧身份只作为明确历史证据保留。
