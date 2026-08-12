# 目标组件与多态装配模型

> 本章最初描述目标模型。006 已实现 `internal/kernel/app`、有序 Plan、Direct/Leased、typed Binding/Input 和三种基础重载策略；Native Reload、Handoff、观察期与回切仍是目标设计。真实 API 以 [Kernel App 组件开发](../../../internal/kernel/app/README.md) 为准。

## 1. 统一组件化，但不统一成同一种运行协议

凡是被当前进程选用、需要形成稳定项目能力契约的底层能力，都统一经过：

```mermaid
flowchart LR
    Third["标准库或第三方库"] --> Pkg["pkg/<name><br/>项目能力契约与实现"]
    Pkg --> App["internal/kernel/app/<name><br/>组件声明与策略"]
    App --> Composition["internal/kernel/composition/<name>.go<br/>手动选择实现并绑定"]
    Composition --> Plan["Kernel 装配计划"]
    Plan --> Runtime["Kernel / Host"]
```

Clock、ID Generator、Validator 也进入 `kernel/app` 和 composition。它们与 Database 的区别不是“是不是组件”，而是选择的策略不同：

| 能力示例 | 构造来源 | 出口 | 生命周期 | 重载 |
| --- | --- | --- | --- | --- |
| System Clock | 代码固定 | 直接 `clock.Clock` | 无 | `NoReload` |
| UUID Generator | 代码固定 | 直接 `idgen.Generator` | 无 | `NoReload` |
| Default Validator | 代码固定 | 直接 `validation.Validator` | 无 | `NoReload` |
| 配置化但不可热换的能力 | 配置契约 | 直接项目接口 | 按需 | `RestartRequired` |
| Database | 配置契约 | 租约 `Access` | Build/Start/Ready/Stop | `KernelInstanceSwap` |

统一组件化带来统一的身份、实现选择、配置归属、依赖声明、诊断和测试入口；可选策略避免简单能力为不需要的资源换代付出 `Access.Use`、drain 或空 Hooks 成本。

## 2. 三层职责

### 2.1 `pkg/<name>`：能力契约与可替换实现

职责：

- 定义项目自有能力接口，例如 `clock.Clock`、`idgen.Generator`、`validation.Validator`；
- 隔离第三方类型、配置细节和错误；
- 提供可脱离 Kernel 直接构造的实现；
- 明确并发安全、资源所有权和 Close 语义；
- 为不同实现提供共同契约测试。

`pkg` 不导入 Kernel、不自注册，也不决定当前进程选用哪个实现。替换第三方库时优先保持本层契约稳定。

### 2.2 `internal/kernel/app/<name>`：组件声明

职责：

- 声明稳定组件 ID 和输出能力契约；
- 将一个或多个 `pkg` 实现封装成可供 composition 选择的 Definition；
- 按需声明配置、默认值、CLI、生命周期、健康和重载策略；
- 声明对其他底层能力的 typed 输入；
- 不调用 Register，不决定自己是否加入进程。

Clock 组件可以同时提供 `System()`、测试 Profile 使用的 `Fixed(...)` 等 Definition；ID Generator 可以提供 `UUID()`，未来替换实现时增加新的 Definition。composition 只选择其中一个，调用方只依赖相同项目契约。

### 2.3 `internal/kernel/composition/<name>.go`：手动选择与绑定

职责：

- 明确选择当前进程使用的 Definition；
- 向尚未安装的本地 Plan 加入 Definition，取得 typed `Binding[T]`；
- 把 Binding 作为其他底层组件的输入，显式形成依赖边；
- 固定启用清单和装配顺序；
- 完整计划冻结成功后一次性安装到 Kernel，失败时不留下半登记运行时；
- 不读取实例，不通过字符串或类型在运行期 Resolve。

## 3. Definition 的四个正交策略

组件 Definition 不再等同于“可换代资源说明书”，而是由四类策略组成。

### 3.1 构造与配置策略

#### `Fixed`

实现和构造参数由代码中的 composition 明确选择：

```go
// 目标伪代码，尚未实现。
func System() app.Definition[clock.Clock] {
	return app.DefineFixed(
		ID,
		func(context.Context) (clock.Clock, error) { return clock.System(), nil },
		app.Direct[clock.Clock](),
		app.NoReload(),
	)
}
```

特点：

- 没有 ConfigPath、Decode 或配置摘要；
- 不贡献默认配置和 CLI；
- 实现变化通过修改 composition 并重启进程生效；
- 适合当前 System Clock、UUID Generator、Default Validator。

“写死”只允许发生在明确的 Definition/composition 选择处，不允许消费者在 nil 时偷偷创建默认实现。

#### `Configured[C,I]`

组件通过 typed 配置构造能力：

```go
// 目标伪代码，尚未实现。
func DefineConfigured[C, I, O any](
	id app.ID,
	contract app.ConfigContract[C],
	build app.Builder[C, I],
	exposure app.Exposure[I, O],
	options ...app.Option[I],
) app.Definition[O]
```

这里 `C` 是配置、`I` 是 Kernel 拥有的实例、`O` 是真正注入出去的契约。Fixed 构造使用相应的 `DefineFixed[I,O]`，不会为了复用泛型而伪造空配置类型。

`ConfigContract[C]` 至少声明 ConfigPath、Decode 和 Validate；以下能力独立可选：

- Defaults：向 `config init` 贡献默认配置或安全空值骨架；
- CLI Contract：贡献组件专用启动前命令；
- Reload Policy：说明配置变化如何生效。

存在配置契约不等于必须提供默认值。未提供 Defaults 时，默认配置生成器不伪造该配置段，组件文档必须说明必填来源；Kernel 启动仍按 Decode/Validate 严格校验。

### 3.2 能力出口策略

#### `Direct[I,O]`

`Direct[I,O]` 形成不持有 Kernel 的 `Exposure[I,O]`：通常 `I` 本身实现项目接口 `O`，暴露步骤只做静态接口收窄。Kernel 在启动装配阶段构造一次 `I`，再把同一实例身份的 `O` 直接注入依赖它的组件。使用方调用普通接口，不经过 `Access.Use`。

适用条件：

- 实例身份在进程运行期间稳定；
- 没有实例换代，或底层对象自身能在不改变身份的前提下原子更新；
- 调用方不需要被 Kernel 逐次跟踪使用租约。

Clock、ID Generator、Validator 默认选择 Direct。

#### `Leased[I,A]`

`Leased[I,A]` 通过组件自有 Access Factory 形成 `Exposure[I,A]`。Kernel 向依赖方注入稳定的 Access 契约 `A`，每次调用通过 `Use` 建立实例 `I` 的租约。只有需要替换对象身份并等待旧使用结束时才选择。

`KernelInstanceSwap` 必须使用 Leased；不能为了接口统一让 Direct 能力也包一层 `Use`。

### 3.3 生命周期策略

生命周期由小契约按需组合：

| 契约 | 使用时机 |
| --- | --- |
| Builder | 所有需要构造实例的组件 |
| Starter | 需要打开连接、启动后台活动或准备资源 |
| Ready | 候选必须通过就绪门禁才能发布 |
| Stopper | 组件拥有需要释放的资源 |
| Health | 需要持续诊断或切换观察期 |

Clock、ID Generator、Validator 只有 Builder，不声明空 Starter、Ready、Stopper 或 Health。Kernel 根据声明编排，不要求实现巨型 Component 接口。

### 3.4 重载策略

目标策略调整为：

- `NoReload`：没有运行期配置，或实现由代码固定；默认适用于 Fixed。
- `NativeAtomicReload`：同一实例内部原子更新配置，实例身份不变，可配合 Direct。
- `KernelInstanceSwap`：构建新实例并租约排空、切换、观察和回切，必须配合 Leased。
- `ComponentHandoff`：排他资源使用组件专用交接协议。
- `RestartRequired`：配置有效但只能在下次进程启动时生效。

Configured 组件必须显式选择后四种配置变化策略之一，不能用含义模糊的“忽略变化”掩盖配置变化。

## 4. 多态能力绑定

### 4.1 `Binding[O]` 是装配声明，不是运行时取值器

目标装配入口返回不透明的 typed Binding：

```go
// 目标伪代码，尚未实现。
type Binding[O any] struct { /* 仅 Kernel 装配器可解释 */ }
```

Binding 只表示：

- 哪个组件提供最终输出契约 `O`；
- 当前进程选择了哪个实现 Definition；
- 其他组件怎样声明依赖该输出。

每个组件 Definition 只有一个主要输出契约，并携带稳定 Component ID；同一能力角色的不同实现 Definition 共用该 ID。计划按 Component ID 拒绝重复实现选择，不依赖反射比较 Go interface 类型。一个组件确需原子暴露多项方法时，应先在 `pkg` 定义有业务含义的聚合接口，不能输出匿名 map 或万能依赖对象。

允许两个不同 Component ID 输出相同 Go 接口，例如 `database.primary` 与 `database.audit` 都输出相同 Access 类型；消费者必须显式接收 composition 传来的具体 Binding，因此不存在按类型猜测使用哪个实例的问题。ID 必须表达稳定角色，不使用随意字符串 qualifier。

Binding 不暴露 `Get`、`Resolve`、当前实例或 Kernel Handle，业务运行期间也不能查询它。Kernel 在启动前冻结有序计划，在构建阶段按登记顺序把实际输出契约 `O` 注入后续组件 Builder。Direct 组件的 `O` 是普通项目接口；Leased 组件的 `O` 是组件自有 Access 接口。

### 4.2 显式手动装配示例

```go
// 目标伪代码，尚未实现。
plan := app.NewPlan()

clockBinding, err := app.Add(plan, clockapp.System())
if err != nil {
	return Result{}, err
}
idBinding, err := app.Add(plan, idgenapp.UUID())
if err != nil {
	return Result{}, err
}
validatorBinding, err := app.Add(plan, validationapp.Default())
if err != nil {
	return Result{}, err
}
```

如果另一个底层组件确实依赖这些能力，它声明 typed Inputs：

```go
// 目标伪代码，尚未实现。
componentBinding, err := app.Add(plan, componentapp.Definition(
	componentapp.Inputs{
	Clock:     app.InputOf(clockBinding.Binding),
	IDs:       app.InputOf(idBinding.Binding),
	Validator: app.InputOf(validatorBinding.Binding),
	},
))
if err != nil {
	return Result{}, err
}
frozen, err := plan.Freeze()
if err != nil {
	return Result{}, err
}
if err := runtime.Install(frozen); err != nil {
	return Result{}, err
}
```

`Input[T]` 同样只是不可运行时查询的装配声明。组件 Builder 最终得到真实接口值：

```go
type ResolvedInputs struct {
	Clock     clock.Clock
	IDs       idgen.Generator
	Validator validation.Validator
}
```

这实现多态注入：composition 可以把 `clockapp.System()` 换成另一个提供 `clock.Clock` 的 Definition，依赖方的输入契约和构造代码不变。

`Binding[databaseapp.Access]` 不能被传给需要 `database.Client` 的 Input，`Binding[clock.Clock]` 也不会凭空获得 `Use` 方法。出口协议由类型固定，不能依靠运行时断言或布尔开关切换。

## 5. 有序手动计划，而不是 DI 图容器

为了支持配置化能力在 Kernel Start 时构造并注入后续组件，目标模型只建立一个有序手动计划：

- composition 逐项 `Add`，后加入的组件只能接收前面已经得到的 typed Binding；
- Binding 必须来自同一个尚未冻结的计划，零值、跨计划或未登记 Binding 立即失败；
- 同一 Component ID 只能选择一次；同一输出接口允许按不同语义 ID 存在多份；
- Build/Start 严格按登记顺序，Stop 严格按反序；
- 前向引用和循环依赖无法表达，不需要图扫描或拓扑排序；
- 计划冻结后不允许运行时新增、删除或 Resolve；
- Kernel 只允许安装一份完整冻结计划，安装失败不会部分替换当前计划；
- 不扫描包、不使用 `init` 注册、不支持业务对象 scope 或任意查询；
- 不把 Binding、Input、Kernel 或实例索引暴露给尚未建设的上层。

如果未来出现无法按有序计划表达的真实底层组件关系，应先检查是否存在职责环或聚合错误，而不是立即升级为通用 DAG 容器。这套计划只解决底层应用组件之间的显式多态装配。

## 6. 当前三个简单能力的目标声明

### Clock

```text
Contract: clock.Clock
Definition: clockapp.System()
Construction: Fixed
Exposure: Direct
Lifecycle: Builder only
Reload: NoReload
Config/Defaults/CLI: none
```

当前没有真实配置需求，不为“统一”虚构 clock 配置。测试 Profile 可以显式选择 Fixed Clock Definition，但生产 composition 不接受隐藏 nil 回退。

### ID Generator

```text
Contract: idgen.Generator
Definition: idgenapp.UUID()
Construction: Fixed
Exposure: Direct
Lifecycle: Builder only
Reload: NoReload
Config/Defaults/CLI: none
```

未来若需要在 UUID、ULID 等实现间通过配置选择，再新增 Configured Definition，并根据实例身份策略选择 RestartRequired 或 Leased 换代；当前不预留不存在的实现枚举。

### Validator

```text
Contract: validation.Validator
Definition: validationapp.Default()
Construction: Fixed
Exposure: Direct
Lifecycle: Builder only
Reload: NoReload
Config/Defaults/CLI: none
```

未来只有出现真实规则集、locale 或扩展 tag 配置时才增加 ConfigContract。一次性 `validation.Struct` 可以继续作为局部便利函数，但进入装配图的消费者使用注入的 Validator 实例。

## 7. 可替换性的检查层次

1. **契约多态**：不同 Definition 输出同一项目能力接口。
2. **实现隔离**：第三方类型、错误和配置不越过 `pkg`。
3. **策略兼容**：替换实现若改变配置、生命周期或 Reload 能力，必须由对应 Definition 明确声明。
4. **装配可见**：实现选择只发生在 composition，代码可搜索、可审阅。
5. **契约验证**：同一套能力测试可以验证旧实现和新实现。

因此“都进入 kernel/app”表示统一治理与装配协议，不表示所有能力都要运行期换代。
