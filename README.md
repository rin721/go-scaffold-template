# go-scaffold2

`go-scaffold2` 是面向后端服务与 CLI 工具的 Go 基础设施脚手架。`pkg` 提供可独立使用的通用能力库，`internal/kernel/app` 声明底层 App 组件，`internal/kernel/composition` 负责手工选择并冻结有序 Plan，Kernel Runtime 负责生命周期和配置切换。

## 当前目标

- 保持 `pkg/*` 通用库不感知 kernel、DI、drain 或热替换。
- 所有当前进程选择的底层能力都通过 `kernel/app/<name>` 和 composition；Clock、ID Generator、Validator 直接输出普通接口，Database 输出稳定租约 Access，Logger 始终输出 Kernel 内置稳定 facade。
- `app.Plan` 只支持显式有序 `Add`、typed `Binding/Input` 和针对既有 typed target 的 `Replace`；不扫描、不提供运行期 Resolver，也不装配尚未建设的业务对象。
- 配置变化时先准备全部候选，再反向排空旧租约；失败恢复旧入口，`RestartRequired` 在任何副作用前阻止整轮应用。
- 配置与默认值是可选组件契约；只有显式选择配置化 Logger replacement 时才贡献 `logger` 段，Database 始终贡献自身默认配置，CLI 也只在显式启用时构造。

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

`composition.Compose(runtime, options)` 先加入 Kernel 内置 Logger target；`options.Logger` 可以保留基线，也可以显式加入配置化 Logger replacement，随后再按 Clock、ID Generator、Validator、Database 建立完整 Plan。返回的 Logger 是同一个稳定 facade，Database 是租约 Access；只有 `options.CLI` 非 nil 时才构造 CLI App。当前 `cmd/app` 明确选择配置化 replacement，因此默认配置仍为 Logger、Database 两段。具体运行方式见 [Kernel 说明](internal/kernel/README.md)。

根目录 [config.example.yaml](config.example.yaml) 提供当前 Logger、Database 的全量字段、合法选项和环境变量示例；它用于人工选择本地方案，不是运行时自动加载的第二个配置来源。Database 的 GORM 实现在 `internal/kernel/app/database` 构造代码中固定选择，配置只选择数据库 Driver 和连接参数。

[docs/changes/001-default-config-cli-contracts](docs/changes/001-default-config-cli-contracts/README.md) 保留本能力的需求、设计和实施证据，不作为当前 API 使用入口。

## 启动项目

推荐先复制带完整注释的示例配置：

```powershell
Copy-Item config.example.yaml config.yaml
```

示例默认选择 `development + sqlite`，可直接创建 `.data/app.db`。切换到 PostgreSQL 或 MySQL 时，按注释选择 Driver，并通过环境变量提供真实 DSN：

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

生成配置默认包含 `database.driver: sqlite` 和 `database.dsn: .data/app.db`。切换远端数据库时必须明确填写 Driver，并通过环境变量提供 DSN，不把凭据写入文件。

环境变量使用 `APP_` 前缀和双下划线表达嵌套字段，也可以覆盖文件配置，例如 `APP_LOGGER__LEVEL`、`APP_DATABASE__DRIVER`、`APP_DATABASE__DSN`。无参数模式会先发布配置化 Logger，再启动 Database 并完成连接与 Ping；应用生命周期 Participant 记录启动、停止后等待 `Ctrl+C` 或 `SIGTERM`，再由 Host 优雅停止。配置缺失、字段非法或数据库不可达都会返回非零退出码；当前未组合 HTTP 服务，因此不会创建网络监听器。

无参数服务模式默认监听 `config.yaml`。watcher 完成父目录注册后会先执行一次 reconciliation，再把稳定后的文件事件交给同一个 Kernel Reload 事务；Database 与配置化 Logger 只有在全部候选准备成功后才切换，单次无效候选保留旧实例并继续监听。环境变量优先级高于文件，因此被 `APP_*` 覆盖的字段仅修改文件不会改变有效配置；运行中的进程也不会读取另一个 shell 后续修改的环境。推荐使用临时文件加 rename 的原子保存方式，原地 truncate/write 的中间内容可能产生一次“候选被拒绝、旧配置保留”的诊断日志。

本地 `config.yaml`、`config.yml` 和 `config.json` 已被 Git 忽略。入口实现与约束记录在 [docs/changes/002-application-entrypoint](docs/changes/002-application-entrypoint/README.md)，配置重载修复与生命周期证据见 [009 配置重载与生命周期修复](docs/changes/009-config-reload-lifecycle-repair/README.md)。

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
