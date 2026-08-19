# Kernel App 组件开发

`internal/kernel/app` 是底层能力的统一组件声明层。组件作者只声明“输出什么、怎样构造、需要哪些可选契约、配置变化如何处理”；是否启用以及选择哪个实现由 `internal/kernel/composition` 决定。

本指南只处理已经被 [应用模块开发指南](../../../docs/development/application-module-development.md) 同时判定为“跨业务复用”且“由当前进程统一选择”的底层资源。业务模块专属第三方 Adapter 留在 `internal/module/<name>/adapter/<technology>` 并完全封装；只满足复用的普通库可以位于 `pkg`，但不进入 Kernel App。拥有 SDK、Client、cache 或 goroutine 本身不会自动决定轨道，应先分别确认真实消费者、选择位置、业务归属、资源 owner 和运行 owner。

## 选择组件形态

| 能力特征 | 构造入口 | 输出 | 重载 |
| --- | --- | --- | --- |
| 代码固定、无资源生命周期 | `app.Value` | 普通项目接口 | 不进入运行节点 |
| 无运行期配置但需 Start/终结 | `app.ManagedFixed` | 稳定 Lease facade | `NoReload` |
| 配置化且新旧实例可并存 | `app.ManagedConfigured` | 稳定 Lease facade | `KernelInstanceSwap` |
| 配置变化只能随进程重启 | `app.ManagedConfigured` | 稳定 Lease facade | `RestartRequired` |
| 明确替换既有稳定 target | `app.ManagedConfiguredReplacement` | 复用 target 输出，不发布第二份输出 | `KernelInstanceSwap` |

006 不支持运行时构造后直接暴露裸实例：Managed 组件必须输出稳定 Lease facade。`NativeAtomicReload`、排他资源 Handoff 和切换观察期是后续能力，不应以空接口或先停旧再启新代替。

需要这些未支持语义、复杂资源依赖、多资源原子性，或者无法证明借用对象与后台任务排空边界时，停止组件接入并回到项目研究。优先比较 `RestartRequired`、模块级受管运行单元和扩展 Kernel 契约；改变公共生命周期或依赖模型时建立独立变更，难以逆转的决策进入 ADR。

## Fixed Direct 示例

Clock 的完整 Definition 只有一个选择：

```go
func System() app.Definition[clock.Clock] {
	definition, err := app.Value(ID, clock.System())
	if err != nil {
		panic(err) // 只允许固定常量与内建非 nil 值的不变量失败。
	}
	return definition
}
```

composition 显式加入并取得普通接口：

```go
added, err := app.Add(plan, clockapp.System())
clock := added.Output
```

这里没有 `Access.Use`、ConfigPath、Defaults、CLI 或空生命周期方法。

## Configured Leased 示例

资源组件通常包含以下步骤：

```go
source, err := app.Configured(ConfigPath, decodeAndValidate, defaults{})
definition, err := app.ManagedConfigured(
	ID,
	source,
	app.FixedDependencies(Dependencies{}),
	build,
	app.Leased(newAccess),
	app.KernelInstanceSwap,
	app.WithReady(ready),
	app.WithTerminalFinalizer(finalize),
)
```

- `decodeAndValidate` 只解码并校验，不打开资源。
- `build` 接收 Context、typed 配置和 typed 依赖，返回 Kernel 私有实例。
- `newAccess` 把 `app.Lease[I]` 收窄为组件自己的 Access；不得泄漏 `I` 的关闭权。
- `WithStart`、`WithReady` 与 `WithTerminalFinalizer` 全部可选，只声明真实行为；不拥有终结动作的值不注册空函数。terminal finalizer 在 admission 关闭并排空后每个实例代际至多执行一次，失败会缓存错误并保留 owner/reference，不承诺 retry 或 force。
- Managed Definition 在冻结时从真实 finalizer 契约确定 `NoFinalization` 或 `DrainThenTerminalClose`。`Ownerships()` 可并发读取 current/candidate/retired、drain、finalizing、finalized 与 terminal-failed 状态，并只保留最近一个安全 terminal tombstone；不保留实例或无界历史。
- finalizer 返回 nil 只证明终结动作完成。没有运行时资源专用 verifier 的当前资源标为 `VerificationNotProven`；无 finalizer 的纯值标为 `VerificationNotRequired`，不会把重复 Close 或 nil error 伪装成物理释放证明。
- 普通 Definition 不允许用隐藏 Activation 接管其他能力；CLI 由独立 Bootstrap composition 拥有，不是组件 contribution。
- Builder 返回 typed nil 会失败，实例不会发布。

## Configured Replacement 示例

只有已经存在稳定 target、且目标身份不应因实现换代而改变的能力才使用 Replacement：

```go
replacement, err := loggerapp.Replacement()
if err != nil {
	return err
}
if err := app.Replace(plan, builtinLogger.Binding, replacement); err != nil {
	return err
}
```

`ReplacementDefinition[T]` 与普通 `Definition[O]` 是不同类型，因此不能传给 `app.Add`。`app.Replace` 只接受同一 Plan 中更早的 `Binding[T]`，从该 Binding 解析并注入同一个 target；组件构造函数不能另收一个可能身份错配的 Manager。一个 target 最多有一个 replacement，失败不会留下 ID、配置节或占用状态。

Replacement 不返回 `Added.Output`。普通消费者继续使用 target 原有输出；替换组件只拥有自己构造的 Resource，并在提交/撤回边界切换 target。当前只有 Logger 有真实 mandatory baseline 和稳定 facade；其他能力不得因为 API 存在就机械套用。

## 声明底层组件依赖

后续组件只能依赖同一 Plan 中更早的 Binding：

```go
clockInput := app.InputOf(clockAdded.Binding)
dependencies, err := app.DependencySet(func(values app.Values) (Dependencies, error) {
	clock, err := app.Resolve(values, clockInput)
	if err != nil {
		return Dependencies{}, err
	}
	return Dependencies{Clock: clock}, nil
}, clockInput)
```

组件必须把所有 Input 传给 `DependencySet`。`Values` 只在该 decoder 调用期间有效；未声明 Input、跨 Plan、零值或前向 Input 都失败。composition 不编写 `any`、反射或类型断言，业务运行期也拿不到 Binding、Values 或 Resolver。

## 手工装配步骤

新增 `<name>` 的最小文件集合：

1. `pkg/<name>`：定义项目能力接口、薄封装、错误和所有权。
2. `internal/kernel/app/<name>`：定义稳定 ID 和一个或多个可选择 Definition；明确替换既有 target 时定义 Replacement。
3. `internal/kernel/composition/<name>.go`：选择当前实现并使用 `app.Add` 或显式 `app.Replace`。
4. `composition.Compose`：在正确顺序调用该 compose 函数；普通组件把 `Added.Output` 放入 `Capabilities`，Replacement 复用 target 输出。
5. 组件、Plan、composition 与文档测试。

验收时逐项确认：

- 第三方类型没有越过 `pkg` 契约；
- ID 表达稳定能力角色，同一 Plan 不重复；
- Replacement 的 target 与实际被切换对象是同一个 typed Binding 输出，且同一 target 不重复；
- 没有虚构配置、默认值或生命周期；
- Direct 输出是普通接口，普通 Swap 输出只能通过 Lease Access 使用，Replacement 只复用稳定 target；
- 创建资源的组件拥有并释放资源，消费者没有 `Close` 权；
- 配置策略符合资源特性，排他资源使用 `RestartRequired`；
- composition 失败返回零 Capabilities，Kernel 在最终 Install 前保持为空；
- Host 先停上层 Participant，再由 Kernel 反向关闭底层资源。

## 当前组件

- `app/clock`：System Clock，Fixed Direct。
- `app/idgen`：UUID Generator，Fixed Direct。
- `app/validation`：Default Validator，Fixed Direct。
- `app/logger`：配置化 Logger Replacement，没有独立 Access；只通过 typed target 在提交/停止时切换或恢复 baseline Manager。
- `app/database`：代码固定选择 GORM，配置化选择 SQLite/PostgreSQL/MySQL，Leased Swap；Build 只创建 pool，Ready 执行唯一 Ping，terminal finalizer 关闭 Kernel 私有 Resource。
- `app/cache`：配置化选择 disabled/Redis，Leased RestartRequired；Access 不暴露 RemoteStore，Redis 连接由 Kernel 关闭，泛型 Client 由调用方关闭。
- `app/i18n`：配置化 Translator，Leased Swap；对消费者输出身份稳定的 `pkg/i18n.Translator` facade。
- `app/storage`：配置化对象存储 Manager，Leased Swap；当前 Manager 没有独占 transport/goroutine，因此不声明 finalizer；按 route 借用无 Close Client，文件工具不进入该组件。
- `app/observability`：Metrics 使用 process-stable `ManagedFixed` Lease facade；Telemetry 使用 generation-owned `ManagedConfigured`，HTTP 请求在 Lease 内完成，排空后 flush/shutdown；两者都不导出 Prometheus Registry、OTel Tracer/Provider 或关闭权。
- `app/execution`：配置化（memory/disabled 开关）的后台任务执行组件，Leased Swap；输出 `pkg/execution` 的 `OperationExecutor` 稳定 facade，向业务模块提供幂等 / 失败重试 / 执行记录能力（035）。组件装配 `pkg/execution.RecoveringStore` 恢复治理（Healthy/Degraded/Recovering 状态机、有界记录缓冲 + 溢出策略、退避/抖动/最大频率探测、可用性验证、恢复后回放并原子切回主实现，goroutine 由 terminal finalizer 停止），并按模块 `Config.Policies` + `Execution.PolicyName` 提供命令式策略隔离；当前 memory backend 同时兼作外部主存储缺位时的降级兜底，多实例下不等价于分布式 Cache 语义。真正的 Cache-primary/数据库外部主存储接入为下一增量。

当前 Service 在 Kernel App Plan 外接入 application-owned HTTP listener，但没有业务 middleware、handler、service、repository 或 model；本组件模型不替它们定义目录、构造器或容器职责。

Observability 的项目契约、稳定 Metrics identity 与 generation-owned Telemetry 分裂实现见 [027](../../../docs/changes/027-business-module-third-party-isolation/README.md)。Application Generation 通过底层 composition 选择 Definition，不直接构造 Prometheus/OTel，也不把技术类型交给 Ops。
