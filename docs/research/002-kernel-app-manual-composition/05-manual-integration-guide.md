# 手动接入指南

> 本章使用 `<pkg-name1>` 和 `<pkg-app-name1>` 表达开发流程。006 已实现本文的核心路径；早期伪代码可能与最终函数名不同，可编译入口和最小清单以 [Kernel App 组件开发](../../../internal/kernel/app/README.md) 为准。

## 1. 固定接入路径

凡是由当前进程统一选择和注入的底层能力，都走同一条路径：

```text
pkg/<pkg-name1>
    -> internal/kernel/app/<pkg-app-name1>
    -> internal/kernel/composition/<pkg-app-name1>.go
    -> Kernel 装配计划
    -> Direct 项目接口或 Leased Access
```

Clock、ID Generator、Validator 不再绕过这条路径。统一的是组件身份、实现选择、依赖声明和装配入口，不是强制所有组件拥有配置、CLI、Start/Stop、Health 或 `Access.Use`。

开始封装前先回答四组问题：

1. **能力契约**：项目真正需要哪些方法，哪些标准库或第三方类型不能暴露？
2. **构造来源**：实现和参数由代码固定，还是来自 typed 配置？配置是否真的需要 Defaults 或 CLI？
3. **出口形态**：进程期间是否保持同一实例身份？能否 Direct 注入，还是必须通过 Leased Access 跟踪使用？
4. **运行治理**：是否有 Start、Ready、Health、Stop，以及配置怎样生效？

四组答案彼此独立。无资源能力通常是 `Fixed + Direct + Builder only + NoReload`；Database 一类资源能力才可能是 `Configured + Leased + Lifecycle + KernelInstanceSwap`。

## 2. 第一步：封装 `pkg/<pkg-name1>`

推荐按真实复杂度组织文件，而不是机械创建固定目录：

```text
pkg/<pkg-name1>/
├── <pkg-name1>.go       # 项目自有能力接口与简单实现
├── config.go            # 仅在确有配置时存在
├── errors.go            # 仅在需要稳定错误语义时存在
├── resource.go          # 仅在拥有资源和 Close 权时存在
├── <pkg-name1>_test.go  # 契约与实现测试
└── README.md            # 直接构造方式和边界
```

资源能力的示意契约：

```go
// 伪代码：具体名称由真实能力语义决定。
type Client interface {
	Do(context.Context, Request) (Result, error)
}

type Resource interface {
	Client
	Close() error
}

type Config struct {
	Endpoint string
	Timeout  time.Duration
}

func ValidateConfig(*Config) error
func New(context.Context, *Config) (Resource, error)
```

约束：

- 对外只暴露项目类型和调用方需要的窄接口；
- `pkg.New` 不读取全局配置、不自注册 Kernel；
- 创建资源的一方明确拥有 Close，常用能力接口不泄漏关闭权；
- I/O 接收 Context，错误转换保留原始错误链；
- 不同底层实现可以通过同一套契约测试；
- `pkg` 仍可脱离 Kernel 独立构造和测试。

Clock 等简单能力不需要模仿资源接口。例如当前 `pkg/clock.Clock` 只需 `Now`、`Sleep` 和明确构造函数。

## 3. 第二步：在 `kernel/app/<pkg-app-name1>` 声明组件

### 3.1 先选择四项策略

| 维度 | 可选策略 | 选择依据 |
| --- | --- | --- |
| 构造 | `Fixed[I]` / `Configured[C,I]` | 参数来自代码还是 typed 配置 |
| 出口 | `Direct[I,O]` / `Leased[I,A]` | 实例身份是否需要运行期替换并排空使用 |
| 生命周期 | Builder + 可选 Starter/Ready/Health/Stopper | 只声明真实存在的动作 |
| 重载 | `NoReload` / Native / Swap / Handoff / RestartRequired | 配置与资源的真实生效能力 |

组件定义必须在登记时就能校验策略组合。例如：

- `Fixed` 应与 `NoReload` 组合；
- `Configured` 不能使用 `NoReload` 掩盖配置变化；
- `KernelInstanceSwap` 必须输出 Leased Access；
- `Direct` 可以配合 `NativeAtomicReload`，因为实例身份不变；
- 没有 Stopper 的组件不得声称自己拥有需要回收的资源。

### 3.2 简单能力只需要一个小组件文件

Clock 的目标结构可以只有：

```text
internal/kernel/app/clock/
├── component.go
└── component_test.go
```

目标伪代码：

```go
const ID app.ID = "clock"

// System 声明一个代码固定、直接输出且不参与重载的 Clock 组件。
func System() app.Definition[clock.Clock] {
	return app.DefineFixed(
		ID,
		func(context.Context) (clock.Clock, error) {
			return clock.System(), nil
		},
		app.Direct[clock.Clock](),
		app.NoReload(),
	)
}
```

这里没有 `ConfigPath`、Defaults、CLI、Start、Ready、Health、Stop 或 Access Adapter。它仍是完整的应用组件，只是采用最轻策略。

ID Generator 和 Validator 使用同一形状，各自输出 `idgen.Generator` 与 `validation.Validator`。如果同一契约存在多个实现，组件包可以提供多个 Definition 构造函数，但 composition 对每个必需能力只选择一个：

```go
clockapp.System()
// 或未来的 clockapp.Monotonic(...)
```

多态发生在“不同 Definition 输出同一项目接口”，不是业务运行时向容器查询实现。

### 3.3 配置契约是可选策略

只有真实需要配置时才选择 `Configured[C,I]`。其最小契约包括：

- 稳定 ConfigPath；
- 从 Snapshot 解码为 `C`；
- 无资源副作用的 Validate；
- 使用 `C` 构建 Kernel 拥有的实例 `I`，再按出口策略暴露 `O` 或 Access `A`。

以下仍然独立可选：

- **Defaults**：只有项目能够给出安全且有意义的初始值时才贡献；
- **CLI Contract**：只有组件确有启动前命令时才贡献；
- **配置摘要**：只包含判断相关变化所需的非敏感信息；
- **Reload Policy**：必须说明配置在当前进程怎样生效。

“写死”通过 `Fixed` 明确表达，不伪造空配置契约；“可配置”通过 `Configured` 明确表达，也不要求必须生成默认配置。密码、Token、DSN 不得出现在 Defaults、摘要或日志中。

### 3.4 生命周期也是可选策略

```text
Build: 构造尚未发布的能力或资源
Start: 打开连接或启动组件拥有的后台活动
Ready: 判断候选是否能够接管
Health: 运行期间提供结构化健康状态
Stop: 停止活动并释放这一代拥有的资源
```

组件只声明真实存在的契约，不提供空方法来满足巨型接口。Kernel 根据声明按依赖顺序执行 Build/Start，按反向顺序 Stop。

### 3.5 只有 Leased 出口提供 `Access.Use`

若选择 `KernelInstanceSwap`，组件输出稳定 Access：

```go
// 目标示意。
type Access interface {
	Use(context.Context, func(pkgname.Client) error) error
}
```

一次 `Use` 是一次实例租约。stream、iterator、Rows、transaction 或 session 必须在回调内完整使用和关闭，Kernel 才能准确排空旧实例。

Direct 组件输出普通项目接口，不实现 `Use`。因此 Clock 的调用保持 `clock.Now()`，而不是 `clockAccess.Use(func(clock.Clock) ...)`。

## 4. 第三步：在 composition 显式选择并绑定

每项能力拥有一个可搜索的装配文件：

```text
internal/kernel/composition/clock.go
internal/kernel/composition/idgen.go
internal/kernel/composition/validation.go
internal/kernel/composition/<pkg-app-name1>.go
```

简单能力示意：

```go
func composeClock(plan *app.Plan) (app.Added[clock.Clock], error) {
	return app.Add(plan, clockapp.System())
}
```

`Binding[O]` 不是实例，也没有 `Get` 或 `Resolve`。它只声明当前进程由哪个组件角色提供输出契约 `O`。总 Compose 先创建一个未安装的本地 Plan，再手工列出所有选择：

```go
plan := app.NewPlan()

clockBinding, err := composeClock(plan)
if err != nil {
	return Result{}, fmt.Errorf("compose clock: %w", err)
}

idBinding, err := composeIDGenerator(plan)
if err != nil {
	return Result{}, fmt.Errorf("compose id generator: %w", err)
}
```

同一 Go 接口可以有多个不同语义角色，例如 primary 与 audit Database；它们使用不同稳定 Component ID，并由 composition 把明确的 Binding 传给消费者，不使用按类型自动挑选或随意字符串 qualifier。同一角色替换底层实现时保留相同 ID。

如果 `<pkg-app-name1>` 依赖这些底层能力，composition 把 Binding 作为 typed Input 传入其 Definition：

```go
componentBinding, err := app.Add(plan, pkgnameapp.Definition(
	pkgnameapp.Inputs{
		Clock: app.InputOf(clockBinding.Binding),
		IDs:   app.InputOf(idBinding.Binding),
	},
))
```

后加入的组件只能使用前面已经取得、且属于同一 Plan 的 Binding。Plan 冻结时校验零值/跨计划 Input 和重复 Component ID，再按登记顺序把真实接口注入 Builder；关闭时严格反序。前向引用和循环依赖无法表达，因此不需要构建通用 DAG 或执行拓扑排序。计划冻结后禁止动态登记和运行时查询。

所有 `compose<Name>` 成功后，composition 才调用 `Freeze` 并把完整计划一次性 `Install` 到 Kernel。任一步失败都丢弃本地 Plan，不让 Kernel 留下半登记组件。

对于 Leased 能力，Binding 的输出类型是组件自有 `Access` 契约，而不是底层 Client。例如 `Binding[databaseapp.Access]` 会注入 Access；`Binding[clock.Clock]` 会注入普通 Clock。前者不能传给要求 `database.Client` 的 Input，后者也没有 `Use`；类型系统直接区分调用协议，避免靠注释或运行时断言约定。

## 5. 第四步：由 Kernel 和 Host 自动执行

目标启动流程：

1. composition 显式加入全部底层组件并冻结计划；
2. composition 把完整计划一次性安装到 Kernel，Kernel 校验有序 typed Binding/Input 和策略组合；
3. Kernel 聚合存在的 Defaults 与 CLI Contract；
4. 启动时按登记顺序解析配置并 Build，按需 Start/Ready；
5. Direct 输出注入稳定接口，Leased 输出注入稳定 Access；
6. Watch 只把相关变化交给声明了配置契约的组件；
7. Kernel 按各组件 Reload Policy 原地重载、换代、交接或报告需要重启；
8. Host 关闭时，Kernel 按反向依赖顺序停止所有已拥有资源。

Clock、ID Generator、Validator 在 composition 选择 Definition 时形成 Direct 输出，之后不参与 Kernel Start、Watch 或换代。Database 等组件才进入运行节点、租约和候选流程；观察期与切换后回切尚未实现。

当前尚无 HTTP、handler、service、repository 或 model。本阶段通过组件与 composition 测试验证输出，不用虚构业务消费者。

## 6. 最小文件集合

### 6.1 Fixed + Direct 简单能力

1. `pkg/<name>`：项目能力接口、实现与契约测试；
2. `internal/kernel/app/<name>/component.go`：Definition 与四项策略；
3. `internal/kernel/app/<name>/component_test.go`：输出契约与声明校验；
4. `internal/kernel/composition/<name>.go`：选择实现并返回 typed Binding；
5. composition 聚合入口：显式加入该 Binding；
6. 权威文档和 composition 测试。

### 6.2 Configured + Lifecycle 资源能力

在上述集合上按需增加 config、defaults、CLI、lifecycle、health、access、reload 文件和相应失败测试。文件可以合并，不能用空实现占位。

## 7. 逐步验收清单

### `pkg` 边界

- [ ] 公共能力契约不泄漏未经允许的第三方类型。
- [ ] 实现可以脱离 Kernel 构造和运行契约测试。
- [ ] 资源所有权、错误、Context 和并发语义明确。

### 组件声明

- [ ] 稳定 Component ID 表达能力角色；同一角色的多态实现共用 ID。
- [ ] 输出是项目自有能力接口或组件 Access，一个组件只有一个主要输出。
- [ ] `Fixed/Configured`、`Direct/Leased`、生命周期和 Reload Policy 均显式且组合合法。
- [ ] 没有真实配置时不存在 ConfigPath、Defaults 或 CLI 空契约。
- [ ] 没有真实生命周期时只保留 Builder，不存在空 Hooks。
- [ ] Direct 能力没有 `Access.Use`，Swap 能力必须使用 Leased。

### composition

- [ ] 每个启用能力都能从唯一装配文件找到实现选择。
- [ ] Clock、ID Generator、Validator 也进入显式清单，没有消费者内部 nil 回退。
- [ ] 依赖只通过 typed Binding/Input 声明。
- [ ] 零值/跨计划 Input、重复 ID 和失败均在安装前暴露；前向引用与循环依赖无法表达。
- [ ] Kernel 只接收完整冻结计划，不保留部分登记。
- [ ] 不暴露 Kernel Handle、运行时 Resolver 或第三方 Client。

### 运行与重载

- [ ] Fixed + Direct 组件只构造一次，不参与配置 Watch。
- [ ] Native Reload 失败保留旧状态；RestartRequired 变化不偷偷应用。
- [ ] Swap 候选失败恢复旧入口，切换后观察失败可回切。
- [ ] 排他资源只使用经过专门验证的 Handoff，否则 RestartRequired。
- [ ] Host 关闭能按反向依赖顺序释放所有已拥有资源并保留清理错误。
