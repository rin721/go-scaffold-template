# 035 后台任务能力装配（幂等 / 重试 / 执行记录）

## 状态

- 当前阶段：已完成。
- 研究门禁：已通过，证据为 `R001`、`R002`。
- 计划状态：已确认并实施完成（用户确认 035 方案，默认内存 backend + 组件开关）。
- 当前授权：授权本地实施、验证与聚焦提交；不授权 push、tag、Release、部署或外部写入。
- 外部副作用：无。不启动服务、不写数据库、不 push、不 tag、不 release。

## 阅读顺序

1. `research/R001-current-background-capability-inventory/report.md` —— 当前已有哪些可复用能力、缺口与事实证据。
2. `research/R002-idempotency-retry-record-semantics/report.md` —— 幂等/重试/执行记录的语义与选型决策。
3. `requirements.md` —— 目标、范围、约束、非目标与验收标准。
4. `design.md` —— 能力契约、组件形态、装配链、失败语义与验证方案。
5. `tasks.md` —— 稳定任务 ID、依赖、完成条件与确认状态。

## 目标一句话

为需要幂等 / 失败重试 / 执行记录的业务模块（如订单、支付、库存）提供由 `pkg -> internal/kernel/app -> internal/kernel/composition` 链治理的底层能力装配：业务模块只消费注入的稳定契约，不自行散写重试循环、幂等键或执行日志。
