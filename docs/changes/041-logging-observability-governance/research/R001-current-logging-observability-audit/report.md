# R001 当前日志覆盖与治理缺口审计

## 1. 研究问题

本报告回答：当前项目日志能力是否需要重建；真实代码中哪些关键路径已经有有价值日志；哪些路径仍难以通过日志定位“发生了什么、在哪里、原因、影响、动作和恢复”；以及计划阶段应怎样补齐而不制造噪声、重复日志或敏感信息泄露。

## 2. 方法与范围

研究快照为 `f86825b52eb19e8dd807c0db7f59d5d7c7e7102a`，初始工作树 clean。检查范围包括：

1. `README.md -> docs/README.md -> docs/development/logging.md` 和 `pkg/logger` 的当前日志规范与能力；
2. `cmd/app -> internal/composition -> internal/kernel` 的 Service、Generation、配置 load/reload、listener、shutdown 与失败边界；
3. `pkg/httpx` access/recovery、Auth audit、Ops management、Health/Diagnostics；
4. `pkg/execution`、`internal/kernel/app/execution`、Scheduler、Messaging、RabbitMQ Provider 和 Consumer；
5. 日志相关测试、架构门禁和历史 028/035/037/038 任务记录。

本轮没有启动 Service、连接数据库、连接 RabbitMQ、修改实现或运行会改变状态的命令。静态代码、测试和权威文档证据足以形成计划；真实进程和协议输出列入确认后的实施验收。

## 3. 当前事实

### 3.1 Logger 与基础治理已经存在

`pkg/logger` 以项目自有 `Logger`、`Field`、`Config` 和 `Resource` 封装 zap；业务层不接触 zap 类型。`internal/kernel/logging.Manager` 提供强制 baseline 与配置化 replacement，普通消费者只拿到不带 `Replace/Restore/Close` 权限的稳定 facade。`docs/development/logging.md` 已规定唯一错误 owner、级别、字段、敏感信息和测试要求。

因此本任务不需要新增公共 Logger 方法、全局 logger、第二套 sink、`slog` 平行封装或新的观测总线。

### 3.2 已有日志覆盖

当前已有日志集中在以下 owner：

| owner | 已有事件 |
| --- | --- |
| Service/Application Generation | service selected/composed、generation load/prepare/candidate/commit/start/reload/no-op/stop、application ready/draining/stopped、service failure、reload reject/cleanup debt |
| HTTP | request completed/rejected/failed、panic recovered，带 method、operation、request_id、trace_id、duration、status、error_code |
| Auth audit | `security decision`，记录 operation/action、actor_kind、subject/resource hash、decision、outcome |
| Execution | recovery state degraded/recovered；异步记录失败 Warn |
| Scheduler | scheduler started/draining/stopped、coordination state changed、task started/completed/failed、fatal state |
| RabbitMQ Provider | provider state changed，记录 provider、driver、state、error_type |

这些日志已经遵守“重要事件、状态变化、边界调用和失败原因”的方向，且多数不记录 payload、DSN、Token、raw URL 或原始错误文本。

### 3.3 当前缺口

**缺口 A：one-shot migration/CLI 外部 I/O 缺少结构化运行日志。**

`internal/composition/migration.go` 会加载配置、校验候选、解析数据库配置、构造 migration module 并执行 status/up。`internal/module/migration/service.go` 会读数据库状态、执行 `runner.Up`、completion resolve/verify 和 close。当前主要依赖 CLI JSON 输出和最终 stderr；没有统一记录 `operation=db.migrate.status/up`、phase、outcome、target/current、dirty/compatible、失败类型和清理结果。CLI 人机输出不应冒充日志，但 migration 是外部 I/O 边界，需要低敏 operation 日志。

**缺口 B：Execution 异步记录失败违反低敏日志规范。**

`internal/kernel/app/execution/execution.go` 的 `asyncCfg.OnError` 当前记录 `logger.String("error", err.Error())`。`docs/development/logging.md` 明确要求结构化运行日志优先记录 `error_type`、`cause_type`、owner、phase 和稳定错误码，不直接记录未经审查的 `err.Error()`。即使当前 memory backend 多数错误低敏，该实现也会在后续真实 backend 接入时变成泄漏风险。

**缺口 C：Messaging Consumer 关键处置没有运行日志事件。**

`internal/kernel/app/messaging/messaging.go` 维护 acknowledged/rejected/deadLettered/redelivered/inFlight/lastError 诊断计数，并按 Execution health 暂停/恢复 Consumer admission。`consumerRuntime.handle` 对 `Ack`、`RetryCounted`、`DeferUncounted`、`DeadLetter` 的决策没有日志；RabbitMQ `handleDelivery` 对 decode 失败会直接 reject。结果是 operator 能在 diagnostics 中看到累计状态，却难以从日志还原一次失败消费采取了 retry、defer 还是 dead-letter，也难以关联 `consumer_id`、route、message_id、trace_id、delivery_count 和 error_type。

**缺口 D：Messaging admission 与 Provider 状态缺少 generation 关联。**

RabbitMQ Provider 状态日志有 provider、driver、state 和 error_type，但 ProviderDependencies 当前不携带 generation。Messaging component 的 admission transition 失败日志有 owner/phase/error_type，也缺少 generation 和 desired state。代际切换或 reload 中排查 Consumer handoff 时，需要把 Provider/Consumer 状态和 Application Generation 关联。

**缺口 E：Management health/diagnostics 异常 outcome 主要靠 HTTP status。**

`internal/module/ops/binding/http/handler.go` 对 probe/diagnostics/build/metrics 提供 management HTTP。探针失败会返回 503 或 fail body，普通 operation 错误返回 `management operation failed`；当前缺少 management owner 的低敏事件，用于记录 readiness 从 pass 到 warn/fail 的决定原因、diagnostics 读取失败、metrics protection 拒绝等结果。公开 HTTP access log 不覆盖独立 management route 的全部语义，Ops diagnostics 本身也不应替代事件日志。

**缺口 F：Scheduler/Messaging 日志测试门禁不足。**

`internal/kernel/app/schedule/schedule.go` 和 `messaging/rabbitmq.go` 已有有价值日志，但测试多验证 state、health、diagnostics 和 disposition，较少断言日志 level、message、字段和敏感信息排除。没有测试保护时，后续改动可能悄悄删除关键事件或引入原始错误文本。

**缺口 G：文档规范需要细化到新增能力。**

`docs/development/logging.md` 已有通用原则，但 execution、schedule、messaging、migration 和 management 场景的必记/不记事件、级别、字段和测试方式仍分散在能力文档、运维文档和代码里。本轮应更新权威规范和能力文档链接，避免把日志计划写成只在任务记录中的历史证据。

## 4. 不应处理为缺口的点

- Todo Service 没有直接注入 Logger：当前 Todo 是同步业务用例，HTTP access log、Auth audit、execution record 和 repository 错误链已经覆盖边界。为每个 CRUD 成功路径添加业务日志会重复且高噪。
- Database/GORM 没有打开底层 SQL logger：当前数据库包已做错误脱敏，默认打印 SQL、参数或 DSN 会提高泄漏风险。数据库连接和 readiness 可由 app component owner 记录低敏结果。
- Scheduler 对每次并发 skip 不一定要逐条 Warn：高频拥塞可能造成日志风暴。应优先记录状态变化、失败、fatal 和可控采样/聚合，不把计数型 diagnostics 全部搬进日志。
- CLI help、prompt 和 JSON 输出不是运行日志。只有涉及外部 I/O、状态改变或最终失败的 one-shot operation 边界需要结构化日志。

## 5. 计划影响

本轮应沿用现有能力，做单轨补齐：

1. 修复 Execution 异步错误日志为低敏字段；
2. 为 migration one-shot operation 增加边界日志；
3. 为 Messaging Consumer 非成功处置、decode reject、admission transition 和 Provider state 增加低敏字段与 generation correlation；
4. 为 management health/diagnostics 关键异常 outcome 增加 owner 日志；
5. 补齐对应 `logger.TestLogger`、RabbitMQ fake/fixture、process smoke 和架构搜索门禁；
6. 更新 `docs/development/logging.md`、execution/schedule/messaging/operations 文档和本任务证据。

不新增第三方依赖，不改变公共 Logger API，不引入远程日志采集，不把 diagnostics/metrics/trace 替换为日志。

## 6. 研究门禁

关键问题已有当前代码、测试、文档和历史任务证据；事实、推断与目标设计已分离。剩余未知主要是确认后具体测试实现和真实协议 smoke 覆盖范围，不妨碍形成计划。研究门禁通过。

