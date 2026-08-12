# 006 Kernel App 多态装配基础

## 状态

- 当前状态：**待确认**。
- 方案建立日期：2026-08-12。
- 代码事实基线：`620c2c877c6f21798828f5bf6052cdc1c1ef618c`。
- 实施门禁：尚未满足；当前只完成方案文档，不得修改源码、测试、配置或依赖。
- 文档交付：用户于 2026-08-12 明确要求把当前纯文档变更独立提交并推送；该 Git 授权不构成对后续实施的确认。

## 目标

本任务开始把 [002 Kernel 底层组件手动装配与安全重载研究报告](../../research/002-kernel-app-manual-composition/README.md) 落地为第一条可运行的单轨 API：

```text
pkg/<name>
    -> internal/kernel/app/<name>
    -> internal/kernel/composition/<name>.go
    -> ordered typed Plan
    -> Kernel / Host
```

Clock、ID Generator、Validator 与 Logger、Database 都进入同一组件清单，但按能力特性选择不同契约：

- Clock、ID Generator、Validator：`Fixed Direct + NoReload`，直接输出普通项目接口，不使用 `Access.Use`；
- Logger、Database：typed 配置、稳定 Leased Access、可选生命周期和 `KernelInstanceSwap`；Access 只暴露不含关闭权的项目调用接口；
- 配置化但不能安全热换的组件：`RestartRequired`，配置变化不伪装成无感重载。

本任务同时用有序 typed Plan 替换当前逐项 `kernel.Register` 的半登记风险，完整计划在 composition 校验、冻结并完成 Defaults/CLI 组合后，才一次性安装到尚未启动的 Kernel。

## 范围边界

本任务会单轨迁移当前 `internal/kernel/capability`、`kernel.Definition/Registration/Handle`、Logger、Database 与 composition；不会保留新旧两套入口。

本任务只实现已有代码和当前简单能力足以真实验收的策略：

- `NoReload`；
- `KernelInstanceSwap`；
- `RestartRequired`。

以下能力继续作为研究报告中的后续目标，不在 006 暴露占位 API：

- `NativeAtomicReload`；
- `ComponentHandoff`；
- 切换后 Health 观察期、保留上一代和自动回切；
- HTTP、middleware、handler、service、repository、model 或业务对象装配。

## 阅读顺序

1. [requirements.md](requirements.md)：当前事实、功能需求、约束、验收标准和非目标。
2. [design.md](design.md)：目标 API、依赖顺序、生命周期、重载、目录迁移和测试方案。
3. [tasks.md](tasks.md)：稳定任务 ID、依赖、工作量、确认门禁和实施记录。

用户确认本方案后，才把状态改为“已确认”并实施任务账本中的 API、迁移、测试与权威文档同步。
