# 开发设计：Kernel 内置能力槽位与显式替换体系

## 1. 设计原则

本设计把“能力实例从哪里来”和“该实例是否替换 Kernel 主能力”拆开。目录名、接口实现和配置内容都不是控制信号，Composition 中的 typed 调用才是唯一装配事实。

```text
internal/kernel/builtin
  ├── config  -- Bootstrap, RequiredActivation
  ├── logger  -- Bootstrap, RequiredActivation
  └── cli     -- PreStart, SelectedActivation
          |
          v
Kernel closed catalog -> BuiltinDefinition -> BuiltinRole[Target]
                                                   |
baseline Target -----------------------> activation endpoint
                                                   |
                         +-------------------------+-------------------------+
                         |                                                   |
            StartupReplace: frozen target                    RuntimeTransaction: stable slot
                                                                         + root Binding[Output]
                                                   |
                                  +----------------+----------------+
                                  |                                 |
                        Kernel / db1 consumers              explicit Replace
                                                                    |
                                                       ReplacementDefinition[Target]

independent Definition[Output] ---> independent Binding ---> db2
```

`Provide/Add` 创建新的实例身份，`Replace` 改变既有 Role 的当前目标，`Decorate` 改变调用链行为。三者不会互相推断或复用同一个模糊入口。

## 2. 目标公共契约

以下符号已经按本设计实现。具体字段保持私有，只公开构造和只读访问所需的最小面。

### 2.1 内置组件目录

目录固定为 `internal/kernel/builtin`，子目录直接使用能力语义名：

```text
internal/kernel/
├── app/
│   └── <name>/        # 进程选择的替代实现或独立能力实例
├── builtin/
│   ├── catalog.go     # Kernel 唯一封闭 catalog
│   ├── config/        # Config baseline Definition
│   ├── logger/        # Logger baseline Definition
│   └── cli/           # CLI baseline Definition
├── config/            # Snapshot、Source、Defaults、Watch 等 Kernel 机制
└── cli/               # Contract 等 Kernel 机制
```

命名选择 `builtin`，原因是它准确表达“Kernel 随仓库提供并登记到封闭 catalog 的实现”。不选择：

- `baseline`：baseline 是 Role 中的实例身份，不是所有目录内容的职责；
- `core`：无法区分运行机制和能力实现；
- `system`：范围过宽；
- `default`：容易与配置默认值混淆。

`builtin/<name>` 与 `app/<name>` 都是组件声明层，统一使用 typed Definition、依赖、输出和所有权契约。区别是：

- builtin Definition 只能由 `builtin/catalog.go` 登记为 Kernel Role baseline；
- app Definition 只能由 Composition 通过 `Add` 创建独立实例，或通过 `Replace` 绑定到已有 Role；
- `pkg/<name>` 仍是第三方技术封装和项目能力契约，不依赖 builtin/app/Kernel。

### 2.2 Definition、阶段与激活方式

```go
type BuiltinPhase uint8

const (
    Bootstrap BuiltinPhase = iota
    PreStart
    Runtime
)

type BuiltinActivation uint8

const (
    RequiredActivation BuiltinActivation = iota
    SelectedActivation
)

type BuiltinDefinition[TTarget, TOutput any] struct {
    // 私有字段保存 Role、baseline factory、output projection、阶段、激活方式、可见性和所有权。
}
```

- `Bootstrap` 在首次配置 Load 与普通 App 构建前完成；Config、Logger 属于该阶段。
- `PreStart` 在 Plan Freeze、Defaults/CLI Contracts 聚合后，且在 CLI 执行或 Kernel Start 前完成；CLI 属于该阶段。
- `Runtime` 随 RuntimeComponent 生命周期治理；当前没有 builtin 使用，保留是为了让阶段集合完整，不得因此创建空节点。
- catalog 中每个 Role 都必须有 baseline Definition，缺失即 Assembly 失败。
- RequiredActivation 在所属阶段必须构造；Config、Logger 使用此模式。
- SelectedActivation 仅在进程模式显式选择后构造；CLI 使用此模式，未选择时其 output Binding 不可请求，且不是失败。
- KernelOnly output 只保存在 Assembly/Runtime 内部；Config、CLI 使用此可见性。
- AppVisible output 才会返回 typed Binding 给 Composition；Logger 使用此可见性。

统一封装不等于统一执行时序。Config 不能依赖自身生成的 Snapshot，CLI 不能要求 Runtime 先启动，Logger 又必须覆盖全部阶段；阶段校验必须阻止循环和倒序依赖。

### 2.3 Role 与策略

```go
type RoleID string

type ReplacementPolicy uint8

const (
    Fixed ReplacementPolicy = iota
    StartupReplace
    RuntimeTransaction
)

type BaselineOwnership uint8

const (
    BorrowedBaseline BaselineOwnership = iota
    AssemblyOwnedBaseline
)

type BuiltinRole[TTarget any] struct {
    // 私有字段只保存可替换 Role 身份。
}

type BuiltinOutput[TOutput any] struct {
    // 私有字段保存 AppVisible root Binding。
}

func (o BuiltinOutput[TOutput]) Binding() Binding[TOutput]
```

`TTarget` 是 replacement 向槽位提供的项目能力类型，`TOutput` 是消费者获得的稳定入口。二者分离，使 Resource 的所有权对象不必直接暴露给消费者。所有 Role 都能被显式 Replace，但只有 `AppVisible` Role 返回 `BuiltinOutput`；不能通过运行时错误模拟可见性。

Plan 内部使用私有的非泛型 runtime role 接口保存不同具体泛型实例；typed facade 和 `Replace` 在编译期保证 target/output 对应关系。实现不得使用反射、字符串类型比较或 `map[string]any` 完成类型路由。

Role policy 决定 activation endpoint 的实现：

- `Fixed`：阶段激活时直接采用 baseline，拒绝 Replace。
- `StartupReplace`：在 Role 所属阶段首次使用前，从 baseline 与唯一显式 replacer 中选定 target 并冻结；不创建运行期 slot、Lease 或 drain。Config、CLI 使用该机制。
- `RuntimeTransaction`：创建稳定 slot，baseline 在 Bootstrap 立即可见，replacement 通过运行期事务改变 current。Logger 使用该机制。

CLI 使用 `SelectedActivation`，所以声明 CLI Replace 之前必须先在 AssemblyOptions 中选择 CLI 模式；Replace 本身不产生激活授权。Config replacement 只能使用 Assembly inputs 和更早的外部固定依赖，不能反向读取 Config Snapshot。

### 2.4 Kernel Assembly、封闭 catalog 与 Plan 创建

生产代码不再由 `cmd/app` 分别创建 Config Loader、Logging Manager，再让 composition 特例创建 CLI。新增 Kernel Assembly，统一选择 builtin Definition、建立阶段图并创建 Runtime：

```go
type Builtins struct {
    Config app.BuiltinRole[config.Provider]
    Logging struct {
        Role   app.BuiltinRole[pkglogger.Logger]
        Output app.BuiltinOutput[pkglogger.Access]
    }
    CLI app.BuiltinRole[kernelcli.Factory]
}

type AssemblyOptions struct {
    Config builtinconfig.Options
    Logger builtinlogger.Options
    CLI    *builtincli.Options
}

func NewAssembly(options AssemblyOptions) (*Assembly, error)
func (a *Assembly) Plan() (*app.Plan, Builtins, error)
func (a *Assembly) Runtime() *Kernel
```

`NewAssembly` 和 `Plan` 完成以下工作：

1. 从 `internal/kernel/builtin` 的封闭 catalog 取得 Config、Logger、CLI Definition；
2. 校验所有 Role 都有 baseline Definition，并构造 RequiredActivation 的 Bootstrap baselines；
3. 为 Config 建立 Bootstrap frozen target，为 Logger 建立 stable slot 和 AppVisible root Binding；
4. 创建空 App Plan，并返回当前 Assembly 可用的 typed `Builtins` handles；
5. Plan Freeze 后聚合 Defaults/CLI Contracts，再构造已选择激活的 PreStart CLI；
6. 所有阶段成功后一次性 Install Runtime，任一失败按反序释放 Assembly-owned Resource。

Role 与 BuiltinDefinition 构造能力只用于 `internal/kernel/builtin` 建立封闭 catalog。普通 App 和 Composition 只能使用 Assembly 返回的 handle，不能按 ID 自行构造 Role。低层单元测试使用专用 fixture；生产装配边界测试必须阻止绕过 Assembly。

当前 catalog 固定声明：

| Role | Definition | Target / Output | 阶段 | 激活方式 | 可见性 | 策略 | 所有权 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Config | `builtin/config` | `config.Provider` / `config.Provider` | Bootstrap | RequiredActivation | KernelOnly | StartupReplace | AssemblyOwned |
| Logger | `builtin/logger` | `pkglogger.Logger` / `pkglogger.Access` | Bootstrap | RequiredActivation | AppVisible | RuntimeTransaction | AssemblyOwned |
| CLI | `builtin/cli` | `kernelcli.Factory` / `pkgcli.App` | PreStart | SelectedActivation | KernelOnly | StartupReplace | AssemblyOwned |

这里的 `config.Provider` 和 `kernelcli.Factory` 是目标项目契约名称，实施时必须是收敛的 typed interface：Config 只暴露 Kernel 需要的 `Load/FilePaths`，CLI Factory 只接收已冻结的 Contract Set 并构造 App。不得直接把可变 Loader 或第三方 CLI 实现暴露为跨层控制面。

默认 Logger Resource 改由 `builtin/logger` 根据 options 构造，并由 Assembly 最后关闭。测试或嵌入场景若确需借用外部 Logger，可由 Definition 的明确 borrowed 构造入口表达，但生产默认路径不再由 `cmd/app` 手工创建 Manager。

### 2.5 实例规格

```go
type Spec struct {
    ID         ID
    ConfigPath string
}
```

- 配置化 `Definition` 和 `ReplacementDefinition` 必须接收 `Spec`，不再在包内写死单例 ID/路径。
- 无配置固定组件可以使用空 ConfigPath；配置化组件必须提供合法非空路径。
- ID 与 ConfigPath 只标识实例和配置所有权，不承担 Role 选择。

### 2.6 ReplacementDefinition 与 Replace

```go
type ReplacementDefinition[TTarget any] struct {
    // 私有字段保存 Startup 或 Managed 模式，调用方不能自行伪造。
}

func StartupReplacement[C, D, I, TTarget any](
    spec Spec,
    configuration C,
    dependencies Dependencies[D],
    build Builder[C, D, I],
    target func(I) (TTarget, error),
    options ...Option[I],
) (ReplacementDefinition[TTarget], error)

func ManagedReplacement[C, D, I, TTarget any](
    spec Spec,
    source ConfiguredSource[C],
    dependencies Dependencies[D],
    build Builder[C, D, I],
    target func(I) (TTarget, error),
    options ...Option[I],
) (ReplacementDefinition[TTarget], error)

func Replace[TTarget any](
    plan *Plan,
    role BuiltinRole[TTarget],
    replacement ReplacementDefinition[TTarget],
) error
```

`ReplacementDefinition` 与 `Definition` 是互不转换的不同类型：

- `Add` 只接受 `Definition[TOutput]` 并返回 `Added[TOutput]`；
- `Replace` 只接受与 Role target 匹配的 `ReplacementDefinition[TTarget]`，成功时不返回 Binding；
- `StartupReplacement` 只能绑定 `StartupReplace` Role，使用显式 fixed configuration 和更早阶段 typed dependencies；不得读取 Kernel Snapshot，也不产生 RuntimeComponent；
- `ManagedReplacement` 只能绑定 `RuntimeTransaction` Role，复用 ConfiguredSource、Builder、Ready/Stop 和运行期候选协议；
- `Fixed` Role 拒绝两种 replacement；构造模式与 Role policy 不符时由 `Replace` 原子拒绝；
- replacement 不创建独立 Lease output，避免同一 Resource 出现两个访问入口和两套排空计数。

`Decorate` 本次不增加类型或函数。未来若需要，必须另建方案，定义 decorator 顺序、所有权、错误和排空语义，不能让 `Replace` 接收 decorator。

## 3. Config、Logger、CLI baseline 组件

### 3.1 `builtin/config`

```go
type Options struct {
    Sources []config.Source
}

func Definition(options Options) (
    app.BuiltinDefinition[config.Provider, config.Provider],
    error,
)
```

- Definition 构造 `config.Loader` 并收窄为只含 `Load(context.Context)`、`FilePaths()` 的 `config.Provider`。
- Config 是 `Bootstrap + RequiredActivation + KernelOnly + StartupReplace`。
- baseline 和 startup replacement 都必须在首次 Load 前冻结；replacement 使用 `StartupReplacement`，不得依赖 Snapshot、Defaults 或 CLI Contracts。
- Source 不得由配置自身动态产生；文件路径和环境前缀来自 AssemblyOptions 的进程固定输入。
- Watch 仍由 Kernel Runtime 使用 `config.Provider.FilePaths` 建立，监听机制留在 `kernel/config`。

### 3.2 `builtin/logger`

```go
type Options struct {
    Config *pkglogger.Config
}

func Definition(options Options) (
    app.BuiltinDefinition[pkglogger.Logger, pkglogger.Access],
    error,
)
```

- Definition 在 Bootstrap 创建 baseline Resource，建立 stable slot 和 Access，并把关闭权登记给 Assembly。
- Logger 是 `Bootstrap + RequiredActivation + AppVisible + RuntimeTransaction`。
- nil Config 使用 `pkg/logger` 的公开默认配置语义；不是构造失败后的隐藏 fallback。
- 测试或嵌入场景如借用现有 Resource，使用名称明确的 borrowed Definition 构造器，并由提供方关闭。

### 3.3 `builtin/cli`

```go
type Options struct {
    App pkgcli.Config
}

type Factory interface {
    Build([]kernelcli.Contract) (pkgcli.App, error)
}

func Definition(options Options) (
    app.BuiltinDefinition[kernelcli.Factory, pkgcli.App],
    error,
)
```

- CLI 是 `PreStart + SelectedActivation + KernelOnly + StartupReplace`。
- Definition 始终存在于 catalog；只有 AssemblyOptions 选择 CLI 模式时才构造 baseline Factory 和最终 App。
- Plan Freeze 后，Assembly 合并配置命令 Contract 与普通组件 CLI Contracts，再一次性调用选定 Factory。
- CLI replacement 使用 `StartupReplacement` 替换 Factory，不直接注入半构造的 App；Replace 不能隐式开启 CLI 模式。
- CLI 执行不要求 Kernel Runtime 先 Start，保持 `config init` 在配置文件不存在时可运行。

## 4. Logger 纵向切片

### 4.1 项目消费者契约

把当前 `internal/kernel/app/logger.Access` 上移为项目自有契约：

```go
package logger

type Access interface {
    Use(context.Context, func(Logger) error) error
}
```

- callback 期间由 Access 持有当前代租约；返回后消费者不得保存 Logger。
- `Use` 校验非 nil Context、Context 状态和 callback，并原样保留 callback 错误链。
- Access 不暴露 `Resource`、`Close`、slot、Manager、Replace 或 Restore。
- 主 slot Access 和独立 Logger Access 具有相同调用形态，但可用性不同：主 slot 强制有 baseline；独立 Access 遵守组件状态。

Kernel 自身也通过主 `pkglogger.Access` 使用日志能力，不再向 Composition 暴露 `*logging.Manager`。启动图建立前、候选失败和清理阶段仍由主 slot 委托到 baseline。

### 4.2 Logger App 两个明确入口

```go
func Replacement(spec app.Spec) (
    app.ReplacementDefinition[pkglogger.Logger],
    error,
)

func Instance(spec app.Spec) (
    app.Definition[pkglogger.Access],
    error,
)
```

两个入口复用同一个私有配置解码、默认值、Resource Builder、Ready/Stop 和错误处理，不复制 Logger 实现：

- `Replacement` 把 Resource 收窄成 `pkglogger.Logger` target，Resource 仍由组件 instance 保存并关闭；
- `Instance` 用组件自身 Lease 投影为 `pkglogger.Access`；
- 两者必须使用不同 Spec 才能在同一 Plan 出现；
- 删除现有 Logger App 对具体 `logging.Manager` 的依赖和 `WithActivation(Replace/Restore)` 接线。

### 4.3 主槽位实现

主槽位持有 baseline、current target、当前来源和在途调用状态。其稳定 Access 永不更换：

```text
root Binding -> stable Access -> slot.Use -> current pkglogger.Logger
                                      |
                                      +-> baseline or replacement generation
```

slot 只借用 target，不关闭 replacement Resource。replacement runtime node 负责候选和旧代 Resource；slot 只负责可见性、调用排空和 target 切换。

## 5. Composition 数据流

目标装配代码形态：

```go
assembly, err := kernel.NewAssembly(kernel.AssemblyOptions{
    Config: builtinconfig.Options{Sources: configSources},
    Logger: builtinlogger.Options{Config: baselineLoggerConfig},
    CLI:    selectedCLIOptions,
})
if err != nil {
    return Capabilities{}, err
}
plan, builtins, err := assembly.Plan()
if err != nil {
    return Capabilities{}, err
}

// 可选：显式替换主 Logger。
replacement, err := loggerapp.Replacement(app.Spec{
    ID:         "logging.main",
    ConfigPath: "logger",
})
if err != nil {
    return Capabilities{}, err
}
if err := app.Replace(plan, builtins.Logging.Role, replacement); err != nil {
    return Capabilities{}, err
}

db1Logger := app.InputOf(builtins.Logging.Output.Binding())
db1, err := app.Add(plan, databaseapp.Definition(
    app.Spec{ID: "database.db1", ConfigPath: "databases.db1"},
    db1Logger,
))

db2Logger, err := app.Add(plan, loggerapp.Instance(app.Spec{
    ID:         "logging.db2",
    ConfigPath: "loggers.db2",
}))
db2, err := app.Add(plan, databaseapp.Definition(
    app.Spec{ID: "database.db2", ConfigPath: "databases.db2"},
    app.InputOf(db2Logger.Binding),
))
```

示例中的错误必须逐项保留上下文并返回，不能按顺序省略处理。若本进程不替换主 Logger，则完全省略 `Replacement + Replace`；db1 的代码和 Binding 不变。

Plan Freeze 后，Assembly 从 FrozenPlan 聚合 Defaults/CLI Contracts，构造已选择的 PreStart CLI，再安装并返回 Runtime。`Capabilities` 按实例语义暴露需要供上层使用的 outputs，不提供 Manager、Role 构造器或运行期按名称查询方法。

## 6. Plan 模型与 Freeze 校验

### 6.1 Plan 记录

Plan 在现有组件、outputs、tokens、defaults 和 CLI 契约之外增加私有记录：

- Kernel 注册的 builtin runtime roles；
- 每个 Role 的 root binding token；
- 可选 replacement runtime node；
- Role 首次被普通组件消费的位置；
- 所有配置化组件的 ID 与 ConfigPath，不再只从 Defaults binding 间接推断。

root Binding 的 plan 身份与普通 Binding 相同，但其生产位置是 Kernel root 区域，逻辑 index 早于所有组件。`InputOf` 和 dependency validation 继续执行 same-plan、producer-before-consumer 和一次性 decode 规则。

### 6.2 原子校验

`Replace` 先校验参数和显式顺序，再以原子方式登记，不得部分写入 Plan。Freeze 在封存前统一校验：

1. Role handle 属于当前 Plan 且来自其 Kernel catalog；
2. Definition 的 Role、Phase、Activation、Visibility、Policy、Ownership 与封闭 catalog 相符；
3. 每个 Role 都有 baseline Definition，Required Config/Logger 已构造，Selected CLI 的选择状态明确；
4. 一个 Role 不超过一个 replacement；
5. Role policy 允许 replacement 的 reload 行为；
6. replacement 的阶段不晚于 Role 首次消费阶段，并早于该 Role 的首个消费者；
7. Component ID 全局唯一；
8. 所有非空 ConfigPath 两两不相等且无父子重叠；
9. 所有 input binding 同 Plan，且 producer 阶段/顺序早于 consumer；
10. runtime node、baseline、activation endpoint 和已激活 output 均非 nil；只有 RuntimeTransaction 要求 slot；
11. selected defaults/CLI 只来自真实加入图中的节点。

任一失败都不产生有效 FrozenPlan。错误包含 Role ID、Component ID 或 ConfigPath 等非敏感定位信息。

## 7. 生命周期事务

### 7.1 初始启动

replacement runtime node 必须排在该 Role 的消费者之前：

```text
baseline already serving
  -> Build replacement candidate
  -> Ready candidate
  -> PublishInitial to slot
  -> start consumers such as db1
```

- Build/Ready 期间主 slot 仍使用 baseline。
- PublishInitial 成功后，后续消费者看到 replacement。
- Build、Ready 或发布失败使 Kernel 启动失败；候选被清理，slot 保持或恢复 baseline。
- baseline 可记录边界诊断，但不能把失败转换成成功。

### 7.2 RuntimeTransaction 重载

通用 replacement node 接入现有 Stage、Build、Ready、BeginDrain、Commit、Resume、Rollback、StopPrevious 阶段：

```text
Stage/Build/Ready candidate while current remains visible
  -> BeginDrain slot: reject new Use, wait in-flight calls
  -> Commit: atomically install candidate target
  -> Resume new Use
  -> StopPrevious: close old replacement Resource
```

- 候选 Ready 期间 root Binding 仍指向当前代。
- BeginDrain 接受 Context；取消或超时保留原因，slot 必须恢复服务。
- Commit 只改变 slot target/代际，不转移 Resource 所有权。
- 只有新 target 已可见且 slot 已恢复调用后，才能关闭旧 replacement Resource。
- 旧代是 baseline 时不执行 replacement Stop；由 baseline ownership 决定最终关闭。

### 7.3 回滚与停止

- 提交前失败：DiscardCandidate，current 不变。
- BeginDrain 后、Commit 前失败：Resume current，再清理 candidate。
- Commit 后本轮后续节点失败：Rollback 恢复旧 target、Resume，再关闭失败候选；旧 target 在整轮成功前不能提前关闭。
- replacement 最终停止：排空主 slot，恢复 baseline，恢复调用，然后关闭 replacement Resource。
- Kernel Runtime 最终停止后，Assembly 按 PreStart、Bootstrap 的反序关闭 `AssemblyOwnedBaseline`；`BorrowedBaseline` 返回提供方，由提供方最后关闭。

这要求 replacement 成为一等 runtime node。现有只操作 Manager 的 `WithActivation` 无法排空 root slot 的全部调用，不作为新体系的实现基础保留。

## 8. 三个场景的确定行为

### 8.1 Baseline-only

```text
builtin/logger Resource -> Logging Role baseline -> root Access
                                                   -> Kernel
                                                   -> db1
```

没有 Logger replacement 节点和 `logger` defaults；额外配置不会改变图。

### 8.2 替换主槽位

```text
builtin/logger baseline +
                        v
root Access -> Logging slot <- logging.main replacement
     |                 current after commit
     +-> Kernel
     +-> db1
```

启动成功后 Kernel/db1 共同跟随新代；失败则启动失败，不静默继续 baseline-only。

### 8.3 独立 db2 Logger

```text
root Access ---------------------------> Kernel, db1

logging.db2 independent Access --------> db2
```

独立实例的状态、失败、重载和关闭只影响其 Binding 消费者。db2 Access 不可用时向 db2 返回错误，不查询或回退 root Access。

## 9. 配置和身份

- 组件图由 Composition 决定，配置只能为已选择节点提供值。
- defaults 输出使用各实例 `ConfigPath`，因此 main/db2、db1/db2 可以分别生成配置。
- ConfigPath 以完整分段判断重叠，`logger` 与 `logger.output`、`databases` 与 `databases.db1` 均冲突；字符串前缀但非分段父子关系不误判。
- replacement policy 属于 Kernel Role 代码契约，不放进配置，避免配置扩大授权。
- Component ID、Role ID 和 ConfigPath 的错误信息可以输出；配置值、DSN 和凭据不得输出。

## 10. 单轨迁移

首次实施按以下边界完成，不保留兼容层：

1. 建立 `internal/kernel/builtin` catalog，以及 config/logger/cli 三个 baseline Definition。
2. Config 的 Loader/Watch 机制保留在 `kernel/config`，但其生产构造入口迁至 `builtin/config`；CLI Contract 机制保留在 `kernel/cli`，App 构造迁至 `builtin/cli`。
3. 增加 Kernel Assembly，按 Bootstrap、App Plan、PreStart、Runtime 阶段完成构造、安装和反序清理；`cmd/app` 不再手工创建 Loader、Logger Manager 或 CLI App。
4. App Framework 增加 Role、root Binding、ReplacementDefinition、Replace 和 runtime replacement node。
5. Logger Access 迁至 `pkg/logger`；`builtin/logger` 构造 baseline，Logger App 改成 `Replacement(spec)` 与 `Instance(spec)`。
6. 删除 Logger App 对具体 Manager 的依赖、专用 Activation 替换接线、`LoggingManager()` 以及旧散落 builtin 构造入口。
7. Database 改为 `Spec + Input[pkglogger.Access]`，支持 db1/db2；Clock、ID Generator、Validator 继续作为普通 App，不登记 builtin。
8. Composition 增加 builtin 阶段、baseline-only、main replacement 和 independent db2 测试图。
9. 同步根 README、Kernel/App 权威文档和配置示例；004/006/旧 007 只作为历史记录，不成为并行现行规范。

若实现探针证明上述 typed API 无法在不引入循环依赖或破坏现有事务状态机的情况下成立，必须停止源码实施、记录证据并重新修正 007；不得退回 Logger 专用分支或弱类型容器。

## 11. 验证方案

### App Framework

- builtin catalog 的封闭性、Definition 阶段/激活方式/所有权和 Assembly 失败清理。
- Config/Logger 在 Bootstrap 可用，CLI 只在选择后于 PreStart 构造，跨阶段倒序依赖被拒绝。
- root Binding 的 same-plan、顺序、单次依赖解析和冻结行为。
- Replace 的 typed 注册、重复 replacer、Fixed/Startup/Runtime 策略和无部分写入。
- ID、ConfigPath 相等/父子重叠和合法相邻路径。
- 初始发布、排空、提交、Resume、Rollback、StopPrevious 和 baseline 所有权。

### Logger 与 Database

- baseline-only 的 Kernel/db1 日志目标。
- main replacement 成功、构建/Ready 失败、重载失败、最终 Restore。
- `With` 或 callback 内 bound Logger 只在租约内使用，并随下一次 Use 获取当前代。
- logging.db2 与 db2 独立重载/停止，不影响 Kernel/db1；不可用时无 fallback。
- Resource 只关闭一次，旧代关闭发生在调用排空和成功提交之后。

### 架构和仓库检查

- Config、Logger、CLI 的生产 baseline Definition 只存在于 `internal/kernel/builtin/<name>`。
- 普通 App 不导入父 Kernel、具体 Logger App、`internal/kernel/logging` 或运行时容器。
- 搜索旧 Manager 控制面、固定 Logger/Database ID、旧 Access 和旧方案标题无现行残留。
- 执行定向测试、全量 `go test ./...`、race、vet、build、Markdown 链接校验和 `git diff --check`。
