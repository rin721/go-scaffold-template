# 开发设计：Kernel App 多态装配基础

## 1. 总体方案

006 用一个本地有序 Plan 把“选择实现”和“安装到运行时”分开：

```mermaid
flowchart LR
    Pkg["pkg 项目能力契约"] --> AppDef["kernel/app/<name> Definition"]
    AppDef --> Add["composition: app.Add"]
    Add --> Plan["本地有序 Plan"]
    Plan --> Freeze["Freeze + Defaults/CLI 校验"]
    Freeze --> Install["Kernel.Install 原子安装"]
    Install --> Start["Kernel Start/Reload/Stop"]
```

Plan 只处理底层应用组件：显式 Add、严格前向顺序、typed Binding/Input、一个稳定输出。它不是业务 DI 容器，不扫描构造函数，也没有运行期 Resolve。

## 2. 包依赖方向

目标依赖固定为：

```text
pkg/<name>
    <- internal/kernel/app/<name>
         -> internal/kernel/app
         -> internal/kernel/config（仅有配置/Defaults 时）
         -> internal/kernel/cli（仅贡献 CLI 时）

internal/kernel
    -> internal/kernel/app（只执行 Frozen Plan）

internal/kernel/composition
    -> internal/kernel
    -> internal/kernel/app
    -> internal/kernel/app/<name>
```

父级 `internal/kernel` 不被 `internal/kernel/app` 导入，避免 `kernel -> app -> kernel` 循环。`app/<name>` 可以依赖 `app` 父包和对应 `pkg`，但不能导入 composition。

## 3. 核心 API 形状

以下代码冻结行为和类型关系；实现时允许为 Go 可见性或测试性微调私有字段和辅助函数名称，但不得改变需求语义。所有伪 API 必须先通过一个不修改生产代码的最小 Go 编译探针验证可表达性；如果泛型方案需要调用方接触 `any`、反射或类型断言，实施必须停止并把代码生成替代方案补入文档重新确认。

### 3.1 Definition、Added、Binding 与 Input

```go
package app

type ID string

type Definition[O any] struct {
	// 私有、不可由调用方直接拼装。
}

type Added[O any] struct {
	Binding Binding[O]
	Output  O
}

type Binding[O any] struct {
	// 私有 Plan identity 与 node index；没有 Get/Resolve。
}

type Input[O any] struct {
	// 私有 typed reference。
}

func NewPlan() *Plan
func Add[O any](*Plan, Definition[O]) (Added[O], error)
func InputOf[O any](Binding[O]) Input[O]
func (*Plan) Freeze() (FrozenPlan, error)
```

`Added.Output` 是装配时已经具备稳定身份的输出：

- Fixed Direct 是实际项目接口；
- Managed Leased 是稳定 Access facade，真实实例在 Kernel Start 后发布。

Binding 只用于声明后续底层组件依赖。它不读取实例，也不作为 Capabilities 输出给运行期任意查询。

### 3.2 Fixed Direct

简单组件使用专门构造入口：

```go
func Value[O any](id ID, output O) (Definition[O], error)
```

Value 校验 ID 和 typed-nil 输出，形成 `NoReload`、无配置、无生命周期节点。它在 Plan 中保留身份和顺序，但 Kernel Start/Reload/Stop 不对它执行空动作。

Clock 组件示意：

```go
package clock

const ID app.ID = "clock"

func System() app.Definition[pkgclock.Clock] {
	return mustDefinition(app.Value(ID, pkgclock.System()))
}
```

公开组件构造函数不应通过 panic 处理用户配置或 I/O 错误。这里的 `mustDefinition` 只用于包内固定常量和非 nil 内建值；若实现选择带运行参数，组件构造函数返回 `(Definition[O], error)`。

### 3.3 Managed 与配置源

Managed Definition 把四类选择组合起来：

```text
Source      = FixedSource 或 ConfigContract[C]
Builder     = Build(ctx, C, resolved dependencies) -> I
Exposure    = Leased[I,O]
Lifecycle   = 可选 Starter / Ready / Stopper / Activation
Reload      = NoReload / KernelInstanceSwap / RestartRequired
Metadata    = 可选 Defaults / CLI Contract
```

目标构造入口采用选项组合小契约，不要求组件实现巨型接口：

```go
func Managed[C, D, I, O any](
	id ID,
	source Source[C],
	dependencies Dependencies[D],
	build Builder[C, D, I],
	exposure Exposure[I, O],
	reload ReloadPolicy,
	options ...Option[I],
) (Definition[O], error)
```

`FixedSource` 提供代码固定的 typed 构造输入且没有 ConfigPath；`ConfigSource` 包含 ConfigPath、Decode/Validate，并可通过 Option 附加 Defaults。Configured 必须选择 Swap 或 RestartRequired；Fixed Managed 选择 NoReload。

006 的 Managed 输出只支持 Leased facade，保证 Output 在装配时稳定存在。运行期才构造且希望 Direct 暴露的 Configured 实例必须等待后续 Stable Facade/Native Reload 设计，不能通过返回 nil、Future 或通用 Getter 绕过。

### 3.4 启动期 typed 依赖解码

Go 不能把异构泛型节点直接存入同一 slice。Plan 内部允许使用按 node index 保存的私有类型擦除值，但公开边界保持 typed：

```go
type Dependencies[D any] struct {
	// 已声明 Input 引用和包内 decoder。
}

func DependencySet[D any](
	inputs []Dependency,
	decode InputDecoder[D],
) (Dependencies[D], error)
```

组件包的 decoder 只能从 Kernel 传入的当前节点输入视图读取已经声明的 typed Input；读取使用泛型辅助函数并校验 Plan identity、node index 和目标类型。Input 视图只存在于 Build 调用期间：

- 不按字符串或 Go 类型检索；
- 不能访问未声明节点；
- 不能保存为运行期 Resolver；
- composition 只传 Binding/Input，不编写类型断言；
- 业务包不得导入或持有输入视图。

Add 时所有 Input 必须指向同一 Plan 内更早的节点，所以不存在前向引用；循环依赖因无法构造对应 Input 而不可表达。

## 4. Plan 状态与原子安装

Plan 状态为 `open -> frozen`：

- `Add` 先完整验证 Definition、ID、Output、策略和 Inputs，再原子追加一个节点；失败不改变节点清单。
- 同一 Plan 内 Component ID 不重复。
- Binding 携带不可伪造的 Plan identity；零值和跨 Plan Input 失败。
- Freeze 复制并封存节点、Defaults、CLI Contracts；Freeze 后 Add 和重复 Freeze 失败。
- Frozen Plan 提供只读启动元数据与 Kernel 执行节点，不暴露可变 slice。

Kernel 增加单一安装入口：

```go
func (k *Kernel) Install(app.FrozenPlan) error
```

Install 只允许在 `kernelCreated`、没有已安装 Plan 时执行。它先验证完整计划，再一次性替换 Kernel 内部空计划；失败不改变 Kernel。旧逐项 `register` 与 `registered map` 删除。

composition 固定事务：

1. 创建本地 Plan；
2. 按清单调用每个 `compose<Name>`；
3. Freeze；
4. 从 Frozen Plan 的可选 Defaults 创建 DefaultManager；
5. 聚合全局 `config` CLI Contract 与组件 CLI Contracts，构造可选 CLI；
6. 最后调用 Kernel.Install；
7. 全部成功后返回 Capabilities。

因此 CLI 或 Defaults 失败时 Kernel 仍为空；重复 Compose 在 Install 阶段失败且不替换原计划。

## 5. 输出和 composition

目标 Capabilities：

```go
type Capabilities struct {
	Logger        loggerapp.Access
	Clock         clock.Clock
	IDGenerator   idgen.Generator
	Validator     validation.Validator
	Database      databaseapp.Access
	Configuration config.DefaultManager
	CLI           cli.App
}
```

固定装配顺序：

```text
logger -> clock -> idgen -> validation -> database
```

这个顺序不是自动类型推导；它是当前进程人工选择的权威清单。Clock/ID/Validator 不贡献 Defaults，因此生成文档仍为 Logger、Database 两段。若未来同一接口存在 primary/audit 等角色，使用不同稳定 Component ID 和显式 Binding，不增加字符串 qualifier 查询。

组件间依赖示意：

```go
clockAdded, err := app.Add(plan, clockapp.System())
if err != nil { /* wrap and return */ }

consumerAdded, err := app.Add(plan, consumerapp.Definition(consumerapp.Inputs{
	Clock: app.InputOf(clockAdded.Binding),
}))
```

consumer app 包内部把 Input 解码成真实 `clock.Clock` 后交给普通 Builder；实现从 System 换为另一个 Clock Definition 时，consumer Builder 不变。

## 6. 生命周期编排

小契约行为：

| 契约 | 006 行为 |
| --- | --- |
| Builder | Managed 必需；响应 Context，失败不发布实例 |
| Starter | 可选；只用于打开连接或启动组件拥有的活动 |
| Ready | 可选；发布前门禁，Database Ping 迁移到这里 |
| Stopper | 拥有资源时提供；候选和当前代均使用同一释放语义 |
| Activation | 可选；不可失败提交动作，Logger manager 使用 |

初始 Start 按节点顺序逐项执行：

```text
resolve inputs -> decode/fixed source -> Build -> optional Start -> optional Ready
-> publish Access -> optional Activate -> next node
```

外部 Participant 尚未启动，所以顺序发布只供后续底层组件构造使用。后续节点失败时，已发布节点反向 drain、Deactivate、Stop；未发布候选反向 Stop。所有主错误和清理错误通过 `errors.Join` 保留。

Fixed Direct 节点没有运行动作。它的 Output 可作为后续节点输入，也可直接进入 Capabilities。

Database 的 Kernel 私有实例继续持有 `pkg/database.Client` 以执行 Ping、Stats 和 Close，但 app 包定义不含 `Close` 的调用契约供 Access 回调暴露。这样 Stopper 仍能关闭连接池，调用方不能提前释放共享实例；Logger 延续现有私有 Resource/公开 Logger 的相同所有权模式。

## 7. Reload 设计

### 7.1 预检

Kernel 串行执行 Reload，先加载完整 Snapshot，再对所有 Configured 节点执行摘要比较和 Decode/Validate。任何 Decode/Validate 错误发生在运行状态变化之前。

如果任一变化节点为 RestartRequired：

- 返回包含全部相关 ID 的 typed `RestartRequiredError`；
- `ReloadResult.RestartRequired` 记录这些 ID；
- 不构建候选、不 drain、不更新当前有效摘要；
- 其他 Swap 节点也不提前应用。

### 7.2 Swap 准备与切换

全部变化都允许 Swap 时：

1. 保持旧 Access serving，按登记顺序 Build、可选 Start、Ready 全部候选；
2. 准备失败时反向 Stop 已建候选，旧实例未被阻断；
3. 候选全部就绪后，按反向登记顺序关闭变化节点的新租约入口并等待旧租约归零；
4. 排空失败时恢复全部旧入口并反向 Stop 候选；
5. 在不可失败提交区按登记顺序替换实例、Activation 和摘要；
6. 恢复全部新入口；
7. 按反向登记顺序 Stop 旧实例。

反向 drain 保守地先阻止依赖方的新使用，再处理其前置能力；候选准备发生在 drain 前，避免依赖组件 Build 使用前置 Access 时互相等待。

006 保持当前“提交后立即清理旧代”的语义。提交后 Stop 失败继续返回 `CommittedCleanupError`，不伪装成已回滚。

### 7.3 后续策略边界

`NativeAtomicReload`、`ComponentHandoff` 和观察期回切没有足够真实组件与状态机证据，不在 006 定义枚举常量、空接口或返回 unsupported。后续变更必须在 006 稳定 Plan/Definition 基础上新增真实执行协议，并重新处理多策略同轮配置事务语义。

## 8. 目录和单轨迁移

预计新增：

```text
internal/kernel/app/
├── contracts.go
├── definition.go
├── binding.go
├── plan.go
├── exposure.go
├── runtime.go
├── clock/
├── idgen/
├── validation/
├── logger/
└── database/
```

文件可按实现职责调整，但包边界不变。迁移后删除：

```text
internal/kernel/capability/**
internal/kernel/definition.go       # 旧 Definition/Registration/typedComponent
internal/kernel/handle.go           # 旧 Handle
```

`internal/kernel/contracts.go` 中旧 Builder、InstanceHooks、Access 和辅助实现一并删除或迁入 app 新小契约；旧测试全部迁移为新 API 测试，不保留兼容 alias。

同步更新：

- `internal/kernel/kernel.go`、Host/Watch 相关测试；
- `internal/kernel/composition/*.go`；
- `cmd/app` 的 Logger Access import；
- 根 README、`docs/README.md`、Kernel/App 主题文档、对应 `pkg` README；
- 002 研究报告的“目标设计/已实现”状态表。

## 9. 测试与验证

### 9.1 API 可表达性门禁

在实现 APP-001 前，先用临时目录或测试草案验证：

- `Add[O]` 能从 `Definition[O]` 推导输出类型；
- `InputOf[O]` 能拒绝不匹配的 Binding；
- 异构 Frozen Plan 可以在内部执行而不把 `any`/反射泄漏给组件和 composition；
- Fixed Direct 与 Managed Leased 都能通过同一个 Added/Binding 形状表达；
- 跨包组件作者不需要编写类型断言。

探针只作为设计验证，不提交占位生产实现。验证失败属于公共 API 实质变化，返回方案阶段。

### 9.2 Plan 和契约测试

- Fixed Direct 输出、typed-nil、空 ID；
- Add 顺序、重复 ID、零值/跨 Plan/前向 Input；
- Add 失败不修改 Plan、Freeze 后不可变、重复 Install；
- fake Clock 多态替换后依赖 Builder 不变；
- Defaults/CLI 缺省与有值聚合；
- Leased Output 不暴露内部实例和关闭权。

### 9.3 Kernel 状态机测试

- 初始顺序 Build/Start/Ready/Publish 与反向失败清理；
- 依赖组件只在前置 Output 发布后构造；
- 候选准备期间旧 Access 继续服务；
- 反向 drain、提交顺序、失败 resume 和候选清理；
- RestartRequired 整轮无副作用；
- Reload/Stop 并发、Context 取消、超时和 race；
- Activation 初始失败回滚、Reload 提交与 Stop 撤回。

### 9.4 组件与 composition 测试

- Clock、ID Generator、Validator 为普通接口且无配置/生命周期；
- Logger 与 Database 当前配置、Defaults、Ready、Close 语义不退化，Database Access 不再暴露 Close；
- Compose 失败不安装部分计划；
- Capabilities 字段和固定清单完整；
- `config init` 内容和 Logger/Database 顺序不变。

### 9.5 最终门禁

```powershell
gofmt -w <006 修改的 Go 文件>
go mod tidy
go build ./cmd/app
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

`go mod tidy` 后 `go.mod/go.sum` 必须无非预期依赖差异。还要搜索确认 `internal/kernel/capability`、旧 Definition/Registration/Handle、逐项 `kernel.Register`、旧 import、自动注册、反射发现和运行期 Resolver 引用归零。

不连接外部 Database，不把未执行的真实服务启动描述为通过；Database 行为由 fake/sqlmock 和现有单元测试覆盖。
