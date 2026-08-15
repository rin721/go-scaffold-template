# Ops 模块

`internal/module/ops` 是 management 与 observability 的唯一应用 owner。它拥有 startup/liveness/readiness 语义、脱敏 diagnostics/build 用例、management HTTP binding、Prometheus Adapter、OpenTelemetry trace Adapter、低基数标签策略和业务 HTTP middleware。

进程组合根只连接三类完成品：Auth 提供 management scope 校验，Kernel/Supervisor 提供 typed runtime snapshot，进程 factory 持有独立 business/management `ListenerHub` 与稳定 Prometheus registry。Ops module 不绑定物理端口，也不查询容器或其他模块内部实现。

默认管理地址为 `127.0.0.1:9090`：

- `/startupz`：首次 generation commit 后通过；
- `/livez`：只表达进程仍能治理生命周期，不因普通下游故障触发重启；
- `/readyz`：汇总 generation admission、Auth verifier 与 Database ping；
- `/metrics`：按 `management.metricsAccess` 关闭、公开或保护；
- `/build`：只返回 version、commit、build time、Go version 与 dirty state；
- `/diagnostics`：始终需要 `management:read`，只返回 typed、脱敏状态；
- `/debug/pprof/*`：当前没有注册，默认和生产 Router 中均不存在。

Trace 只使用显式 operation inventory，不记录 raw path、subject、Todo ID、SQL、query 或 error text。OTLP/HTTP exporter 使用 generation-owned 有界队列；队列满或导出失败不会阻断业务，但会累计 drop、export 与最后 error type，并在 generation 停止时按预算 flush。
