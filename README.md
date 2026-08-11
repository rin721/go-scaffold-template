# go-scaffold2

`go-scaffold2` 是面向后端服务与 CLI 工具的 Go 基础设施脚手架。`pkg` 提供可独立使用的通用能力库，`internal/kernel/capability` 定义受托管的底层能力，`internal/kernel/composition` 负责显式组合，Kernel Runtime 负责生命周期和配置切换。

## 当前目标

- 保持 `pkg/*` 通用库不感知 kernel、DI、drain 或热替换。
- `internal/kernel/capability` 只定义需要托管的 pkg 能力；`internal/kernel/composition` 显式选择并登记定义，业务构造函数接收稳定 Access。
- 配置变化时先排空旧租约并准备候选，全部成功后统一发布，失败继续使用旧实例。
- 当前接入 Logger 和 Database；不提前引入业务对象图、依赖 DAG、Service Locator 或诊断平台。
- 每个成功登记的 Capability Definition 都携带自身默认配置契约；composition 聚合这些契约，并可选择性构造启动前 CLI。

## 已实现底层库

当前 `pkg` 保留以下底层能力库：

- `logger`、`httpx`、`i18n`、`database`、`cache`、`cli`、`storage`
- `validation`、`fault`、`supervisor`、`health`
- `idgen`、`clock`、`secrets`、`resource`
- `resilience`、`concurrency`、`codec`、`testkit`

## 包入口

- [docs/README.md](docs/README.md)：项目文档入口与任务级变更记录。
- [pkg/README.md](pkg/README.md)：底层能力库封装规范、当前能力清单和暂缓路线。
- [internal/kernel/README.md](internal/kernel/README.md)：显式组合、配置事务、租约排空和运行方式。
- [internal/kernel/capability/README.md](internal/kernel/capability/README.md)：Kernel Capability 定义职责以及 Logger、Database 实现。
- [pkg/cli/README.md](pkg/cli/README.md)：项目自有 CLI 契约、I/O 与退出码语义。
- [AGENTS.md](AGENTS.md)：AI Agent 协作红线和工程底线。

## 默认配置与可选 CLI

`composition.Compose(runtime, options)` 始终按 Logger、Database 顺序返回稳定 Access 和默认配置管理器；只有 `options.CLI` 非 nil 时才构造 CLI App。配置管理器可由代码直接生成 YAML/JSON，也可通过启动前 `config init` 命令生成当前项目实际组合的完整默认配置骨架。具体运行方式见 [Kernel 说明](internal/kernel/README.md)。

[docs/changes/001-default-config-cli-contracts](docs/changes/001-default-config-cli-contracts/README.md) 保留本能力的需求、设计和实施证据，不作为当前 API 使用入口。

## 启动项目

先从 `cmd/app` 入口生成本地配置骨架：

```powershell
go run ./cmd/app config init
```

编辑 `config.yaml`，明确填写 `database.engine` 和 `database.driver`。DSN 通过环境变量提供，不写入文件；PowerShell 示例：

```powershell
$env:APP_DATABASE__DSN = '<database-dsn>'
go run ./cmd/app
```

环境变量使用 `APP_` 前缀和双下划线表达嵌套字段，也可以覆盖文件配置，例如 `APP_LOGGER__LEVEL`、`APP_DATABASE__ENGINE`、`APP_DATABASE__DRIVER`。无参数模式会先发布配置化 Logger，再启动 Database 并完成连接与 Ping；应用生命周期 Participant 记录启动、停止后等待 `Ctrl+C` 或 `SIGTERM`，再由 Host 优雅停止。配置缺失、字段非法或数据库不可达都会返回非零退出码；当前未组合 HTTP 服务，因此不会创建网络监听器。

本地 `config.yaml`、`config.yml` 和 `config.json` 已被 Git 忽略。入口实现与约束记录在 [docs/changes/002-application-entrypoint](docs/changes/002-application-entrypoint/README.md)。

## 本地验证

```powershell
gofmt -w .
go mod tidy
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

消息、任务调度、分布式锁、认证、邮件、搜索、特性开关、基础能力依赖 DAG、业务对象装配和观测采集适配仍需等待真实场景确认。
