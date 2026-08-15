# 需求：成熟 Go HTTP API 脚手架验收门禁

## 1. 目标

定义 `go-scaffold-template` 何时可以诚实地声明为成熟 Go Server HTTP API 后端脚手架，并把从当前高级基础模板到成熟 baseline 的差距转换为稳定、可验证、可分批确认的要求。

当前事实依据为 [022-R001](research/R001-current-readiness-reassessment/report.md)，外部语义依据为 [019-R002](../019-http-api-maturity-gap-assessment/research/R002-http-api-maturity-reference/report.md)。本轮只交付文档，不修改实现或运行状态。

## 2. 标签定义

- **Foundation-ready**：进程、配置、资源、模块和基本 HTTP slice 可运行、可测试。当前已达到。
- **Copy-ready**：固定 release 可在支持平台完整复制、迁移 identity、保留/移除示例并独立演进。当前仅 Windows 隔离验证部分达到。
- **Production HTTP API-ready**：协议、安全、管理、遥测、数据演进、交付和兼容保证均有默认实现或明确 fail-closed 接入点，并由复制副本和部署验收证明。当前未达到。
- **Scenario-ready**：认证产品、消息、租户、搜索等特定能力由真实业务验收。它不等同 baseline，也不能用占位实现提前满足。

项目只有同时满足 Copy-ready 与 Production HTTP API-ready，才允许在当前权威文档和 release 说明中使用“成熟 HTTP API 后端脚手架”标签。

## 3. 必需 baseline gate

### 3.1 产品、版本与可移植性

- `BASE-001`：copy-owned 是唯一当前消费模型；正式复制指南明确 tracked baseline、identity 映射、Todo 保留/删除和独立 Git 历史。
- `BASE-002`：发布固定 tag/version、provenance、支持平台、兼容政策、安全公告和人工迁移说明；不得承诺自动上游升级。
- `BASE-003`：Windows 与 Linux 对 module、行尾、build、test、race、vet、config init 和 identity 扫描给出同一成功语义。
- `BASE-004`：仓库和复制副本不携带凭据、运行数据、本机缓存、`.git` 或未声明 writable path。

### 3.2 API authority 与兼容

- `API-001`：只选择 spec-first 或 typed code-first 一条 authority；Router、OpenAPI、权限、inventory 和 contract tests 从同一 operation identity 派生。
- `API-002`：Operation 表达稳定 ID、method/path、版本/弃用、request/response schema、错误、安全、reliability 和 observability metadata。
- `API-003`：建立 lint、生成或一致性校验、breaking/operational diff 与删除门禁；生成 DTO 不进入领域 Model/Service。
- `API-004`：至少用 Todo 全部 operation 和一个独立复制副本证明 contract 与运行路由无漂移。

### 3.3 HTTP 协议与 edge policy

- `PROTO-001`：严格定义 Content-Type/Accept、空 Body、unknown field、trailing value、Body 上限、validation details、HEAD/204 和 response commit。
- `PROTO-002`：统一 machine-readable problem contract；业务错误、validation、panic、404、405 和 middleware failure 使用同一 presenter，内部 cause 不泄露。
- `PROTO-003`：定义分页、排序、过滤、条件更新、幂等和 retry 的适用边界；不要求所有 operation 启用，但启用时只有一套语义。
- `EDGE-001`：trusted proxy、client IP、CORS/CSRF、request budget、header/body limits、rate/overload class 有明确 owner、配置、安全默认和诊断。

### 3.4 身份、安全与审计

- `SEC-001`：每个 operation 显式声明 public 或受保护政策；遗漏声明必须在生成、构建或启动前失败。
- `SEC-002`：项目自有 Principal/Policy/Decision/Audit 边界不泄露第三方 Token/claims 类型；对象级授权在真实资源事实可用后执行。
- `SEC-003`：具体 credential Adapter 只随真实 actor 选择；启用受保护 operation 时缺少 Adapter 必须 fail closed。
- `SEC-004`：敏感字段、日志、trace、错误和 diagnostics 具有统一 redaction 规则和负向测试。

### 3.5 管理面与可观测性

- `MGMT-001`：startup/liveness/readiness 有不同语义、依赖贡献、timeout/cache/concurrency 限制和停止时序。
- `MGMT-002`：management listener 默认只绑定受控地址；probes、metrics、build info、diagnostics 和可选 pprof 不直接混入公网业务路由。
- `OBS-001`：入站、出站和后台工作传播 trace context，日志关联 request/trace ID，指标使用稳定 operation/route 低基数 identity。
- `OBS-002`：exporter failure 不阻断业务，但有有界队列、丢弃策略、自诊断和敏感属性 allowlist。

### 3.6 数据演进与交付

- `DATA-001`：production migration 使用 version、checksum、owner/lock、bounded step、durable result 与独立 command/job；服务启动不静默执行不可逆变更。
- `DATA-002`：定义 schema/app compatibility、expand/backfill/contract、失败前滚和最小数据库权限；本地 auto-migrate 必须是显式 profile。
- `DELIVERY-001`：提供可重复 build metadata、最小非 root 容器、只读文件系统边界、probe/termination/resource 示例和 smoke 验收。
- `DELIVERY-002`：CI 覆盖 test/race/vet/tidy/build、contract diff、fuzz smoke、`govulncheck`、secret/artifact scan；失败政策不能静默降级。
- `DELIVERY-003`：release 产出 checksum、SBOM/签名、release notes、兼容声明和 rollback Runbook，并在隔离环境验证。

## 4. 最终验收

- `ACCEPT-001`：当前仓库完整门禁通过，且工作树、module 和文档无漂移。
- `ACCEPT-002`：至少两个从同一正式 release 复制的独立工作区通过门禁：一个保留 Todo，另一个移除 Todo 并装配独立最小业务模块。
- `ACCEPT-003`：Windows 与 Linux CI 均通过；容器 smoke 验证 startup、ready、正常请求、无效请求、依赖失败、SIGTERM 排空和 migration failure。
- `ACCEPT-004`：API compatible 与 breaking 两类演进样例都经过 contract diff；breaking 样例必须被默认门禁拒绝。
- `ACCEPT-005`：安全和观测负向场景证明未分类 route、未授权资源、超限请求、panic、敏感错误与高基数属性不会绕过政策。

## 5. 非目标

- 不恢复 018 插件 Runtime，不引入运行时万能 Resolver 或第二套生命周期容器。
- 不开发 generator；020 的 copy-owned 决策保持有效。
- 不在没有真实 actor 时选择 JWT/OIDC/session/RBAC 产品。
- 不预装消息、调度、分布式锁、邮件、搜索、租户或特性开关。
- 不要求所有消费者采用 Kubernetes、某个云、某个 APM 或某个 API gateway。

## 6. 本轮验收标准

- 研究事实与目标设计分离，结论可由当前代码和验证复核。
- 需求覆盖成熟标签的硬门禁与场景边界。
- 设计给出单轨依赖顺序，不把目标 API 写成已实现接口。
- 任务使用稳定 ID，明确每个后续非文档任务必须重新研究、计划和确认。
- 文档链接、metadata YAML、Markdown 结构和 `git diff --check` 通过。
