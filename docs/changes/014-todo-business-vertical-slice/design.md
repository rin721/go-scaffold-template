# 开发设计：Todo 业务垂直切片

## 1. 设计结论

依据 [R001](research/R001-current-todo-vertical-slice-feasibility/report.md)，采用“Kernel 资源平面 + application composition + 业务模块内分层”单轨实现：

- Kernel Plan 继续只治理底层 capabilities；Todo Model、Service、Repository 和 Handler 是普通 Go 对象。
- `internal/composition` 成为唯一业务/进程 composition root；`cmd/app` 只负责标准流、信号、baseline logger 和退出码。
- Todo 同一个 Service 被 HTTP Handler 与 CLI command adapter 调用。
- 配置、Schema、HTTP Route、CLI Command 各自通过 typed binding 显式贡献，不扫描、不用 `init` 注册。
- 实施不新增第三方依赖；复用 `pkg/database`、`pkg/httpx`、`pkg/cli`、`pkg/fault`、`pkg/i18n` 和 `pkg/supervisor`。

## 2. 目录与职责

```text
internal/
├── module/
│   ├── contracts.go              # ID、Route、Contribution 与集中校验
│   ├── README.md                 # 新业务模块学习入口
│   └── todo/
│       ├── model/                # Todo、Status、纯业务转换与不变量
│       ├── service/              # Command/Query/Result、Repository port、UseCases
│       ├── repo/                 # Record 转换与 Database Access 实现
│       ├── handler/              # HTTP DTO、Handler、错误 Presenter
│       ├── binding/
│       │   ├── model/            # Schema 与 migration Participant
│       │   ├── config/           # Config、defaults、strict binding
│       │   ├── http/             # Route contribution
│       │   └── cli/              # CommandSpec 与 lazy Executor port
│       ├── module.go             # 模块局部纯装配
│       └── README.md              # 运行、扩展和测试说明
└── composition/
    ├── application.go            # 模式选择、Bootstrap Config/CLI contract 聚合
    ├── database.go               # Kernel Database Access 到业务窄契约的 Adapter
    ├── service.go                # Service Host、Router 与进程 lifecycle 装配
    └── todo.go                   # Todo 依赖绑定与 Application CLI one-shot 装配
```

目录只创建有实现内容的包。Go 包名使用 `modelbinding`、`configbinding`、`httpbinding`、`clibinding`，避免连字符和同名导入冲突；文件夹仍保留 `binding/model` 等可读语义。

## 3. 业务对象与 Service

### 3.1 Model

`model.Todo` 使用项目自有类型：

```go
type Status string

const (
    StatusPending   Status = "pending"
    StatusCompleted Status = "completed"
)

type Todo struct {
    ID          string
    Title       string
    Status      Status
    CreatedAt   time.Time
    UpdatedAt   time.Time
    CompletedAt *time.Time
    Version     uint64
}
```

Model 提供 title 规范化、Status 校验和 `Complete(now)`；不生成 ID/时间，不携带 JSON/GORM/mapstructure tag。`Complete` 对已完成对象幂等并保留原 `CompletedAt`。

### 3.2 Service API

`service.UseCases` 是 Handler/CLI 所需的窄接口：

```go
type UseCases interface {
    Create(context.Context, CreateCommand) (model.Todo, error)
    Get(context.Context, GetQuery) (model.Todo, error)
    List(context.Context, ListQuery) (ListResult, error)
    Complete(context.Context, CompleteCommand) (model.Todo, error)
}
```

Service 构造函数显式接收 Repository、Clock、ID Generator 和已校验 Config。构造时拒绝 nil 依赖；方法先检查 context，再执行业务校验。Repository port 由 `service` 定义，只暴露 `Create/Get/List/Save` 语义，不出现 GORM、Schema、Query 或 Tx 类型。

错误使用 `pkg/fault` 粗分类并保留 cause：输入为 invalid、缺失为 not-found、Version 为 conflict、资源不可用为 unavailable、context 保持 canceled/timeout，其他为 internal。Service 不记录最终错误日志。

## 4. 配置 binding

`binding/config` 拥有：

- `CapabilityID = "module.todo"`、`ConfigPath = "todo"`。
- `Config{TitleMaxRunes, DefaultListLimit, MaxListLimit}` 及 mapstructure tag。
- ordered defaults `120/20/100`。
- `Decode(snapshot)`：strict decode 后验证正数、title 上限 200 和 default ≤ max。
- `Binding()`：返回同源 `config.Binding`。

`internal/kernel/composition.ComposeBootstrap` 改为接收显式 `BootstrapOptions`：

```go
type BootstrapOptions struct {
    Configuration []config.Binding
    Commands      []kernelcli.Contract
}
```

Kernel Bootstrap 先聚合现有底层 bindings，再追加 application bindings；默认配置管理器和 CLI 都在任何资源构造前完成重复/非法契约校验。`internal/composition` 传入 Todo config 与 CLI contract，Kernel composition 不导入 Todo。

Service 与 Application CLI 都把 HTTP/Todo application bindings 交给同一个 Coordinator；Todo section 变化自动进入现有 `RestartRequired` preflight。

## 5. Model binding、Migration 与 Repository

### 5.1 持久化 Record

`repo.Record` 与 domain model 分离，字段类型严格匹配 `pkg/database.Schema`：ID/title/status 为 string，时间为 `time.Time`，CompletedAt 为 `*time.Time`，Version 为 uint64。repo 负责双向转换，并把非法持久化 status 或缺失完成时间识别为内部数据损坏。

### 5.2 Schema binding

`binding/model.Schema()` 返回唯一 `todos` Schema：

- `ID` string(36) 主键；`Title` string(200)；`Status` string(16)。
- `CreatedAt/UpdatedAt` time；`CompletedAt` nullable time；`Version` uint64。
- `VersionField = "Version"`。
- 索引 `idx_todos_status_created_at(Status, CreatedAt)`。

`binding/model.Migrator` 实现 `supervisor.Participant`，名称 `module.todo.schema`。Start 在 `Database.Access.Use` 内调用 `client.Migrate(ctx, Schema())`；Stop 不拥有长期资源，固定返回 nil。Schema participant 位于 Coordinator 之后、HTTP/operation 之前。

### 5.3 Repository 实现

`repo.Repository` 保存由使用方定义的稳定 `repo.Access` 与 Schema，不导入 Kernel Access 类型，也不保存 borrowed Client。`internal/composition/database.go` 是唯一把 Kernel-owned Database Access 适配为该窄契约的位置。每次方法在 `Use` 或 `WithinTx` 回调内创建 `pkg/database.BaseRepository[Record]` 并完成全部读取/写入/转换。

- Create：创建 Record 并返回数据库管理后的 Version。
- Get：按 ID First；`database.ErrNotFound` 转 Todo not-found。
- List：在同一个 `WithinTx` 事务租约内执行 Count 与 Find，按 Status 可选过滤并稳定排序，避免 total/items 观察到不同并发快照。
- Save：按 ID+Version 更新 status/completedAt/updatedAt，依赖 BaseRepository 原子递增 Version；零影响转 conflict。

CLI/HTTP 不直接接触 Access、Schema 或 Record。

## 6. HTTP binding 与路由

`internal/module.Route` 包含 Method、Path、已绑定 Handler 和 route middlewares；`Contribution` 包含非空唯一 ID、Routes 和 Participants。集中 validator 在 listener Start 前：

- 拒绝空/重复 ID。
- 要求 Method 为项目支持的非空 HTTP method。
- 要求 Path 以 `/` 开头、没有 query/fragment，且与规范化结果一致。
- 以大写 Method + canonical Path 拒绝重复 route。
- 拒绝 nil Handler、nil Participant 和重复 owner name。

`binding/http.Routes(handler)` 只返回 requirements 冻结的四条路由。`handler.Handler` 把 DTO 转为 Service Command/Query；Presenter 根据 `fault.CodeOf` 生成 `httpx.StatusError`，用 I18n `todo.error.*` message ID 和中文 default message，不把内部 cause 暴露给客户端。

application Router 按固定顺序安装：Recovery → RequestID（注入现有 ID Generator）→ AccessLog → SecureHeaders，再安装已验证 route。CORS、认证、限流和动态路由不在 014。

## 7. CLI binding 与 one-shot 生命周期

### 7.1 Lazy Executor

`binding/cli` 拥有 `Executor` port，方法与四个 UseCases 对应。CommandSpec 在 Bootstrap 阶段只捕获 Executor，不构造 Kernel 或 Service；Cobra 完成参数/flag 校验并调用 Run 后，Executor 才进入 application operation。

CLI Adapter 负责 flag 转换、JSON stdout 和安全错误；不调用 HTTP Handler，也不通过 loopback HTTP 访问本进程。四个命令都标记 ExternalWrite，因为 one-shot 启动阶段可能首次执行 additive Schema migration。

### 7.2 Supervisor API

为区分长期 runner 与 one-shot，新增公开方法：

```go
func (s *Supervisor) RunOperation(
    ctx context.Context,
    operation func(context.Context) error,
) error
```

语义固定为：校验无长期 Task → 正序 Start Participants → 标记 running/ready → 同步执行一次 operation → 无论结果如何在总 shutdown timeout 内反序 Stop → `errors.Join` 保留 operation 与 cleanup 错误 → 更新 stopped/failed。nil context、nil operation、重复运行、startup failure、取消和 stop timeout 都返回明确错误；不把 operation 正常返回当作 runner 异常。

`internal/composition` 的 Todo Executor 每次调用共享的 prepare 流程：创建 Loader/Kernel、执行现有完整 foundation Compose、Coordinator.Prepare、strict decode Todo Config、纯内存构造 Todo module，然后用 Coordinator + Migrator 的 Supervisor 执行一个 Service 方法。它不构造或启动 HTTP Server/Watcher。细粒度 capability profile 留待有量化启动成本时再设计。

## 8. Service 进程数据流

```text
cmd/app
  -> internal/composition.Application.Run(args=[])
  -> Loader(File -> Env) + Kernel foundation Plan
  -> Coordinator.Prepare(HTTP + Todo bindings)
  -> decode HTTP/Todo config
  -> todo.New(Database Access, Clock, ID, I18n, Config)
  -> validate Contribution -> Router -> HTTP Server
  -> Host participants:
       Coordinator -> Todo Migrator -> Application -> HTTP Server
  -> runners: HTTP Serve + config watcher
  -> ready
```

启动失败按 Supervisor 既有语义反序停止。Todo config reload 变化在任何资源副作用前返回 RestartRequired；Database generation 切换继续由稳定 Access 与 Lease 处理，Todo Repository 不持有旧 Client。

## 9. Application CLI 数据流

```text
cmd/app args
  -> Bootstrap CLI 构造（foundation defaults + Todo config/commands）
  -> Cobra 解析并校验 CommandSpec
  -> lazy Todo Executor
  -> prepare Kernel/Coordinator/Todo module
  -> RunOperation: Kernel -> Todo Migrator -> Service method -> reverse Stop
  -> JSON stdout / stable exit code
```

配置/迁移/operation/stop 任一失败都完整向上返回；cleanup error 与主错误通过 Join 同时保留。CLI 多次运行使用同一 SQLite 文件验证真实持久化。

## 10. 架构治理与兼容

- package graph 新增规则：model/service 禁止导入 Kernel、HTTP、CLI、database 或第三方；repo/handler/binding 不得导入 `internal/composition`；只有 `internal/composition` 可导入 `internal/kernel/composition`，只有 `cmd/app` 可导入 application composition。
- `ComposeBootstrap` 的调用方和测试一次性迁移到 `BootstrapOptions`，不保留旧重载或兼容函数。
- HTTP 默认 404 路径被真实 Router 单轨替换；未匹配路由仍由 Router 返回 404。
- Schema migration 只有 additive 语义；014 不删除/重命名已有表列。真实 smoke 只使用临时 SQLite 文件，用户默认 `.data/app.db` 由人工运行时创建。
- 实施完成后更新根 README、config example、business/module README、012 的业务门禁去向和 014 证据；历史方案不冒充当前实现。

## 11. 验证设计

- Model/Service：固定 Clock/ID + fake Repository，覆盖标题边界、创建、列表、幂等完成、并发冲突、取消与错误链。
- Config/Model binding：default round-trip、unknown/type/semantic failure、Schema 字段和 migration participant 顺序。
- Repository：临时 SQLite 合约测试，覆盖 create/get/list/save、not-found、version conflict、transaction/cancel 和 borrowed resource 不逃逸。
- Handler/Route：httptest 覆盖 DTO、四路由、错误映射、I18n、middleware 顺序和重复 route。
- CLI：stub Executor 覆盖 CommandSpec、flag、JSON、退出分类；进程测试覆盖真实 SQLite 多次 invocation。
- Lifecycle：RunOperation 成功/失败/取消/start/stop/timeout；Service 启动顺序和 graceful stop。
- 总门禁：全量 test/race/vet/build、tidy diff、package graph、旧 NotFound-only 装配与旧 Bootstrap 调用搜索、文档链接和 Diff 检查。
