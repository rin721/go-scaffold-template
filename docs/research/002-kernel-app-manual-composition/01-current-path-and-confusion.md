# 研究基线路径与困惑来源

> 本章记录 006 实施前的历史基线，用于解释问题来源。当前装配路径已迁移为 `kernel/app -> Plan -> Kernel.Install`，见 [Kernel App 组件开发](../../../internal/kernel/app/README.md)。

## 1. 研究基线的真实装配链

当前实现不是“封装一个库，然后把实例传进应用”这么短的路径。Logger 和 Database 的真实路径是：

```mermaid
flowchart LR
    Entry["cmd/app"] --> Loader["config.Loader"]
    Entry --> Runtime["kernel.New"]
    Runtime --> Compose["composition.Compose"]
    Compose --> LComp["composition/logger.go"]
    Compose --> DComp["composition/database.go"]
    LComp --> LDef["capability/logger.Definition"]
    DComp --> DDef["capability/database.Definition"]
    LDef --> Register["kernel.Register"]
    DDef --> Register
    Register --> Access["typed Access"]
    Register --> Kernel["Kernel component list"]
    Access --> Application["cmd/app applicationLifecycle"]
    Kernel --> Host["Host + Supervisor"]
    Application --> Host
```

入口需要完成以下动作：

1. 构造基线 Logger Resource 和 Kernel logging manager；
2. 选择 File/Env 配置源并创建空 Kernel；
3. 调用 `composition.Compose` 登记 Logger、Database；
4. 可选构造启动前 CLI；
5. 从组合结果取得 typed Access；
6. 构造业务 Participant；
7. 创建 Host 并运行 Supervisor。

`kernel.New` 不会自动发现或登记能力。真正的选择发生在 `internal/kernel/composition`，真正的资源构造要等 `Kernel.Start` 加载配置后才发生。

### 1.1 Clock、ID Generator、Validator 的研究基线事实

研究基线已经在 `pkg` 定义了 `clock.Clock`、`idgen.Generator` 和 `validation.Validator` 项目契约，但尚未形成统一装配；006 已补齐三项 Fixed Direct App Definition 和 composition 输出。历史缺口是：

- `internal/kernel/composition.Capabilities` 只有 Logger、Database、Configuration 和 CLI；
- 三项简单能力都没有对应 `internal/kernel/capability` 或目标 `internal/kernel/app` 组件；
- 当前部分 `pkg` 代码仍直接调用 `time.Now()`；
- `pkg/httpx.RequestID` 在 generator 为 nil 时自行创建 UUID Generator；
- `validation.Struct` 每次调用都自行创建默认 Validator。

这些便利入口本身不是错误，但它们不能代表“当前进程已明确选择并统一注入了哪一种实现”。因此“Clock、ID Generator、Validator 统一进入 `kernel/app + composition`”是本轮新增的目标设计，不是对现有能力的描述。

## 2. 新增一个能力目前需要理解什么

以 Database 为例，能力接入者至少要同时理解：

- `pkg/database` 的业务接口、Config、构造入口和 Close 所有权；
- Kernel `Definition[C,T]` 的 ID、ConfigPath、Decode、Defaults、Builder、Hooks 和可选 Activation；
- `kernel.Register` 为什么返回稳定 Handle 和默认配置 Binding；
- `Access.Use` 为什么要求 Client、Rows、Row 和事务都不能逃逸回调；
- composition 为什么还要为每项能力创建同名文件和私有 binding；
- 总 `Capabilities` 为什么要增加字段；
- DefaultManager 和可选 CLI 怎样聚合能力契约；
- 登记顺序怎样影响启动、提交和反向关闭；
- Reload 怎样构建候选、阻断新租约、等待旧租约并替换实例；
- Host 和 Supervisor 如何管理 Kernel 与上层 Participant。

这些概念各自都有价值，但缺少一个从开发者任务出发的入口。当前文档优先解释运行机制，没有先给出“新能力最少改哪些位置、每个位置只负责什么”的黄金路径。

## 3. 当前 Definition 为什么显得重

当前 `Definition` 不是普通 DI Provider。普通 Provider 通常只表达：

```text
dependencies + config -> instance + cleanup + error
```

当前 Definition 同时表达：

```text
能力身份
+ 配置段所有权
+ typed 解码与校验
+ 默认配置生成
+ 实例构造
+ 启动与关闭
+ 可选发布激活
+ 运行期换代所需元数据
```

因此它更接近“资源代际说明书”，而不是“构造函数注册”。当所有底层能力都从 Definition 开始学习时，无资源的简单能力和真正需要热换代的资源能力看起来一样复杂。

## 4. 当前 Reload 已经实现了什么

当前 `Kernel.Reload` 已具备以下真实行为：

1. 重新加载完整配置快照；
2. 对变化配置段完成 Decode 和校验；
3. 关闭受影响 Handle 的新租约入口；
4. 并行构建、启动候选并等待旧租约排空；
5. 提交前失败时丢弃候选、恢复旧实例服务；
6. 成功时替换 Handle 中的实例并恢复调用；
7. 反向 Stop 已被替换的旧实例；
8. 提交后清理失败返回 `CommittedCleanupError`，不回滚新配置。

这是一套严肃的运行期换代协议，而不是普通配置回调。它解释了 Kernel 内部代码量，但也暴露出当前模型与最新目标之间的差距：

- 没有组件级重载策略，所有受托管能力进入同一种候选换代路径；
- 没有切换后的观察期；
- 成功切换后旧实例立即进入 Stop，不再可用于回切；
- `pkg/health` 尚未作为候选 Ready 或观察期 Health 的 Kernel 契约；
- 对不能双实例并存的排他资源，没有 `ComponentHandoff` 或 `RestartRequired` 表达。

## 5. 为什么用户不知道从何下手

困惑主要来自三个断点。

### 5.1 目录语言与心智模型不一致

使用者想的是“通用库 → 可运行组件 → 手动装配”，当前目录表达的是“通用库 → capability definition → composition”。`capability` 没有直接说明它是一个带配置和生命周期的“应用组件”，而 `composition` 又同时承担登记、默认配置和 CLI 聚合。

### 5.2 简单替换与运行期换代被混为一谈

“第三方库可替换”只要求 `pkg` 对第三方类型形成稳定边界，并让 composition 可以选择另一实现。它不天然要求每次业务调用都进入租约，也不天然要求进程内同时维持两代资源。

当前模型直接把运行期换代作为受托管能力的通用语义，使开发者在替换一个数据库驱动之前，先被迫理解资源代际状态机。

### 5.3 当前阶段终点没有说清

当前项目尚未建设 HTTP 入站服务、middleware、handler、service、repository 和 model。底层装配在本阶段的终点应是：组件已被显式登记，Kernel/Host 能按契约构建、启动、监控、重载和关闭，并对外暴露不泄漏第三方实现的项目能力契约。

原报告继续推演未来业务对象怎样持有 Access、怎样构造 repository/service/handler，超出了当前阶段。缺少业务消费者不能成为提前设计业务分层的理由；底层组件应先通过自身契约、生命周期和受控调用测试独立验收。

## 6. 本报告的问题定义

需要解决的不是“怎样增加更多自动化”，而是：

- 让组件作者从一个清晰入口完成封装和手动登记；
- 让简单能力不为不需要的热换代付出成本；
- 让确实需要无感重载的能力继续获得强状态机保障；
- 让排他资源可以如实声明无法通用无感换代；
- 不替尚未建设的业务层决定目录、构造方式或依赖图；
- 把复杂性留在拥有复杂责任的 Kernel 内，而不是扩散到每个调用方。
