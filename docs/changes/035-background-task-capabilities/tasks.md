# 035 任务清单：后台任务能力装配（幂等 / 重试 / 执行记录）

任务 ID 稳定；状态：研究门禁通过，计划待确认。

| ID | 依赖 | 任务 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- |
| EXEC-001 | — | 新增 `pkg/execution`：Key/Result/Status/Execution/OperationExecutor/Store 契约、状态机、命名错误（ErrDuplicate/ErrBackend/ErrRetryExhausted/ErrCanceled/ErrTimeout）、执行记录模型；复用 `pkg/resilience`/`pkg/fault`/`pkg/concurrency`/`pkg/clock`/`pkg/idgen` | 契约编译；错误保留原因链；不 import internal；单测覆盖语义 | 待确认 |
| STORE-001 | EXEC-001 | 实现幂等占用表与执行记录表持久化（复用 `pkg/database`）；TTL 过期与占用/完成状态迁移；提供内存 backend 供测试 | 建表/Schema/迁移接线；backend 失败可区分；并发占用经 singleflight 合并 | 待确认 |
| COMP-001 | STORE-001 | 新增 `internal/kernel/app/execution`：配置化 + Leased 组件（`app.ManagedConfigured`/`Leased`/`KernelInstanceSwap`），集中声明应用默认配置（032），输出稳定 `OperationExecutor` facade | Definition 组装/Lease/配置默认值/backend 生命周期测试；不泄漏 backend 类型与关闭权 | 待确认 |
| WIRE-001 | COMP-001 | `internal/kernel/composition/execution.go` 新增 `composeExecution`，`app.DependencySet` 注入 Database；`composition.go` 的 `Capabilities` 增 `Execution`，`Compose` 在 database 后调用 | 装配成功产出可用 `Capabilities.Execution`；失败返回零 Capabilities；无循环/反向依赖 | 待确认 |
| TEST-001 | EXEC-001, STORE-001, COMP-001, WIRE-001 | 端到端与门禁测试：幂等重复提交、重试耗尽、可重试 vs 不可重试、超时、backend 失败、错误链、记录写入、单飞合并；composition 装配 | `go test ./... -count=1` 相关包通过；architecture validators 通过 | 待确认 |
| DOC-001 | 全部 | 同步 `pkg/README.md`（能力清单从"暂缓路线"移到"当前能力"）、`internal/kernel/app/README.md`（当前组件 + 手工装配步骤）、模块开发指南能力表、035 文档状态 | 文档与实现一致；`pkg/README` 不再把后台任务列在暂缓 | 待确认 |
| GOV-001 | WIRE-001 | 门禁/反例：`pkg/execution` 不得 import internal；业务模块不得反向 import backend 具体类型；execution 组件不加隐藏第三方 | 架构 validator 覆盖并通过；无循环/反向依赖 | 待确认 |
| VER-001 | 全部 | 全量验证：`go build/vet/gofmt ./...`、`go test ./... -count=1`、`go mod tidy -diff`、`go generate ./...`、`git diff --check` | 全部通过；更新 035 状态为已完成并聚焦提交（不 push） | 待确认 |

## 确认状态

- 研究门禁：已通过（R001、R002）。
- 计划状态：**待确认**。此为新增源码、配置、测试的非文档实施，须用户在计划报告后的后续消息中明确确认后，方可将状态改为"已确认"并开始实施。
- 本轮未修改实现文件、未执行状态变更命令、未暂存未提交。

## 剩余风险

- backend 具体 SQLite 表能力（TTL 清理、并发占用写入）以真实实现验证为准。
- 跨进程严格竞争/分布式锁/消息调度暂不引入；出现该需求时回退研究重新论证。
- 组件默认启用与否由配置决定，确认后再定。
