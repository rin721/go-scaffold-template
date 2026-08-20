# 038 任务清单

## 1. 当前状态

- 研究门禁：已通过（`MSG-R01..R03`）
- 计划状态：已确认
- 非文档实施：已授权；实现与本地工程门禁已完成，RabbitMQ 实机门禁受环境阻断
- Git：实施前 fetch 成功；HEAD `00083be`、`origin/main` `a675f33`，无 merge/rebase，038 将由本轮 Conventional Commit 收口
- 外部状态：Windows 无 `docker` 命令且 WSL 无可运行发行版；未创建容器、未连接远端 Broker、未修改生产 topology

用户已在计划报告后的后续消息中明确确认实施 `MSG-000`、`MSG-EXE-001` 与 `MSG-001..MSG-011`，并授权
在本机启动、清理一个隔离的 RabbitMQ 4.3 测试容器执行 `MSG-010`；
不包含 push、部署、远端 Broker 写入或生产 topology 修改授权。

## 2. 研究与计划证据

| ID | 工作量 | 状态 | 完成条件 | 证据 |
| --- | --- | --- | --- | --- |
| MSG-R01 | M | 完成 | 追踪 Module/Generation/Execution/Schedule/Ops，并证明 ClaimTTL/失败 claim 缺口 | `research/R001-*/metadata.yaml`、`report.md` |
| MSG-R02 | M | 完成 | 以官方主源比较 Provider，并固定 RabbitMQ 4.3 delayed retry 语义 | `research/R002-*/metadata.yaml`、`report.md` |
| MSG-R03 | M | 完成 | 形成公共语义、Provider、可靠性和代际单轨结论 | `research/R003-*/metadata.yaml`、`report.md` |
| MSG-P01 | M | 完成 | 需求、设计、任务互相引用，范围/非目标/验收完整 | `requirements.md`、`design.md`、本文件 |

## 3. 实施任务

### MSG-000 固定前置基线

- 工作量：S
- 依赖：用户确认
- 状态：完成
- 动作：复查 Git、HEAD、037 状态和验证证据；确认 `00083be` 或其等价后继仍是可追溯基线，且不把
  `origin/main` 的状态作为未经 fetch 的当前事实。
- 完成条件：无 merge/rebase/conflict；038 可以精确识别自己的文件与 Diff；若 037 基线被重写、
  出现共享文件冲突或无法隔离，停止实施并报告。

### MSG-EXE-001 补齐 Execution claim 生命周期

- 工作量：L
- 依赖：MSG-000
- 状态：完成
- 动作：把 `Execution.ClaimTTL` 单轨替换为 running `LeaseTTL` 与 completed `RetentionTTL`；扩展 Store
  `Complete(retention)` 与同步 `Release`，迁移 Memory/Recovery/Async wrapper、Schedule 等全部调用方及主题文档。
- 完成条件：失败 release 后相同 key 可重试；成功 retention 从完成时起算；进程消失后 lease 到期可重试；
  release/record 多错误完整保留；无 `ClaimTTL` alias/旧调用/兼容分支，当前调度语义与测试保持明确。

### MSG-001 建立 `pkg/messaging` 公共契约

- 工作量：L
- 依赖：MSG-000
- 状态：完成
- 动作：实现 Contract/Message/Producer/Consumer Binding、Publisher/Receipt、策略、诊断、错误与不可变构造；中文 Go Doc。
- 完成条件：完整校验、typed nil/拷贝/并发安全/错误链测试；包不导入第三方中间件、Kernel 或业务模块。

### MSG-002 扩展模块显式贡献

- 工作量：M
- 依赖：MSG-001
- 状态：完成
- 动作：扩展 `module.Contribution.Messages` 与聚合/冲突验证；更新模块边界与架构测试。
- 完成条件：两个 fixture module 的 Contract/Producer/Consumer 聚合、排序、冲突/未知引用均有测试；无扫描/Registry。

### MSG-003 建立 Messaging Kernel App 与 Provider SPI

- 工作量：L
- 依赖：MSG-001、MSG-002
- 状态：完成
- 动作：实现强类型配置、Configured Leased Component、Output(Publisher/Control)、显式 Factory、Route/capability 校验、candidate freeze 与 fake Provider。
- 完成条件：默认 disabled 零外部资源；两个命名 fake Provider 并存；Control 只 freeze 一次；构造期 publish/consume 被拒绝；资源 owner/终结测试通过。

### MSG-004 实现 RabbitMQ Provider

- 工作量：XL
- 依赖：MSG-003
- 状态：完成
- 动作：复核并固定官方 `amqp091-go` 版本；实现连接/channel、confirm/return、manual ack、prefetch、topology probe、错误转换、TLS/Secret 边界和安全关闭。
- 完成条件：AMQP 类型零泄漏；confirm 表与 goroutine 有界；context/timeout/Close/恢复通知测试通过；许可证与依赖审计记录完成。

### MSG-005 完成可靠 Publisher

- 工作量：L
- 依赖：MSG-003、MSG-004
- 状态：完成
- 动作：实现 Producer/Contract/Route 校验、persistent+mandatory publish、confirm Receipt、unroutable/ambiguous/timeout/retire 错误与 Trace headers。
- 完成条件：未 confirm 永不返回成功；无内存成功 fallback；并发与断连下无未决泄漏；日志低敏。

### MSG-006 完成 Consumer 与 Execution 协作

- 工作量：XL
- 依赖：MSG-EXE-001、MSG-003、MSG-004
- 状态：完成
- 动作：实现 manual ack dispatcher、bounded prefetch/concurrency、Contract decode Adapter、Telemetry、Execution 单次策略；
  retryable business failure 走 counted reject，Execution/active lease 阻塞走 non-counting nack，terminal 走 DLX，并隔离 panic。
- 完成条件：success/duplicate/retry/backend/active lease/permanent/invalid/panic/cancel/ack ambiguity 映射测试；
  Broker 重投与 Execution 不形成隐式 N×M retry；Execution unavailable 暂停 intake；memory 幂等边界有文档和诊断。

### MSG-007 接入 Application Generation 与 ConsumerHub

- 工作量：XL
- 依赖：MSG-002、MSG-003、MSG-005、MSG-006
- 状态：完成
- 动作：在 composition 按 Provider -> module -> freeze 顺序装配；实现 process-stable ConsumerHub、Prepare/Commit/handoff/Retire/Abort/Stop/ForceStop；更新资源池和 failure monitor。
- 完成条件：Commit 前零消费；旧代 quiesce 后新代激活；旧 HTTP 请求可在 drain 前发布；候选失败保留旧代；部分 Commit 可回滚。

### MSG-008 完成暂时故障恢复与健康语义

- 工作量：L
- 依赖：MSG-004、MSG-007
- 状态：完成
- 动作：实现 Connecting/Ready/Recovering/Degraded/Failed/Draining 状态、有界探测、required/optional 聚合与自动重建 Publisher/Consumer。
- 完成条件：Broker down 不使进程 terminal；required readiness fail、optional warn；Broker up 自动恢复；确定性错误不无限重试；正常关停无 Error 日志。

### MSG-009 同步配置、Ops、主题文档和治理门禁

- 工作量：L
- 依赖：MSG-001..MSG-008
- 状态：完成
- 动作：更新 config defaults/example、Bootstrap bindings、Capabilities、Ops model/snapshot、metrics/diagnostics、
  Execution/messaging README、development/operations 文档与 boundary tests。
- 完成条件：五项能力入口清晰；配置生成/严格校验一致；第三方泄漏、隐式注册、物理 destination/Secret/payload 日志由测试守护；任务文档不成为第二套 authority。

### MSG-010 完成协议集成与工程验证

- 工作量：XL
- 依赖：MSG-001..MSG-009
- 状态：部分完成（RabbitMQ 4.3 实机门禁受环境阻断）
- 动作：使用隔离 RabbitMQ 4.3 测试容器验证 confirm/unroutable、manual ack、counted delayed reject、
  non-counting delayed nack、delivery-limit/at-least-once DLX、断连恢复、generation handoff；运行全量工程门禁并清理测试容器。
- 完成条件：
  - `gofmt` 无差异；
  - `go test ./...`；
  - `go test -race ./...`；
  - `go vet ./...`；
  - `scripts/Verify-Quality.ps1`（及适用的 artifact/跨平台检查）；
  - RabbitMQ 集成门禁真实通过并记录 Broker/Client 版本；
  - `git diff --check`；
  - 完整 Diff、依赖、敏感信息和旧符号残留审阅通过。

### MSG-011 收口文档、任务证据与提交

- 工作量：M
- 依赖：MSG-010
- 状态：完成（由本轮提交收口）
- 动作：把 requirements/design/tasks 与当前主题文档同步为真实实现；记录逐轮证据、剩余限制和 Commit；按 Conventional Commit skill 只暂存 038 文件并提交。
- 完成条件：所有 AC 有直接证据；没有把未执行 Docker/Linux/远端检查写成通过；提交信息建议
  `feat(messaging): add governed RabbitMQ adapter capability`；提交后复查工作区。默认不 push。

## 4. 依赖顺序

```text
MSG-000
  -> MSG-EXE-001 -----------------------> MSG-006
  -> MSG-001 -> MSG-002 -> MSG-003 -> MSG-004
                                   -> MSG-005
                                   -> MSG-006
  -> MSG-007 -> MSG-008 -> MSG-009 -> MSG-010 -> MSG-011
```

`MSG-004..008` 可在私有实现细节上交错，但每次提交前必须保持可构建；公共语义、依赖、模块边界或可靠性变化触发重新确认。

## 5. 计划风险

| 风险 | 处理 |
| --- | --- |
| 037 基线漂移或与 038 共享文件冲突 | MSG-000 fail-closed；重新研究受影响边界，不混合提交 |
| confirm 丢失导致发布结果歧义 | 返回 typed ambiguous error，不自动重发或虚假成功 |
| Execution 失败 claim 不释放或完成窗口从 Claim 起算 | MSG-EXE-001 单轨拆分 lease/retention/release；不在 Consumer 另建幂等状态 |
| Broker retry 与 Execution retry 相乘 | Consumer 强制单次 Execution；Broker 是跨 delivery retry owner |
| `basic.nack` 不增加 4.3 delivery count 而形成无限业务重试 | 业务失败只用 counted reject；nack 仅用于基础设施延后；两者要求 delayed-retry Policy |
| DLX 被配置但不具备无损保证 | 要求 quorum + delivery-limit + Policy + 真实集成验收，诊断标明能力状态 |
| Candidate 提前消费 | Consumer 构造/Freeze 与 Hub Activate 分离，Commit 前零 delivery 测试 |
| Reload 时旧请求与 Consumer 生命周期冲突 | Publisher generation-local 保留到 HTTP drain；Consumer 单独 Hub handoff |
| 自动恢复成为无限忙重试 | 每次有 timeout、退避/抖动/最大频率，resource context 可停止，确定性错误 fail-fast |
| 公共契约抹平 Kafka/NATS 差异 | capability requirement fail-closed，driver union 保留专属配置，不宣称未实现 Provider |
| Outbox/Exactly-once 范围膨胀 | 明确非目标；真实事务用例出现后另建研究和变更 |

## 6. 实施与验证证据

| 日期 | 范围 | 证据 | 结论 |
| --- | --- | --- | --- |
| 2026-08-20 | MSG-EXE-001 | `LeaseTTL`/`RetentionTTL`/`Store.Release` 单轨迁移；Execution/Schedule 与全量测试 | 完成；Go 源码中 `ClaimTTL` 引用为 0 |
| 2026-08-20 | MSG-001..003 | `pkg/messaging`、模块 Catalog、Configured Leased Component、fake Provider 与 immutable/冲突/准入测试 | 完成 |
| 2026-08-20 | MSG-004..008 | `amqp091-go` v1.14.0；confirm/return、manual ack、counted reject、uncounted nack、DLX、passive topology、TLS、恢复、Execution health 与 Hub rollback 单元测试 | 完成；新依赖许可证为 BSD-2-Clause |
| 2026-08-20 | MSG-009 | Bootstrap/one-shot 配置、Application Generation、Ops diagnostics/readiness、当前 development/operations/pkg/kernel/module 文档与 AMQP import 架构门禁 | 完成 |
| 2026-08-20 | 工程门禁 | `scripts/Verify-Quality.ps1`：gofmt、tidy diff、generate/生成物、`go test ./...`、`go test -race ./...`、vet、CGO-free build、artifact 全通过；346 份 Markdown 的 712 个本地链接有效；`git diff --check` 通过 | 完成 |
| 2026-08-20 | 安全与依赖 | 消息范围 `gosec` 0 issue、`gitleaks` 0 leak、消息范围 `govulncheck` 无漏洞；全仓 `govulncheck` 命中既有 `kin-openapi` v0.142.0 的 GO-2026-6112，HEAD 已使用同版本 | 新增范围通过；既有漏洞不在 038 越权升级 |
| 2026-08-20 | RabbitMQ 协议 | integration build-tag 编译通过；无 `RABBITMQ_MESSAGING_URI` 时测试明确 skip；PowerShell verifier 语法通过 | 未做真实 Broker 验收 |
| 2026-08-20 | 环境阻断 | `docker` command not found；WSL 无可运行发行版 | `scripts/verify-messaging-rabbitmq.ps1` 未执行，未创建或清理容器 |

仍待环境恢复后执行：

```powershell
./scripts/verify-messaging-rabbitmq.ps1
```

只有该命令在真实 RabbitMQ 4.3 容器上成功，`MSG-010` 才能由“部分完成”改为“完成”，并记录实际 Broker 版本。

## 7. 已确认决策

待用户确认以下计划选择：

1. 首个生产 Provider 为 RabbitMQ，采用实施时复核的官方 `amqp091-go` v1.14.0；
2. 首版只做 at-least-once + broker confirm/ack/redelivery/DLX，不做 Outbox/Inbox/exactly-once；
3. required/optional 只影响 readiness/degraded，发布失败始终显式返回；
4. `MSG-EXE-001` 删除 `ClaimTTL`，单轨替换为 `LeaseTTL`、`RetentionTTL`、`Store.Release` 并迁移现有调用方；
5. Consumer 的跨 delivery retry 由 Broker 拥有：RabbitMQ 4.3 reliable Route 必须启用 quorum delayed retry；
   retryable business failure 使用 counted reject，基础设施暂时阻塞使用 non-counting nack，Execution 单次业务尝试；
6. 允许 `MSG-010` 启动并清理本地隔离 RabbitMQ 4.3 测试容器；
7. 实施完成后创建本地 Conventional Commit，不 push。
