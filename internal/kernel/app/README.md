# Kernel App 组件开发

`internal/kernel/app` 是底层能力的统一组件声明层。组件作者只声明“输出什么、怎样构造、需要哪些可选契约、配置变化如何处理”；是否启用以及选择哪个实现由 `internal/kernel/composition` 决定。

## 选择组件形态

| 能力特征 | 构造入口 | 输出 | 重载 |
| --- | --- | --- | --- |
| 代码固定、无资源生命周期 | `app.Value` | 普通项目接口 | 不进入运行节点 |
| 无运行期配置但需 Start/Stop | `app.ManagedFixed` | 稳定 Lease facade | `NoReload` |
| 配置化且新旧实例可并存 | `app.ManagedConfigured` | 稳定 Lease facade | `KernelInstanceSwap` |
| 配置变化只能随进程重启 | `app.ManagedConfigured` | 稳定 Lease facade | `RestartRequired` |
| 明确替换既有稳定 target | `app.ManagedConfiguredReplacement` | 复用 target 输出，不发布第二份输出 | `KernelInstanceSwap` |

006 不支持运行时构造后直接暴露裸实例：Managed 组件必须输出稳定 Lease facade。`NativeAtomicReload`、排他资源 Handoff 和切换观察期是后续能力，不应以空接口或先停旧再启新代替。

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
	app.WithStop(stop),
)
```

- `decodeAndValidate` 只解码并校验，不打开资源。
- `build` 接收 Context、typed 配置和 typed 依赖，返回 Kernel 私有实例。
- `newAccess` 把 `app.Lease[I]` 收窄为组件自己的 Access；不得泄漏 `I` 的关闭权。
- `WithStart/WithReady/WithStop/WithCLI` 全部可选，只声明真实行为。普通 Definition 不允许用隐藏 Activation 接管其他能力。
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

`ReplacementDefinition[T]` 与普通 `Definition[O]` 是不同类型，因此不能传给 `app.Add`。`app.Replace` 只接受同一 Plan 中更早的 `Binding[T]`，从该 Binding 解析并注入同一个 target；组件构造函数不能另收一个可能身份错配的 Manager。一个 target 最多有一个 replacement，失败不会留下 ID、Defaults、CLI 或占用状态。

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
- 没有虚构配置、默认值、CLI 或生命周期；
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
- `app/database`：配置化 Database，Leased Swap，Ready 执行 Ping，Stop 关闭 Kernel 私有 Client。

当前项目还没有 HTTP、middleware、handler、service、repository、model；本组件模型不替它们定义目录、构造器或容器职责。
