# R002：成熟 HTTP API 脚手架参考模型

## 1. 研究范围与主源

本报告不比较 Go Web 框架，也不选择认证、APM、云或部署厂商。研究只提取成熟 HTTP API 项目需要显式决定的语义边界，使用以下主源：

- [RFC 9110 HTTP Semantics](https://www.rfc-editor.org/rfc/rfc9110.html)
- [RFC 9457 Problem Details for HTTP APIs](https://www.rfc-editor.org/rfc/rfc9457.html)
- [OpenAPI Specification 3.2.0](https://spec.openapis.org/oas/latest.html)
- [OpenTelemetry HTTP semantic conventions](https://opentelemetry.io/docs/specs/semconv/http/http-spans/)
- [Kubernetes liveness、readiness 与 startup probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
- [OWASP API Security Top 10 2023](https://owasp.org/API-Security/editions/2023/en/0x10-api-security-risks/)
- [Go Security](https://go.dev/doc/security/)

这些标准和官方资料定义问题空间与互操作语义，不自动决定本项目应采用 spec-first/code-first、JWT/OIDC、Prometheus/OTLP 或 Kubernetes。

## 2. 最小成熟度模型

### 2.1 契约可发现、可验证、可演进

OpenAPI 描述 operation、输入、输出、状态码、安全方案和 schema，使服务、客户端、文档与测试有机器可读契约。成熟项目必须选择一个权威来源，并以生成或 diff 门禁防止运行路由与文档漂移；不能同时手工维护 Router、Swagger 和权限清单。

版本成熟度也不等于统一添加 `/v1`：必须定义哪些变化兼容、弃用窗口、破坏性 diff、消费者沟通和删除条件。

### 2.2 HTTP 语义不能退化为 JSON RPC

RFC 9110 给出 method safety/idempotency、条件请求、缓存、内容协商和 status/header 语义。脚手架不必为每个项目启用缓存或 ETag，但必须让设计者明确：

- 哪些操作可以安全重试；
- 创建、替换、部分更新和删除使用何种 method/status；
- 并发写如何通过 version、ETag/If-Match 或业务命令处理；
- `Location`、`Retry-After`、缓存和条件 Header 由谁产生；
- proxy、client 和 server timeout 如何共同形成 request budget。

### 2.3 错误是公开协议，不是日志字符串

RFC 9457 提供 `application/problem+json`、稳定 problem `type`、status/title/detail/instance 和扩展字段，并明确不得泄露调试和敏感信息。项目可以选择其他格式，但必须同样定义机器字段、验证详情、关联 ID、本地化边界、404/405 和未知错误降级。

### 2.4 身份、授权与业务流保护必须进入操作契约

OWASP API Security Top 10 将对象级授权、认证、属性级授权、资源消耗、敏感业务流和 API inventory 等列为核心风险。仅有一个解析 JWT 的 middleware 不能证明安全：操作必须声明访问策略，Service 必须在真实资源边界执行授权，并有审计、限流和 inventory 证据。

脚手架可以暂不选择身份提供方，但不能没有 Principal、Policy、拒绝语义和 public route 的显式标记。

### 2.5 可观测性必须跨入站、业务和出站传播

OpenTelemetry HTTP conventions 强调使用低基数 route/template，而不是原始 URL path 作为 span/metric identity。成熟设计需要：

- trace context 的提取与注入；
- server/client span 与统一 resource attributes；
- request duration、active requests、response size、status 和 error metrics；
- 日志中的 request/trace correlation；
- sampling、敏感字段和高基数控制；
- SLO/告警由稳定指标语义建立，而不是只输出日志。

### 2.6 健康状态是外部控制协议

Kubernetes 将 startup、liveness 和 readiness 分为不同用途：启动是否完成、是否必须重启、是否接收流量。即使不部署 Kubernetes，这三个语义也适用于负载均衡器和进程管理器。

成熟脚手架必须定义依赖是否影响 readiness/liveness、检查超时与缓存、并发限制、停止时何时先撤销 readiness，以及管理端点如何隔离和保护。

### 2.7 安全门禁必须进入日常工具链

Go 官方建议用 `govulncheck` 分析实际可达的已知漏洞，并提供原生 fuzz 支持。成熟 CI 除 test/race/vet/build 外，还应定义漏洞扫描、输入 fuzz、依赖更新、秘密扫描、构建产物与发布证明；具体工具可以替换，但门禁结果和失败策略要稳定。

## 3. 脚手架与普通示例仓库的差别

一个普通示例证明“这个 Todo 能运行”；成熟脚手架还必须证明“复制或生成的新服务默认继承同样保证”。因此需要额外设计：

1. 产品形态：template、generator、library 或组合；
2. 可配置点与禁止修改区；
3. 新项目和新模块生成后的命名、module path、配置和 contract 校验；
4. 上游升级、破坏性变化、版本和迁移方式；
5. 默认 CI、开发环境、容器和运行手册；
6. 最小安全默认与场景化扩展边界。

如果没有这层，能力越多，fork 后漂移和升级成本越高。

## 4. 分层参考模型

| 平面 | 必须拥有的语义 | 不应承担 |
| --- | --- | --- |
| Product/Distribution | 创建、配置、升级、版本、兼容 | 业务运行期依赖查询 |
| API Contract | operation、schema、error、security、compatibility | 领域不变量和数据库类型 |
| HTTP Runtime | decode/encode、middleware、timeout、server lifecycle | 业务授权决定和持久化 |
| Application | use case、Principal/Policy 使用、事务与业务错误 | chi/GORM/Otel 具体类型 |
| Management | health、metrics、diagnostics、build info | 对公网暴露业务数据 |
| Data Evolution | migration、lock、backfill、schema compatibility | 启动时静默做不可逆变更 |
| Delivery/Governance | CI、artifact、scan、deploy、rollback | 以绿色构建替代运行验收 |

## 5. 适用边界与结论

适用于：定义 go-scaffold2 的最低 HTTP API 成熟基线和后续任务依赖顺序。

不适用于：直接实现所有安全/平台能力、要求 Kubernetes、或在没有用户模型时选择 JWT/OIDC/RBAC 产品。

研究门禁通过。主源足以证明 019 应以协议、运营、分发和治理平面补齐为目标，而不是继续扩张底层工具清单。
