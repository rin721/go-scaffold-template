# 装配、配置与生命周期

## 1. 当前问题

现有 Kernel 内部读取配置并管理底层资源，当前 Supervisor 又把 Participant.Start/Stop 与 Task.Run 分成两个无法闭合的集合。未来直接加入 HTTP 或业务 composition 会产生三类确定性问题：同一次启动读取两份 snapshot、Serve 运行错误没有 owner，以及 Task Wait 必须等 Participant.Stop 才能结束的停止互锁。

本设计保留 Kernel 资源平面，通过薄 application coordinator 和局部 Supervisor/httpx 调整解决这些问题；不建立第二个容器或通用 DAG。

## 2. 单一配置候选

目标流程：

1. baseline Logger 先建立。
2. coordinator 调用 Loader 一次，取得 immutable candidate、digest 和来源诊断。
3. 每个已注册 config owner 独立 decode/default/validate，并返回 change classification；不把 Secret 回显给 coordinator。
4. 所有 RestartRequired 预检成功后，Kernel 从同一 candidate view 执行 Plan 的 Stage/Build/Start/Ready/Publish 或 Reload。
5. coordinator 只在全局决策成功后更新 committed candidate/state。

Kernel 仍拥有 component generation；application owner 只拥有自己的不可变配置和运行单元。具体 API 可以是显式 snapshot 参数或受控 candidate interface，但必须证明没有第二次 Load 和可变共享对象。

## 3. 初始启动

Service mode 的目标事件序列：

```text
baseline logger
  -> load/decode/validate/preflight one candidate
  -> build/freeze/install Kernel Plan
  -> build application owners and validate unique registrations
  -> Kernel Start/Ready/Publish
  -> other startup owners
  -> HTTP listener bind
  -> start supervised runners
  -> publish process ready
```

构造阶段纯内存，不执行依赖尚未启动的 I/O。网络 bind 属于 startup owner，因为其失败必须阻止 ready。任一步失败都撤销 ready、取消已启动 runner、按依赖反序 StopAndWait，并合并 primary 与 cleanup error。baseline Logger 最后停止。

## 4. Supervisor 目标语义

### 4.1 概念

- Startup owner 同步确认资源已获得；没有长期工作者的普通对象不需要包装。
- Runner 阻塞到 context 取消或运行失败；运行期结果返回统一 owner。
- One-shot operation 显式属于 CLI 模式，完成 nil 是成功；Service runner 的非预期 nil 完成是终止事件。

所有名称非空唯一且顺序固定。Supervisor 不默默跳过 nil 注册，不依赖 map 顺序。

### 4.2 运行失败与终止

收到 signal、runner error 或关键 runner 非预期完成时：

1. 原子记录 primary termination cause，状态变 draining/failed 并先撤销 readiness。
2. cancel 所有 runner context，阻止新工作。
3. 在统一 shutdown deadline 内按依赖反序调用 owner StopAndWait；HTTP 必须先完成请求排空，再停止依赖它的模块/Kernel。
4. 等待所有受管 goroutine；超时标明未退出 owner并继续尝试可安全执行的后续清理。
5. 合并 Stop/Wait/Close 错误，形成退出结果；不得在日志后返回成功。

具体实现可以是扩展接口、runner group 或内部 adapter，但不能维持“无限 Wait 全部 Task 后才开始 Stop”的旧顺序。

## 5. HTTP 受管单元

目标 HTTP owner 在 Start 中预绑定 listener；随后 `Serve(listener)` 作为受监督 runner。非 `http.ErrServerClosed` 错误触发 process failure。Stop 先撤销 ready/停止接受新连接，调用带 context 的 Shutdown，并等待 Serve 返回；必要的 Close 必须有明确触发条件，所有错误合并。

这要求单轨调整 `pkg/httpx.Server` 或增加项目自有 lifecycle adapter。业务模块不能各自创建 Server、listener 或 fire-and-forget goroutine。当前没有 hijacked connection 需求，不预建相关管理器。

## 6. Readiness 与状态

候选组件 `Ready` 保持原义。进程 readiness 由 coordinator 汇总：Kernel generation 已发布、所有必需 startup owners 成功、关键 runner 仍运行、HTTP 已绑定且未 drain、没有阻断型 degraded。状态至少包含 starting、ready/running、draining、stopped、failed/degraded。

`pkg/health` 可以承载有界 checker 和 snapshot，但不能成为 lifecycle owner。诊断暴露 state/reason、generation、candidate digest、last reload/cleanup 和未退出 owner；输出必须确定、并发安全且脱敏。

## 7. Reload 事务

1. coordinator Load 一次 candidate。
2. 所有 owner decode/validate/change classify。
3. 任一 application immutable 变化返回 RestartRequired，Kernel 未产生副作用。
4. Kernel 从同一 candidate prepare；失败保持旧 generation/candidate。
5. reload drain 失败可 Resume；commit 后更新全局 candidate/state。
6. cleanup old generation；失败不回滚新代，标记 degraded/restart-required 并拒绝后续 reload。

初版不重建业务对象图、路由、命令或 listener，不实现 NativeAtomicReload/ComponentHandoff/自动观测回滚。若真实资源需要，需提供额外证据和重新确认。

## 8. 终止排空

进程终止与 reload 不同：ready 一经撤销就不恢复，新 work 被拒绝。drain timeout 不能调用 Resume 把 resource 重新变 serving；Supervisor 报告不完整排空并失败退出。对仍有活跃 Lease 的共享资源不强制 Close 造成 use-after-close，OS 是最终边界，但这种退出不得描述为优雅停止成功。

当前 `Kernel.Stop` 的可回滚行为需与 process termination path 分离，具体公开契约在实施前复核外部消费者后确定。

## 9. 运行模式

- **Service**：Kernel + startup owners + supervised runners；任何关键 runner 非预期完成终止进程。
- **Bootstrap CLI**：当前 `config init` 等，无需 Kernel 业务资源，完成 nil 是成功。
- **Application CLI**：只有真实业务命令需要时才启用；启动明确资源、执行 one-shot、反序停止。

帮助、参数错误和未知命令不应为了分类而先启动全部资源。没有真实 Application CLI 需求时只保留语义，不创建占位入口。

## 10. 验证

- Loader spy、candidate identity 与应用节变更测试。
- 端口占用、Serve error、阻塞请求 Shutdown、Stop/Wait timeout。
- startup failure、runner error、runner nil 提前完成、signal 和不合作 runner 的确定性事件序列。
- reload drain 回滚与 termination drain 不 Resume 对照。
- committed cleanup degraded、二次 reload 拒绝和 diagnostics/redaction。
- 非空/重复 ID、固定顺序与所有错误原因链。
