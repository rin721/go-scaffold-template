# go-scaffold2

`go-scaffold2` 是面向后端服务与 CLI 工具的 Go 基础设施脚手架。`pkg` 提供可独立使用的通用能力库，`internal/kernel/capability` 定义受托管的底层能力，`internal/kernel/composition` 负责显式组合，Kernel Runtime 负责生命周期和配置切换。

## 当前目标

- 保持 `pkg/*` 通用库不感知 kernel、DI、drain 或热替换。
- `internal/kernel/capability` 只定义需要托管的 pkg 能力；`internal/kernel/composition` 显式选择并登记定义，业务构造函数接收稳定 Access。
- 配置变化时先排空旧租约并准备候选，全部成功后统一发布，失败继续使用旧实例。
- 当前只接入 Database；不提前引入业务对象图、依赖 DAG、Service Locator 或诊断平台。
- 每个成功登记的 Capability Definition 都携带自身默认配置契约；composition 聚合这些契约，并可选择性构造启动前 CLI。

## 已实现底层库

当前 `pkg` 保留以下底层能力库：

- `logger`、`httpx`、`i18n`、`database`、`cache`、`cli`、`storage`
- `validation`、`fault`、`supervisor`、`health`
- `idgen`、`clock`、`secrets`、`resource`
- `resilience`、`concurrency`、`codec`、`testkit`

## 包入口

- [pkg/README.md](pkg/README.md)：底层能力库封装规范、当前能力清单和暂缓路线。
- [internal/kernel/README.md](internal/kernel/README.md)：显式组合、配置事务、租约排空和运行方式。
- [internal/kernel/capability/README.md](internal/kernel/capability/README.md)：Kernel Capability 定义职责和 Database 实现。
- [pkg/cli/README.md](pkg/cli/README.md)：项目自有 CLI 契约、I/O 与退出码语义。
- [AGENTS.md](AGENTS.md)：AI Agent 协作红线和工程底线。

## 默认配置与可选 CLI

`composition.Compose(runtime, options)` 始终返回 Database Access 和默认配置管理器；只有 `options.CLI` 非 nil 时才构造 CLI App。配置管理器可由代码直接生成 YAML/JSON，也可通过启动前 `config init` 命令生成当前项目实际组合的完整默认配置骨架。具体运行方式见 [Kernel 说明](internal/kernel/README.md)。

[design/001-default-config-cli-contracts](design/001-default-config-cli-contracts/README.md) 保留本能力的需求、设计和实施证据，不作为当前 API 使用入口。

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
