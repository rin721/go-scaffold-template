# 007 Kernel 内置 Logger 的可选 App 替换

## 状态

- 当前状态：**已完成**。
- 方案建立日期：2026-08-13。
- 代码事实基线：`main` 分支 `10e1bf33a1471ae0935ec23b1a0a6013d7234085`。
- 用户已在方案报告后的后续消息中明确确认落实本方案，允许按 [tasks.md](tasks.md) 实施 `APP-001` 至 `VER-001`。
- 实施与验证已完成；最终任务提交记录见 [tasks.md](tasks.md)。
- 本方案以当前 `main` 为唯一实现基线，不把其他分支上的 007 实现视为现状，也不计划直接合并或搬运它。

## 目标

Kernel 始终提供一个由启动入口注入的内置 Logger 基线，并把同一个稳定 Logger facade 作为当前进程的能力输出。composition 可以显式选择一个 `internal/kernel/app/logger` 配置化组件来替换该内置能力，也可以完全不选择替换组件。

替换关系必须同时在组件声明与装配调用处可见：

```go
replacement, err := loggerapp.Replacement()
if err != nil {
	return err
}
if err := app.Replace(plan, builtinLogger.Binding, replacement); err != nil {
	return err
}
```

`loggerapp.Replacement` 表明该组件不是独立 Logger；`app.Replace` 的 typed target Binding 表明它明确替换哪一个 Kernel 内置能力。普通 `app.Add` 不能接收替换声明。

## 核心边界

- 内置 Logger 是强制基线，配置加载前、未选择替换、替换构造失败和替换停止后都必须可用。
- 配置化 Logger 替换是 composition 选择，不由配置文件、环境变量、自动扫描或运行期查找暗中启用。
- `Capabilities.Logger` 始终是 Kernel 内置 target 向只读 `pkg/logger.Logger` 的投影；替换组件不再输出第二个 Logger Access。
- 当前 `cmd/app` 会显式选择配置化替换，以保持现有 `logger` 配置、`config init` 和服务运行行为；其他 composition 调用方可以选择只使用基线。
- 本任务只处理 Logger，不建立通用内置能力 Catalog、优先级、多级覆盖、动态卸载或 Config/CLI/Database 替换体系。

## 阅读顺序

1. [requirements.md](requirements.md)：当前事实、需求、验收标准和非目标。
2. [design.md](design.md)：目标 API、Plan 约束、生命周期、配置和文件影响。
3. [tasks.md](tasks.md)：稳定任务 ID、依赖、工作量、确认门禁与实施证据。

实施只覆盖当前已确认范围。若公共 API、依赖选择、模块边界、默认选择或替换事务发生实质变化，方案回到待确认状态并重新报告。
