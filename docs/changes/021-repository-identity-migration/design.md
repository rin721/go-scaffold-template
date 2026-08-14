# 设计：repository、module 与产品身份单轨迁移

## 1. 迁移原则

GitHub rename 只改变远端 canonical 地址，不会自动修改本地 `.git/config`、`go.mod`、Go imports、应用名或文档。本任务把这些属于源脚手架的当前身份放在同一轮迁移，避免旧、新名称长期并存。

历史任务文档具有证据属性，不参与无差别替换。迁移判断以“该位置声明当前可执行/可导入身份，还是记录旧快照”作为边界。020 已完成的忽略目录副本同样是旧 baseline 证据：保留但不在 021 中重复加工。

## 2. 身份映射

| 语义 | 当前值 | 目标值 | 处理范围 |
| --- | --- | --- | --- |
| Git canonical remote | `git@github.com:rin721/go-scaffold2.git` | `git@github.com:rin721/go-scaffold-template.git` | 本地 `origin` fetch/push URL |
| Go module | `github.com/rin721/go-scaffold2` | `github.com/rin721/go-scaffold-template` | `go.mod`、`.go` imports、架构常量、当前包文档示例 |
| 源脚手架产品名 | `go-scaffold2` | `go-scaffold-template` | `cmd/app` 默认应用名、入口测试、根/当前主题文档、配置示例标题 |
| 历史事实 | 旧 commit/module/name | 保留 | 已完成 `docs/changes/**` snapshot、020 已有隔离副本、迁移报告中的 source value |
| 下游应用 identity | `APP_`、`config.yaml` | 不变 | 留给复制项目按 020 清单一次性调整 |

## 3. 实施顺序

```text
确认新 canonical URL 可访问
  -> 更新 origin URL
  -> fetch --prune 并核对 main 分歧
  -> 迁移 go.mod 与全部当前 Go imports
  -> 迁移架构测试与当前 package 示例
  -> 迁移应用名和当前产品文档
  -> 更新 020 与新 identity 的完成关系
  -> 分类扫描旧身份
  -> 完整 Go/文档/Diff 验证
  -> 仅提交本任务文件，不 push
```

如果 canonical URL 验证或 fetch 失败，实施在任何源码修改前停止。如果 fetch 显示远端包含本地未知提交，也停止并报告，不 rebase、force 或基于陈旧 `origin/main` 继续。

## 4. 文件影响

### 4.1 必改

- `.git/config` 中的 `origin` URL（通过 `git remote set-url`，不进入 commit）；
- `go.mod`；
- 包含旧 module import 的生产代码与测试；
- `pkg/boundary_test.go`、`internal/kernel/app/boundary_test.go`、`internal/kernel/composition/architecture_test.go` 等身份门禁；
- `cmd/app/main.go`、对应测试、根 README、`config.example.yaml`；
- 当前 `pkg/**/README.md` 和其他当前主题文档中的 import 示例；
- 020 当前 baseline/完成关系说明和 `docs/changes/README.md`；
- 021 任务证据。

### 4.2 条件修改

- `.github/workflows/**`、`.agents/**` 和其他交付脚本：当前扫描无旧 identity 命中，实施时复核；只有发现可验证旧引用时才修改。
- 其他当前文档：只有核验到旧 canonical identity 时才修改。
- 既有 `docs/changes/**`：仅更新仍声明当前目标或当前入口的段落；历史 snapshot 不重写。

### 4.3 明确不改

- `AGENTS.md` 的用户行尾状态；
- `APP_`、`config.yaml`、Todo 业务语义和依赖版本；
- 当前本地父目录名；
- `tmp/scaffold-copy-validation/` 已有内容；
- 另一个 `go-scaffold` 仓库或 GitHub 设置。

## 5. 失败语义

- remote 不可达或 fetch 失败：不修改源码，报告阻塞。
- 远端与本地历史不符合预期：不 rebase、不 amend、不 force，先报告差异。
- module/import 残留：迁移未完成，不保留旧 alias 或 `replace` 绕过。
- 测试或 vet 失败：保留错误链和完整命令证据，修复仅限已确认身份迁移范围；若暴露新的架构或依赖问题，退回研究。
- 历史残留无法判断：保留并记录，不能用全局替换破坏证据。

## 6. 验证设计

1. `git remote -v` 与 `git remote get-url --all origin` 核验 canonical URL。
2. `go list -m`、旧 module path `rg` 与 module-aware tests 核验 Go identity。
3. 对当前源码、根/主题文档和历史变更分别扫描 `go-scaffold2`，输出允许残留清单。
4. 单独扫描 `.github/**`、`.agents/**`、YAML/JSON/TOML、PowerShell/Shell 和交付文件，证明无旧 repository identity 或同步必要修改。
5. 对明确改动的 Go 文件执行 `gofmt`，运行 `go mod tidy -diff`、`go build ./...`、`go test ./...`、`go test -race ./...`、`go vet ./...`。
6. 验证 Markdown 相对链接时忽略 fenced 与 inline code，最后运行 `git diff --check`。
7. 审阅完整 Diff 和 staged Diff，只纳入 021 范围，不包含 `AGENTS.md`。
