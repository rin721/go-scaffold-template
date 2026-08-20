# 消息系统适配能力需求

## 1. 文档状态

- 变更编号：`038`
- 当前阶段：研究门禁已通过，计划已确认
- 实施状态：实现与本地工程门禁完成；RabbitMQ 4.3 真实协议门禁受环境阻断
- 研究依据：[R001](research/R001-current-messaging-capability-inventory/report.md)、
  [R002](research/R002-messaging-primary-source-comparison/report.md)、
  [R003](research/R003-single-track-messaging-synthesis/report.md)

本文件保存已确认需求与验收边界。当前使用方式以
[消息系统适配能力](../../development/messaging-capability.md)和[消息系统运维](../../operations/messaging.md)为准；
未执行的 RabbitMQ 真实协议门禁不得从本文件推断为已通过。

## 2. 目标

在现有幂等、失败重试、执行记录、定时调度之后，增加统一消息系统适配，使业务模块只声明：

1. 生产或消费哪个稳定 Message Contract；
2. 使用哪个逻辑 Route；
3. 需要何种确认、交付、并发、死信和重要性策略；
4. 实际业务 Handler 或发布内容。

消息适配负责 Provider 选择、连接/Channel、Broker 确认、消费准入、故障恢复、健康与诊断，不承载业务逻辑，
不复制 Execution、Telemetry、Logger、Health、Supervisor 或 Broker 已有可靠性机制。

## 3. 术语

### 3.1 Message Contract

稳定 wire contract，由 Contract ID、Schema Version、Content Type 和 payload 编解码规则组成。Contract identity
与业务语义绑定，不包含 RabbitMQ exchange、Kafka topic 或 NATS subject。

### 3.2 Binding

- Producer Binding：稳定 Producer ID、Contract、逻辑 Route 与发布确认要求。
- Consumer Binding：稳定 Consumer ID、Contract、逻辑 Route、Handler、交付/并发/重要性策略。

Binding 是不可变声明，不访问网络、不启动 goroutine。业务模块通过 `module.Contribution` 显式输出。

### 3.3 Route 与 Provider

Route 是业务 Binding 引用的逻辑地址；运行配置把 Route 映射到命名 Provider 及其物理 topology。
Provider 是具体消息中间件 Adapter。多个 Provider 可显式并存，但一个发布动作不做隐式跨 Provider 双写。

### 3.4 重要性

- `required`：依赖不可用时进程仍 live，但 readiness 失败，发布返回错误，消费暂停。
- `optional`：依赖不可用时 health 为 degraded，发布仍返回错误；是否忽略由业务决策边界负责。

## 4. 功能需求

### FR-001 项目自有 Contract 与 Binding

- 新增 `pkg/messaging`，定义稳定 ID、Contract 描述、Envelope、Producer/Consumer Binding、Publisher、Receipt、
  Handler、交付策略、重要性、诊断与可识别错误。
- 构造器必须集中校验 ID、版本、content type、payload limit、nil Handler、并发、delivery 上限与策略组合。
- 具体业务 schema 不得进入 `pkg/messaging`；业务 Service 不直接处理裸 Broker Delivery。
- 被多个模块共享的 wire contract 使用应用级稳定 contract 包，依赖方向为 module -> contract -> `pkg/messaging`。

### FR-002 唯一模块贡献与装配

- 扩展 `module.Contribution` 显式贡献消息 Contract、Producer 和 Consumer Binding。
- `internal/composition` 是唯一聚合、排序、冲突校验、Route/Provider 绑定与能力注入位置。
- 禁止 `init`、反射扫描、全局 Registry、运行时 Service Locator 和业务模块自行 Dial/Subscribe。
- 重复 ID、冲突 Contract、未知 Contract/Route、未选择 Provider 或 Provider 能力不足必须在候选 Prepare 失败。

### FR-003 Provider 可替换、扩展与并存

- `internal/kernel/app/messaging` 通过显式 `ProviderFactory` 集合构造命名 Provider；不允许 Provider 自注册。
- 公共层只定义共同语义；driver-specific config 使用封闭类型并留在 Adapter 边界。
- 首版实现 RabbitMQ Provider 和 deterministic fake Provider；至少两个命名 RabbitMQ/fake Provider 可被不同 Route 同时使用。
- 未来 Kafka/NATS Provider 的接入不得要求业务 Contract 或 Handler 导入其 Client 类型。
- 不满足 ordering、confirm、dead-letter 等 Binding requirement 时必须拒绝，不能弱化后继续运行。

### FR-004 可靠发布

- `Publisher.Publish` 接收 context、Producer ID 与有效 Message，返回 Receipt 或可识别错误。
- 可靠 Route 必须等待 Broker 接管确认；RabbitMQ 使用 persistent message、mandatory routing 与 publisher confirm。
- negative confirm、unroutable、timeout、断连、退役代际和无效 Contract 必须保持不同错误类型并保留原因链。
- 未确认发布不得返回成功；首版不得用易失内存缓冲、日志后成功或隐藏 fallback 模拟可靠发布。
- Receipt 只证明 Broker 接管，不声称 Consumer 完成、业务落库或 exactly-once。

### FR-005 消费可靠性闭环

- RabbitMQ Consumer 只能使用 manual ack，并设置正数有界 prefetch 与并发。
- Handler 成功或 Execution 判定已完成重复后才 ack。
- RabbitMQ 4.3 reliable Route 使用 quorum queue 原生 delayed retry：retryable 业务失败用
  `basic.reject(requeue=true)` 增加 `delivery-count`，Execution backend unavailable/active lease 等基础设施暂时阻塞用
  `basic.nack(requeue=true)` 延后但不消耗业务 delivery budget；所有动作 `multiple=false`。
- 永久错误、Contract 解码失败、handler panic 或耗尽上限使用 `basic.reject(requeue=false)`，由已验证 DLX 接管。
- 取消、连接丢失或 ack 结果不确定时不得伪造成功；允许 Broker 按 at-least-once 重投。
- Route 声明 reliable/dead-letter required 时，配置与部署门禁必须验证 quorum、`delayed-retry-type=all`、
  正数 min/max、正数 delivery-limit、at-least-once DLX 与目标能力；运行期仅能通过 AMQP 证明的部分不得被扩大描述，
  无法证明的外部 Policy 由独立拓扑验收作为上线前置。
- Consumer 必须能处理 redelivery，且不会出现无界立即 requeue 热循环。

### FR-006 与 Execution 单轨协作

- 每次 delivery 使用稳定 key `ConsumerID + ContractID/Version + MessageID` 调用现有 Execution。
- Execution 继续拥有幂等 Claim、单次业务调用与执行记录；Broker 继续拥有跨 delivery 重投与死信。
- 现有 `ClaimTTL` 无法同时正确表达 running lease 与 completed retention，且失败 `Record` 不释放 claim；038 必须
  在 `pkg/execution` 中以单轨替换为 `LeaseTTL`、`RetentionTTL` 与同步 `Store.Release`，不能在 Consumer 内另建幂等表。
- `Store.Complete` 从成功完成时建立 retention；Handler 失败先同步 Release，再沿用现有 Record。Release/Record
  任一失败都保留错误链且不得 ack；running lease 到期用于进程终止后的最终恢复，不等同于业务重试预算。
- Schedule 等现有调用方必须在同一任务中迁移到新字段并保持原行为，不保留 `ClaimTTL` alias 或兼容分支。
- Consumer 默认显式提交单次执行策略，不叠加默认多次进程内 retry。
- Execution 整体不可用时暂停对应 Consumer intake；竞态中已取得的 delivery 使用 non-counting delayed nack，
  恢复后再开放消费，避免基础设施故障消耗业务 delivery limit。
- Execution memory backend 的进程内边界必须进入文档、诊断和测试，不得宣称跨实例 exactly-once。
- Handler error 的 retryable/permanent 分类复用项目 fault 语义，不在 RabbitMQ Adapter 自建第二套业务分类。

### FR-007 Application Generation 生命周期

- Candidate Prepare 可校验配置、Contract/Binding、Provider capability 并建立不消费的连接候选。
- Commit 前不得取得或执行任何业务 delivery。
- 进程稳定 Consumer Hub 在 Commit 时 quiesce 旧代、完成有界 handoff、再激活新代。
- Retire/Abort/Stop 必须停止新 delivery、取消并等待 in-flight、处理 ack/channel 顺序、关闭 Provider 并聚合错误。
- generation-owned Publisher 在本代 HTTP 请求排空前保持可用；Consumer admission 与 Publisher 生命周期不得混为一锁。
- 非预期恢复循环终止进入现有 Generation monitor/Supervisor，不创建第二套守护器。

### FR-008 暂时不可用与自动恢复

- 网络拒绝、节点切换等暂时故障进入 Connecting/Recovering，不直接使进程 terminal failure。
- 所有连接、confirm、恢复探测和关闭有 context/timeout；退避含抖动并有最大探测频率。
- `required` Route 使 messaging readiness fail；`optional` Route 使 messaging health warn/degraded。
- Publisher 在两类 Route 上均返回 `ErrUnavailable`；底层不得因 optional 静默丢弃。
- 恢复后必须自动恢复 confirm Publisher 与 Consumer，无需重启应用；恢复状态和次数可诊断。
- 确定性配置、认证、Contract、capability 或 topology 错误使候选失败，不无限重试。

### FR-009 配置与安全

- 新增唯一 `messaging` 强类型配置节和安全默认值，默认 disabled，不连接外部 Broker。
- 配置包含全局 timeout/handoff、安全上限、命名 providers 与逻辑 routes；driver 配置严格拒绝未知字段/组合。
- Secret 通过现有配置/环境覆盖进入，不写入示例、日志、诊断或错误文本。
- Route 物理 destination、URI、credentials、headers、payload 均不得进入普通日志/diagnostics。
- RabbitMQ DLX 优先由 Broker Policy 管理；应用不把不可热改的 `x-arguments` 当作通用 Contract。AMQP 无法读取的
  effective delayed-retry/delivery-limit/DLX Policy 由运维拓扑 verifier 验证，不为此把 Management Client 泄漏进
  业务运行链。

### FR-010 可观测与运维

- publish/consume 使用现有 Telemetry `Observe`，传播受控 trace/correlation identity，不暴露 OTel 类型。
- Logger 只在决定策略的边界记录 Provider/Route/Consumer ID、phase、generation、state、outcome 和 error type。
- Ops 现有 snapshot 增加 messaging health、Provider/Route/Consumer 状态、in-flight、confirmed/failed、
  redelivered/acked/rejected/dead-lettered 与 recovery 计数。
- 正常断连恢复使用 Warn/Info，健康路径不打印 Warn/Error，正常取消/关停不记 Error。
- 不新增 `/messaging/health`、第二个 metrics registry 或第二套 diagnostics endpoint。

## 5. 非功能需求

### NFR-001 边界

- `amqp091-go` 与 AMQP 类型只允许出现在 RabbitMQ Adapter 和测试边界。
- 业务模块依赖自身窄 port 或 `pkg/messaging`，不得导入 Kernel App/Provider。
- 不新增万能容器、`utils/helpers/common`、循环依赖或共享可变全局状态。

### NFR-002 有界并发与资源

- 每个 connection/channel/goroutine 有唯一 owner、停止信号与 Wait。
- payload、header、prefetch、consumer concurrency、in-flight confirm 与恢复状态缓存均有上限。
- Publisher/Consumer 借用对象不暴露 Close；终结错误与主错误使用 `errors.Join` 保留。

### NFR-003 可测试性

- Contract、Provider、Clock/ID、连接状态与 delivery 可替换为 deterministic fake。
- 代际/并发/恢复单测不依赖长时间 sleep 或偶然时序。
- RabbitMQ 协议保证必须有真实 Broker 集成测试，fake 只证明项目状态机。

### NFR-004 单轨演进

- 实现完成后只有一条模块消息贡献、一条 Provider 构造、一条 Consumer 生命周期和一套健康/重试/记录路径。
- 不保留旧别名、兼容配置、平行 Router 或静默降级。
- 当前主题文档必须同步，038 只保存任务证据。

## 6. 非目标

- 不实现具体 Todo/订单/支付等业务消息或业务 Handler。
- 不实现 Outbox、Inbox、CDC、Saga、工作流、请求响应 RPC、跨 Provider 原子双写或迁移镜像。
- 不实现 Kafka/NATS/SQS/Pulsar Provider；只保证扩展点由 fake + 多命名 Provider 测试证明。
- 不承诺 exactly-once 业务副作用、全局顺序或跨区域一致性。
- 不在应用中创建 RabbitMQ 管理控制面；生产 topology/DLX Policy 由运维配置，Adapter 只验证和使用。

## 7. 验收标准

### AC-001 Contract/Binding 与边界

- 两个测试模块可声明 Producer/Consumer，共享 Contract identity 一致时聚合成功，冲突时 Prepare 失败。
- 搜索证明业务模块无 AMQP/Provider import，无 `init`/全局 Registry/直接 Dial。
- 两个命名 Provider 可由不同 Route 并存，未满足 capability 的 Binding 被拒绝。

### AC-002 发布

- 真实 RabbitMQ 验证 confirmed、negative/unroutable、timeout、断连与恢复；未 confirm 不返回 success。
- Publisher context 取消和 generation retire 可识别，未泄漏 goroutine/channel。

### AC-003 消费

- Prepare 后 Commit 前零 delivery；Commit 后开始，reload 时旧代 quiesce/handoff，新旧不并行取新消息。
- 成功/duplicate ack；retryable business failure 走 counted delayed reject；Execution/active lease 暂时阻塞走
  non-counting delayed nack；permanent/invalid/panic/exhausted 进入已验证 DLX。
- Execution 失败同步释放 claim 后，同 Message ID 可再次执行；未释放/并发 claim 只延后不消耗业务 delivery budget；
  成功 retention 从完成时计算并在窗口内阻止重复业务操作。
- prefetch/concurrency 有界；ack 丢失产生的重复不会重复成功业务操作（在已声明的 Execution 边界内）。

### AC-004 故障与恢复

- Broker 启动前应用仍 live；required/optional readiness/health 各自符合策略。
- Broker 上线或重启后 Publisher/Consumer 自动恢复，无应用重启；错误分类、日志和诊断正确。
- 配置/认证/topology 确定性错误不进入无限恢复。

### AC-005 生命周期与观测

- Abort/Retire/Stop/ForceStop、候选失败、handler panic、consumer cancel 与恢复循环 terminal failure 都有测试。
- publish/consume trace、Execution record、Ops snapshot 与低敏日志一致；不输出 URI/credential/payload/headers。

### AC-006 工程验证

- `gofmt`、`go test ./...`、`go test -race ./...`、`go vet ./...`、仓库质量/架构检查与 `git diff --check` 通过。
- 在真实 RabbitMQ 4.3 上验证 quorum delayed retry 的 counted reject、non-counting nack、delivery-limit 与
  at-least-once DLX 并保存证据；Docker 不可用时不得声明该验收完成。
- 完整 Diff 只包含 038 已确认任务；037 先行基线与 038 变更可追溯、可分离。
