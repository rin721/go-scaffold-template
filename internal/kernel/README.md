# Kernel 与 App 组件装配

`internal/kernel` 治理当前进程选择的底层 App 组件：加载配置、按计划启动、监控配置变化、执行安全换代并反向关闭资源。它不扫描包、不构造业务对象，也不提供运行期 Service Locator。

## 固定接入路径

```text
pkg/<name>
    -> internal/kernel/app/<name>
    -> internal/kernel/composition/<name>.go
    -> app.Plan / Kernel.Install
    -> Kernel / Host
```

- `pkg/<name>` 定义项目能力契约并隔离第三方库。
- `internal/kernel/app/<name>` 把独立实现声明为无安装副作用的 `Definition[O]`；明确接管内置 target 的实现使用 `ReplacementDefinition[T]`。
- `internal/kernel/composition` 手工选择 Definition、建立有序 Plan，并用 typed `Replace` 指明替换目标；所有检查成功后才一次性安装。
- Kernel 只执行冻结计划；Host 保证上层 Participant 先于 Kernel 停止。

当前显式清单固定为 Kernel 内置 Logger、可选配置化 Logger replacement、Clock、ID Generator、Validator、Database。修改清单只发生在 composition，不使用 `init` 自动注册。

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

一次 `Use` 是一次实例使用租约。回调取得的 Database Client 不含 `Close`；Rows、事务、stream 或 session 不得逃逸回调。这样 Kernel 才能等待旧代使用结束并安全关闭连接池。

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

## 配置、Defaults 与 CLI

配置化组件通过 `app.Configured` 声明 ConfigPath、typed Decode/Validate 和可选 Defaults。没有 Defaults 的组件不会生成虚假配置段。composition 在本地 Plan Freeze 后聚合真实 Defaults 与 CLI Contract，全部构造成功后才 `Kernel.Install`。

当前 `cmd/app` 显式选择配置化 Logger replacement，因此 `config init` 仍只生成 Logger、Database 两段：

```powershell
go run ./cmd/app config init
```

CLI 未启用时 `Capabilities.CLI` 为 nil；直接配置生成可使用 `Capabilities.Configuration.Generate`。

## 生命周期与重载

初始启动按运行节点顺序执行：

```text
Decode/Validate -> Build -> optional Start -> optional Ready
-> Publish Access 或 Activate Replacement
```

启动失败时反向排空并关闭已发布节点。Stop 时 Host 先停上层 Participant，Kernel 再反向 drain；Replacement 先恢复 target，再关闭自身实例。

006 已实现三种策略：

| 策略 | 当前行为 |
| --- | --- |
| `NoReload` | 无运行期配置；Fixed Managed 只在初始启动构造 |
| `KernelInstanceSwap` | 候选先 Build/Start/Ready；随后反向 drain 旧租约、提交、恢复入口并反向清理旧代 |
| `RestartRequired` | 同轮预检发现变化时返回 typed 错误，不构建、排空或应用任何变化组件 |

候选准备期间旧 Access 继续服务。Decode、Build、Ready、排空或超时失败时，候选被清理且旧入口恢复。提交后旧实例清理失败返回 `CommittedCleanupError`，表示新代已生效，不能伪装成回滚。

`NativeAtomicReload`、`ComponentHandoff`、切换观察期与健康失败自动回切尚未实现；当前成功切换后立即清理旧代。

## 运行示例

```go
runtime, err := kernel.New(loader, kernel.Options{Logging: loggingManager})
if err != nil {
	return err
}
capabilities, err := composition.Compose(runtime, composition.Options{
	Logger: composition.ConfiguredLoggerReplacement,
})
if err != nil {
	return err
}
host, err := kernel.NewHost(runtime, kernel.HostOptions{
	Watch: &kernel.WatchOptions{OnReloadError: reportReloadError},
})
if err != nil {
	return err
}
return host.Run(ctx)
```

`kernel.New` 只创建空运行时并要求显式 baseline logging manager。`Options{}` 保留内置 baseline；示例显式选择配置化 replacement。`Compose` 完成底层组件装配；创建 Host 不会新增或查找组件。

## 边界

- 当前没有 HTTP Server、middleware、handler、service、repository、model 等业务层装配。
- Kernel App Plan 只服务底层组件；不为未来业务对象预设容器或构造职责。
- 基线 Logger 由应用入口拥有和关闭；配置化 Logger Resource 由 Logger App 关闭。
- Database App 私有实例持有 `Close`，Access 只暴露使用能力。
- 文件 Watch 的单次 Reload 错误通过回调上报并继续监听；底层 watcher 错误才终止 Task。
- HTTP 同端口、文件锁、单消费者等排他资源不能套用双实例 Swap；在专用 Handoff 落地前应选择 `RestartRequired`。
