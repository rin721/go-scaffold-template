# Observability

本包只定义业务 HTTP 与管理面使用的项目自有观测契约。

- `Metrics` 只提供进程私有 registry 的 exposition Handler；
- `Telemetry` 只提供请求级 HTTP 包装与低敏 diagnostics；
- `Capabilities` 不包含 Prometheus Registry、OpenTelemetry Tracer/Provider、Exporter、Option、配置或关闭权。

Prometheus、OpenTelemetry 与 OTLP 的具体实现由 `internal/kernel/app/observability` 持有，并在底层 composition 中选择。业务模块和 application composition 不得自行构造第二套 registry/provider。
