# 需求：成熟 Go HTTP API 脚手架缺口治理

## 1. 目标

在不破坏当前显式依赖、Kernel 资源治理和应用模块边界的前提下，定义一个新生成或复制的 Go HTTP API 服务必须继承的成熟度基线，并把能力拆成有依赖顺序、可独立确认的后续变更。

本轮只交付差距评估与路线，不修改源码、配置、依赖、测试、CI 或运行状态。事实依据为 [R001](research/R001-current-http-api-baseline/report.md)，外部语义基线为 [R002](research/R002-http-api-maturity-reference/report.md)。

## 2. 成熟度原则

- `MAT-001`：成熟度以调用方和运维方获得的可验证保证衡量，不以包数量或 middleware 数量衡量。
- `MAT-002`：每项能力必须有唯一语义 owner、公开契约、配置、失败、生命周期、诊断和验收；仅存在工具函数不算完成。
- `MAT-003`：默认基线与场景化能力分离。所有 HTTP 服务必需的协议、安全、运营和交付保证进入 baseline；消息、租户、搜索等由真实需求触发。
- `MAT-004`：第三方类型继续留在项目 Adapter 后面；业务 Service 不导入 chi、GORM、OpenTelemetry SDK、JWT SDK 或部署平台类型。
- `MAT-005`：不能同时维护多个权威来源；API、权限、健康、迁移和发布规则都必须有单轨事实源与漂移门禁。

## 3. P0 必须先设计的能力

### 3.1 脚手架产品与兼容边界

- `FORM-001`：明确项目是可复制 template、代码 generator、可依赖 library 还是组合形态，并定义每种产物的所有权。
- `FORM-002`：定义应用名、Go module path、配置前缀、端口、示例模块和品牌字段如何生成或替换，禁止靠全仓任意字符串替换。
- `FORM-003`：定义版本、发布、上游升级、破坏性变化、迁移指南和生成产物再生策略。

### 3.2 API Contract 与兼容性

- `API-001`：每个 HTTP operation 必须拥有稳定 ID、method、path、summary/tags、输入、成功响应、错误、security、版本和弃用元数据。
- `API-002`：选择且只选择一个 API 权威来源；OpenAPI 文档、Router、测试、权限矩阵和客户端不得独立手写相同事实。
- `API-003`：建立 contract lint、生成/校验、breaking diff 和运行路由一致性门禁。
- `API-004`：公开 DTO 与领域 Model 分离；生成代码只能停在协议边界，不渗入 Service/Model。
- `API-005`：定义版本策略、兼容变化、弃用窗口和删除条件，不把 `/v1` 当作完整版本治理。

### 3.3 请求、响应与错误协议

- `PROTO-001`：JSON 解码必须定义空 Body、unknown field、trailing value、最大 Body、Content-Type、Accept 和 validation details。
- `PROTO-002`：响应必须定义提交点、编码失败、streaming、HEAD/204、Content-Type 与缓存 Header 的 owner。
- `PROTO-003`：建立统一 problem contract，包含稳定机器类型/代码、HTTP status、可本地化人类信息、发生实例与 request/trace ID；不得泄露内部错误。
- `PROTO-004`：404、405、panic、decode、validation、authn/authz、rate limit 和业务 fault 使用同一错误协议与 media type。
- `PROTO-005`：定义 pagination/filter/sort、时间、ID、金额、枚举、null/empty 和批量操作的公共表示。
- `PROTO-006`：为重试敏感写操作定义 Idempotency-Key、ETag/If-Match 或显式业务命令策略，不能仅依赖客户端不重试。

### 3.4 身份、安全与入口政策

- `SEC-001`：定义项目自有 `Principal` 与认证结果，不让业务代码解析 Token 或依赖身份 SDK。
- `SEC-002`：operation 显式标记 public、authenticated 或 PolicyRef；默认不允许未声明访问策略的路由进入 production。
- `SEC-003`：授权在真实资源/对象边界执行，支持对象级与字段级策略；middleware 预检查不能替代 Service 授权。
- `SEC-004`：定义认证失败、授权拒绝、凭据过期、challenge 和 audit event 的稳定语义。
- `SEC-005`：设计可信代理、client IP、Forwarded Header、TLS termination、CORS、CSRF/cookie（如适用）和安全 Header 边界。
- `SEC-006`：入口必须有 route/principal/IP 维度的 request budget、Body/Header/Query 限制、并发/速率控制和 overload 语义。
- `SEC-007`：Secret source、rotation、日志/trace/metric 脱敏和诊断权限形成一套生产接入设计。

### 3.5 管理面、健康与可观测性

- `OPS-001`：建立与业务 API 分离或明确隔离的 management plane，承载 startup/liveness/readiness、metrics、diagnostics 和 build info。
- `OPS-002`：组件与应用模块可以向唯一健康 registry 贡献检查；每个检查声明 Kind、timeout、敏感输出和 readiness 影响。
- `OPS-003`：停止时先撤销 readiness，再停止接流量和排空请求；degraded/restart-required 必须可由运维系统观察。
- `OBS-001`：定义项目 tracing/metrics contracts 和 OpenTelemetry Adapter，覆盖入站、出站、Database/Cache 与后台任务传播。
- `OBS-002`：访问日志至少使用低基数 route、method、status、duration、bytes、request ID、trace ID 和受控 client metadata。
- `OBS-003`：定义 metric names/attributes、cardinality、sampling、retention、SLO 与告警的 owner；不得把原始 path、用户 ID 或错误消息直接作为 label。

### 3.6 数据演进与发布协议

- `DATA-001`：生产迁移必须版本化、可审计、有锁和明确执行 owner；应用启动、独立 job 和运维命令只能选择一种默认路径。
- `DATA-002`：定义 expand-contract、backfill、前滚恢复、兼容窗口、多副本和最小权限策略。
- `DATA-003`：Schema、应用版本和 migration 状态必须可诊断；未满足兼容条件时拒绝 ready，而不是边服务边猜测。
- `DATA-004`：跨 Database/消息/搜索的一致性需由 outbox、幂等或补偿等真实契约处理，不由 Kernel reload 推断。

## 4. P1 成熟交付能力

- `OUT-001`：出站 HTTP/SDK capability 统一管理 Transport、DNS/连接池、timeout budget、重试、Retry-After、breaker、bulkhead、遥测和关闭。
- `TEST-001`：建立 API contract、handler/component/integration/process 四级测试边界与 fixtures，不让每个模块重复启动整套环境。
- `TEST-002`：对 JSON、header、语言、配置和协议 parser 增加 fuzz；为 HTTP/DB 热点建立 benchmark、负载和泄漏预算。
- `SUPPLY-001`：CI 增加 `govulncheck`、依赖/secret/license 检查、产物扫描和明确的失败处置。
- `DELIVERY-001`：提供可重复 build、版本/Commit/build-time 注入、最小容器、非 root、只读文件系统、signal/probe 和 rollback 示例。
- `DELIVERY-002`：提供本地开发依赖、配置/Secret 注入、migration、smoke test 和故障诊断 Runbook。
- `DX-001`：新项目/模块生成、命名、最小示例、验证和删除旧示例形成一条可测试工作流。

## 5. P2 场景化能力

- `ASYNC-001`：消息、后台任务和调度必须先定义交付、幂等、背压、dead-letter、leader/lock 和排空。
- `TENANT-001`：多租户必须先定义身份归属、数据隔离、迁移、限额、审计和运维模型。
- `INTEGRATION-001`：邮件、搜索、特性开关、webhook 等按真实用例选择 Adapter 与一致性边界。

## 6. 非目标

- 019 不恢复 018 插件 Runtime、Catalog/Profile 或动态加载方向。
- 本轮不决定 spec-first 还是 code-first；该选择必须由后续 ADR 使用最小原型验证。
- 本轮不选择 JWT/OIDC、RBAC/ABAC、Prometheus/OTLP、migration、container 或 deployment 产品。
- 不要求所有项目启用多租户、消息、缓存、搜索或 Kubernetes。
- 不用占位 package、空 middleware、假配置或 TODO 代表设计完成。

## 7. 本轮验收标准

- 018 在入口、需求、设计、任务和研究索引中明确标记为已废除，不能再被误认为待确认方案。
- 019 的研究能逐项定位当前代码证据，并区分已实现、局部能力、已识别未设计和未设计。
- 019 给出 baseline 与 scenario-driven 能力边界、依赖顺序、风险和未来任务 ID。
- 当前权威运行文档不被改写为目标能力已实现。
- 文档链接、YAML、Diff 和工作区范围验证通过；当前 Go 测试基线如实记录。
