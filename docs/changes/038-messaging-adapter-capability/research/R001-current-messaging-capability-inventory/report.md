# R001 当前消息相邻能力与真实缺口

## 1. 问题与方法

本记录回答：当前代码能否直接表达跨中间件消息生产与消费，哪些治理能力应复用，缺口应落在哪一层。
研究从根 README、文档入口、Git 状态、模块 Contribution、Application Generation、Kernel App、Execution、
Schedule、Telemetry、Health/Ops 与 Supervisor 的真实定义、构造、调用和停止路径开始，不以 038 目标反推事实。

研究开始时 HEAD 为 `a675f33`，工作区包含 037 定时调度实现；复核时该能力已收口为本地提交
`00083be`，而 `origin/main` 仍为 `a675f33`。下文结论已按 `00083be` 复核；038 实施前仍必须重新
验证 HEAD 与 037 的验证证据，不能把本地提交描述为已推送事实。

## 2. 当前事实

### 2.1 没有消息能力或中间件依赖

- `go.mod` 没有 RabbitMQ、NATS、Kafka 或 Watermill 依赖。
- `pkg` 没有 Message、Publisher、Consumer、Contract、Broker 或 delivery/ack 契约。
- `internal/kernel/app` 没有 messaging Component；`internal/kernel/composition.Capabilities` 不输出消息入口。
- `module.Contribution` 的当前形态只有 `ID`、`Participants` 与 037 新增的 `Schedules`，不能声明消息
  Contract、Producer、Consumer、Route、交付保证或故障重要性。
- `internal/composition` 不创建消息连接、Publisher、Consumer，也没有消息 admission/handoff。

因此消息系统适配是新底层能力，不能靠打开现有开关完成。

### 2.2 可复用的唯一装配与运行 owner 已存在

- `internal/composition` 是 application composition root；业务模块通过显式构造和 `Contribution` 接入，
  当前没有包扫描、`init` Registry 或业务运行期 Service Locator。
- `GenerationCoordinator` 已拥有 `Prepare -> Commit -> Retire/Abort/Stop`，候选失败保留旧 Generation。
- `Supervisor` 已拥有进程级启动、Ready、运行失败与停止，不需要消息能力再建 Runtime 或守护循环。
- 037 的 `ScheduleHub` 证明了“候选可构建、Commit 才开放业务准入、Retire 撤销准入”的当前模式。

普通 `supervisor.Participant` 在 Generation Prepare 中会立即 `Start`。如果把 Consumer 直接做成模块 Participant，
候选在 Commit 前就可能取得消息并执行业务，违反代际隔离。因此 Consumer 必须使用独立的 generation admission，
不能直接复用 Participant 形态。

### 2.3 Execution 可复用，但不是 Broker

`pkg/execution.OperationExecutor` 已组合：

- 幂等 Claim/Complete；
- `pkg/resilience` 单次调用内的重试/超时；
- 成功/失败执行记录与 Trace identity；
- 可识别错误链。

`internal/kernel/app/execution` 还提供命名策略、恢复状态、异步记录和 Health。它适合包围一次消息业务处理，
但不能替代以下 Broker 职责：

- Publisher 被 Broker 接受的确认；
- Consumer delivery ack/nack；
- 未 ack 交付的重投递；
- consumer group/queue/partition 的分发与背压；
- delivery limit、死信和 Broker offset/cursor。

当前 Execution backend 仍是 memory/disabled。它只提供进程内去重；崩溃或消息转移到另一实例后不能证明跨实例
幂等，更不能把业务数据库提交与 Broker ack 原子化。038 必须如实保留这一保证边界。

完成性复核还发现 Execution 不能原样承接 Broker redelivery：

- `Execution.ClaimTTL` 同时决定 running claim 与 completed 去重的过期时间，无法分别表达“处理崩溃后的短 lease”与
  “成功后的较长去重保留”；当前 `MemoryStore.Complete` 还沿用 Claim 时计算的截止时间，而不是从完成时重新计算。
- Handler 最终失败时，`executor.run` 只调用 `Store.Record`。`MemoryStore.Record` 不改变 running claim；相同 Message ID
  立即重投时会得到 `ErrAlreadyRunning`，`ClaimTTL=0` 时甚至不会自动释放。
- 因而不能只把 Message ID 填进现有 `ClaimTTL` 就宣称 Broker 重投闭环成立，也不能在 Consumer 内另建第二套
  幂等表规避这个缺口。

最小单轨补齐应在 `pkg/execution` 分离运行 `LeaseTTL` 与完成 `RetentionTTL`，让成功完成从完成时建立保留窗口，
并增加同步 `Release` 状态转换：失败先释放 running claim，再沿用现有 `Record` 记录失败。所有现有调用方在同一变更中
迁移；消息 Consumer 只消费增强后的唯一 Execution 契约。

### 2.4 现有恢复状态机不能直接复制为消息 Store

`RecoveringStore` 的 Healthy/Degraded/Recovering、有界缓冲与探测模式可作为状态设计参考，但消息发布不能机械
复用它的本地记录缓冲：把未持久化消息放入内存后向业务返回成功，会在进程退出时丢失并破坏可靠性。

消息 Provider 应复用恢复原则（有 owner、超时、退避、状态与停止），但发布失败必须返回可识别错误；没有持久
Outbox 时不得把内存缓存称为可靠降级。

### 2.5 Telemetry、日志、健康与诊断已有唯一出口

- `pkg/observability.Telemetry.Observe` 已能包围 background work，不需要暴露 OTel Tracer/Span。
- `pkg/logger` 是唯一日志契约；状态边界记录低敏结构化事件，不应打印 URI、凭据、payload 或原始 headers。
- `pkg/health` 已表达 pass/warn/fail；Ops management 是 readiness/diagnostics 的唯一 HTTP 出口。
- 037 已把调度快照投影到现有 Ops；消息 Provider、Route、Producer、Consumer 也应扩展同一个 snapshot。

### 2.6 数据库事务存在，但没有 Outbox/Inbox 契约

`pkg/database.Client.WithinTx` 提供受控事务，但当前业务 Repository、Execution Store 与消息 Publisher 没有共同的
事务协议。仅仅“项目有数据库”不足以声称业务写入与消息发布原子，也不足以实现通用 Outbox/Inbox。

## 3. 能力评估

| 维度 | 当前结论 |
| --- | --- |
| 用例 | 模块声明生产/消费的消息和治理要求；适配层执行传输、确认、恢复与观测 |
| 现有能力 | 复用 Generation、Supervisor、Execution、Telemetry、Logger、Health/Ops、Config、Clock/ID |
| 最小补齐 | Execution 分离运行 lease/完成 retention，并在失败记录前同步释放 claim |
| 新消息能力 | Message Contract/Binding、Publisher、Provider SPI、RabbitMQ Adapter、Consumer Hub |
| 归属 | 公共消息语义跨模块且 Provider 由进程统一选择，进入 Kernel App；业务 schema/handler 属模块或应用 contract |
| 资源 | Provider 拥有 connection/channel/consumer goroutine；Consumer Hub 拥有代际准入；业务无 Close 权 |
| 运行 | Candidate 可连接但不消费；Commit 开放 Consumer；Retire 停取、处理 ack 边界并排空 |
| 配置 | `messaging` 拥有 Provider/Route/恢复安全边界；业务 Contract/Handler 不从 YAML 动态注入 |
| 出口 | 业务生产只拿项目 Publisher 或更窄模块 port；消费只输出 immutable Binding |
| Reload | Provider 新旧连接可短时并存；Consumer 必须 quiesce/handoff；物理 topology 不可热改时拒绝或 RestartRequired |
| 契约适配 | Participant 不适用；需要与 ScheduleHub 同级但消息专用的 Consumer admission |
| 失败 | 确认失败向上返回；消费按 ack/requeue/dead-letter 分类；暂时断连进入 typed recovery state |
| 日志 | Provider/Route/Consumer 状态 owner 记录稳定 ID、phase、generation、outcome、error type |
| 影响 | module contract、pkg/kernel app/composition、generation、Ops、config/docs/tests、依赖声明 |

## 4. 推断

1. 项目需要 `pkg/messaging` 稳定契约和 `internal/kernel/app/messaging` 完整能力链。
2. 生产与消费要分开治理：generation-owned Publisher 可供该代请求使用到排空结束；Consumer 则由进程稳定 Hub
   单轨切换，避免候选提前消费和新旧代同时拉取。
3. 多 Provider 并存应由显式命名 Factory 与 Route 映射完成；不能靠运行时扫描注册，也不应自动跨 Provider 双写。
4. Broker redelivery 是跨交付重试 authority。Execution 默认每个 delivery 只运行一次业务操作并记录；其 running
   lease、completed retention 与失败 release 必须先单轨补齐，避免 claim 阻塞重投，也避免一次 delivery 内重试 N 次、
   Broker 再重投 M 次形成未声明的 N×M 尝试。
5. 暂时断连不是进程 terminal failure：必需 Route 影响 readiness，可选 Route 影响 degraded；不返回虚假发布成功。
6. Exactly-once、Outbox/Inbox、业务事务与 offset/ack 原子化需要真实用例和新的契约研究，不应混入基础首版。

## 5. 局限与对 038 的影响

- 当前未运行真实 Broker；连接恢复、ack 歧义、DLX 与拓扑验证要在确认后的集成测试证明。
- Execution 的 lease/retention/release 是 038 的前置公共契约变化，必须随本计划明确确认和迁移，不能在实现中临时绕过。
- 037 已形成可追溯的本地提交 `00083be`，但尚未出现在 `origin/main`；038 实施前置门禁仍须确认
  该基线未被重写且工作区可精确隔离。
- 首版应以一个真实 Provider + 一个 deterministic fake Provider 证明公共边界；不能仅写接口宣称可替换。
- 研究门禁可进入计划，但不构成实施授权。
