# R001：Todo 首个真实业务垂直切片可行性

## 1. 研究问题与范围

本研究回答：当前仓库能否把 Todo 作为首个真实业务能力，完成 HTTP、SQLite 持久化、真实配置和真实 CLI 命令的同一 Service 闭环；如果可以，实施前还缺哪些项目契约。

范围覆盖当前 Git/测试基线、进程入口、Kernel composition、严格配置、Database Access、HTTP Router/Server、CLI command mode、Supervisor 生命周期和 package graph 门禁。不包含认证授权、缓存、消息、OpenAPI、远程数据库部署或 UI。

## 2. 方法与既有研究复用

先检索现有 metadata，再打开 012 的模块边界、HTTP/CLI、模块开发指南、首个垂直切片任务和 `R021` 实施报告。`R021` 的刷新触发器已被“首个真实业务用例确认”命中，因此继续逐项核验当前 HEAD `abca7f44cf9c35ec60796fec9680964e9cc4298e` 的真实代码和测试。

2026-08-15 执行以下只读验证：

- `go test ./...`：通过。
- `go test ./cmd/app ./internal/kernel/composition ./pkg/database ./pkg/httpx ./pkg/cli -count=1`：通过。
- Git 状态显示当前分支已有与 014 无关的 `.agents/skills` 迁移和 `tmp/` 未跟踪内容；本任务不得触碰或纳入。

当前技术选型已经由仓库实现与 012 研究固定，本任务不需要新增第三方依赖，因此没有进行重复的外部框架对比。

## 3. 当前事实

### 3.1 已具备的底座

- `cmd/app` 已明确区分有参数 Bootstrap CLI 与无参数 Service；Service 使用 `Kernel -> Coordinator -> Host -> Supervisor`，配置只加载一个严格候选。
- `internal/kernel/composition.Compose` 已提供 Logger、Clock、ID Generator、Validator、Database、Cache、I18n 和 Storage 稳定能力；普通业务对象尚未进入 Kernel Plan。
- `internal/kernel/app/database.Access` 只在租约回调内暴露无 Close 权限的 Client；`pkg/database` 已提供可移植 Schema、additive `Migrate`、受字段白名单约束的 `BaseRepository`、事务和稳定错误分类。
- 默认 Database 是 pure-Go SQLite，当前配置可直接使用 `.data/app.db`；同一 Schema 仍可运行于现有 PostgreSQL/MySQL Driver，但 014 产品验收只要求 SQLite。
- `pkg/httpx.Router` 隐藏 chi，支持项目 Handler、Middleware、path 参数和 JSON；`pkg/httpx.Server` 已由 Host 监督 Start/Run/Stop。
- `pkg/cli.CommandSpec` 已声明 `CommandModeApplication` 与副作用类别，CLI registry 会校验完整命令树冲突；项目尚无真实 Application command。
- `internal/kernel/config.Binding` 已把默认值、ConfigPath 和 strict candidate validation 绑定；application-owned section 变化会在副作用前返回 `RestartRequired`。
- `pkg/fault` 已提供粗粒度 invalid/not-found/conflict/unavailable/timeout/canceled/internal 分类，可由 Todo Adapter 映射为协议 reason，同时保留错误链。

### 3.2 当前缺口

- Service 仍把 `http.NotFoundHandler()` 交给 HTTP Server，没有 Router、业务 Handler 或路由贡献。
- 仓库没有 `internal/business`、业务 Model、Service、Repository port、数据库 Adapter 或模块局部装配。
- `ComposeBootstrap` 只聚合底层配置和 `config init`；业务 config/command 还没有可由应用 composition 显式加入的参数。
- `CommandModeApplication` 只有声明和校验，当前 `len(args) > 0` 一律进入无 Kernel 的 Bootstrap；没有“解析命令后启动资源、执行一次用例、反序停止”的运行路径。
- `Supervisor.Run` 只适合长期任务，零 runner 会等待 context；不能用 runner 的正常返回冒充 one-shot，因为正常提前返回会被判为 `UnexpectedCompletionError`。
- 当前 package graph 门禁只约束 Kernel/composition；还没有业务 model/service/repo/handler/binding 的依赖方向、模块 ID 与 route 冲突测试。

## 4. 用户决策

- Todo 是本仓库首个真实业务垂直切片，不是占位目录或空 CRUD 示例。
- 目录采用“模块内分层”，每个业务能力内部收口 model、service、repo、handler 和各类 binding。
- 采用完整闭环：HTTP、SQLite、真实配置项和真实 CLI 命令必须复用同一个 Service。
- 本轮只完成研究与实施计划；源码、配置、测试、服务启动、暂存和提交必须等计划报告后的独立确认。

## 5. 推断与方案影响

### 5.1 可行性推断

Todo 可以在不新增第三方依赖、不改变 Kernel 资源平面、不暴露 GORM/chi/Cobra 类型给 Service 的前提下实现。现有 Database、HTTP、CLI、Config、Clock、ID、I18n 与 Supervisor 能力足够；缺口是应用级组合契约，而不是底层技术选型。

### 5.2 必须新增的最小机制

- 建立 `internal/business/todo` 模块内分层和 `internal/business` 的最小 typed route/module contribution；不建立扫描、Registry 或通用 DI。
- 用 `binding/model` 提供 Todo Schema 和 migration participant，用 `binding/config` 提供 strict section，用 `binding/http` 和 `binding/cli` 提供已绑定入口。
- 建立 `internal/composition` 作为业务/进程唯一 composition root，让 `cmd/app` 不再直接拼装 Todo 细节。
- 让 Kernel Bootstrap 接受 application config/command contract，但仍保持构造期无资源副作用。
- 为 Supervisor 增加明确的 one-shot operation 入口，使 Application CLI 正常完成后执行反向停止，而不是复制或伪造长期 runner 语义。

### 5.3 复用与限制

- Todo Repository 必须在每次 `Database.Access.Use/WithinTx` 回调内创建和使用 `BaseRepository`，不能让 borrowed Client/Repository/Tx 逃逸。
- Todo Schema 使用 additive migration；014 不执行删除、重命名或列类型变更，也不把自动 migration 描述为通用版本化迁移系统。
- 首版 application CLI 可以复用当前完整 Kernel capability plan，但不得启动 HTTP listener 或 config watcher。细粒度 capability profile 不是 014 的前置条件。

## 6. 局限与非适用场景

- 当前只证明底座可行，Todo 代码、Schema、路由、CLI 和端到端路径尚未实现或运行。
- 未研究认证、租户、权限、幂等键、缓存、OpenAPI、Web UI、远程数据库和部署；不得从本报告推断这些能力存在。
- HTTP 错误协议、Todo 字段和 CLI 输出将在 014 计划中冻结；后续改变公开路径、字段、配置或迁移语义必须退回计划并重新确认。

## 7. 研究门禁结论

研究门禁通过。关键当前事实、能力边界、缺口、用户决策和风险均有可复核证据，剩余未知不妨碍形成实施计划。通过研究门禁不代表计划已确认，更不授权修改非文档文件。
