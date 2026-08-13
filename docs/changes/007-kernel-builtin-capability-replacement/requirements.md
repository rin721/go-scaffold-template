# 产品需求：Kernel 内置能力槽位与显式替换体系

## 1. 背景与当前事实

当前 Kernel 在配置加载前由应用入口创建 baseline Logger，并通过 `internal/kernel/logging.Manager` 提供动态委托。`internal/kernel/app/logger` 又构造配置化 Logger，并在 Activation 阶段直接操作该 Manager。

这套实现能完成 Logger 的专用切换，但尚未形成通用 Kernel 能力治理体系：

- Plan 不知道哪些能力是 Kernel 内置能力，也没有稳定 root Binding；
- 组件实现相同接口、注册独立实例和替换主能力之间没有统一语义；
- Logger App 同时承担普通输出和 Manager 控制面接线，边界依赖具体实现；
- Config Loader 由 `cmd/app` 直接构造，CLI App 由 composition 在 Plan Freeze 后特例构造，三项 Kernel 自带能力没有统一的组件声明位置；
- 固定组件 ID 与配置路径不能表达 db1/db2、logger.main/logger.db2 等多实例；
- 通用 App 生命周期只能排空组件自己的 Lease，不能排空主槽位的全部消费者。

因此不能继续通过增加 Logger 专用判断解决问题。本任务必须建立一个可复用于 Kernel 内置能力的显式模型，再以 Logger 完成第一条纵向验证。

## 2. 目标

### 2.1 通用目标

- Kernel 为每个内置能力声明稳定 Role、强制 baseline、消费者输出、替换策略和 baseline 所有权。
- Kernel 自带实现统一由 `internal/kernel/builtin/<name>` 封装；`internal/kernel/app/<name>` 只承载进程显式选择的替代实现或独立实例。
- 内置组件使用统一的 `BuiltinDefinition`、Role 和 Binding 规则，同时显式声明执行阶段和激活方式，不把 Config、Logger、CLI 强行伪装成相同生命周期。
- Role 显式声明 `KernelOnly` 或 `AppVisible` 可见性；Config、CLI 只供 Kernel Assembly/Runtime 使用，Logger 的 root Binding 才允许注入普通 App。
- 内置 Role 来自 Kernel 封闭清单，不能由配置文件、包扫描、`init`、反射或普通 App 动态创建。
- Kernel 与组件通过 typed Binding 显式获得能力，不查询容器，不按接口或字符串名称运行时查找。
- `Add`、`Replace` 与未来 `Decorate` 是不同操作，调用点可以直接看出装配意图。
- 初始安装和运行期替换都具有确定的准备、排空、提交、回滚和关闭语义。

### 2.2 首次落地目标

- 建立 `internal/kernel/builtin/config`、`internal/kernel/builtin/logger`、`internal/kernel/builtin/cli` 三个 baseline 组件目录。
- Config、Logger 使用 required activation；CLI Role 与 baseline Definition 始终存在，但由进程模式显式选择是否激活。
- Logger 成为首个可事务替换的内置 Role；Config 与 CLI 只完成 baseline 组件化、root Binding 和 `StartupReplace` 策略登记，不在本轮提供替代实现。
- 未注入替换组件时，Kernel 和依赖主 Logger Binding 的组件全部使用 baseline。
- 注入 Logger replacement 并成功提交后，主 Binding 的既有消费者统一切换到新实现。
- 支持额外创建独立 Logger 实例，并由指定组件通过其独立 Binding 使用。
- Database 支持显式 db1/db2 实例，用于验证 db1 跟随主槽位、db2 使用独立 Logger。

## 3. 核心语义

### 3.1 内置组件目录与边界

目标目录固定为：

```text
internal/kernel/
├── app/
│   └── <name>/       # 可选择的替代实现或独立实例
├── builtin/
│   ├── config/       # Kernel 默认 Config 组件
│   ├── logger/       # Kernel 默认 Logger 组件
│   └── cli/          # Kernel 默认 CLI 组件
├── config/           # 配置快照、默认值和监听等 Kernel 机制
└── cli/              # CLI Contract 聚合等 Kernel 机制
```

- `builtin/<name>` 负责 baseline 的 Definition、依赖、构造、输出适配和生命周期声明，是与 `app/<name>` 对称的组件层。
- `pkg/<name>` 继续定义项目能力契约并封装第三方库，不感知 Kernel Role。
- `internal/kernel/config`、`internal/kernel/cli` 等只保留多个组件共同使用的 Kernel 机制，不能继续充当隐式 baseline 组件。
- Logger 专用 `internal/kernel/logging.Manager` 在通用 slot 落地后删除；不保留 `builtin/logger` 与旧 Manager 两条切换路径。
- 目录位置只表达组件归属，不自动注册 Role；Kernel catalog 仍是唯一清单。

### 3.2 内置能力 Definition 与 Role

每个 `BuiltinDefinition` 必须声明：

- Role ID、baseline 构造和稳定输出；catalog 中的每个 Role 都必须有 baseline Definition；
- `Bootstrap`、`PreStart` 或 `Runtime` 执行阶段；
- `RequiredActivation` 或 `SelectedActivation` 激活方式；
- `KernelOnly` 或 `AppVisible` 输出可见性；
- 替换策略和 baseline 所有权。

阶段语义：

- `Bootstrap`：在读取首份配置和构建普通 App 前可用；Config、Logger 属于该阶段。
- `PreStart`：Plan Freeze 并完成契约聚合后、执行 CLI 或启动 Kernel 前构造；CLI 属于该阶段。
- `Runtime`：随普通 App RuntimeComponent 生命周期治理；当前封闭清单没有该阶段的 baseline，保留类型供真实需求使用。

激活语义：

- catalog 缺少任何 Role 的 baseline Definition 都使 Assembly 失败，不存在无 baseline Definition 的内置 Role。
- `RequiredActivation`：Assembly 必须在声明阶段构造 baseline；Config、Logger 属于此类。
- `SelectedActivation`：baseline Definition 始终登记，但仅在进程模式显式选择后构造实例；CLI 属于此类。未选择不是失败，选择后构造失败必须返回错误。

当前 catalog 固定为：

| Role | 目录 | 阶段 | 激活方式 | 可见性 | 替换策略 | baseline 所有权 |
| --- | --- | --- | --- | --- | --- | --- |
| Config | `builtin/config` | `Bootstrap` | `RequiredActivation` | `KernelOnly` | `StartupReplace` | `AssemblyOwnedBaseline` |
| Logger | `builtin/logger` | `Bootstrap` | `RequiredActivation` | `AppVisible` | `RuntimeTransaction` | `AssemblyOwnedBaseline` |
| CLI | `builtin/cli` | `PreStart` | `SelectedActivation` | `KernelOnly` | `StartupReplace` | `AssemblyOwnedBaseline` |

阶段与替换策略相互独立：Logger 需要在 Bootstrap 阶段立即提供 baseline，但允许在 RuntimeTransaction 中换代。

### 3.3 Role 类型与策略

每个 Role 还必须声明：

- 稳定且唯一的 Role ID；
- replacement 提供的目标类型；
- 消费者获得的输出类型及 root Binding；
- 非 nil baseline；
- `Fixed`、`StartupReplace` 或 `RuntimeTransaction` 替换策略；
- `AssemblyOwnedBaseline` 或 `BorrowedBaseline` 所有权。

策略语义：

- `Fixed`：只允许 baseline，任何 Replace 声明都在 Freeze 阶段失败。
- `StartupReplace`：允许启动图在消费者启动前选择替代实现，不支持运行期换代；配置变化应报告需要重启。
- `RuntimeTransaction`：允许初始替换和运行期候选事务，失败时保留当前代。

策略对应两种不同执行机制：

- `Fixed` 与 `StartupReplace` 在所属阶段首次使用前解析出唯一 target，随后冻结；不创建运行期 slot、Lease 或 drain 状态机。
- `RuntimeTransaction` 才建立稳定 slot；baseline 立即可见，replacement 通过候选、排空、提交和回滚改变 current target。
- Config 的 startup replacement 不能读取由待替换 Config Provider 自身生成的 Snapshot，只能使用 Assembly inputs 和更早的外部固定依赖。
- CLI 的 startup replacement 只有在 CLI `SelectedActivation` 已选择时才允许声明；Replace 不会隐式启用 CLI。未选择 CLI 却声明 replacer 在 Freeze 阶段失败。

所有权语义：

- `AssemblyOwnedBaseline`：baseline 由 Kernel Assembly 创建，并在 CLI、Runtime 和失败清理全部结束后关闭。
- `BorrowedBaseline`：baseline 由外部提供方创建和关闭，Assembly 只借用；只能通过明确的 Definition 构造入口选择。
- replacement Resource 始终由 replacement 组件拥有并关闭，不因安装进槽位而转移所有权。

Logger 固定采用 `RuntimeTransaction + AssemblyOwnedBaseline`，因为 Kernel Assembly 在 App 图建立前创建它，并在所有 CLI/Runtime 工作和失败清理结束后关闭。Config replacement 必须在首次 `Load` 前完成选择，且不能依赖由自身产生的 Snapshot；CLI replacement 必须在 CLI App 构造和命令执行前完成选择。测试或嵌入场景如需借用外部 baseline，必须显式选择 `BorrowedBaseline` Definition，不能由 nil 或接口类型自动推断。

### 3.4 Provide、Replace 与 Decorate

- `Add(plan, definition)` 表示新增独立能力实例，返回该实例独有的 typed Binding。
- `Replace(plan, role, replacement)` 表示替换指定内置 Role；不返回独立 Binding。
- `StartupReplacement` 只用于 `StartupReplace`，使用 Assembly fixed inputs 和更早阶段 typed dependencies，不读取 Kernel Snapshot。
- `ManagedReplacement` 只用于 `RuntimeTransaction`，参与配置候选、排空、提交和回滚；两种模式不能互换。
- `Decorate` 表示在当前能力前后增加行为而不取得实现所有权，是与 Replace 不同的未来语义；本次只记录边界，不提供 API 或实现。
- 实现相同 Go 接口只证明 compatibility，不产生替换关系。
- 同一 Resource 不得既作为 replacer 又作为独立实例发布；两种用途必须分别构造并分别拥有生命周期。

### 3.5 显式多实例

所有可重复组件使用稳定实例规格：

```go
type Spec struct {
    ID         app.ID
    ConfigPath string
}
```

- Component ID 标识实例身份，不承担接口类型或替换目标的推断。
- ConfigPath 标识配置所有权，必须与所有其他配置化组件路径唯一且不重叠。
- 配置段存在不表示组件已安装；只有 Composition 中的 `Add` 或 `Replace` 声明决定图中实例。

## 4. 必须支持的场景

### 4.1 Baseline-only

- Composition 不为 Logger Role 声明 replacer。
- Kernel 创建的 root Binding 始终指向 baseline-backed 稳定入口。
- Kernel、db1 和其他显式依赖 root Binding 的组件使用 baseline。
- Plan 不创建 Logger replacement 节点，也不生成 replacement 的默认配置。
- 配置中即使出现未绑定 Logger 段，也不得隐式创建或启用组件。

### 4.2 替换主槽位

- Composition 显式把 typed `ReplacementDefinition` 绑定到 Logger Role。
- replacement 必须先于该 Role 的普通消费者声明和启动。
- 初始 replacement 构造或 Ready 失败时，Kernel 启动失败；baseline 仍可用于诊断和清理，但失败不得被报告成成功。
- replacement 成功发布后，Kernel、db1 和其他 root Binding 消费者统一使用新实现，无需重新注入。
- 运行期候选失败时，本轮重载失败并继续使用当前代，不静默回退 baseline。
- replacement 最终停止后，主槽位恢复 baseline，再关闭 replacement Resource。

### 4.3 主槽位与独立实例共存

以两个 Database 实例验证：

- db1 显式依赖 Logger root Binding；未替换时使用 baseline，替换后跟随主槽位。
- db2 显式依赖通过 `Add` 创建的 `logging.db2` Binding，只使用该独立 Logger。
- `logging.db2` 不替换主槽位，不影响 Kernel 或 db1。
- 独立 Logger 尚未发布、正在排空或已经停止时，其 Access 返回有上下文的明确错误；db2 不得回退到主 Logger。
- 如果既要替换主槽位又要给 db2 独立 Logger，Composition 必须建立 `logging.main` replacement 与 `logging.db2` instance 两个组件。

## 5. 生命周期和失败要求

- Assembly 按 `Bootstrap -> App Plan Freeze -> PreStart -> Runtime` 固定阶段构造能力；跨阶段依赖只能从更早阶段指向更晚阶段。
- Config baseline 或其 startup replacement 必须在任何 Snapshot Load、App Decode 和 Watch 建立前确定。
- Logger baseline 必须在 Assembly 创建时可用；运行期 replacement 只能在 baseline 成功建立后声明和提交。
- CLI baseline 或 startup replacement 只能在 FrozenPlan 的 Defaults/CLI Contracts 聚合成功后构造；CLI 模式不得要求业务 RuntimeComponent 先启动。
- root Binding 在 Plan 创建时由 Kernel 注入，逻辑上早于所有 App 组件，并由 baseline 支撑。
- 只有 `AppVisible` Role 对普通 App 发布 root Binding；KernelOnly Role 的 target/output 只在 Assembly/Runtime 内部解析。
- Replace 必须在首个消费该 Role 的组件之前声明；顺序错误在 Freeze 阶段失败。
- 一个 Role 最多有一个 replacer；多个声明直接失败，不允许 last-wins。
- 运行期提交前，主槽位必须阻止新调用并等待全部在途调用退出，随后原子切换、恢复调用，再关闭旧 replacement Resource。
- 候选准备期间对 root Binding 的调用仍进入当前代；候选在成功提交前不可见。
- 提交前失败或取消必须清理候选且不改变当前代；提交阶段失败必须恢复旧代和可用状态。
- Assembly-owned、borrowed baseline 与 replacement 所有权必须分别执行，不得双重关闭或泄漏。
- 错误必须保留原因并向上返回；只有决定处理策略的边界记录日志，不能用日志代替错误。

## 6. 消费契约

可替换且拥有资源生命周期的能力应暴露项目自有 Context Access。Logger 首次采用：

```go
type Access interface {
    Use(context.Context, func(Logger) error) error
}
```

- 主槽位 Access 与独立 Logger Access 使用同一个消费者契约。
- callback 获得的 Logger 只在 callback 期间有效，不得缓存到外部跨越租约。
- Access 不暴露 `Close`、Replace、Restore、Manager 或第三方具体类型。
- Kernel、Database 等消费者依赖 `pkg/logger.Access`，不得导入具体 Logger App。
- 主槽位因 baseline 强制存在，不进入无实例状态；独立实例严格遵守 pending/draining/stopped 状态。

非资源型内置能力不强制套用 Logger Access；其 Role 可以声明更合适的强类型 Output，但不得绕过显式 Binding 和替换策略。

## 7. 计划冻结与配置要求

Freeze 必须在产生 FrozenPlan 前一次性校验并报告错误，不留下部分安装：

- Role 属于 Kernel 封闭清单；
- builtin Definition 的 Role、阶段、激活方式、可见性、策略和所有权与 catalog 一致；
- 每个 Role 都有 baseline Definition；Required activation 已构造，Selected activation 的选择状态明确；
- replacement 的 typed target 与 Role 完全匹配；
- Role 策略允许该替换模式；
- 每个 Role 最多一个 replacer；
- Replace 早于该 Role 的首个消费者；
- Component ID 唯一；
- 所有配置化组件的 ConfigPath 唯一且不存在相等、父子覆盖；
- Binding 来自同一 Plan，且依赖在消费者之前声明；
- replacement 和独立实例各自具有非空且不冲突的 ID、ConfigPath。

默认配置只聚合已经显式加入图中的 replacement 和独立实例。不得根据配置反向改变组件图。

## 8. 质量约束

- 不新增 Service Locator、全局能力、反射扫描、万能容器、`map[string]any` 核心契约或字符串 qualifier 查找。
- 不保留 Logger 专用替换路径与通用 Role 路径双轨运行。
- 不新增第三方依赖；继续通过项目自有 Logger、Database 契约隔离第三方类型。
- 中文注释和文档必须解释 Role、代际、排空、失败及资源所有权。
- 配置、错误和日志不得泄漏 DSN、密码、Token、私有路径等敏感内容。

## 9. 验收标准

- `internal/kernel/builtin/config`、`logger`、`cli` 分别成为当前 baseline 的唯一组件声明入口；旧散落构造和 Logger 专用控制面不再形成第二套现行路径。
- Config 在首次 Load 前建立，Logger 从 Assembly 到最终清理可用，CLI 在 FrozenPlan 契约聚合后且 Kernel Start 前建立；阶段顺序有测试保护。
- 任一 Role 缺少 baseline Definition 都导致 Assembly 失败；Config/Logger baseline 构造失败也失败；CLI 未选择时不构造且不是错误，选择后构造失败必须显式返回。
- 三个目标场景均有 composition 级确定性测试，且依赖关系只能通过 typed Binding 建立。
- 未替换时 Kernel/db1 使用 baseline；替换成功时共同切换；db2 独立实例始终不影响主槽位。
- 启动失败、运行期候选失败、取消、回滚、排空和关闭顺序均有测试，且没有静默 fallback。
- Freeze 能拒绝未知 Role、多 replacer、策略冲突、顺序错误、跨 Plan Binding、重复 ID 和 ConfigPath 重叠。
- 普通组件不导入 `internal/kernel/logging`、具体 Logger App、父 Kernel 或运行时容器。
- 普通组件不能取得 Config Provider 或 CLI App 的 KernelOnly Binding；需要配置时继续使用自己的 typed ConfiguredSource。
- 旧 `LoggingManager()` 控制面穿透、固定 Logger/Database 实例假设和旧 App 私有 Logger Access 被单轨清理。
- 定向测试、全量测试、race、vet、build、Markdown 链接检查和 `git diff --check` 通过。

## 10. 非目标

- 首次实施不把 Clock、ID Generator、Validator、Database Client 登记为 Kernel builtin；它们继续是普通 `internal/kernel/app` 组件。
- 本轮不提供 Config 或 CLI 的 replacement 实现，只建立 baseline Definition、Role、Binding 和 `StartupReplace` 门禁。
- 不实现 Decorate、优先级选择、多个 replacer 仲裁、自动发现或配置驱动的组件增删。
- 不新增 HTTP、业务 Service、Repository、请求 Scope、观测后端或数据库日志协议。
- 不实现跨进程能力替换、分布式协调、自动回切或长期并行兼容层。
