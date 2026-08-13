# 009 配置重载与生命周期修复

## 状态

- 当前状态：**已完成**。
- 方案建立日期：2026-08-14。
- Git 事实基线：`main@139d437e4407583f6a71afd17808e149a9663d72`，与本地 `origin/main` 一致。
- 当前工作树同时存在独立 `008-web-framework-baseline` 的未提交实施改动；这些改动不属于 009，009 只把当前工作树作为待兼容事实，不修改、不暂存、不提交 008 文件。
- 用户已在方案报告后的后续消息中明确要求开始修复 009；`ENT-001` 至 `VER-001` 已实施并通过验证，最终提交信息由交付报告记录。

## 问题结论

用户运行 `go run ./cmd/app` 后修改 `config.yaml` 的 `database` 配置，控制台只有启动与 Ctrl+C 停止日志，没有配置重载日志。代码已经确认直接原因：

```go
host, err := kernel.NewHost(
	runtime,
	kernel.HostOptions{},
	applicationLifecycle{logging: capabilities.Logger},
)
```

服务入口传入空 `HostOptions`，而 `NewHost` 只有在 `HostOptions.Watch != nil` 时才注册 `kernel-config-watch`。因此文件修改没有进入 `Kernel.Watch -> Kernel.Reload`，也不可能触发 Database 候选构造、租约排空和实例切换。用户给出的日志与“正常启动后未启用 watcher，最终由 Ctrl+C 正常停止”完全一致。

Kernel 的通用热换机制已有单元测试，但默认应用原先没有接通它；Database App 也缺少真实 SQLite 实例跨配置换代的集成证据。当前实现已显式启用 Watch、增加 watcher ready reconciliation，并补齐当前已实现能力的应用级生命周期验证；最终验证和提交证据见 [tasks.md](tasks.md)。

## 目标

- 默认长期服务模式显式启用配置文件监听；启动前 CLI 模式不启动 watcher 或 Kernel 资源。
- 文件变化能驱动完整候选配置事务；Database 与配置化 Logger 按当前 `KernelInstanceSwap` 语义安全换代。
- 无效候选、候选资源不可用、排空超时和 `RestartRequired` 不破坏当前有效实例；失败后继续监听并允许后续有效配置恢复。
- watcher 注册与初始快照之间不丢失配置变化，Ctrl+C 时 watcher、上层 Participant、Kernel 资源和 baseline Logger 按所有权有界退出。
- 用当前完整 composition 和真实 SQLite 验证 Database 新实例已发布、旧实例已关闭，而不是只观察一条日志或复用泛型测试夹具。
- 明确 Clock、ID Generator、Validator、CLI、默认配置管理器和应用 Participant 的实际生命周期，不把 008 尚未完成的 Web、Plugin、Runner 写成当前能力。

## 阅读顺序

1. [requirements.md](requirements.md)：事实、范围、需求与验收标准。
2. [design.md](design.md)：入口修复、监听握手、事务语义和验证设计。
3. [tasks.md](tasks.md)：独立 009 任务 ID、依赖、门禁与当前证据。

实施只覆盖已确认的 `ENT-001` 至 `VER-001`。公共 API、监听失败策略、配置优先级、热换边界或外部副作用发生实质变化时，方案重新回到待确认。
