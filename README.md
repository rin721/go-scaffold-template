# go-scaffold-template

`go-scaffold-template` 是面向后端服务与 CLI 工具的 Go 基础设施脚手架。`pkg` 提供可独立使用的通用能力库，`internal/kernel/app` 声明底层 App 组件，`internal/kernel/composition` 负责手工选择并冻结有序 Plan，Kernel Runtime 负责生命周期和配置切换。

## 当前目标

- 保持 `pkg/*` 通用库不感知 kernel、DI、drain 或热替换。
- 所有当前进程选择的底层能力都通过 `kernel/app/<name>` 和 composition；Clock、ID Generator、Validator 直接输出普通接口，Database、Cache、Storage 输出稳定租约 Access，Logger 与 I18n 输出稳定 facade。
- `app.Plan` 只支持显式有序 `Add`、typed `Binding/Input` 和针对既有 typed target 的 `Replace`；不扫描、不提供运行期 Resolver，也不装配尚未建设的业务对象。
- 长期 Service 以完整不可变 Application Generation 应用配置：同一 Snapshot 构造全部 Capability、Todo、Router 与 HTTP Server，候选失败保留旧代，提交后旧连接排空并反向释放资源。
- 配置节的默认值与严格绑定由同一个 owner 契约提供；Service 由唯一 GenerationCoordinator 加载候选，Bootstrap CLI 则只构造默认配置所需契约，不创建资源、listener 或 goroutine。
- 首个 Todo 应用模块位于 `internal/module/todo`，以显式 `model/service/repo/handler/binding` 结构提供 HTTP 与 CLI；模块对象由 `internal/composition` 手工装配，不进入 Kernel Plan。

## 已实现底层库

当前 `pkg` 保留以下底层能力库：

- `logger`、`httpx`、`i18n`、`database`、`cache`、`cli`、`storage`
- `validation`、`fault`、`supervisor`、`health`
- `idgen`、`clock`、`secrets`
- `resilience`、`concurrency`、`codec`、`testkit`

## 包入口

- [docs/README.md](docs/README.md)：项目文档入口与任务级变更记录。
- [应用模块开发指南](docs/development/application-module-development.md)：新增模块前的用例、Capability、资源 owner、生命周期和契约适配评估。
- [pkg/README.md](pkg/README.md)：底层能力库封装规范、当前能力清单和暂缓路线。
- [internal/kernel/README.md](internal/kernel/README.md)：显式组合、配置事务、租约排空和运行方式。
- [internal/kernel/app/README.md](internal/kernel/app/README.md)：组件 Definition、Direct/Leased、Plan、typed Input 和接入步骤。
- [pkg/cli/README.md](pkg/cli/README.md)：项目自有 CLI 契约、I/O 与退出码语义。
- [AGENTS.md](AGENTS.md)：AI Agent 协作红线和工程底线。
- [交付与运维](docs/operations/README.md)：构建、容器、迁移、发布、复制与安全响应。

## Bootstrap CLI 与 Service

有参数时，`cmd/app` 选择 Bootstrap/one-shot mode；`composition.ComposeBootstrap` 只构造配置节契约、默认配置管理器和命令树，具体命令才按 invocation 创建 Kernel 资源。无参数时创建 Loader、GenerationCoordinator、typed resource pools、ListenerHub 和完整 Application Generation。每代显式拥有 Logger、Database、Cache、I18n、Storage、Todo、Router 与 `http.Server`；Clock、ID Generator 和 Validator 是无配置的普通能力。

当前 `cmd/app` 选择配置化 Logger replacement；默认配置由 Bootstrap 聚合 Logger、Database、Cache、I18n、Storage、application-owned HTTP 与 Todo 七段。默认值在写文件前经过与运行时相同的严格解码和语义校验。具体运行方式见 [Kernel 说明](internal/kernel/README.md)。

根目录 [config.example.yaml](config.example.yaml) 提供当前 Logger、Database、Cache、I18n、Storage、Todo、HTTP 的全量字段、合法选项和环境变量示例；它用于人工选择本地方案，不是运行时自动加载的第二个配置来源。Database 的 GORM 实现在 `internal/kernel/app/database` 构造代码中固定选择；Cache 只选择 `disabled/redis`；Storage 的 Kernel 配置只治理对象存储，不接管进程内文件工具。

[docs/changes/001-default-config-cli-contracts](docs/changes/001-default-config-cli-contracts/README.md) 保留本能力的需求、设计和实施证据，不作为当前 API 使用入口。

## 启动项目

推荐先复制带完整注释的示例配置：

```powershell
Copy-Item config.example.yaml config.yaml
```

示例默认选择 `development + sqlite`。首次启动 Service 前必须由独立命令应用 versioned migration；Service 自身只读校验版本与 Todo owner 完成状态，不执行 DDL：

```powershell
go run ./cmd/app db migrate status
go run ./cmd/app db migrate up
go run ./cmd/app
```

旧库存在 Todo 行时，`up` 会保留隔离占位 owner 并拒绝完成，必须由 operator 显式提供真实 subject 后重试：

```powershell
go run ./cmd/app db migrate up --legacy-owner-subject <subject>
```

切换到 PostgreSQL 或 MySQL 时，按注释选择 Driver，并通过环境变量提供真实 DSN：

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

生成配置默认使用 SQLite `.data/app.db`、禁用共享 Cache、创建空资源的 I18n Translator，把 Storage 设为本地 `.data/storage`，启用 Todo 默认分页/标题限制，并让开发 HTTP 仅监听 `127.0.0.1:8080`。切换远端 Database、Redis 或对象存储时必须明确填写 Driver，并通过环境变量提供 DSN、密码和密钥，不把凭据写入文件。

环境变量使用 `APP_` 前缀和双下划线表达嵌套字段，也可以覆盖文件配置，例如 `APP_DATABASE__DSN`、`APP_AUTH__MODE`、`APP_AUTH__JWT__JWKSURL`、`APP_CACHE__REDIS__PASSWORD`、`APP_STORAGE__S3__SECRETACCESSKEY`、`APP_TODO__MAXLISTLIMIT`、`APP_HTTP__ADDR`。同一 EnvSource 的重复逻辑路径、大小写别名、空 segment 或祖先/后代路径会确定性失败；File/Env 之间只允许 object/object 递归合并或 non-object/non-object 覆盖，object 与 scalar、array、null 不得相互改形状。Service 初始候选完成 strict decode、资源 Ready、Todo migration compatibility、Auth profile、listener bind 和 Server Serve-ready 后才提交第一代；所有代都只读校验 schema version/dirty/owner completion，绝不执行 migration。配置缺失、未知字段、资源不可达或 migration 不兼容都会保留旧代；`Ctrl+C` 或 `SIGTERM` 会撤销 admission 并触发有界排空。

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

[`api/openapi.yaml`](api/openapi.yaml) 是 HTTP operation、路径、DTO、响应、security 与兼容性的唯一权威。`oapi-codegen` 生成的 strict Chi server interface、DTO 与 route binding 位于 `internal/transport/http/api`；Todo 手写 operation Handler、DTO 映射、错误呈现和 Actor 窄端口收口在 `internal/module/todo/binding/http`。模块 Handler 不创建 Router，也不满足整份应用接口；`internal/composition` 静态聚合所有模块 operation，`internal/transport/http` 只绑定一次 OpenAPI validator、strict middleware 与生成路由，最外层 application Router 只安装全局 middleware 并挂载该 API route tree。缺失或不支持的 `Content-Type` 返回 RFC 9457 `415 unsupported_media_type`，未知字段、非法参数和尾随 JSON 返回稳定 Problem Details。旧 `module.Route`、旧 Todo 自建路由 authority 和 route middleware 已删除。

同一套 UseCases 也通过 one-shot Application CLI 提供。CLI 不解析 bearer token，必须显式传入本机 operator 的 `--subject` 与 `--scopes`，并与 HTTP 共用对象授权和低敏审计：

```powershell
go run ./cmd/app todo create --subject <subject> --scopes todos:read,todos:write --title "学习 Go"
go run ./cmd/app todo list --subject <subject> --scopes todos:read --status pending --offset 0 --limit 20
go run ./cmd/app todo get --subject <subject> --scopes todos:read --id <todo-id>
go run ./cmd/app todo complete --subject <subject> --scopes todos:write --id <todo-id>
```

目录职责、依赖方向和扩展方式见 [应用模块说明](internal/module/README.md) 与 [Todo 模块说明](internal/module/todo/README.md)。业务垂直切片与旧 route middleware 的历史证据仍保存在 [014](docs/changes/014-todo-business-vertical-slice/README.md) 和 [015](docs/changes/015-todo-route-middleware-example/README.md)，当前 HTTP 契约只以 OpenAPI 与生成物为准。

无参数 Service 默认监听 `config.yaml`。FileSource 对 Windows sharing violation、atomic rename 的短暂不存在和仍在变化的内容执行有界稳定双采样；watcher 使用容量一的 latest-wins 通知串行触发完整 reload。`logger/database/cache/i18n/storage/http/auth/todo/management/observability` 参与代际候选；`migration` 只供显式命令使用。未变化的底层资源按 section digest 引用复用，Todo、Auth、Ops、模块 operation Handler、strict API aggregate、route binding、Router 和两个 `http.Server` 每代重建。两个 ListenerHub 分别独占业务与 management 物理 listener，同地址切虚拟 route，已接受连接固定旧代并 graceful drain；地址变化先 bind 新地址。稳定非法候选、资源 Ready、migration compatibility 或 bind 失败均保留旧代并继续监听；提交后清理失败进入 degraded 并阻断后续 reload。Database/Storage 目标变化不自动迁移数据，旧 keep-alive 连接也会在排空前继续使用旧代。

`internal/module/ops` 在独立 management listener 提供 `/startupz`、`/livez`、`/readyz`、`/metrics`、`/build` 和受 `management:read` 保护的 `/diagnostics`；业务 listener 不暴露这些路径，pprof 不注册。Prometheus registry 由进程稳定持有，OTel provider/exporter 随 generation 构造和有界 flush。GenerationCoordinator 继续提供 attempt、candidate/current/retiring generation、Snapshot digest、changed sections、phase、地址、活动连接/请求、资源 build/reuse、cleanup debt 和脱敏失败类型；当前仍不提供跨进程 retry/force CLI。

当前 Prometheus/OTel 运行行为已经实现，但第三方封装层级仍有已知偏差：Ops 与 application composition 的装配协议可见具体 Adapter/OTel 类型。[027 第三方封装与分轨装配](docs/changes/027-business-module-third-party-isolation/README.md) 已完成纯文档设计，要求业务专属第三方留在模块内 Adapter 并零泄漏，非业务 Observability 先形成项目自有契约再从 Kernel App 底层装配；源码迁移仍待后续明确确认。

本地 `config.yaml`、`config.yml` 和 `config.json` 已被 Git 忽略。入口实现与约束记录在 [docs/changes/002-application-entrypoint](docs/changes/002-application-entrypoint/README.md)，底层组件生命周期见 [009 配置重载与生命周期修复](docs/changes/009-config-reload-lifecycle-repair/README.md)，能力装配见 [011 Cache、I18n、Storage 装配](docs/changes/011-cache-i18n-storage-composition/README.md)，首个应用模块见 [014 Todo 业务垂直切片](docs/changes/014-todo-business-vertical-slice/README.md)。[023 全配置无感重载](docs/changes/023-full-configuration-seamless-reload/README.md) 是 `RLD-001..015` 的实施与验收 authority；[024 生产就绪模板一次性竣工](docs/changes/024-production-ready-one-shot-completion/README.md) 继续管理其余生产竣工范围。两项任务都不授权外部发布。

## 本地验证

```powershell
gofmt -w .
go mod tidy
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

固定版本扫描器、distroless container smoke、GoReleaser 本地候选、SPDX SBOM 与 Cosign checksum 验证见 [交付与运维](docs/operations/README.md)。正式 release workflow 只响应受保护的 `v*` tag 并创建 draft；当前任务没有授权 push、tag、GitHub Release 或 registry 写入。

消息、任务调度、分布式锁、认证、邮件、搜索、特性开关和观测采集适配仍需等待真实场景确认。Kernel 当前只提供严格前向的底层 Component 有序计划，不是通用依赖 DAG 容器；Todo 模块对象由 application composition root 显式构造。
