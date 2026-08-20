# pkg 封装规范与能力清单

`pkg` 只存放面向应用模块开放的通用底层 Capability。这里不是示例代码区，也不是第三方库转发层；每个包都必须有明确能力名、构造入口、错误语义、配置语义、资源边界和测试。

## 封装原则

- 只收纳跨业务域复用的底层能力库，不放具体业务规则、领域模型、页面逻辑或一次性 glue code。
- 不创建 `common`、`utils`、`helpers`、`shared` 等无明确所有权的杂物包。
- 第三方库必须被项目自有能力接口或 adapter 隔离，业务常用接口不得泄漏易变的第三方类型。
- 完整 `pkg/<name> -> internal/kernel/app/<name> -> internal/kernel/composition` 链只治理同时跨业务复用且由进程统一选择的底层资源；只满足复用的普通库不虚构 Kernel 组件，只服务单模块的第三方留在该模块 Adapter。上层不得直接持有具体 Adapter。
- 阻塞 I/O、后台任务、重试和资源释放必须有 `context`、超时、关闭或等待边界。
- 错误必须保留原始原因并增加项目语义，敏感值在日志、错误和本地调试输出中必须脱敏。
- `pkg` 不得导入 `internal`；受托管能力定义、显式组合、生命周期钩子和配置切换统一由 `internal/kernel/**` 承担。

**配置职责边界（032）**：`pkg/*` 只提供通用能力与基础默认行为（库自身最低可用默认）。属于具体应用环境、组件装配、业务场景或运行时变化的配置，由对应 `internal/kernel/app/*` 组件或使用者显式声明并注入；`kernel/app/*` 的默认配置不得整体复用 `pkg/*.DefaultConfig()`。允许引用 `pkg/*` 的基础默认常量（如缓存 `redisstore.DefaultTagPrefix`）作为组件内未声明该值时的回退默认。业务模块不得自行读取或改写 `pkg/*` 默认值/配置，应按注入的契约能力使用。

## 当前能力

| 能力 | 当前底层技术 | 项目边界 |
| --- | --- | --- |
| `logger` | `go.uber.org/zap` | 业务依赖窄 Logger；构造方通过 Resource 独占 Sync/Close 和文件 sink；提供 noop/test logger 和审计字段。 |
| `httpx` | `net/http` + `go-chi/chi/v5` | 构造 HTTP 客户端、路由和服务端；Server 由单一 owner 预绑定、阻塞 Serve，并显式拆分 graceful Stop 与有损 ForceStop；提供 recovery、request id、access log、secure headers、CORS、body limit、rate limit。`httpx/contract` 提供代码优先的模块自有契约 DSL 与 typed Handler 适配，封装 schema/OpenAPI 渲染内部使用的第三方库。 |
| `i18n` | `go-i18n/v2` + `x/text/language` + `yaml.v3` | 构造翻译器并加载本地化资源；Kernel 组合输出身份稳定、内部可换代的 Translator facade。 |
| `database` | `gorm`、SQLite、PostgreSQL、MySQL | 提供项目自有 Schema、Repository、事务、迁移与资源契约，不暴露 GORM 类型。 |
| `cache` | `go-cache`、`go-redis/v9`、`msgpack` | 构造调用方拥有的泛型缓存客户端；Kernel 可治理 disabled/Redis 后端与连接生命周期。 |
| `cli` | `cobra`、`Bubble Tea`、`Lip Gloss` | 构造 CLI 应用、命令、flag 和交互式提示；TUI option 不进入业务契约。 |
| `storage` | 本地文件系统、AWS SDK v2 S3 兼容对象存储、文件辅助库 | 构造对象存储和本地文件工具；Kernel 只治理对象存储 Manager，公开接口不泄漏第三方类型或共享资源关闭权。 |
| `validation` | `go-playground/validator/v10` | 构造结构体验证器，输出项目自有字段错误。 |
| `fault` | 标准库 `errors` | 封装错误码、分类、可重试、取消/超时、关闭错误聚合和脱敏输出。 |
| `supervisor` | `context`、`os/signal`、`errgroup` | 监督进程级 Participant 和长期 runner，共享总 shutdown budget，并分类记录 graceful、forced、pending Participant 与 pending Task。 |
| `health` | 标准库 | 构造健康检查 registry，提供超时、快照、liveness/readiness/startup 分类和 degraded 状态。 |
| `idgen` | `google/uuid` | 构造 ID generator，提供请求和资源 ID。 |
| `clock` | 标准库 `time` | 构造系统时钟或固定时钟，封装时间格式边界。 |
| `secrets` | 标准库 `crypto/rand`、`crypto/hmac`、`crypto/pbkdf2` | 封装敏感值、脱敏、随机 token、HMAC、KDF 和 secret source。 |
| `resilience` | 标准库 | 提供 retry、timeout 和 circuit breaker 策略执行器。 |
| `concurrency` | `x/sync` + 标准库 | 提供项目自有 singleflight、固定 worker pool 和 context 感知任务执行，不导出 errgroup 类型。 |
| `codec` | `encoding/json`、`yaml.v3`、`msgpack` | 构造 JSON/YAML/msgpack 编解码器，提供内容类型、大小限制和统一错误语义。 |
| `testkit` | 标准库 + 项目包 | 提供 fake clock、临时文件、健康 fixture 和底层库测试辅助。 |
| `observability` | 项目自有契约 | 提供 HTTP observation、后台 `Work` span、Metrics endpoint 与低敏 diagnostics；Prometheus/OTel/OTLP 只存在于 Kernel App 实现。 |
| `execution` | 项目自有契约 + 标准库 + `pkg/resilience`/`fault`/`concurrency`/`health` | 提供幂等键/执行记录存储与带失败重试的受托管操作执行契约 `OperationExecutor`；执行记录可携带经 context 传递的全链路追踪标识（`WithTrace`/`TraceFrom`）；外部依赖治理 Store `RecoveringStore`（主存储故障降级到本地、有界记录缓冲 + 溢出策略、退避/抖动/最大频率探测、可用性验证、恢复后回放并原子切回主实现，`Snapshot()`/`OnStateChange`/`Health()` 观测）与执行记录异步持久化 `AsyncRecorder`（幂等占用/完成同步、记录异步有界队列 + 溢出策略 + 排空式 Shutdown）（035）；backend 由 Kernel App 在选择后注入。 |
| `schedule` | 项目自有声明契约；`gocron/v2` 只存在于内部 Adapter | 模块声明 cron/fixedDelay、任务级并发和分布式执行策略；不暴露 scheduler、生命周期或注册权。 |
| `messaging` | 项目自有 Contract/Binding/Publisher；`amqp091-go` 只存在于内部 RabbitMQ Adapter | 模块声明消息 Contract、生产/消费关系、交付预算、重要性和并发；composition 解析逻辑 Route 并治理 confirm、ack、重投、死信、恢复和 Consumer 代际，不暴露 Broker Client 或物理 topology。 |
| `coordination` | 项目自有租约契约；production Adapter 复用 Cache 的 `go-redis/v9` client | 表达 acquire/renew/release、未获得、不可用与失权；不把 Redis 类型或 token 暴露给业务模块。 |

## 暂缓路线

以下能力属于成熟项目常见能力，但当前不阻塞底层库封装。必须等真实组件或业务场景明确后再定义契约：

- 消息扩展：Kafka、NATS JetStream 等后续 Provider 与其分区、顺序、事务、Stream/Consumer Group 专属能力；当前只实现 RabbitMQ 4.3 Provider，不把未支持能力伪装为公共语义。
- 通用业务锁：数据库锁、fencing-aware 资源写入等必须先明确一致性、租约与续期语义；当前 `coordination` 只服务统一调度执行权，不是通用 Lock API。
- 认证授权：JWT/OIDC/password hash/RBAC，只有第一批组件包含用户身份时再进入。
- 邮件：SMTP/API adapter、模板、重试和投递诊断。
- 搜索：Meilisearch/Elasticsearch adapter，等待真实检索模型。
- 特性开关：本地/远端开关、灰度规则和审计。
- 架构扩展：基础能力依赖 DAG、业务对象图、诊断报告体系和观测采集适配。

## 验收要求

- `go test ./...` 必须通过。
- `git diff --check` 必须通过。
- 旧模块路径、旧 import、旧文档路径必须为零。
- 公开 API 不泄漏未允许的第三方类型。
- `pkg` 层不得反向依赖 `internal/kernel`。
