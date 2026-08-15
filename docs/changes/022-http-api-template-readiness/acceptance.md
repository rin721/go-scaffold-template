# 验收：底层闭环、业务解锁与成熟模板

## 1. 用途

本文件把研究结论转换成可复核门禁。生命周期目标证据已按 [`FOUNDATION-LIFECYCLE-001`](plans/foundation-lifecycle-001.md) 产生；其他门禁仍由对应后续计划负责，不能从本轮测试外推为 Foundation-closed。

## 2. 十一门当前矩阵

| 门禁 | 当前状态 | 已有证据 | 阻断项 | 目标证据 |
| --- | --- | --- | --- | --- |
| 事实门 | 通过 | R002 全链源码/测试追踪，R003 官方主源 | 无 | 变更后刷新快照 |
| 目标门 | 通过 | copy-owned HTTP API server baseline；不做通用 Runtime | 无 | 后续任务不得扩大产品目标 |
| 边界门 | 部分通过 | business/kernel/http/third-party import 边界 | Runner/Health/new resource 场景无模块接入契约 | 真实需求触发时完成 capability assessment |
| 装配门 | 当前 profile 通过 | application composition + forward-only Plan | 后台 profile 未证明 | 同步 profile 复制验收；异步 profile 单独验收 |
| 生命周期门 | **通过（当前 profile）** | terminal timeout 可继续 Stop；candidate/retired/current terminal result cache；Supervisor/HTTP graceful-force；逐资源释放测试 | 新 retryable Adapter、hijacked/WebSocket 和部署信号政策仍需真实场景 | 新资源继续执行 capability assessment |
| 一致性门 | 部分通过 | 同一候选、提交前原子；cleanup debt 保留 owner/generation 并阻断后续 reload | 跨 Kernel/Supervisor 统一 management view 未实现 | `FOUNDATION-DIAGNOSTICS-001` |
| 错误门 | 部分通过 | error wrapping/join、typed cleanup error | 缺 owner/generation/pending diagnostics | 结构化安全诊断与 error chain 同时验收 |
| 治理门 | 部分通过 | import graph、binding、route/participant 唯一性、terminal attempt/force 负向门禁 | EnvSource 冲突仍未闭环 | `FOUNDATION-CONFIG-001` |
| 演进门 | 部分通过 | immutable generation、RestartRequired、cleanup debt 不覆盖 | sticky recovery 和受控 management recovery 未决定 | diagnostics 与 P1 reconciliation 研究 |
| 复杂度门 | 通过 | 当前规模无需 Fx/Wire/general DAG | 防止借加固扩成万能框架 | Diff/ADR 证明只改必要状态与契约 |
| 业务扩展门 | **未通过** | Todo 同步 HTTP/CLI profile | 底层 P0 未闭环；新模块 profile 未评估 | 第 4 节全部条件通过 |

## 3. Foundation-closed 阻断验收

### `FND-ACCEPT-001` 终止排空

- 活跃 Lease 存在时，Stop 先阻断新 Use，但不强关活跃实例。
- 首次宽限期超时后状态不是假 stopped，diagnostics 列出 owner/generation/phase。
- 活跃 Use 最终释放后，由 owner 进程继续场景化 finalization；一个 generation 只能成功终结一次，每个 attempt 至多执行一次。
- `NoFinalization`、`DrainThenTerminalClose`、`GracefulShutdown` 与 `GracefulThenForce` 分别验收，不存在万能 `Close(force bool)`；`RetryableFinalize` 因当前无已证明资源而验收为“未实现且不能被配置启用”。
- 只有资源 Adapter 证明再次调用会真实补做安全步骤时才允许 retry；terminal attempt 失败必须进入可诊断 terminal-failed，而不是盲重试或丢失引用。
- caller deadline、process 总 shutdown budget 和外部 forced exit 分别验收，子层不得重建无限或更长预算。

### `FND-ACCEPT-002` 失败清理所有权

- initial start compensation、candidate discard、committed previous cleanup、terminal current cleanup 四类 stop error 都保留原始 error chain。
- 每个未完成清理都能定位 capability owner 和 generation；引用只在清理成功或明确终态后释放。
- 新代已提交时不伪装成回滚；readiness、reload block 和最终退出政策一致。
- 分配资源后的构造失败必须证明补偿完成，或把残余 handle 转交 owner；Database 构造/Ready 不得留下返回 nil instance 后无人持有的 pool。
- 能力不可借用、Close attempt、物理句柄释放和 release verification 四个结果可分别断言。

### `FND-ACCEPT-003` Supervisor 与长期责任

- Participant Stop 和 Task Run 忽略 context 时，分别在 `PendingParticipants` 与 `PendingTasks` 中保留 owner；显式 force 结果进入 `ForcedParticipants`。
- Supervisor 在总 shutdown budget 内返回，且不会把仍运行的 goroutine 描述为已停止。
- Service 与 CLI 使用同一参与者顺序、错误聚合和最终清理不变量。
- 独立 CLI 启动的新进程不能声称关闭了运行中服务的资源；未来 retry/force command 必须通过受控 management operation 定位 owner/generation。

### `FND-ACCEPT-004` 配置确定性

- 同一 EnvSource 内 scalar/object、object/scalar、空 segment、大小写碰撞有唯一拒绝语义。
- Source 间覆盖只允许同形值，并保留顺序和 provenance；任何冲突不进入 Build。
- 同一候选仍由所有 Kernel/application owner 严格校验后一次提交。

### `FND-ACCEPT-005` 验证层次

- 状态机单元测试覆盖正常、超时、重试/最终释放和 cleanup error。
- Kernel/Supervisor/Host 集成测试覆盖 start、ready、runner failure、reload、drain、stop。
- 逐资源至少覆盖 Database、Redis、logger、HTTP、fsnotify，以及当前 StorageManager 的 no-finalization 证明；consumer-owned typed cache Client 和公共文件 Storage 的 baseline 去留也有结论。
- release verification 至少包含 Windows 文件锁、listener 重绑定、Serve/task goroutine done 或等价所有权证据；重复 Close 无错误不算释放证明。
- scope-appropriate `test/race/vet/build` 与文档/架构 gate 通过；未执行平台或外部资源明确标注。

## 4. 业务模块详细设计解锁清单

新的 Handler/Service/Repo/Model 详细设计只有同时满足以下条件才可进入：

- 第 3 节全部通过，R002/R004 刷新为 `Foundation-closed`。
- 新模块先完成 actor/use case/outcome/error/data owner/transaction/config/resource/lifecycle/health/HTTP/CLI/background needs 清单。
- 需求属于已证明的同步 HTTP/CLI profile；若需要 Runner、Ready、Health、新共享资源或 reload policy，先单独完成底层研究、计划、确认与验收。
- 业务 core 只依赖使用方定义契约；第三方具体类型留在 Adapter，装配留在 `internal/composition`/模块局部组合。
- 没有 `init` 隐式注册、service locator、全局 mutable state、私起不可等待 goroutine、第二套 client/connection 或跨模块内部访问。

该清单通过只解锁业务详细设计，不自动授予源码实施权限。

## 5. Production HTTP API-ready 验收

完成 Foundation-closed 后，仍需满足 requirements 中的 API authority、protocol/edge、security、management/observability、data/delivery 和复制 release gate。最终验收至少包含：

- 两个正式 release 的隔离复制副本：保留 Todo 与移除 Todo/装配独立模块；
- Windows/Linux 同义 validation；
- 正常请求、协议错误、未授权、依赖失败、reload cleanup failure、SIGTERM drain、migration failure 和 breaking API diff；
- 容器/部署、SBOM/签名、rollback 和安全负向证据。

## 6. 当前结果

截至 2026-08-15：`Foundation-closed = FAIL`，`Business-extension = BLOCKED`，`Production HTTP API-ready = FAIL`。失败是计划输入，不是实现失败声明；本轮未执行源码测试或运行时故障注入。
