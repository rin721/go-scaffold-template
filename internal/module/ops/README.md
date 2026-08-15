# Ops 模块

`internal/module/ops` 当前同时实现 management 用例与 observability 技术装配。它拥有 startup/liveness/readiness 语义、脱敏 diagnostics/build 用例和 management HTTP binding；当前源码还包含 Prometheus Adapter、OpenTelemetry trace Adapter、低基数标签策略和业务 HTTP middleware。

这段“当前实现”不是后续模块模板。Prometheus、OTel、OTLP exporter 和通用 HTTP observation 实际服务整个进程，且当前导出协议已经暴露具体 Adapter/第三方类型。[027 第三方封装与分轨装配](../../../docs/changes/027-business-module-third-party-isolation/README.md) 已把目标边界修订为：Ops 只消费项目自有 Observability 契约，具体技术经过 `pkg -> internal/kernel/app -> internal/kernel/composition` 底层装配。该源码迁移尚未获得确认，不能把目标描述成已实现。

当前进程组合根连接 Auth management scope、Kernel/Supervisor typed runtime snapshot、独立 business/management `ListenerHub` 与稳定 Prometheus registry。Ops module 不绑定物理端口，也不查询容器或其他模块内部实现；但 composition 仍直接持有 Prometheus 具体 Adapter，这是 027 待消除的边界偏差。

默认管理地址为 `127.0.0.1:9090`：

- `/startupz`：首次 generation commit 后通过；
- `/livez`：只表达进程仍能治理生命周期，不因普通下游故障触发重启；
- `/readyz`：汇总 generation admission、Auth verifier 与 Database ping；
- `/metrics`：按 `management.metricsAccess` 关闭、公开或保护；
- `/build`：只返回 version、commit、build time、Go version 与 dirty state；
- `/diagnostics`：始终需要 `management:read`，只返回 typed、脱敏状态；
- `/debug/pprof/*`：当前没有注册，默认和生产 Router 中均不存在。

Trace 只使用显式 operation inventory，不记录 raw path、subject、Todo ID、SQL、query 或 error text。OTLP/HTTP exporter 使用 generation-owned 有界队列；队列满或导出失败不会阻断业务，但会累计 drop、export 与最后 error type，并在 generation 停止时按预算 flush。
