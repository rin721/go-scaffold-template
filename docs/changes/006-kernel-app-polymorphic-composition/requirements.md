# 产品需求：Kernel App 多态装配基础

## 1. 背景与当前事实

当前仓库已实现 Logger、Database 的配置化 Kernel Capability：

- `kernel.Definition[C,T]` 强制要求 ConfigPath、Decode、Defaults、Builder 和包含 Start/Stop 的 `InstanceHooks`；
- `kernel.Register` 登记单个 Definition 后立即修改 Kernel，并返回 `Registration{Access, Defaults}`；
- `composition.Compose` 先后登记 Logger、Database，再手工搬运 Access、Defaults 和 CLI Contract；
- Kernel 对所有已登记组件使用同一种候选实例、租约排空和立即清理旧代协议；
- `internal/kernel/capability/logger` 与 `database` 分别把内部 Handle 收敛成能力 Access。

当前模型能治理可换代资源，但不能自然表达无配置、无资源、无生命周期且无需租约的能力。`pkg/clock.Clock`、`pkg/idgen.Generator`、`pkg/validation.Validator` 已存在项目契约和默认实现，却尚未进入 Kernel composition。当前 `composition.Capabilities` 也只输出 Logger、Database、Configuration 与 CLI。

另一个已确认问题是组合原子性：当前逐项 `kernel.Register` 会立即修改 Kernel。后续组件、默认配置或 CLI 组合失败时，`Compose` 虽然返回零值 Capabilities，Kernel 仍可能保留已经登记的前半部分。

## 2. 用户目标

- 所有由当前进程选择的底层能力都经过 `kernel/app/<name>` 和 `kernel/composition`，不让简单能力绕过统一装配入口。
- 统一组件身份、实现选择、依赖声明、配置契约聚合和 Kernel 安装方式，但不强制所有能力使用相同生命周期或重载协议。
- Clock、ID Generator、Validator 可以直接注入普通项目接口，不增加无意义的 `Access.Use`、空 Start/Stop 或虚构配置。
- 资源能力继续由 Kernel 自动构建、启动、就绪检查、租约排空、换代和关闭。
- 实现选择保持显式、可搜索、可测试；不引入反射扫描、自动注册、运行期 Service Locator 或通用 DI 容器。
- 第三方实现继续隐藏在 `pkg` 项目契约之后；替换实现只影响 `pkg`、对应 app Definition 和 composition 选择。

## 3. 功能需求

### 3.1 统一 App Definition

- 新增 `internal/kernel/app` 作为底层组件声明与有序计划包；它不得导入父级 `internal/kernel`，避免包循环。
- 每个 Definition 必须声明稳定 Component ID，并且只暴露一个主要项目输出契约。
- 同一语义角色的不同实现共用 Component ID；同一 Plan 内重复 ID 必须在安装前失败。
- Definition 只能通过显式 composition 加入 Plan，不允许 `init`、目录扫描、反射发现或环境变量隐式启用。
- `internal/kernel/app/<name>` 只定义组件与策略，不持有 Kernel、不自行安装，也不导入其他 composition 文件。

### 3.2 构造和配置可选

- Fixed Direct 组件可以直接持有一个代码选择的不可变项目接口，不声明 ConfigPath、Decode、Defaults 或配置摘要。
- Managed 组件可以选择代码固定的构造源，或声明 typed `ConfigContract[C]`。
- Configured 组件必须声明 ConfigPath、Decode/Validate 和配置变化策略；Decode/Validate 不得打开资源或启动 goroutine。
- Defaults 是独立可选契约。没有 Defaults 时，`config init` 不生成该组件配置段，Kernel 启动仍严格读取和校验真实配置。
- CLI Contract 是独立可选契约，不要求每个组件贡献命令，也不允许组件自行创建 CLI App。
- Fixed 值不参加配置 Watch；不能使用“忽略变化”掩盖已经声明的配置。

### 3.3 Direct 与 Leased 输出

- Fixed Direct 输出在 composition 阶段就是稳定项目接口，可直接放入 `composition.Capabilities`，调用方不经过 `Access.Use`。
- Managed replaceable 实例通过组件自有 Leased Access 输出；Access 在 Plan 装配时形成稳定身份，实例由 Kernel Start 后发布。
- Leased 回调只能获得项目窄接口，不能获得 Kernel Handle、第三方 Client 私有类型或共享 Resource.Close 权限。
- Database app 必须把当前包含 `Close` 的 `pkg/database.Client` 收敛为不含 `Close` 的项目调用接口；Close 只保留给 Kernel 私有实例所有者，修复当前 Access 回调仍可关闭共享连接池的边界漏洞。
- `KernelInstanceSwap` 必须使用 Leased 输出；Fixed Direct 不得为了统一形式包装成租约。
- 本任务不支持把运行期才构造、又没有稳定 facade 的 Configured 实例作为 Direct 输出；需要此语义时必须在后续 Native Reload/facade 任务中单独设计。

### 3.4 Typed Binding/Input 与有序 Plan

- `app.Add` 返回 typed Binding 和当前 composition 可交付的 typed Output；Binding 不暴露 `Get`、`Resolve` 或 Kernel Handle。
- 后加入的组件只能通过 `InputOf` 引用同一 Plan 中已经成功加入的 Binding。
- 零值、跨 Plan、前向引用、未登记 Input 必须在 Add/Freeze 阶段失败；严格顺序使循环依赖无法表达。
- 同一 Go 接口允许用不同稳定 Component ID 表达不同语义角色；composition 必须显式传递具体 Binding，不按类型自动选择实例。
- Plan Freeze 后不可继续 Add；完整 Frozen Plan 只能一次性安装到处于 created 状态且尚未安装计划的 Kernel。
- 任一 Definition、Defaults、CLI 或安装校验失败时，Kernel 不得保留半登记组件，Compose 返回零值 Capabilities。

### 3.5 可选生命周期

- Builder 只存在于确实需要 Kernel 构造实例的 Managed 组件。
- Starter、Ready、Stopper 和 Activation 是独立小契约；组件没有对应行为时不提供空方法。
- Ready 用于发布前门禁。Database Ping 迁移为 Ready；Logger 不再用只做 nil 检查的空 Start 模拟生命周期。
- Stopper 只由拥有待释放资源的组件提供；候选失败、初始启动回滚、Reload 清理和进程关闭都必须释放 Kernel 已拥有资源并保留全部错误链。
- Activation 继续只允许在不可失败发布边界执行无 I/O、无阻塞、无错误的稳定 facade 切换。

### 3.6 初始启动与依赖顺序

- Kernel 按 Plan 登记顺序处理 Managed 组件；Fixed Direct 值无需运行生命周期。
- 每个 Managed 组件完成 Build、可选 Start、Ready 后再发布其稳定 Output，使后续组件的 Builder 可以安全使用已经发布的前置 Access。
- 后续组件启动失败时，Kernel 必须按反向顺序撤回 Activation、停止已发布和候选实例，并使 Start 整体失败。
- Host 仍把 Kernel 作为第一个 Participant；上层 Participant 只能在完整 Kernel Start 成功后启动，停止时先于 Kernel 退出。

### 3.7 006 重载策略

- `NoReload` 只适用于无运行期配置的 Fixed 组件，不进入配置变更事务。
- `KernelInstanceSwap` 保留候选构建、可选 Start/Ready、旧租约排空、原子替换、失败恢复和旧代反向清理。
- Reload 准备候选时旧 Access 必须继续服务；全部候选就绪后按反向依赖顺序阻断并排空旧租约，减少依赖组件在候选 Build 时调用前置 Access 造成的死锁风险。
- 提交前任一失败必须恢复全部旧入口并反向清理候选；成功后按登记顺序发布新代，再反向关闭旧代。
- `RestartRequired` 在 Decode/Validate 后报告受影响 Component ID，保持当前实例和有效摘要不变，不执行停旧启新，也不自动重启进程。
- 同一轮候选含 `RestartRequired` 时不得先应用其他组件，整轮保持旧运行状态，避免产生无法准确描述的部分配置提交。
- 006 成功切换后仍立即清理旧代；上一代观察、Health 自动回切和后续变化合并不属于本任务。

### 3.8 组件与 composition 迁移

- 新增 `internal/kernel/app/clock`、`idgen`、`validation`，分别选择当前 `pkg` 的 System、UUID、Default 实现。
- Logger、Database 单轨迁移到 `internal/kernel/app/logger` 与 `database`，配置语义、默认配置顺序、日志基线接管、Database Ping 和资源 Close 行为保持不退化；Database Access 同步移除调用方 Close 权。
- `composition.Capabilities` 增加 `Clock`、`IDGenerator`、`Validator` 普通接口，并继续输出 Logger、Database Access、Configuration 与可选 CLI。
- composition 固定显式顺序为 Logger、Clock、ID Generator、Validator、Database；默认配置文档仍只包含实际提供 Defaults 的 Logger、Database，并保持 Logger 在 Database 之前。
- 删除旧 `internal/kernel/capability`、旧 `kernel.Definition/Registration/Handle/Access/InstanceHooks` 入口及全部引用，不保留 alias、legacy 路径或双轨测试。

## 4. 质量与约束

- 不新增运行时第三方依赖。若 Go 1.25 无法在不使用反射和运行时 Resolver 的前提下实现跨包泛型 `InputOf(Binding[T])` 编译期约束，允许在当前任务内引入成熟的开发期代码生成方案，但必须先把生成器选择、版本、生成物归属、可重复命令和 CI 漂移检查补入本方案并重新确认；不得临时退化为 `any` 类型断言装配。
- 公开 Go Doc 和维护注释以中文为主，说明依赖顺序、输出身份、并发、资源和失败不变量。
- 核心装配不得使用 `map[string]any`、字符串类型查询或反射解析依赖；异构 Plan 的必要类型擦除必须封装在 `internal/kernel/app` 内，并由 typed Binding/Input 边界和测试保护。
- 所有 Context、超时、取消和清理错误继续完整向上返回；日志不包含 DSN、Token、密码或完整配置。
- API 单轨迁移必须同步源码、测试、根 README、Kernel/App/pkg 说明和研究报告状态边界。

## 5. 验收标准

- Clock、ID Generator、Validator 都能从 `composition.Capabilities` 取得普通接口，并且没有 `Use` 方法、配置段或空生命周期。
- 一个 fake Managed 组件可以通过 typed Input 接收前置 Clock/ID/Validator，替换这些 Definition 的具体实现后，fake 组件构造代码不变。
- Binding 无运行时实例查询入口；跨 Plan、重复 ID、前向引用、Freeze 后 Add 和重复 Install 都稳定失败。
- Compose 任一步失败都不改变 Kernel；成功后 Start/Stop 顺序与 Plan 一致。
- Logger 基线、动态 With、Activation 和文件 Resource Close 语义保持；Database Ready/Ping、租约和 Kernel 私有 Close 语义保持，Access 回调无法关闭共享连接池。
- Reload 候选准备期间旧 Access 可用；排空、提交、回滚和反向清理顺序有确定性测试。
- RestartRequired 不改变当前实例或摘要，并返回可识别错误与 Component ID。
- `config init` 仍按 Logger、Database 顺序生成相同配置语义，不出现 Clock、ID Generator、Validator 空段。
- 旧 capability 目录、旧类型、旧 import、旧文档和旧逐项 Register 调用引用归零。
- `go build ./cmd/app`、`go test ./...`、`go test -race ./...`、`go vet ./...`、文档链接、架构残留搜索和 `git diff --check` 全部通过。

## 6. 非目标

- 不实现 `NativeAtomicReload`、`ComponentHandoff`、切换后观察期、Health 自动回切或保留两代。
- 不增加 HTTP/RPC Server、middleware、handler、service、repository、model、业务 scope 或请求级依赖。
- 不建立按类型自动装配、named binding 字符串查询、运行时 Resolver、Service Locator 或对外 Container。
- 不迁移所有 `pkg` 内部对 `time.Now`、UUID 便利函数或默认 Validator 的局部使用；只有进入当前进程 composition 的能力需要走本任务装配路径。
- 不修改第三方库选择，不增加数据库驱动、日志后端、配置格式或部署行为。
