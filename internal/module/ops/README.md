# Ops 模块

`internal/module/ops` 只拥有 management、startup/liveness/readiness、脱敏 diagnostics/build 用例和 management HTTP binding。它消费 `pkg/observability` 项目契约，不导入 Prometheus、OpenTelemetry、OTLP 或底层具体 Adapter。

Prometheus、OTel、OTLP exporter 和通用 HTTP observation 同时覆盖 Auth/Todo 业务 HTTP 与 Ops management/diagnostics，且由进程统一选择和治理，因此按 [027 第三方封装与分轨装配](../../../docs/changes/027-business-module-third-party-isolation/README.md) 进入 `pkg -> internal/kernel/app -> internal/kernel/composition` 底层链。Ops 的依赖和输出只包含项目类型、标准库 Handler 与 module contribution。

当前进程组合根连接 Auth management scope、Kernel/Supervisor typed runtime snapshot、独立 business/management `ListenerHub` 与 Observability Capabilities。Ops module 不绑定物理端口、不查询容器、不穿透其他模块或 Kernel App，也没有 registry/provider 的关闭权。

默认管理地址为 `127.0.0.1:9090`：

- `/startupz`：首次 generation commit 后通过；
- `/livez`：只表达进程仍能治理生命周期，不因普通下游故障触发重启；
- `/readyz`：汇总 generation admission、Auth verifier、Database ping 与当前 scheduler 任务策略状态；严格任务 `pause` / `fail` 会阻止 ready，`skip` / 显式 best-effort `local` 以 degraded 诊断呈现；
- `/metrics`：按 `management.metricsAccess` 关闭、公开或保护；
- `/build`：只返回 version、commit、build time、Go version 与 dirty state；
- `/diagnostics`：始终需要 `management:read`，只返回 typed、脱敏状态；
- `/debug/pprof/*`：当前没有注册，默认和生产 Router 中均不存在。

HTTP observation 只使用显式 operation inventory，不记录 raw path、subject、Todo ID、SQL、query 或 error text。具体 Prometheus Registry、OTel Provider/Exporter、有界队列和请求租约位于 `internal/kernel/app/observability`；Metrics identity 跨 Application Generation 稳定，Telemetry 在旧代请求排空后按预算 flush。scheduler 不新建管理端点或 Health Registry，只把 typed、低敏快照合并到现有 diagnostics/readiness。
