# R008：剩余 Foundation 闭环复核

## 1. 研究问题与快照

本报告以 `3a936a5` 为当前实现快照，重新回答三个问题：

1. `FOUNDATION-LIFECYCLE-001`、`FOUNDATION-DIAGNOSTICS-001` 与 `FOUNDATION-CONFIG-001` 实施后，旧 R002/R004 的哪些阻断已经失效；
2. 哪些剩余项是当前已证明 profile 的真实 Foundation 缺口，而不是 HTTP 产品、部署、外部资源或新业务场景；
3. 未启动的 acceptance 与 reconciliation 是否能合并成一个窄、可执行、可回滚的计划。

研究从根 README、022 authority、三个施工计划、当前源码、测试名与 Git 历史进入，没有把施工计划中的目标描述当作实现证据。

## 2. 当前事实

### 2.1 已经闭环的部分

| 平面 | 当前事实 | 主要证据 |
| --- | --- | --- |
| 配置 | File/Env 经唯一 Loader 产生同一 Snapshot；同源 path 和跨源 object/non-object 冲突确定性拒绝；冲突发生在 Stage/Build 前 | `internal/kernel/config`、`TestCoordinatorRejectsSourceShapeConflictBeforeComponentWork` |
| 装配 | `cmd/app -> internal/composition` 是唯一应用组合根；Kernel Plan typed、forward-only、freeze-once；业务对象手工装配 | composition 与 architecture tests |
| 生命周期 | candidate/retired/current owner 与 generation 持续存在；terminal timeout 可由后续 Stop 收敛；terminal attempt 失败缓存同一结果且不伪装 stopped | `internal/kernel/app`、`internal/kernel/kernel_test.go` |
| Supervisor | Participant/Task 共享总 shutdown budget；pending/failed/forced 分离；普通 owner 不被假 force | `pkg/supervisor` tests |
| HTTP | listener 在 ready 前绑定；graceful timeout 不暗中 force；显式 force 后等待 Serve 结束并可重绑 | `pkg/httpx/server_test.go` |
| Diagnostics | Kernel 与 Supervisor 保持各自 typed ledger；Host 输出唯一 `ProcessDiagnostics`；Health 消费同一 authority | `internal/kernel/diagnostics*`、`host*` |
| 当前 profile | 真实 SQLite、local Storage、HTTP 与 Todo Service/CLI 已有进程级正常路径；真实 SQLite reload 已有集成测试 | `cmd/app/main_test.go`、`internal/kernel/composition/reload_integration_test.go` |

因此 R002/R004 关于 terminal cleanup、owner/generation、Supervisor names-only 和 EnvSource 覆盖的结论已经过时。它们仍保留为实施前历史，但不能继续作为当前 Foundation 状态。

### 2.2 当前完整验证

2026-08-15 在 Windows amd64、Go 1.25.7 上重新执行：

```text
go test ./... -count=1        PASS
go test -race ./... -count=1  PASS
go vet ./...                  PASS
go build ./...                PASS
```

这证明当前 HEAD 没有一般性编译、单测、race 或 vet 阻断，但不能替代缺失场景的精确断言。

## 3. 剩余真实缺口

### 3.1 `RestartRequired` 是 preflight-only，却永久锁住 reload

`Coordinator.Reload` 在 application binding 或 Kernel component 发现 `RestartRequired` 时没有提交候选，也没有 Build 资源；这是正确的无副作用 preflight。随后它把 `diagnostics.RestartRequired` 设为 true。

当前下一轮 `Reload` 在加载新候选之前直接检查该 flag 并返回 `kernel reload blocked until process restart`；`update` 只会把 flag 设为 true，从不清除。结果是：

- 操作者把文件恢复为当前有效配置后，Coordinator 仍不读取恢复候选；
- watcher 继续运行，但永远无法证明配置已经恢复；
- 一个没有产生副作用的临时修改变成必须重启的永久 latch；
- 该行为与普通无效候选“拒绝本轮、保留旧配置、继续监听”的恢复语义不一致。

这不是 cleanup debt。提交后旧代清理失败的 `LifecycleDegraded + CleanupRequired` 必须继续 fail-closed；只有 preflight-only `RestartRequired` 可以在后续候选重新完整加载、全 owner 校验且相对当前 generation 不再要求重启时解除。

### 3.2 缺少当前 profile 的单一跨层验收 authority

三个施工计划已经分别实现大量故障测试，但 022 尚未把它们与当前 HEAD 的缺失集成证据统一起来：

- Host 的真实 diagnostics 还缺一条从实际 uncooperative Participant/Task 到 `ProcessDiagnostics` 的跨层断言，当前主要由 Supervisor test 与纯 compose test 分别证明；
- 进程级 Service 正常退出后，现有测试没有同时断言 HTTP listener 可重绑和 SQLite 文件所有权已释放；
- sticky recovery 没有正向、持续拒绝、无效候选与 degraded 不可恢复的成组测试；
- `acceptance.md` 末尾仍错误写着 EnvSource 未完成，十一门状态不是当前实现 authority。

这些缺口适合通过窄生产修复、现有 test harness 增量和证据矩阵完成，不需要新增生命周期框架、management transport 或外部服务。

## 4. 不进入本次 Foundation 阻断线的项目

| 项目 | 当前判断 | 去向 |
| --- | --- | --- |
| 真实 PostgreSQL/MySQL/S3 | 当前默认/已证明 profile 不依赖；CI 已有数据库 contract，S3 属于特定远端场景 | 新资源或生产 profile 的 capability assessment/交付验收 |
| HTTP hijacked/WebSocket | `net/http.Server.Shutdown` 明确不拥有该连接；当前路由没有此场景 | 首个真实长连接需求触发独立 lifecycle 计划 |
| 第二次 signal、deployment grace | 当前 Supervisor 总预算有界，main 返回即可由进程边界结束；加速退出和平台预算属于部署政策 | `DELIVERY-001` |
| management retry/force operation | 当前没有 retryable finalizer，也不承诺运行中人工操作；缺失入口比无鉴权入口更安全 | `MANAGEMENT-001` 或真实 retryable 资源计划 |
| Linux runtime | 当前没有可运行 WSL distro；Linux cross-build 与 Ubuntu CI 可提供静态/远端证据，但不是本地 runtime | `PORTABILITY-001`/release acceptance |
| Windows `go mod tidy -diff` CRLF/LF | 探针只显示 `go.sum` 全文件行尾差异，没有依赖内容变化 | `PORTABILITY-001`，不借 Foundation 计划改全仓行尾 |
| 新 Runner/Health/resource profile | Todo 只证明同步 HTTP/CLI/migration | 真实业务需求先做 module capability assessment |

把这些项目塞进当前计划会同时改变部署、平台、资源和产品边界，反而使 Foundation 计划不可确认、不可回滚。

## 5. 合并决策

未启动的 `FOUNDATION-ACCEPTANCE-001` 与 `FOUNDATION-RECONCILIATION-001` 具有同一个完成判据：当前已证明 profile 的十一门从代码事实、故障测试、跨层状态与物理释放证据上同时通过。单独保留 P1 reconciliation 会让 Foundation 总验收仍携带“演进门部分通过”的第二条尾巴。

因此采用单轨替换：

```text
FOUNDATION-ACCEPTANCE-001 + FOUNDATION-RECONCILIATION-001
  -> FOUNDATION-CLOSURE-001
```

新计划只覆盖：

1. preflight-only RestartRequired 的安全候选恢复；
2. 缺失的 Host/process/current-profile 故障和物理释放断言；
3. 复用既有测试并形成唯一门禁证据；
4. 刷新 R002/R004 与 022 authority，按已证明 profile 判断 Foundation-closed。

## 6. 结论与任务影响

- 当前状态仍是 **Foundation-partial**，但剩余范围已经从“可能还有底层框架缺口”缩到一个窄行为修复和一组跨层验收。
- 推荐实施 [`FOUNDATION-CLOSURE-001`](../../plans/foundation-closure-001.md)，不再建立两个并行 Program。
- 计划完成后可以声明 `Foundation-closed(current synchronous HTTP/CLI profile)`，并只解锁落入该 profile 的业务详细设计；Runner、Health、新共享资源与长连接仍不自动解锁。
- 该结论不改变 `Production HTTP API-ready = FAIL`，也不授权 API authority、management、telemetry、delivery、release 或外部系统操作。
