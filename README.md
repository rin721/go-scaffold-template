# go-scaffold2

`go-scaffold2` 是面向后端服务与 CLI 工具的 Go 基础设施脚手架。`pkg` 提供可独立使用的通用能力库，`internal/kernel/app` 声明底层 App 组件，`internal/kernel/composition` 负责手工选择并冻结有序 Plan，Kernel Runtime 负责生命周期和配置切换。

## 当前目标

- 保持 `pkg/*` 通用库不感知 kernel、DI、drain 或热替换。
- 所有当前进程选择的底层能力都通过 `kernel/app/<name>` 和 composition；Clock、ID Generator、Validator 直接输出普通接口，Logger、Database 输出稳定租约 Access。
- `app.Plan` 只支持显式有序 `Add` 和 typed `Binding/Input`，不扫描、不提供运行期 Resolver，也不装配尚未建设的业务对象。
- 配置变化时先准备全部候选，再反向排空旧租约；失败恢复旧入口，`RestartRequired` 在任何副作用前阻止整轮应用。
- 配置与默认值是可选组件契约；当前只有 Logger、Database 贡献默认配置，CLI 也只在显式启用时构造。

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
- [internal/kernel/app/README.md](internal/kernel/app/README.md)：组件 Definition、Direct/Leased、Plan、typed Input 和接入步骤。
- [pkg/cli/README.md](pkg/cli/README.md)：项目自有 CLI 契约、I/O 与退出码语义。
- [AGENTS.md](AGENTS.md)：AI Agent 协作红线和工程底线。

## 默认配置与可选 CLI

`composition.Compose(runtime, options)` 按 Logger、Clock、ID Generator、Validator、Database 顺序建立完整 Plan。返回值中三项简单能力是普通接口，两项资源能力是稳定 Access；只有 `options.CLI` 非 nil 时才构造 CLI App。默认配置仍只有 Logger、Database 两段。具体运行方式见 [Kernel 说明](internal/kernel/README.md)。

根目录 [config.example.yaml](config.example.yaml) 提供当前 Logger、Database 的全量字段、合法选项和环境变量示例；它用于人工选择本地方案，不是运行时自动加载的第二个配置来源。

[docs/changes/001-default-config-cli-contracts](docs/changes/001-default-config-cli-contracts/README.md) 保留本能力的需求、设计和实施证据，不作为当前 API 使用入口。

## 启动项目

推荐先复制带完整注释的示例配置：

```powershell
Copy-Item config.example.yaml config.yaml
```

示例默认选择 `development + gorm + postgres`。按注释切换 Logger、Database Engine 或 Driver 后，通过环境变量提供真实 DSN：

```powershell
$env:APP_DATABASE__DSN = '<database-dsn>'
go run ./cmd/app
```

也可以从 `cmd/app` 入口生成不带方案注释的最小配置骨架：

```powershell
go run ./cmd/app config init
```

该命令默认拒绝覆盖已有目标。确认现有 `config.yaml` 可以被替换后，才使用：

```powershell
go run ./cmd/app config init --force
```

生成后必须明确填写 `database.engine` 和 `database.driver`。DSN 仍通过环境变量提供，不写入文件。

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

消息、任务调度、分布式锁、认证、邮件、搜索、特性开关、业务对象装配和观测采集适配仍需等待真实场景确认。Kernel 当前只提供严格前向的底层组件有序计划，不是通用依赖 DAG 容器。
