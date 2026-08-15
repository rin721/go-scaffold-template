# Kernel 与 App 组件装配

`internal/kernel` 同时提供底层 App 组件运行时和长期 Service 的完整代际协调协议。one-shot CLI 仍由 Coordinator/Kernel 按 Plan 启停资源；长期 Service 由 GenerationCoordinator 从同一 Snapshot 构造不可变 Application Generation。它不扫描包、不提供运行期 Service Locator，也不让业务代码查询容器。

## 固定接入路径

```text
pkg/<name>
    -> internal/kernel/app/<name>
    -> internal/kernel/composition/<name>.go
    -> app.Plan / Kernel.Install
    -> Coordinator / Kernel / Host
```

- `pkg/<name>` 定义项目能力契约并隔离第三方库。
- `internal/kernel/app/<name>` 把独立实现声明为无安装副作用的 `Definition[O]`；明确接管内置 target 的实现使用 `ReplacementDefinition[T]`。
- `internal/kernel/composition` 手工选择 Definition、建立有序 Plan，并用 typed `Replace` 指明替换目标；所有检查成功后才一次性安装。
- one-shot Coordinator 是对应 Loader 的唯一调用者；Service GenerationCoordinator 是长期 watcher 的唯一 Loader 调用者。Kernel 只执行冻结计划，完整代际由 application composition root 显式构造。

当前显式清单固定为 Kernel 内置 Logger、可选配置化 Logger replacement、Clock、ID Generator、Validator、Database、Cache、I18n、Storage。修改清单只发生在 composition，不使用 `init` 自动注册。

## 两类输出

Clock、ID Generator、Validator 使用 `app.Value`，输出普通项目接口：

```go
clockAdded, err := app.Add(plan, clockapp.System())
if err != nil {
	return err
}
clock := clockAdded.Output
now := clock.Now()
```

它们没有配置、Defaults、CLI、生命周期或换代，因此也没有 `Access.Use`。需要这些能力的后续底层组件通过 `Binding/Input` 声明依赖；当前尚未建设的上层消费者未来只需接收普通接口。

Database 使用 `ManagedConfigured + Leased + KernelInstanceSwap`，输出稳定 Access：

```go
err := capabilities.Database.Use(ctx, func(client databaseapp.Client) error {
	return useDatabase(ctx, client)
})
```

一次 `Use` 是一次实例使用租约。回调取得的 Database Client 不含 `Close`，动态对象也不是 Resource；回调结束后逃逸的 Client、Repository 和 Tx 返回 `ErrClientUnavailable`。这样 Kernel 才能等待旧代使用结束并安全关闭连接池。

Database App 在 `build` 中明确调用 `pkg/database.NewGORM`，配置只选择 `sqlite/postgres/mysql` Driver 与连接参数，不包含可切换底层实现的 Engine。业务仓储通过项目 `Schema`、`BaseRepository` 和 `Tx` 使用数据库，不接触 GORM 类型；就绪检查通过 `Database Access.Ping` 在当前资源租约内执行，不暴露 Stats 或 Close。

Cache 的底层 App 仍使用 `ManagedConfigured + Leased`，稳定 Access 不公开 `RemoteStore`；调用方通过 `cacheapp.NewClient[T]` 构造自己拥有且必须关闭的泛型 Client。长期 Service 不再对该 App 执行局部 reload，而是把 Cache backend 纳入完整 Application Generation，因此 Cache section 可同进程生效。当前 Todo 没有 Cache Client；后续模块若创建泛型 Client/L1，必须由对应 generation 持有并关闭，不能跨代共享 L1/tag index。

I18n 使用 `ManagedConfigured + Leased + KernelInstanceSwap`，但输出仍是普通 `pkg/i18n.Translator` 稳定 facade。消息文件相对进程工作目录读取；成功重载后 facade 身份不变、内部 Translator 换代，候选资源加载失败则旧翻译器继续服务。

Storage 使用 `ManagedConfigured + Leased + KernelInstanceSwap`，只治理对象存储 Manager。调用方通过 `Access.Use(ctx, Route, callback)` 借用不含 `Close` 的 Client；回调结束后逃逸 Client 会返回 `ErrClientUnavailable`。`pkg/storage.New` 提供的文件工具仍由直接调用方创建和关闭，不进入全局装配。

Logger 不是第二个 Leased Access。Kernel 构造时强制接收 baseline Manager，并把同一 Manager 作为 typed target 加入 Plan：

```go
builtin, err := app.Add(plan, builtinLoggerDefinition)
replacement, err := loggerapp.Replacement()
err = app.Replace(plan, builtin.Binding, replacement)
logger := builtin.Output // 对普通消费者仅暴露 pkg/logger.Logger。
```

未选择 replacement 时，Logger 从配置加载前开始始终委托 baseline，也不生成 `logger` 默认配置。选择后，配置化 Logger 只有在候选成功提交时才替换 Manager 当前目标；失败继续使用原目标，Stop 先恢复 baseline 再关闭配置化 Resource。`Capabilities.Logger` 始终是同一个稳定只读 facade；其动态类型也不具备 `Replace`、`Restore` 或 `Close`，typed target 只留在 Kernel composition。

## Plan 与 typed Input

`app.NewPlan -> app.Add/app.Replace -> app.InputOf -> app.Freeze -> Kernel.Install` 是唯一装配流程：

- Definition 私有字段不能由 composition 随意拼装；
- Component ID 在同一 Plan 唯一；
- Input 必须来自同一 Plan 内更早的 Binding；
- Binding 没有 `Get/Resolve`，业务运行期不能查询容器；
- Input 只在组件 Build 前通过声明的 decoder 解析，视图离开 decoder 即失效；
- Replacement 只能通过 `app.Replace` 绑定同一 Plan 中更早的 typed target；没有第二份输出，同一 target 最多替换一次；
- Freeze 后不能继续 Add；零值 FrozenPlan 不能安装；
- Install 只接受 created 状态的空 Kernel，重复安装失败且不替换原计划。

有底层依赖的组件在自身 app 包中定义 typed 依赖：

```go
clockInput := app.InputOf(clockAdded.Binding)
dependencies, err := app.DependencySet(func(values app.Values) (Dependencies, error) {
	clock, err := app.Resolve(values, clockInput)
	return Dependencies{Clock: clock}, err
}, clockInput)
```

这里的 `Resolve` 只解析该 Definition 已声明的 Input，不接受字符串或类型查询，也不能保存为运行期 Resolver。

## 配置、Defaults 与 Bootstrap CLI

配置化组件通过 `app.Configured` 声明 ConfigPath、typed Decode/Validate 和可选 Defaults。没有 Defaults 的组件不会生成虚假配置段。Plan Freeze 后只把真实配置节注册安装到 Kernel；服务能力 composition 不再持有 CLI。

当前 `cmd/app` 的有参数分支调用 `composition.ComposeBootstrap`，只构造默认配置管理器和 CLI，不创建 Kernel、稳定 facade、资源、listener 或 goroutine。`config init` 聚合 Logger、Database、Cache、I18n、Storage、HTTP 与 application-owned Todo 七段：

```powershell
go run ./cmd/app config init
```

生成前先用每个 owner 的同一 strict binder 和 semantic validator 完成回环校验；未知字段、重复字段和跨类型值在资源副作用前失败。CLI registry 在第一次执行时冻结，命令树冲突、mode、副作用分类和位置参数契约在执行前校验。

## 生命周期与重载

### one-shot Kernel

初始启动按运行节点顺序执行：

```text
Decode/Validate -> Build -> optional Start -> optional Ready
-> Publish Access 或 Activate Replacement
```

启动失败时反向排空并终结已发布节点。Stop 时 Host 先撤销进程 readiness、取消 runner、反向停止上层 Participant，再让 Coordinator 触发 Kernel 的不可回滚终止 drain；终止超时进入 `cleanup-pending`，不会恢复旧入口，后续 `Stop` 继续等待同一个 drain channel。Replacement 先恢复 target，再终结自身实例。

006 已实现三种策略：

| 策略 | 当前行为 |
| --- | --- |
| `NoReload` | 无运行期配置；Fixed Managed 只在初始启动构造 |
| `KernelInstanceSwap` | 候选先 Build/Start/Ready；随后反向 drain 旧租约、提交、恢复入口并反向清理旧代 |
| `RestartRequired` | 同轮预检发现变化时返回 typed 错误，不构建、排空或应用任何变化组件；后续完整有效候选不再要求重启时解除该状态 |

候选准备期间旧 Access 继续服务。Decode、Build、Ready、reload 排空或超时失败时，候选被清理且旧入口恢复。无副作用的 `RestartRequired` 不会永久阻止 watcher：下一候选仍从 Loader 和全部 owner 校验开始，只有成功恢复到当前有效配置或完成合法热切换后才清除 restart latch。每次成功 Build 获得独立 instance generation；candidate、retired、current 的一次性 terminal finalizer 失败后缓存同一错误并保留 owner/reference，不盲目重试。提交后旧实例清理失败返回 `CommittedCleanupError`，表示新代已生效；Coordinator 会进入 `degraded`、撤销 readiness、暴露脱敏 ownership snapshot 并阻断后续 Reload，恢复配置不能误清 cleanup debt。

上述三种策略仍是独立 Kernel Plan 的底层契约，用于 one-shot 资源和可复用组件测试；长期 Service 不再把七节配置拆成多个运行中 Kernel reload 事务。

### 长期 Service Application Generation

Service 使用 `GenerationCoordinator -> GenerationFactory -> typed resource pools -> ListenerHub`：

1. FileSource 先完成有界稳定双采样，Loader 合并 File < Env 并执行全部 owner strict validation；
2. Logger、Database、Cache、I18n、Storage 按 section digest 获取 typed 引用，未变化资源复用，变化资源建立独立 immutable component runtime；
3. 从同一 Snapshot 重建 Todo Repository/Policy/Service/Handler/Router 和 `http.Server`；初始代执行 migration，reload 只读校验 Schema；
4. ListenerHub 在候选期 bind 新地址或复用同一物理 listener，候选 Server 先 Serve-ready 但没有 admission；
5. commit 切换 route 与 process logger target，旧 route 停止接收新连接；旧 Server 排空后按 Storage、I18n、Cache、Database、Logger 反向释放引用；
6. candidate 失败反向 Abort 且 current 不变；提交后清理失败形成 cleanup debt、撤销 readiness 并 fail-closed。

`WatchFiles` 监听配置文件父目录，Write/Create/Rename/Remove 经防抖后只投递容量一的 latest-wins 通知。GenerationCoordinator 串行加载和提交；非法候选上报后 watcher 继续工作。普通同步 HTTP 连接固定一个 generation；当前没有 TLS/HTTP3、WebSocket、SSE 或 hijacked connection 的跨代保证。

Loader 按声明顺序合并 Source，当前应用是 `FileSource -> EnvSource`，因此环境变量覆盖文件。每个 object scope 的大小写等价 key 必须唯一；object/object 递归合并，scalar/array/null 组成的 non-object 由高优先级 Source 整体替换，object 与 non-object 任一方向改形状都会在 Snapshot 产生前失败。同一 EnvSource 还会确定性拒绝重复逻辑路径、大小写别名、空 segment 和祖先/后代路径，错误只携带 Source、路径与类别，不输出配置值。

Reload 比较的是合并后的有效配置段摘要：如果文件字段已被环境变量覆盖，文件变化不会重建相关组件。运行中只能重新读取进程启动时继承的环境，另一个 shell 后续设置的变量不会进入该进程。Source syntax/shape 失败由 Coordinator 在 owner validation、Kernel Stage/Build 和任何资源副作用前返回；失败候选没有 Snapshot 或 provenance。

`NativeAtomicReload`、`ComponentHandoff`、切换观察期与健康失败自动回切尚未实现；当前成功切换后立即清理旧代。

## 统一运行诊断

`Host.Diagnostics()` 是当前唯一 process-level management authority。它把 Coordinator 的 Kernel state、config generation/digest/provenance、restart/cleanup required 和 component ownerships，与 Supervisor 的 process state、共享 shutdown budget、Participant/Task units 组合为一份 `ProcessDiagnostics`。Health 也只消费这份视图，不再自行拼接两份快照。

`Responsibilities` 使用专用类型区分 capability、participant 和 task，并表达 generation、phase、state、exit policy、attempt、release verification、error type 与 since；`PendingUnits` 只从仍在 drain/finalize/Stop 等待中的责任确定性派生。Stop 已返回 error 是 failed，显式 force 是 forced，只有实际仍未返回的 operation 是 pending。clean `stopped` 要求全部已启动 process responsibility clean terminal，Kernel 当前代也已 finalized。

快照只包含稳定 owner ID、有限状态、摘要、来源名、时间和 Go error type；不保存 config value、实例、指针、原始 error text、DSN 或凭据。当前没有 management listener、HTTP diagnostics endpoint、跨进程 CLI recovery、retry/force operation 或持久化审计；这些仍属于后续独立设计。

## 运行示例

当前进程级示例集中在 `internal/composition`：`Application.Run` 按参数选择 one-shot CLI 或长期 Service；`prepareTodo` 只服务 invocation-scoped CLI，`runService` 创建 GenerationCoordinator、完整 generation factory、ListenerHub 与 Supervisor。`cmd/app/main.go` 只负责进程 I/O、基线日志与信号入口。

`kernel.New` 只创建空运行时并要求显式 baseline logging manager。`composition.Compose` 完成底层组件装配；`Coordinator.Prepare` 只加载一次初始候选，供 application-owned HTTP/Todo 配置与 Kernel 共用。创建 Host 不会新增或查找组件。应用模块的实际目录和运行命令见根 [README](../../README.md) 与 [Todo 模块说明](../module/todo/README.md)。

## 边界

- 当前默认 Service 已接入完整 Application Generation；Router 安装进程 middleware 和 Todo route contribution，未匹配请求仍保持 404。
- Kernel App Plan 只服务底层组件；不为未来业务对象预设容器或构造职责。
- 基线 Logger 由应用入口拥有和关闭；配置化 Logger Resource 由 Logger App 关闭。
- Database App 的私有实例持有 `Close`，Access 只暴露使用能力；Cache App 终结 Redis，泛型 Cache Client 由其构造调用方关闭。当前 StorageManager 属于 `NoFinalization`，公开 `Close` 只保留独立消费者 API。
- 文件 Watch 的单次 Reload 错误通过回调上报并继续监听；底层 watcher 错误才终止 Task。
- 默认 Service 显式选择 generation watcher；Todo Application CLI 解析成功后才创建 one-shot Kernel/数据库，并且不创建 HTTP listener 或 watcher。
- 当前七节配置都进入完整 generation；同地址 HTTP 由进程级 ListenerHub 交接。外部数据迁移和未纳入当前 profile 的排他 consumer 仍需独立设计。
