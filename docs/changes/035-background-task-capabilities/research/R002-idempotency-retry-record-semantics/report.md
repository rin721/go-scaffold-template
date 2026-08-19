# R002 幂等 / 重试 / 执行记录语义与装配决策

## 研究问题

如何把幂等判重、失败重试、执行记录组合成稳定可注入契约，并判断持久化后端是否应进入完整底层能力链。

## 事实（可复核）

- `internal/kernel/app/README.md` 第 5 行：只有"同时跨业务复用 + 由当前进程统一选择"的底层资源才进入 Kernel App；只满足复用的普通库留在 `pkg`。
- `pkg/README.md` 第 10 行：完整链只治理上述双条件资源；上层不得直接持有具体 Adapter。
- `pkg/resilience` + `pkg/fault.Retryable` 已覆盖重试/超时/熔断引擎；`pkg/concurrency.SingleFlight` 覆盖同进程并发合并。
- `composition.Compose` 已把 Logger/Clock/IDGenerator/Validator/Database/Cache/I18n/Storage 收集进 `Capabilities`，即"进程统一选择"既有出口。
- 幂等键与执行记录持久化需要一个稳定 backend；`pkg/database` 是现有、由进程统一持有的库，SQLite/PostgreSQL/MySQL 均已治理。

## 推断

1. **关注点拆分**：
   - 幂等：按 `key` 判重 + 状态机（进行中/成功）+ TTL 过期；并发时用 singleflight 合并同 key。
   - 重试：对 `fault.Retryable(err)` 且非幂等结果，用 `pkg/resilience.Do` 指数退避；`MaxAttempts` 可配、有上限，禁止无限重试。
   - 执行记录：写 `key/状态/结果/原因/耗时/触发者/时间/TTL`，保留错误链，供诊断/审计。
2. **装配位置**：
   - `pkg/resilience` 保持纯引擎，**不**重建、**不**升级 kernel/app。
   - 新增"操作执行器 + 幂等/记录后端"进入完整链：`pkg/execution`（契约、引擎、状态机、错误）→ `internal/kernel/app/execution`（提供配置化 + Leased 组件，输出稳定 facade）→ `internal/kernel/composition/execution.go`（选择后端并把 facade 加入 `Capabilities`）→ 业务模块经 composition 注入最小契约。
   - 双条件论证：幂等/执行记录对所有业务模块语义一致（跨业务复用）；由进程统一选择 backend 与组件（进程统一选择）→ 满足进入完整链的条件。
3. **失败语义**（对齐 AGENTS 3.3）：幂等冲突、backend 失败、重试耗尽、取消/超时各自可区分；重复成功返回可识别"已完成"信号而不重试；记录写入失败保留原错误并向上导出，不吞错。
4. **边界**：不做跨进程分布式锁（当前无多实例选择语义，使用"进行中 + TTL 过期"的乐观占用即可）；不做消息队列/死信；订单/支付/库存不实现，仅消费能力。

## 对计划的影响

- 设计一个 `OperationExecutor` 契约：`Execute(ctx, Execution{Key, Policy, Operation})`。
- 后端存储两表模型：幂等占用表（key/状态/过期）与执行记录表（key/结果/原因/耗时/触发者/时间）。
- 重试继续复用 `pkg/resilience` + `pkg/fault`，不引入新第三方。

## 局限与刷新触发器

- 若出现多实例严格竞争、需要分布式锁或消息调度，应回退研究重新论证。
- 具体 schema 与 backend 能力（如 SQLite 支持）以真实实现与测试为准。
