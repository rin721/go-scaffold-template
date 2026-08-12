# Kernel 底层组件手动装配与安全重载

- 状态：已完成
- 研究日期：2026-08-12
- 当前仓库基线：`a1a3a65b8f180ca9b571b8e3d7424c74403746e0`

## 结论摘要

当前项目让人“看着有点懵”，不是因为手工装配本身困难，而是当前开发路径同时暴露了四类不同问题：第三方库封装、Kernel 生命周期接入、配置默认值与 CLI 聚合、运行期资源代际切换。对一个只想新增底层能力的人来说，必须先理解 `Definition`、`Access.Use`、租约排空、候选实例、配置事务和 composition 聚合，才能找到第一处可修改代码，认知入口确实过重。

本报告确认的目标不是引入自动扫描、反射容器或业务 DI 框架，而是把手工装配路径收敛为：

```text
pkg/<name>
    ↓
internal/kernel/app/<name>
    ↓
internal/kernel/composition/<name>.go
    ↓
Kernel / Host
```

其中：

- `pkg/<name>` 隔离第三方库并提供稳定的项目能力契约。“底层可替换”首先在这里实现：未来替换第三方库时，业务契约和 Kernel 装配方式不随之改变。
- `internal/kernel/app/<name>` 把确实需要托管的底层能力描述成应用组件，只暴露一个定义入口，内部按需组合配置、构建、启动、就绪、关闭、健康和重载等小契约。
- `internal/kernel/composition/<name>.go` 是人工选择实现和登记组件的唯一位置。没有扫描、`init` 注册、Service Locator 或隐藏默认组件。
- Kernel 负责组件运行治理，并向尚未建设的上层暴露项目自有能力契约。本报告不设计 handler、service、repository 等业务层，也不判断它们未来由谁构造。
- 配置重载由组件根据底层库和资源特性选择策略，不能把最重的实例换代协议机械施加给所有能力。

## 为什么主流框架显得更轻

Fx、Kratos、go-zero 和常见手工 composition root 的默认路径主要解决“启动时怎样得到完整对象图”以及“怎样统一 Start/Stop”。它们通常直接把稳定实例交给调用方，配置变化后通过重启进程得到新对象图；默认路径并不承担旧调用租约排空、双实例并存、观察期健康判断和失败自动回切。

当前项目追求的安全换代语义比普通 DI 更强，所以 Kernel 内部存在必要复杂度。问题不在于这部分逻辑存在，而在于它被当成了每个新能力都必须理解和使用的默认开发入口。目标设计把复杂状态机留在 Kernel 内部，让组件作者只选择并满足一种明确的重载策略。

## 重载策略结论

每个受托管组件必须显式选择：

| 策略 | 适用条件 | Kernel 职责 |
| --- | --- | --- |
| `NativeAtomicReload` | 底层库能原子应用配置，失败后旧状态仍有效 | 委托调用、验证结果并记录诊断 |
| `KernelInstanceSwap` | 底层库不能原子重载，但新旧实例可并存 | 构建候选、租约排空、切换、观察和回滚 |
| `ComponentHandoff` | 资源排他，但组件有可靠所有权交接协议 | 编排组件专用交接状态机 |
| `RestartRequired` | 不能并存，也没有可靠交接协议 | 报告需要重启，不伪装无感重载 |
| `Ignore` | 变化与组件无关或配置固定 | 不执行重载 |

`KernelInstanceSwap` 的目标语义是带观察期的两阶段切换：新实例通过 Build、Start、Ready 后接管新租约；上一代暂时保留但不接收新租约；观察期健康失败时排空新租约并切回上一代；观察期通过后才异步关闭上一代。Kernel 同时最多保留当前代和上一代。

上述策略名、`internal/kernel/app` 目录、Ready/Health 观察期及切换后回滚均是**目标设计，尚未实现**。当前实现只有统一的候选换代路径，并在成功切换后立即清理旧实例。

## 阅读顺序

1. [当前路径与困惑来源](01-current-path-and-confusion.md)
2. [复杂度根因](02-complexity-root-causes.md)
3. [目标组件模型](03-target-component-model.md)
4. [组件级重载策略](04-reload-strategies.md)
5. [手动接入指南](05-manual-integration-guide.md)
6. [当前阶段边界](06-current-stage-boundary.md)
7. [迁移建议](07-migration-recommendations.md)
8. [来源与复核入口](08-sources.md)

## 状态边界

| 内容 | 状态 |
| --- | --- |
| `pkg` 与 Kernel 单向依赖、显式 composition、typed `Access.Use` | 当前已实现 |
| 初始 Build/Start、Reload 候选构造、旧租约排空、失败恢复旧实例 | 当前已实现 |
| Supervisor 按顺序启动、反向停止，配置文件 Watch | 当前已实现 |
| `pkg/health` 健康检查库 | 当前已实现，但未接入 Kernel/Host 组件观察期 |
| `internal/kernel/app/<name>` 和组件级 Reload Policy | 目标设计，尚未实现 |
| 切换后观察期、自动回切、最多保留两代 | 目标设计，尚未实现 |
| HTTP 入站服务及 middleware、handler、service、repository、model | 尚未建设，全部排除在本报告设计与验收之外 |

本报告是研究结论和后续设计输入，不是已经确认的源码改造。实施目录迁移、公共契约变更或重载状态机前，必须另建任务级变更方案并获得确认。
