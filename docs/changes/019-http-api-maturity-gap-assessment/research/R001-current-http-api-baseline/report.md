# R001：当前 HTTP API 运行链与成熟度基线

## 1. 研究问题、方法与判定口径

本报告回答：当前项目有哪些已经由代码和测试证明的 HTTP API 能力，哪些只有局部函数或暂缓列表，哪些连 owner、契约、失败语义和验证方式都没有形成。

证据快照为 `28fbc7a9cfe01e4e7c45505217c15f4d56e711b3`。核验路径为：

```text
cmd/app
  -> internal/composition
  -> internal/module/todo
  -> pkg/httpx / fault / validation / database
  -> Kernel / Coordinator / Host / Supervisor
  -> config.example.yaml / GitHub Actions / repository root
```

同时复核 012、014、015、017 的当前有效研究，并于 2026-08-15 执行 `go test ./...`，结果通过。

本报告使用以下口径：

- **已实现**：有当前入口、行为、owner、失败语义和覆盖该主张的测试。
- **局部能力**：存在包或函数，但未进入 production composition，不能宣称应用已获得保证。
- **已识别未设计**：文档提到或明确暂缓，但没有当前权威契约、owner、配置、失败与验收。
- **未设计**：代码和当前文档都没有形成可实施、可验证的项目决定。

一个 README 中的能力名称、某个 middleware 函数或未来路线条目，都不等于设计完成。

## 2. 已实现且应保留的基础

### 2.1 进程、配置与资源生命周期

- `cmd/app` 区分 Bootstrap CLI、Application CLI 和长期 Service，避免 `config init` 提前创建资源。
- Coordinator 对一个不可变配置候选完成严格绑定；Kernel 管理候选构建、排空、提交、回滚和 degraded。
- Supervisor 管理 Participant、runner readiness、异常退出、信号和有界反向停止。
- HTTP Server 在 ready 前同步绑定 listener，长期 `Serve` 受监督，停止时 Shutdown、Close、Wait 并聚合错误。

这些是成熟服务的重要底座，不需要为 019 建立第二个生命周期框架。

### 2.2 HTTP 与业务垂直切片

- `pkg/httpx` 隔离 chi，提供项目 `Router/Context/Handler/Middleware/Server/Client` 契约。
- Service 真实安装 `Recovery -> RequestID -> AccessLog -> SecureHeaders`。
- Todo HTTP 与 CLI 复用同一 Service、Database、Clock、ID 和 I18n；进程测试证明跨入口读写同一 SQLite 数据。
- Todo route middleware 对创建请求执行 Content-Type 校验，统一错误处理输出 JSON。
- Server 已有 header/read/write/idle timeout 和最大 header 配置。

### 2.3 数据、错误和 CI 基础

- `pkg/database` 隔离 GORM，提供 Schema、Repository、事务、错误转换、乐观并发和 additive migration。
- `pkg/fault` 提供粗粒度业务分类，Todo Adapter 将其映射到 HTTP status/reason。
- 当前 GitHub Actions 执行全量 test、race、vet、module diff、CGO-free build，并对 PostgreSQL/MySQL 跑数据库契约测试。

## 3. 核心事实矩阵

| 维度 | 当前证据 | 判定 | 尚缺的设计 |
| --- | --- | --- | --- |
| API 权威 | Route 只有 Method、Path、Handler、Middlewares；无 OpenAPI 资产 | 未设计 | operation ID、schema、status、security、版本、弃用、兼容与漂移门禁 |
| 请求解码 | `BindJSON` 只调用一次 `json.Decoder.Decode` | 局部能力 | unknown field、trailing value、空 body、media negotiation、字段错误位置和统一大小限制 |
| 响应提交 | Handler 直接写 status/body；编码失败可能发生在 header 已提交后 | 局部能力 | response buffering/streaming 边界、提交后错误、内容协商、HEAD/204 约束 |
| 错误协议 | `{error,message}`；Todo 在 Handler 内逐项映射 | 局部能力 | 全局 problem model、验证详情、request/trace ID、404/405、一致 media type 和兼容策略 |
| 路由政策 | 全局四个 middleware；route 可附加函数 | 局部能力 | route metadata、public/authenticated/policy、审计、限流、timeout 与所有者 |
| CORS/限流/body | `CORS`、`BodyLimit`、`RateLimit` 存在但 production 未使用 | 局部能力 | 配置 owner、可信来源、主体维度、分布式语义、Retry-After、默认拒绝策略 |
| 身份与授权 | production 无 Principal/authn/authz；`pkg/README` 仅列暂缓 | 已识别未设计 | credential Adapter、Principal、Policy、对象级授权、拒绝语义和审计 |
| 健康与管理面 | Host 内部有 liveness/readiness Snapshot，无 HTTP 暴露 | 局部能力 | startup/liveness/readiness 路由、依赖贡献、管理 listener、访问控制和响应契约 |
| 可观测性 | zap 日志、request ID；AccessLog 只记录 method/path/duration/error | 已识别未设计 | trace/metric、传播、route template、status/bytes、resource attrs、SLO 和采样 |
| 出站 HTTP | `httpx.Client` 有 timeout/body limit/retry；production 无调用方 | 局部能力 | capability owner、Transport 配置、trace、Retry-After、幂等、breaker/bulkhead 和指标 |
| 数据演进 | Todo 启动 Participant 执行 additive `Migrate` | 局部能力 | 版本表、锁、前滚/回滚、backfill、expand-contract、发布顺序和生产权限 |
| 幂等与并发 | Todo Complete 业务幂等、Version 冲突；无 HTTP 条件契约 | 局部能力 | Idempotency-Key、ETag/If-Match、重复请求存储、冲突/过期与清理语义 |
| 分页查询 | Todo 使用 offset/limit/total | 示例级 | 全局分页形态、最大页、稳定排序、cursor、filter/sort 语法和 OpenAPI schema |
| 交付 | 单个 CI workflow；无镜像、部署、release、SBOM、vuln gate | 未设计 | build metadata、容器、探针、信号预算、供应链和回滚 |
| 脚手架产品 | 一个可运行仓库；无 tags、generator 或升级文档 | 未设计 | template/library/generator 定位、创建/重命名、升级和兼容政策 |

## 4. 隐蔽但高影响的缺口

### 4.1 `Route` 无法成为 API 的单一事实源

`internal/module.Route` 只能防止 method/path 冲突。它不能回答：该操作叫什么、是否公开、需要什么授权、请求与响应 schema 是什么、产生哪些状态码、是否允许重试、属于哪个 API 版本、何时弃用。

因此当前无法可靠生成 OpenAPI、权限矩阵、API catalog、审计策略或兼容 diff。若先分别建设 Swagger、RBAC 和限流配置，会形成多份路由事实源。

### 4.2 健康能力存在，但运维不可消费

Host 创建私有 `health.Registry`，只注册进程 liveness/readiness，外部调用方无法注册 Database 等依赖检查；production Router 也没有 health route。代码单元测试能调用 `Host.Health`，部署系统却无法读取。

这不是“补三个 Handler”即可完成：还要定义 startup/readiness/liveness 的不同依赖、超时、缓存、并发、敏感输出和管理面访问边界。

### 4.3 HTTP 中间件工具不等于生产策略

当前 CORS 的空配置会使用 `*`，RateLimiter 是单实例全局令牌桶，BodyLimit 只包裹 Body。它们没有进入应用配置或 composition，也没有 principal/IP/route 维度、可信代理或集群一致性。

直接启用这些函数会制造无法解释的默认安全和容量策略。应先设计 Policy/Route metadata 与配置 owner，再决定具体实现。

### 4.4 当前迁移适合示例，不足以承担发布协议

Todo migration 在应用 ready 前自动执行 additive schema 变更，适合单进程 SQLite 学习示例。成熟部署还必须处理多副本竞争、迁移锁、应用与 schema 版本兼容、长 backfill、不可逆 DDL、最小权限和失败恢复。`Migrate` 已明确不承诺删除、重命名、类型变化或跨数据库 DDL 回滚。

### 4.5 当前仓库还没有回答“脚手架如何被消费”

仓库名称和 README 称其为脚手架，但当前形态是一个具体应用仓库：module path、应用名、Todo、配置键和目录都已固定。没有生成器、模板变量、重命名校验、升级补丁或版本标签，因此无法定义使用者是 fork 一次、依赖库，还是持续跟随上游。

这是所有后续能力的分发边界，必须先于大规模扩展决定。

## 5. 不应误判为通用缺口的事项

以下能力常见，但不能仅凭“成熟”二字直接落地：

- JWT/OIDC/password/RBAC 的具体组合取决于真实 actor、credential 和信任域；当前只能先设计 Principal 与 Policy 边界。
- 多租户需要数据隔离、身份归属和运维模型；不应预先在所有查询中加入 `tenant_id`。
- 消息、后台任务、调度、锁、邮件、搜索、特性开关需要真实交付、一致性和资源生命周期。
- HTTP/2、HTTP/3、mTLS、服务网格和 Kubernetes 不是所有部署的强制选型；脚手架应声明接入边界和兼容要求。

## 6. 局限与剩余未知

- 本轮没有进行容量压测、漏洞扫描、容器运行或远端部署，不能给出性能、安全或可部署性通过结论。
- 只有 Todo 一个应用模块，无法从代码证明多模块 API contract 和 cross-module policy 的开发体验。
- 没有真实用户、租户或第三方 API，因此不能冻结认证、授权、审计、限流和出站策略的具体产品。
- 没有现有 API 消费者数据，未来错误格式、版本和 OpenAPI 迁移必须按破坏性变更处理。

## 7. 研究结论

当前项目的强项是进程与资源治理，弱项是 API 产品治理和交付闭环。019 应优先建立脚手架产品形态、API 单一权威和管理/安全/观测基线，再逐步实现能力；不应回到 018 的插件 Runtime，也不应通过继续堆积互不连接的 `pkg` 包来宣称成熟。
