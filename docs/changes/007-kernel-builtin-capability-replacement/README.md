# 007 Kernel 内置能力槽位与显式替换体系

## 状态

- 当前状态：**已完成**。
- 方案建立日期：2026-08-13。
- 最近修正日期：2026-08-13。
- 代码事实基线：`10e1bf33a1471ae0935ec23b1a0a6013d7234085`。
- 实施确认日期：2026-08-13。
- 实施门禁：用户已在新版 007 需求报告之后明确确认，可以实施 [tasks.md](tasks.md) 中 `BLT-001` 至 `VER-001`。
- 提交策略：方案阶段不暂存、不提交；确认后将方案、实现、测试和权威文档作为同一任务变更提交。

## 要解决的问题

本方案解决的不是“Logger 不存在时如何 fallback”这一单点问题，而是 Kernel 内置能力的通用治理问题：

1. Kernel 怎样提供从启动早期到最终退出始终存在的 baseline 能力；
2. 外部 App 组件怎样明确声明“替换某个内置能力”，而不是因实现了相同接口就被自动识别；
3. 替换主槽位与新增独立实例怎样同时存在且互不影响；
4. 替换在初始启动、运行期重载、失败回滚和资源关闭时怎样保持一致语义。

Kernel 自带实现也必须像 `internal/kernel/app/<name>` 一样具有明确的组件声明位置，不能继续散落在 `kernel/config`、`kernel/logging`、`kernel/cli` 和 composition 特例中。目标目录统一命名为：

```text
internal/kernel/builtin/<name>
```

`builtin` 表达“由 Kernel 随仓库提供的内置实现”，不会与 `app` 的可选择实现或 `pkg` 的技术封装混淆。当前封闭清单包含 `builtin/config`、`builtin/logger`、`builtin/cli`。目录名不再添加 `baseline-` 前缀；`builtin` 已经表达默认来源，具体 baseline 身份由 Role Definition 声明。

目标模型将 `Provide`、`Replace` 和未来的 `Decorate` 分开：

```text
internal/kernel/builtin: config / logger / cli
        |
Kernel closed BuiltinRole catalog
        |
        +-- root Binding -------> Kernel / db1 / other consumers
        |       ^
        |       |
        |   explicit Replace(replacement definition)
        |
        +-- Add(independent definition) -----> db2 only
```

- 未调用 `Replace`：root Binding 始终使用 Kernel baseline。
- 显式调用 `Replace`：root Binding 在替换事务成功提交后切换，所有依赖它的消费者共同跟随。
- 调用 `Add`：产生独立 Binding，只影响显式依赖该 Binding 的消费者。

## 当前事实与目标设计

当前代码只有 Logger 专用的 `internal/kernel/logging.Manager`，`internal/kernel/app/logger` 通过 `WithActivation` 调用 `Replace/Restore`；Config 由 `cmd/app` 直接创建 Loader，CLI 则由 composition 在 Plan Freeze 后特例构造。三项能力尚未在统一的内置组件目录中声明。Plan 也还没有内置 Role、root Binding、通用替换定义或替换冲突校验。Database 仍是固定 ID、固定配置路径且没有日志依赖。

`internal/kernel/builtin`、`BuiltinDefinition`、`BuiltinRole`、`ReplacementDefinition`、`app.Replace`、多实例 `Spec` 和通用替换事务现已实现。Config、Logger、CLI 三项内置能力已收敛到统一 catalog；Logger 完成可替换纵向切片，测试图已验证 baseline-only、主槽位替换和 logging.db2/database.db2 独立实例共存。Config 与 CLI 本轮只完成 baseline 组件化和 `StartupReplace` 策略登记，不提供生产替代组件。

## 固定结论

- Kernel 维护封闭的内置能力 Role 清单；配置、目录和接口实现都不能自行扩展或触发替换。
- Kernel 内置实现统一位于 `internal/kernel/builtin/<name>`；外部或配置化实现继续位于 `internal/kernel/app/<name>`。
- 内置组件共享 Definition、Role 和 Binding 规则，但按 `Bootstrap`、`PreStart`、`Runtime` 阶段执行，不能为了目录统一而伪造相同生命周期。
- 相同接口只证明类型兼容；只有 Composition 显式调用 `app.Replace` 才表达替换意图。
- 每个 Role 声明替换策略和 baseline 所有权；一个 Role 最多存在一个 replacer。`StartupReplace` 在阶段激活前冻结一次选择，只有 `RuntimeTransaction` 使用稳定 slot 和排空事务。
- Config、Logger、CLI 是当前封闭内置清单；Config 为 `Bootstrap + RequiredActivation + StartupReplace`，Logger 为 `Bootstrap + RequiredActivation + RuntimeTransaction`，CLI 为 `PreStart + SelectedActivation + StartupReplace`。
- Config、Logger、CLI 的生产 baseline 均由 Kernel Assembly 构造并治理；测试或嵌入场景需要借用外部 baseline 时，必须使用明确的 `BorrowedBaseline` 构造入口。
- 可替换资源能力通过 Context Access 暴露稳定入口；独立实例不可用时明确失败，不回退主槽位。
- replacer 只替换主槽位，不同时发布独立 Binding；独立使用必须创建另一个具名实例。
- 启动期替换失败使启动失败；运行期替换失败保留当前代；禁止静默降级和 last-wins。

## 阅读顺序

1. [requirements.md](requirements.md)：问题、场景、约束、验收标准和非目标。
2. [design.md](design.md)：目标契约、数据流、生命周期、失败语义和迁移边界。
3. [tasks.md](tasks.md)：稳定任务 ID、确认状态、依赖和完成条件。

本目录是任务方案和实施证据，不替代当前权威文档。实现完成后，当前有效结论必须同步至根 README 和 Kernel/App 主题文档。
