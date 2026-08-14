# 设计：HTTP API 成熟度分层与演进路线

## 1. 设计结论

019 不设计一个“大而全框架类”，而是建立七个彼此有明确输入输出的治理平面：

```text
Product / Distribution
          |
          v
API Contract -------> Compatibility Gates
     |
     v
HTTP Runtime + Edge Policy
     |
     v
Application Modules -----> Data Evolution
     |
     +--------------------> Outbound / Async Adapters
     |
     v
Management + Observability
          |
          v
Delivery / Operations
```

现有 Kernel、Coordinator、Host、Supervisor 继续负责进程和资源生命周期；019 补的是 API 产品与交付语义，不建立第二容器，也不引入 018 的插件 Runtime。

## 2. 第一前置决策：脚手架究竟是什么

当前仓库同时像 library、template 和具体 Todo 应用，但没有声明消费者模型。推荐通过 ADR 在以下模型中单轨选择：

| 模型 | 优点 | 主要代价 |
| --- | --- | --- |
| Template repository | 最简单，生成后完全归使用者 | 上游升级只能人工合并，能力易漂移 |
| Generator + owned runtime modules | 可稳定生成项目并持续升级公共能力 | 需要模板 schema、golden、版本与迁移工具 |
| Importable framework library | 升级依赖直接，但公共 API 兼容压力最大 | 易把 application composition 和第三方边界暴露成框架 API |
| 组合模式 | runtime packages 版本化，generator 只生成 application glue | 初期建设成本最高，但最符合当前 `pkg/internal/module` 分工 |

推荐验证“组合模式”：稳定的项目自有 runtime/capability packages 加一个小型 generator/template；生成项目拥有业务模块和 composition。该建议不是已确认决策，必须先用第二个独立消费项目验证 import、rename、upgrade 和删除 Todo 示例的成本。

## 3. API Contract 平面

### 3.1 单一权威

后续 ADR 必须比较两条路线：

1. **spec-first**：OpenAPI 是权威，生成 transport DTO、server interface、client 和 contract tests；领域 Model/Service 手工实现。
2. **typed code-first**：项目自有 typed Operation 是权威，生成 OpenAPI、route、catalog 和政策矩阵。

不允许第三条“双手工同步”路线。选型至少验证：泛型/nullable/oneOf、错误响应、security、代码可读性、生成稳定性、breaking diff、IDE 体验和第三方 client。

### 3.2 Operation 需要表达的最小语义

下面是目标字段，不是已实现 Go API：

```text
Operation
  ID / Method / Path / Version / Deprecation
  Request(content types, params, body schema, limits)
  Responses(status, headers, body schema, problems)
  Security(public | authenticated | policy refs)
  Reliability(idempotency, timeout class, rate class)
  Observability(route name, audit category, sensitivity)
  Handler binding
```

Router 只消费编译后的 operation；OpenAPI、权限矩阵、API inventory、测试和访问日志都从同一 identity 派生。业务 Handler 不接收整个 Operation 或运行时 Registry。

### 3.3 兼容性门禁

每次 API 变更产生 machine diff，并按项目政策分类：

- compatible：新增可选字段、操作或响应；
- conditionally compatible：扩大 enum、改变默认值、增加新的错误，需要消费者政策；
- breaking：删除/重命名、必填变化、类型变化、status/security 变化；
- operationally breaking：timeout、rate、pagination、ordering 或数据一致性变化。

仅语法 OpenAPI diff 不足以发现 operational breaking，任务文档仍需记录行为契约。

## 4. Protocol 平面

### 4.1 请求管线

推荐固定顺序并由测试证明：

```text
trusted proxy normalization
  -> request/trace identity
  -> panic recovery
  -> route match and metadata
  -> request budget / overload
  -> CORS or CSRF boundary
  -> authentication
  -> coarse route policy
  -> media type and body limit
  -> strict decode + validation
  -> Handler / Service authorization
  -> response encode
  -> access log / metrics / trace completion
```

顺序必须根据“失败是否需要 request ID、是否计入 rate、是否允许预检、谁能看见 body”解释，不能只靠 `Router.Use` 调用顺序。

严格 JSON 解码至少拒绝 unknown field 和 trailing JSON，并区分空 Body、syntax/type、大小超限和 validation。大文件/streaming 使用独立接口，不能被通用 JSON buffering 强制覆盖。

### 4.2 统一错误

推荐以 RFC 9457 语义为目标，未来通过独立 breaking-change 任务替代当前 `{error,message}`：

```json
{
  "type": "https://project.example/problems/invalid-argument",
  "status": 400,
  "title": "Request is invalid",
  "detail": "One or more fields are invalid",
  "instance": "urn:request:<request-id>",
  "code": "todo_invalid_argument",
  "errors": [{"pointer": "/title", "rule": "maxLength"}]
}
```

`type/code/status` 是机器契约；`title/detail` 可以本地化但客户端不得解析；内部 cause 只进入受控日志/trace。404/405 和 middleware failure 也必须走同一 presenter。

### 4.3 分页、条件与幂等

- 小型管理列表可以继续 offset pagination；高写入或大数据列表需要稳定 cursor 和明确 sort key。
- 所有列表响应采用同一 metadata 形态，最大 limit 和默认排序由 operation 声明。
- 资源更新选择 Version/ETag + `If-Match`，或选择明确的业务 command；不能两套并存却语义不同。
- `Idempotency-Key` 只用于明确 operation，必须定义 scope、请求摘要、结果保存、并发占用、TTL、错误重放与清理 owner。

## 5. Security 平面

### 5.1 项目自有身份契约

目标形态：

```text
Credential Adapter -> Authentication Result -> Principal
Operation Policy + Resource facts -> Authorization Decision
Decision -> Audit Event
```

`Principal` 只包含项目稳定语义，例如 subject、issuer、authentication time、assurance、scopes/claims view；原始 Token 和第三方 claims 不进入业务层。Service 在加载真实对象后完成对象级授权，防止仅按 route 权限检查。

具体 JWT/OIDC/session/API key 只有真实 actor 与信任域出现后选择。public operation 必须显式声明，未声明策略默认编译或启动失败。

### 5.2 边缘信任与资源保护

必须在部署契约中明确：

- 哪一层终止 TLS，哪些 Forwarded Header 可相信；
- client IP 的推导与 spoofing 边界；
- CORS origin/credentials/preflight 策略；
- cookie 场景的 SameSite/CSRF；
- 每 route 的 Body、Header、Query、duration、concurrency 和 rate class；
- 本地、单实例和分布式限流分别能保证什么。

当前全局令牌桶不能升级为身份级或集群级限流的默认实现。

## 6. Management 与 Observability 平面

### 6.1 独立管理面

推荐使用单独的 management listener，默认绑定 loopback 或内部网络；业务 listener 不直接暴露 diagnostics/pprof。最小端点：

- `/startupz`：启动阶段是否完成；
- `/livez`：进程是否处于不可恢复状态；
- `/readyz`：是否应接收新流量；
- `/metrics`：标准指标 exporter；
- `/diagnostics`：脱敏 build/config generation/restart-required/degraded 摘要；
- 可选受保护 `pprof`。

如果实际部署只能单 listener，必须用独立 route group、认证/网络政策和文档说明替代，而不是无保护混入业务 API。

### 6.2 健康贡献

Host 当前私有 registry 应演进为 composition 拥有的唯一 registry。Kernel capability、module participant 和外部 adapter 贡献命名 Check；注册发生在 listener 启动前，重复 owner 失败。

- liveness 不把普通下游故障当作重启理由；
- readiness 汇总当前服务必需依赖和 degraded；
- startup 只在初始化/migration/runner ready 后成功；
- 检查执行有 timeout、并发上限和短缓存，响应不泄露 DSN 或内部拓扑。

### 6.3 遥测

项目自有窄契约定义 span/metric 所需语义，OpenTelemetry Adapter 位于技术边界。优先覆盖：

- inbound/outbound HTTP context propagation；
- stable operation ID 与低基数 route；
- status/duration/active/bytes/error class；
- Database/Cache/Storage 和 background work；
- request ID、trace ID 与结构化日志关联；
- telemetry export 失败不阻断业务，但必须有自诊断与有界队列。

## 7. Data Evolution 平面

当前 `database.Migrate` 保留为 schema 描述和本地开发能力，但 production migration 需要独立协议：

```text
versioned migration artifacts
  -> validate checksum and current version
  -> acquire migration ownership/lock
  -> apply bounded step
  -> record durable result
  -> release lock
  -> application compatibility/readiness check
```

推荐默认：本地 profile 可以显式 auto-migrate；production 由独立 `migrate` command/job 执行，服务账号默认只有运行所需 DML 权限。破坏性变更采用 expand -> dual-compatible code -> backfill -> contract，失败优先前滚修复。

## 8. Reliability、Testing 与 Delivery

### 8.1 出站调用

`httpx.Client`、`resilience` 和未来 SDK Adapter 应由 composition 组合为命名 dependency client，而不是业务代码各自 `NewClient`。每个 client 有 service identity、base URL、transport、timeout budget、retryable operation、breaker/bulkhead、telemetry 和 Close owner。

### 8.2 测试金字塔

```text
contract diff / generated artifact checks
        -> protocol Handler tests
        -> Service and Adapter contract tests
        -> real DB/process tests
        -> smoke/load/failure tests
```

新增 API 至少证明 success、decode/validation、authn/authz、not-found/conflict、取消/timeout、contract 同步。Parser 与边界输入进入 fuzz；性能优化必须先有 benchmark 和预算。

### 8.3 交付产物

成熟默认应包含：

- 可重复的 `go build` 与 version/commit/build-time 诊断；
- 最小、非 root、只读优先的容器及明确 writable paths；
- migration 和 app 分离运行方式；
- probe、termination grace period、resource limit 和配置/Secret 示例；
- test/race/vet/tidy/build、contract diff、fuzz smoke、govulncheck 和产物扫描；
- release notes、兼容声明、SBOM/签名和 rollback Runbook。

这些资产必须是可选 deployment adapter，不让业务包依赖 Kubernetes 或某个云。

## 9. 实施依赖顺序

```text
Product form ADR
  -> API authority ADR + prototype
  -> Operation/Error/Protocol contract
  -> Security policy + Management/Telemetry
  -> Migration protocol + delivery baseline
  -> generator/upgrade workflow
  -> scenario-driven capabilities
```

如果先实现 auth middleware、Swagger 注解或 metrics handler，再决定 operation authority，会导致多份元数据和重复迁移。因此第一批后续任务只应研究 `FORM-001` 与 `API-001/API-002`，不直接大规模改造运行链。

## 10. 风险与控制

| 风险 | 后果 | 控制 |
| --- | --- | --- |
| 把成熟度等同包数量 | 产生孤立工具，无 production guarantee | 每项能力强制 owner/config/failure/lifecycle/acceptance |
| OpenAPI 与 Router 双权威 | 文档、权限和运行行为漂移 | 单一 authority + generated/verified artifacts |
| 通用 auth 早于真实 actor | 巨型 Principal、错误安全默认 | 先 Policy/Principal 边界，Adapter 等真实用例 |
| 管理端点混入公网 API | 泄露诊断和扩大攻击面 | 独立 listener 或受保护 group |
| 自动 migration 用于生产 | 多副本竞争和不可逆失败 | versioned job、lock、expand-contract |
| 观测标签高基数/含敏感值 | 成本与泄露 | route ID、attribute allowlist、redaction tests |
| generator 与 runtime 紧耦合 | 升级困难、生成物不可维护 | 先消费模型 ADR，再冻结模板 schema |
| 一次实施全部 P0 | 审查面过大且公共协议易返工 | 每个子域独立研究、原型、确认和单轨迁移 |

## 11. 未决项

1. template/library/generator/组合模式的最终产品形态；
2. OpenAPI spec-first 或 typed code-first；
3. 当前 Todo 错误协议的 breaking migration 策略；
4. management listener 与业务 listener 的部署约束；
5. 第一种真实认证 actor、credential 和资源授权用例；
6. production migration 工具和 artifact 格式；
7. 支持的平台、容器和发布渠道。

这些未决项不阻塞完成 019 差距评估，但阻塞对应源码实施。
