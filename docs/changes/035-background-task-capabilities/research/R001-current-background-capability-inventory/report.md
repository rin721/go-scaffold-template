# R001 当前后台能力清单（幂等 / 重试 / 执行记录）

## 研究问题

针对"需要幂等、失败重试、执行记录的业务操作"这一需求，当前项目已有哪些可复用底层能力、缺口在哪、证据可否复核。

## 方法与证据定位

- 阅读 `pkg/README.md`、`pkg/*/README.md` 与对应 `*.go`。
- 阅读 `internal/kernel/app/README.md` 与 `internal/kernel/composition/composition.go`。
- 核对 Git 状态与本任务无关的最近提交（f6ac785 为 034 收敛装配）。

## 事实（可复核）

1. **能力路线声明**：`pkg/README.md` 第 45-46 行"暂缓路线"明确列出：
   `后台任务：幂等、失败重试和执行记录。` —— 这是当前刻意未实现、等待真实场景的能力，与 035 目标重合。

2. **重试引擎已存在（纯代码，无资源）**：`pkg/resilience` 提供：
   - `RetryPolicy{MaxAttempts, InitialWait, MaxWait, Retryable}`；
   - `Do(ctx, policy, op)` —— 指数退避重试，`MaxAttempts<=0` 只执行一次，不隐式无限重试；
   - `WithTimeout`、`NewBreaker/Reset`。
   该项目是进程内策略执行器，不持有 backend 资态。

3. **可重试错误分类已存在**：`pkg/fault` 的 `Retryable(err)` 与 `Wrap(..., retryable)` 可直接作为 `RetryPolicy.Retryable`。

4. **并发合并/受限执行已存在**：`pkg/concurrency` 的 `SingleFlight.Do` 与 `NewPool.Run`。

5. **幂等键来源**：`pkg/idgen.Generator` 生成 UUID；`pkg/clock.Clock` 提供时间。

6. **持久化后端**：`pkg/database` 提供项目自有 Schema/Repository/事务契约，可承载幂等键表与执行记录表；`internal/kernel/composition.Capabilities.Database` 是既有注入出口。

7. **可观测**：`pkg/observability` 提供 HTTP observation 与低敏 diagnostics；执行记录可采用它分发诊断，但数据语义（记录 schema、状态机）应由能力或模块拥有。

8. **装配模式**：`internal/kernel/app/*` 是"跨业务复用 + 进程统一选择"底层资源组件层；`composition.Compose` 把这些组件的 `Added.Output` 收集进 `Capabilities`，业务模块只消费注入的最小契约。i18n（`internal/kernel/app/i18n`）是"配置化 + Leased Swap"的模板。

9. **缺口**：当前没有任何 `pkg` 或 kernel/app 组件给出"先幂等判重 → 带策略执行（可重试 / 熔断 / 超时） → 记录执行结果（成功/失败/原因/耗时/TTL）"的组合操作执行契约。重试引擎是离散纯函数，业务模块若要用还需自行拼装，正是 AGENTS 禁止的"每个调用点散写重试循环"。

10. **业务模块**：仓库现有业务模块为 todo/auth/ops/migration，均不消费上述能力；订单/支付/库存是 035 目标语境中的示例模块，本任务不实现它们。

## 推断（与事实分离）

- 重试引擎本体已覆盖，**不应重建**，也不应把它整体搬进 kernel/app；它是纯代码，不满足"拥有资源且由进程统一选择"的组件必要条件（见 `internal/kernel/app/README.md` 第 5 行）。
- 真正缺失的是一个**操作执行装配层**：把幂等判重 + 重试/熔断/超时 + 执行记录组合为一个稳定、可注入的契约，正对应 `pkg/README` 的"后台任务能力"。
- 幂等键与执行记录需要持久化后端，因此应先评估后端是否满足"跨业务复用 + 进程统一选择"，再决定走纯 `pkg` 还是完整 `pkg -> kernel/app -> composition` 链（见 R002）。

## 对 035 的影响

- 不必再造重试/熔断/超时轮子；基于 `pkg/resilience + pkg/fault` 组合。
- 新能力核心 = 幂等存储 + 执行记录 + 带策略执行器，统一装配给业务模块。
- 是否进入 kernel/app 链取决于 R002 的双条件论证；若只满足复用、不满足进程统一选择，则停留 `pkg`，业务模块经 composition 显式接线即可。

## 局限与刷新触发器

- 本记录是 f6ac785 时的快照；真实订单/支付/库存模块落地或引入消息/任务调度时需重新复核。
- 不评估外部消息/任务库迁移，因为当前需求无跨进程调度。
