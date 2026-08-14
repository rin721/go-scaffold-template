# go-scaffold2

`go-scaffold2` 是面向后端服务与 CLI 工具的 Go 基础设施脚手架。`pkg` 提供可独立使用的通用能力库，`internal/kernel/app` 声明底层 App 组件，`internal/kernel/composition` 负责手工选择并冻结有序 Plan，Kernel Runtime 负责生命周期和配置切换。

## 当前目标

- 保持 `pkg/*` 通用库不感知 kernel、DI、drain 或热替换。
- 所有当前进程选择的底层能力都通过 `kernel/app/<name>` 和 composition；Clock、ID Generator、Validator 直接输出普通接口，Database、Cache、Storage 输出稳定租约 Access，Logger 与 I18n 输出稳定 facade。
- `app.Plan` 只支持显式有序 `Add`、typed `Binding/Input` 和针对既有 typed target 的 `Replace`；不扫描、不提供运行期 Resolver，也不装配尚未建设的业务对象。
- 配置变化时先准备全部候选，再反向排空旧租约；失败恢复旧入口，`RestartRequired` 在任何副作用前阻止整轮应用。
- 配置节的默认值与严格绑定由同一个 owner 契约提供；服务入口由唯一 Coordinator 加载同一不可变候选，Bootstrap CLI 则只构造默认配置所需契约，不创建 Kernel、资源或监听器。
- 首个 Todo 业务模块以显式 `model/service/repo/handler/binding` 结构提供 HTTP 与 CLI；业务对象由 `internal/composition` 手工装配，不进入 Kernel Plan。

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

## Bootstrap CLI 与 Service

有参数时，`cmd/app` 先选择 Bootstrap mode，`composition.ComposeBootstrap` 只构造配置节契约、默认配置管理器和命令树。无参数时才创建 Loader、Kernel、服务能力、Coordinator、HTTP Server 和 Host。`composition.Compose(runtime, options)` 先加入 Kernel 内置 Logger target；`options.Logger` 可以保留基线，也可以显式加入配置化 Logger replacement，随后按 Clock、ID Generator、Validator、Database、Cache、I18n、Storage 建立完整 Plan。返回的 Logger 和 I18n Translator 是稳定 facade，Database、Cache、Storage 是稳定 Access。

当前 `cmd/app` 选择配置化 Logger replacement；默认配置由 Bootstrap 聚合 Logger、Database、Cache、I18n、Storage、application-owned HTTP 与 Todo 七段。默认值在写文件前经过与运行时相同的严格解码和语义校验。具体运行方式见 [Kernel 说明](internal/kernel/README.md)。

根目录 [config.example.yaml](config.example.yaml) 提供当前 Logger、Database、Cache、I18n、Storage、Todo、HTTP 的全量字段、合法选项和环境变量示例；它用于人工选择本地方案，不是运行时自动加载的第二个配置来源。Database 的 GORM 实现在 `internal/kernel/app/database` 构造代码中固定选择；Cache 只选择 `disabled/redis`；Storage 的 Kernel 配置只治理对象存储，不接管进程内文件工具。

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

生成配置默认使用 SQLite `.data/app.db`、禁用共享 Cache、创建空资源的 I18n Translator，把 Storage 设为本地 `.data/storage`，启用 Todo 默认分页/标题限制，并让 HTTP 监听 `:8080`。切换远端 Database、Redis 或对象存储时必须明确填写 Driver，并通过环境变量提供 DSN、密码和密钥，不把凭据写入文件。

环境变量使用 `APP_` 前缀和双下划线表达嵌套字段，也可以覆盖文件配置，例如 `APP_DATABASE__DSN`、`APP_CACHE__REDIS__PASSWORD`、`APP_STORAGE__S3__SECRETACCESSKEY`、`APP_TODO__MAXLISTLIMIT`、`APP_HTTP__ADDR`。无参数模式只加载一次候选，所有 Kernel 与 application owner 在资源副作用前完成严格解码和校验；随后启动 Database、Cache、I18n、Storage、Todo migration 和 HTTP listener。只有 migration 完成、listener 已绑定、受监督 Serve runner 已确认运行且全部必需 owner 启动后，进程才 ready。未匹配请求仍返回 404。配置缺失、未知/重复字段、类型非法或资源不可达都会返回非零退出码；`Ctrl+C` 或 `SIGTERM` 会撤销 readiness 并触发有界反向停止。

## Todo 快速学习示例

启动服务后可使用以下 REST API；创建与完成写入同一份 Database 配置指向的数据源：

```text
POST  /api/v1/todos
GET   /api/v1/todos/{id}
GET   /api/v1/todos?status=pending&offset=0&limit=20
PATCH /api/v1/todos/{id}/complete
```

创建请求只需要 JSON 标题：

```powershell
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8080/api/v1/todos -ContentType application/json -Body '{"title":"学习 Go"}'
```

同一套 UseCases 也通过 one-shot Application CLI 提供。CLI 会按 `Coordinator -> Todo migration -> operation -> reverse Stop` 完整管理资源，但不会启动 HTTP listener 或配置 watcher：

```powershell
go run ./cmd/app todo create --title "学习 Go"
go run ./cmd/app todo list --status pending --offset 0 --limit 20
go run ./cmd/app todo get --id <todo-id>
go run ./cmd/app todo complete --id <todo-id>
```

目录职责、依赖方向和扩展方式见 [Todo 模块说明](internal/business/todo/README.md)，实施契约与验收证据见 [014 Todo 业务垂直切片](docs/changes/014-todo-business-vertical-slice/README.md)。

无参数服务模式默认监听 `config.yaml`。watcher 完成父目录注册后会先执行一次 reconciliation，再把稳定后的文件事件交给同一个 Coordinator Reload 事务；Database、I18n、Storage 与配置化 Logger 只有在全部候选准备成功后才切换，单次无效候选保留旧实例并继续监听。Cache 和 HTTP 配置变化属于 `RestartRequired`：同轮预检会在任何构建、排空或提交前拒绝整轮变更。提交后的旧代清理失败会进入 `degraded`、撤销 readiness 并阻断后续重载，要求重启恢复。环境变量优先级高于文件，因此被 `APP_*` 覆盖的字段仅修改文件不会改变有效配置；运行中的进程也不会读取另一个 shell 后续修改的环境。推荐使用临时文件加 rename 的原子保存方式，原地 truncate/write 的中间内容可能产生一次“候选被拒绝、旧配置保留”的诊断日志。

本地 `config.yaml`、`config.yml` 和 `config.json` 已被 Git 忽略。入口实现与约束记录在 [docs/changes/002-application-entrypoint](docs/changes/002-application-entrypoint/README.md)，配置重载修复与生命周期证据见 [009 配置重载与生命周期修复](docs/changes/009-config-reload-lifecycle-repair/README.md)，三项能力装配见 [011 Cache、I18n、Storage 装配](docs/changes/011-cache-i18n-storage-composition/README.md)，Bootstrap/Config/监督/HTTP/诊断闭环见 [012 业务模块架构](docs/changes/012-business-module-architecture/README.md)，首个业务模块见 [014 Todo 业务垂直切片](docs/changes/014-todo-business-vertical-slice/README.md)。

## 本地验证

```powershell
gofmt -w .
go mod tidy
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

消息、任务调度、分布式锁、认证、邮件、搜索、特性开关和观测采集适配仍需等待真实场景确认。Kernel 当前只提供严格前向的底层组件有序计划，不是通用依赖 DAG 容器；Todo 业务对象由 application composition root 显式构造。
