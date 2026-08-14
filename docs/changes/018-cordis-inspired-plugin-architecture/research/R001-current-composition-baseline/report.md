# R001：当前显式装配与生命周期基线

> **历史档案。** 018 方案已废除；本报告的代码快照可供检索，但结论不再授权插件架构实施。当前 HTTP API 成熟度基线由 `019-R001` 取代。

## 1. 研究问题与方法

本报告回答：当前项目已经具备哪些“插件内核”性质，哪些事实仍使它只是手工装配的模块化单体，以及新目标会与哪些既有决策发生冲突。

证据快照为 `28fbc7a9cfe01e4e7c45505217c15f4d56e711b3`。核验范围包括根文档、`internal/kernel/app`、Kernel/Coordinator/Host、两个 composition 层、`internal/module`、Todo 模块、相关测试，以及 012/017 的有效研究。只读检查时工作树为 clean。

## 2. 当前已实现事实

### 2.1 底层组件已经是无副作用声明

`internal/kernel/app.Definition[O]` 把 ID、配置契约和实例化闭包封装在一起；`ManagedConfigured`、`ManagedFixed`、`Value` 与 `ReplacementDefinition` 分别表达不同构造和换代语义。Definition 建立时不打开资源，真正的 Build/Start/Ready 由 Kernel 执行。

`Binding[O]` 与 `Input[O]` 只在同一 Plan 内表达 typed 前向依赖。`Values.Resolve` 只能读取本 Definition 已声明的 Input，使用期结束后失效；没有字符串或类型任意查询入口。这已经具备“依赖显式、调用方获得普通接口、运行期没有 Service Locator”的重要基础。

### 2.2 Plan 是显式、冻结且确定的

`internal/kernel/composition.Compose` 以固定代码顺序加入 Logger、Clock、ID Generator、Validator、Database、Cache、I18n、Storage，随后 Freeze 并一次性 Install。重复 ID、跨 Plan Binding、前向依赖、冻结后修改和错误 Replacement 都会失败。

这保证了装配可读和失败发生在资源启动前，但同时意味着：

- Definition 必须按依赖顺序人工书写；
- 可用组件集合与启用集合没有分离；
- 没有 Catalog、Profile、Bundle 或装配配置；
- FrozenPlan 启动后不能增加、删除或替换节点；
- 依赖提供方变化不会自动使依赖方失活或重建。

### 2.3 Kernel 已具备候选事务和资源所有权

`RuntimeComponent` 已有 Stage、Build、Start、Ready、BeginDrain、Commit、Rollback、StopPrevious 与终止停止语义。Kernel reload 会先对全部变化组件准备候选，再反向排空、提交并清理旧代；RestartRequired 在构造副作用前拒绝整轮变化。Lease/facade 让调用方不获得 Close 权。

这对应 Cordis“作用必须可撤销”的一部分，但粒度只到预先存在的底层组件实例。路由、命令、health checker、事件监听、模块 participant 等注册动作没有统一归入某个实例的 LIFO Effect ledger，也没有统一的卸载证明。

### 2.4 应用模块是静态 contribution，不是插件

`internal/module.Contribution` 只含稳定 Module ID、已绑定 Route 和 Participant；`ValidateContributions` 在 listener 启动前拒绝重复模块、路由和 participant。Todo 在 `internal/composition` 中显式构造，`runService` 手工创建 Router、HTTP Server、participant 顺序和 Host。

这种设计边界清晰，但它与底层 Plan 是两套装配协议：

- Kernel App 用 Definition/Binding/Input/RuntimeComponent；
- 应用模块用构造函数/Contribution/Host Participant；
- CLI command、配置 Binding、HTTP route 和 runner 分散在不同 composition 函数；
- 模块不能作为一个拥有完整配置、依赖、贡献和清理的实例被统一解释、检查和诊断。

### 2.5 当前入口只有模式分支，没有 Profile

`Application.Run` 按是否有参数选择 Service 或 CLI；Bootstrap composition 会避免创建 Kernel 和资源。这是正确的副作用边界，但不是可扩展 Profile：当前二进制不能用一个确定的声明回答“本模式启用了哪些插件、为什么启用、提供和依赖什么、按何种顺序退出”。

## 3. 已有自动门禁与其边界

现有测试能保护：

- Kernel App 不能泄漏 Resolver；
- 组件依赖必须是同一 Plan 的 earlier Binding；
- 模块核心不导入 Kernel 或底层技术；
- 唯一 composition root 和 contribution 冲突；
- 配置候选、资源换代和停止顺序。

它们不能证明：

- 插件依赖图无环且与声明一致；
- Profile 引用的插件都编译进当前二进制；
- 每个插件产生的所有注册动作都能完整撤销；
- provider 替换时 dependents 会先退出；
- 插件 API、Capability ID 与版本兼容；
- 动态增删后的图和启动时从零构建等价。

## 4. 与既有决策的冲突

012 的当前方向明确拒绝 runtime DI、自动扫描、全局 Registry、Service Locator 和动态插件；R009 进一步拒绝没有第三方分发需求的进程外插件。这些拒绝在当前事实下仍然成立。

本次用户目标提供了新的产品方向，但没有提供第三方不可信代码、独立二进制发布或不停机 mount/unmount 的真实验收。因此只能复核“是否需要统一插件装配语义”，不能直接推导“需要运行期加载任意代码”。

需要新 ADR 才能改变的部分是：从“composition 永久手写固定顺序”演进到“显式 Catalog + 声明 Profile + typed 图编译”。继续保留的红线是：不扫描、不 `init` 自注册、不公开 Resolver、不让业务对象查询容器、不用动态机制掩盖依赖。

## 5. 判断

当前项目不是从零开始，已有约一半插件 Runtime 所需基础：

- 时间维度：候选构造、Lease 排空、反向停止、清理错误；
- 空间维度：typed Input、唯一 ID、冻结 Plan、显式 composition；
- 应用扩展：Route/Participant contribution 与冲突校验。

主要缺口是把这些能力提升为同一套“插件定义 -> Catalog -> Profile -> 图编译 -> 实例 Effect -> 生命周期/诊断”模型。第一阶段应只替代装配方式，不扩大为运行期任意代码加载。

## 6. 适用、不适用与局限

适用于：统一现有内置能力与应用模块的装配语义、支持多个编译期 Profile、提升可诊断性和可替换性。

不适用于：直接证明热卸载安全、第三方插件安全、跨进程兼容或 Wasm 沙箱；这些需要独立需求与威胁模型。

本报告未运行服务或外部资源，因为研究问题可由代码、测试与文档静态核验回答。实现开始前仍需为拟修改的具体 API 搜索全部调用方。

## 7. 对当前任务的影响

研究支持建立 018 计划，且要求：

1. 复用现有 Definition/Plan/Kernel/Host，不另建第二容器；
2. 先做启动期 compiled-in plugin，不承诺 live mount/unmount；
3. 用 typed capability token 和构造注入替代 `ctx.get(string)`；
4. 将所有可撤销注册统一为插件实例 owned Effect；
5. 用新 ADR 单轨调整 012 的固定装配决策。
