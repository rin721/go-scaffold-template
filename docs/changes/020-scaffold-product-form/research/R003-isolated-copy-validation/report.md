# R003：固定快照的隔离复制与示例移除验证

## 研究问题

固定 Git commit 的 tracked baseline 能否在排除源 Git 和运行态文件后复制为独立 Go module；身份迁移、Todo 保留与完整移除是否都能保持底层装配链可编译、可测试、可初始化配置？

## 快照与方法

- 源快照：`main@bba180266cba99ec84e2da0296df7fca373764b4`。
- 输入：`git archive` 产生的 432 个 tracked 文件，归档 SHA-256 为 `ebe3de6f175d031c7b49b94dda968b08761c5dcea138f7327c6fbd9ffbd1df63`。
- 隔离位置：Git 忽略的 `tmp/scaffold-copy-validation/`。
- 排除：源 `.git`、未跟踪文件、`tmp`、`.data`、本地配置、数据库、凭据、缓存和构建产物没有进入归档。
- 两个副本都新增 `docs/scaffold-baseline.md`，记录来源仓库、完整 commit、复制日期和 copy-owned 语义；该记录不参与构建或运行。

## 身份迁移结果

| 语义 | 保留示例副本 | 最小副本 |
| --- | --- | --- |
| module | `example.com/acme/order-service` | `example.com/acme/platform-service` |
| application | `order-service` | `platform-service` |
| description | `订单 HTTP API 服务` | `平台 HTTP API 服务` |
| env prefix | `ORDER_` | `PLATFORM_` |
| local config | `order-service.yaml` | `platform-service.yaml` |
| 独立 Git commit | `acd8f1720d5370b4aee019f61e588bacf58697e6` | `1543443626ff669b4b6ddd1333b0f6e7fd280be1` |

残留扫描必须包含 `.gitignore` 等隐藏文件，同时排除新项目 `.git/**` 与 provenance 文件。第一次只扫描普通文件时漏掉了 `.gitignore` 中的旧配置名；改为 `rg --hidden --glob '!.git/**' --glob '!docs/scaffold-baseline.md'` 后完成修正。两个副本的旧 Go import、旧运行身份和源 module 依赖均为 0。

## 保留 Todo 的结果

- `go mod tidy -diff`：通过。
- `go test ./... -count=1`：通过。
- `go vet ./...`：通过。
- `go build -o NUL ./cmd/app`：通过。
- `go run ./cmd/app config init`：通过，生成 Logger、Database、Cache、I18n、Storage、HTTP 与 `module.todo` 配置。
- 新 Git 仓库无 remote，生成的 `order-service.yaml` 被副本自己的 `.gitignore` 排除，`go list -deps ./...` 不包含源 module。

Windows checkout 的 `go.sum` 使用 CRLF，`go mod tidy -diff` 会报告纯换行差异。验证副本先把 `go.sum` 规范化为 LF，再执行语义门禁；这说明正式复制说明应明确行尾策略，而不是把纯换行差异误报为依赖变化。

## 完整移除 Todo 的结果

实际移除边界包括：

- 删除 `internal/module/todo/**` 共 20 个文件；
- 删除专用于 Todo 的 `internal/composition/todo.go` 与 `database.go`；
- 用通用 `prepareService` 保留 Kernel、Capabilities、Coordinator 和 HTTP 配置装配；
- 从 Bootstrap CLI、Router、Host participants、配置示例、入口测试和当前文档中删除 Todo binding、route、CLI、migration 与说明；
- 保留通用 `internal/module` contribution 契约，供真实业务模块显式接入。

移除后的 412 文件副本通过 `go mod tidy -diff`、完整 test、vet、build 和 `config init`。生成配置只有 Logger、Database、Cache、I18n、Storage 与 `application.http` 六段；当前源码与现行文档中的 Todo 业务引用为 0。副本无 remote、工作树干净，目标配置被自身 `.gitignore` 排除。

该结果证明 Todo 是可删除示例，不是 Kernel 或 `pkg` 的隐式依赖。删除后的装配链为：

```text
cmd/app
  -> internal/composition.Application
  -> internal/kernel.New + Coordinator
  -> internal/kernel/composition.Compose
  -> internal/kernel/app/*
  -> pkg/*
```

依赖箭头在源码层面实际从装配方指向被装配方；上图表达从入口追踪的构造路径。业务模块只能由 `internal/composition` 从 `Capabilities` 提取最小契约后显式注入，不能查询 Kernel 或运行期容器。

## 平台结果

- Windows PowerShell、Go `1.25.7 windows/amd64`：复制、身份迁移、两个副本的 Go 门禁、配置初始化和独立 Git 检查均通过。
- WSL：`wsl.exe` 存在，但 `wsl.exe --list --quiet` 没有可运行发行版，因此未执行 Linux 等价命令，不声明 Linux 通过。

## 事实、推断与限制

- 已验证事实：固定 commit 的完整 tracked baseline 可以复制为独立 module；两个示例策略均通过 Windows 验证；副本没有源 module、workspace、relative replace 或 remote 依赖。
- 设计推断：正式复制指南只需描述一次性人工迁移和可复核门禁，不需要 generator 或公共 Runtime。
- 限制：本记录验证的是改名前 `bba1802` 快照。021 完成后，正式发布指南必须在新 canonical identity 上刷新来源值和残留清单；Linux 仍需 CI 或可用发行版补验。

## 对 020 的影响

`COPY-001`、`IDENTITY-001`、`TODO-001`、`TODO-002`、`POLICY-001` 和 Windows 部分 `PORTABLE-001` 均有实证。产品形态可固定为 copy-owned；后续独立任务负责正式复制指南、release baseline、安全公告模板和 Linux CI，不应把这些未实现能力写成当前已交付工具。
