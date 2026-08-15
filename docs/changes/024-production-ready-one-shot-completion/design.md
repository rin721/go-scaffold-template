# 设计：生产就绪模板单轨施工

## 1. 设计目标

本设计把“当前 Foundation 已闭环”到“可复制、可运维、可发布的生产 HTTP API 模板”之间的剩余工作压成一条可执行依赖链。它不回写已完成事实，也不把发布侧副作用包含在默认授权中。

依据：[R001 当前基线](research/R001-current-one-shot-baseline/report.md)、[R002 API/安全选型](research/R002-api-security-stack-selection/report.md)、[R003 交付/发布选型](research/R003-delivery-release-stack-selection/report.md) 与 [ADR-003](decision.md)。

## 2. 总体依赖链

```text
authority arbitration + portability/toolchain
        |
        v
ApplicationGeneration + ListenerHub
        |
        +------------------+
        v                  v
OpenAPI authority      versioned migration
        |                  |
        v                  |
protocol + edge            |
        |                  |
        v                  |
security + Todo ownership  |
        |                  |
        +--------+---------+
                 v
        management + observability
                 |
                 v
        container + CI + release
                 |
                 v
        two-copy / two-platform acceptance
```

只有上游 authority 和代际模型稳定后，后续 Router、鉴权、management 与 exporter 才能有唯一 owner。migration 与 API 可在代际模型后并行施工，但必须在 Todo 对象授权前汇合。

## 3. 单一 authority 与检查点

### 3.1 实施起点

`ONE-001` 先检查工作区、Git 历史和任何 023 成果：

- 已进入已确认 commit 且不冲突的 Application Generation/ListenerHub 实现，迁入 024 任务证据并继续复用；
- 只有未提交研究/计划时，将其事实吸收到 024，023 不再作为并行施工 authority；
- 若出现与 ADR-003 不一致的公共接口或迁移语义，停止并重新提交计划，不自行拼接双轨。

### 3.2 检查点提交

计划采用八个可回滚检查点，而不是一个巨大提交：

1. toolchain、LF、依赖与 governance；
2. `ApplicationGeneration`、`ListenerHub` 与完整 watched config；
3. OpenAPI authority、strict transport、Problem Details 与 edge；
4. security contract、JWT/JWKS、Todo ownership 与 audit；
5. management 与 observability；
6. versioned migration 与 startup compatibility；
7. container、CI、release、SBOM 与签名；
8. 双副本/双平台总验收、权威文档与竣工标签。

每个检查点只表示依赖链中的可审阅状态；024 只有第八项总验收通过才完成。

## 4. Process baseline 与 Application Generation

### 4.1 所有权

| Owner | 长期拥有 | 禁止拥有 |
| --- | --- | --- |
| Process baseline | config watcher、Host、Supervisor、ListenerHub、稳定 metrics registry、signals、build info | generation resource client、Router、JWT verifier、Todo service |
| Application generation | immutable Snapshot/digest、capabilities、database/cache/storage/logger/i18n、Todo、auth verifier、HTTP route set、OTel provider/exporter、Ready/cleanup state | 物理 listener、watcher、全局 mutable locator |
| Migration command | embedded migration set、driver、lock、version/dirty result | service listener、runtime generation |
| Release tooling | source snapshot、tool versions、artifacts、SBOM/signature | runtime secret、production data |

`ApplicationGeneration` 使用 typed field/slot，不使用 `map[string]any` 或 runtime resolver。每个 field 记录 owner、generation、构造/启动/Ready/停止结果；第三方实例继续隐藏在项目 Adapter 后。

### 4.2 构造与提交

```text
load immutable candidate
  -> validate every owner
  -> build generation (with compensation ledger)
  -> start generation
  -> wait all required readiness within candidate budget
  -> ListenerHub.commit(old, candidate)  // no I/O
  -> new admissions bind candidate
  -> old generation drain
  -> old generation finalize and verify
```

任何 pre-commit 失败都按反向 owner 顺序补偿，旧代保持 admission。commit 后不再声称 rollback；旧代 cleanup failure 写入 process diagnostics 并阻止下一次不安全切换，直到责任 finalized/failed/forced。

### 4.3 ListenerHub

- Process 启动时一次性绑定业务和 management 地址；重载不 rebind 同地址。
- accept loop 只负责连接 admission 与 generation lease，不解析业务路由。
- 每个 accepted connection/request 持有固定 generation lease；keep-alive 下一请求在 admission 边界重新取当前 generation，进行中的请求不跨代。
- Windows/Linux 分别测试 pending connection、accept cancellation、地址冲突、drain timeout 与 shutdown。
- WebSocket/SSE/HTTP3/hijack 明确不在当前支持 profile；遇到 upgrade 请求由协议门禁确定性拒绝。

### 4.4 watched 配置

在现有 runtime section 上新增 `auth`、`management` 与 `observability`，全部进入同一 candidate：

- logger、database、cache、i18n、storage、http、todo；
- auth issuer/audience/JWKS/algorithms/timeouts；
- management bind/policy/budgets；
- observability service identity/exporter/budgets。

地址不可原位切换、migration path、release 配置和 build flags 仍为 restart/deploy-required。候选 JWKS 拉取或 exporter 初始化失败时，按“是否为 required dependency”决定 reject 或可诊断降级，策略必须在 typed config 中显式声明。

## 5. OpenAPI 与 transport

### 5.1 文件 authority

```text
api/openapi.yaml               public contract authority
api/oapi-codegen.yaml          pinned generation configuration
internal/transport/http/api/   committed generated DTO/server bindings
internal/transport/http/       strict handler adapters and presenters
internal/module/todo/          project-owned use cases and contracts
```

OpenAPI 3.0.3 包含稳定 `operationId`、schema、Problem response、public/protected security 与 version/deprecation。生成配置固定 package/output/options；生成头记录工具版本。CI 在固定工具版本运行 generation 并要求 clean diff。

`operationId` 同时生成或校验 operation inventory。Router、policy、metrics 和 tests 只能从该 inventory 获取 identity；不再维护手写 `module.Route`。生成 DTO 在 Adapter 转换为 Todo command/query/result，不能进入 module core 或 repository。

### 5.2 协议流水线

建议顺序：

```text
request id -> recovery -> trusted proxy -> security headers
-> request budget -> body/header limits -> CORS
-> authentication -> operation authorization -> rate/overload
-> strict OpenAPI handler -> use case -> Problem/response presenter
-> access audit/metrics/trace completion
```

404、405、panic、middleware 与 handler error 都进入同一 project-owned Problem presenter。Problem `type` 使用稳定项目 URI/URN，`status` 与 HTTP status 相同，`instance` 只使用安全 request identity；内部 error chain 留在受控日志/trace，不进入 response。

JSON decoder 拒绝 unknown field、trailing token、空 body 和错误媒体类型。响应提交后发生错误只记录一次决策边界诊断，不尝试写第二个 response。

### 5.3 Edge policy

trusted proxy 使用显式 CIDR；未匹配时忽略 forwarded headers。CORS 使用 allowlist，Bearer-only 模式不建立 cookie/session，因此 CSRF 标记为不适用。业务/management 各自配置 timeout、body/header、in-flight 和 token bucket。429 表示调用方配额，503 表示服务器 overload，并提供受控 `Retry-After`。

## 6. Security 与 Todo actor

### 6.1 Auth application module

认证授权按 [R005](research/R005-security-module-ownership/report.md) 收口为 `internal/module/auth`，而不是顶层 `internal/security`、顶层 `internal/adapter/security` 或 Kernel App：

```text
internal/module/auth/
├── model/
├── service/
├── adapter/jwt/
├── adapter/audit/
├── middleware/
├── binding/config/
└── module.go
```

`module.New` 只构造内存对象；JWT/JWKS Adapter 以未启动 participant 进入 module contribution，由 Application Generation 在 Router 构造和 admission 前 Start/Ready，在旧代 drain 后 Stop。OpenAPI inventory 由 composition 转成 module 构造输入，composition 不实现 claims、policy 或 audit。

### 6.2 项目契约

建议由稳定应用边界定义：

```go
type Principal struct {
    Subject string
    Scopes  ScopeSet
}

type CredentialVerifier interface {
    Verify(context.Context, Credential) (Principal, error)
}

type Authorizer interface {
    Decide(context.Context, Principal, Action, ResourceFacts) (Decision, error)
}

type AuditSink interface {
    Record(context.Context, AuditEvent) error
}
```

实际字段在实现前可保持私有/收敛，但必须保留 subject、scope/action、真实 resource facts、decision reason class 与 audit outcome；第三方 claims/JWK 不得泄漏。

Todo `service` 定义自己需要的 actor、对象授权和审计窄 port；composition 使用小 Adapter 连接 Auth module 完成品。HTTP/CLI 边界显式传递 actor，不通过全局变量、万能 Context 值或 runtime locator 隐藏依赖。

### 6.3 JWT/JWKS Adapter

Adapter 使用内部封装的 `jwx v3.2.0`，只接受 Bearer，显式固定 issuer、audience 与允许算法。JWKS cache/refresh 有 deadline、single-flight、last-success 和 Ready；未知 key 可触发一次有界 refresh，失败不回退未验证 claims。时钟由项目 Clock 注入以测试 exp/nbf/iat/leeway。`jwx v4` 的 `GOEXPERIMENT=jsonv2` 不进入当前构建契约。

开发 anonymous actor 同时满足 development 与 loopback 才可构造；production config validation 直接拒绝。CLI 不解析 bearer token，要求显式 `--subject`/scopes，由本机 operator 边界负责输入，但 use case 仍走相同 authorization/audit。

### 6.4 Todo ownership migration

1. expand：添加 nullable `owner_subject` 与新索引，新代码兼容旧行；
2. backfill：由显式参数/映射完成，不依据创建时间或默认用户猜测；
3. contract：确认无 null 后加约束并删除兼容读取；
4. service：create 写 actor subject；get/list/update/delete 加 owner fact；
5. HTTP/CLI contract：验证跨 subject 拒绝与审计，错误不泄露对象是否存在的敏感差异。

## 7. Management 与 observability

按 [R006](research/R006-remaining-module-ownership/report.md)，C5 收口为单一 `internal/module/ops`：

```text
internal/module/ops/
├── model/
├── service/
├── adapter/otel/
├── adapter/prometheus/
├── middleware/
├── binding/config/
├── binding/http/
└── module.go
```

Ops module 拥有 probe/diagnostic/build use cases、management HTTP、OTel/Prometheus Adapter、标签策略与 generation contribution。Application composition 只持有/连接 business 与 management listener、稳定 registry identity、Auth/diagnostics/logger 等完成品，不实现 Ops 规则。

### 7.1 Management listener

独立 listener 使用更小的 header/body/concurrency/budget。公开 probes 只返回最小状态；`/metrics` 的暴露策略显式配置；`/diagnostics` 需要 management scope，内容复用 Host 的 typed process diagnostics 并继续脱敏。`/build` 返回非敏感构建元数据。pprof 默认不存在于 Router。

Readiness 在 candidate 未 Ready、generation degraded、required auth/database unavailable 或 drain 开始时失败。Liveness 只表达进程能否继续治理责任，不能因普通下游暂时失败重启进程。Startup 在首次 generation commit 后成功。

### 7.2 Trace、metrics 与日志

- inbound span 使用 operationId，传播 W3C Trace Context；outbound DB/cache/JWKS/exporter 通过项目 Adapter instrumentation；
- Prometheus registry 由 process owner 稳定持有，generation collector 使用可注销或 generation-neutral callback，防止重复注册；
- 标签禁止 raw path、subject、Todo ID、SQL、error text；
- 结构日志加入 operation/request/trace/generation/error class，继续由 Kernel baseline logger 保证早期阶段可用；
- OTel exporter 有界、可丢弃并计数，generation shutdown 在总预算内 flush；不接入 OTel logs Beta。

## 8. Versioned migration

按 [R006](research/R006-remaining-module-ownership/report.md) 固定布局：

```text
pkg/database/migrate/                       generic golang-migrate adapter
internal/module/migration/                  status/version/up use cases and CLI binding
internal/module/todo/binding/migration/
├── sqlite/*.sql
├── postgres/*.sql
└── mysql/*.sql
cmd/app db migrate ...                      explicit invocation owner
```

命令至少支持只读 `version/status` 与受控 `up`；dirty repair 需要显式版本、确认参数和 Runbook，不提供启动时自动 repair/down。锁和 step 有 deadline。service 启动调用 schema compatibility probe：版本过旧、过新或 dirty 时 not ready 并返回分类错误，不修改 schema。

迁移完成后删除 `database.Client.Migrate`、Todo GORM AutoMigrate/Schema DSL 与旧测试；三个 driver 的 SQL 是各自唯一 authority，不建立运行时跨方言 SQL 拼接器。

## 9. 交付、CI 与发布

### 9.1 可重复构建与容器

- `go.mod`/CI/builder/release 统一 Go 1.26.5；使用 `-trimpath` 和受控 ldflags 注入 build info；
- `.gitattributes` 固化文本 LF，Windows/Linux 均执行 `go mod tidy -diff`；
- multi-stage Dockerfile 构造静态 linux/amd64 binary，运行层 digest pin `distroless/static-debian13:nonroot`；
- 默认 nonroot、read-only rootfs、无 shell，显式 mount config/data/cert；SIGTERM 进入 Host drain；
- `.dockerignore` 排除 `.git`、本地 config、数据、缓存、测试输出和 artifacts。

### 9.2 CI 门禁

CI 拆为只读、可并行的 jobs：

- generated/OpenAPI/format/tidy/import-boundary/documentation；
- unit/integration、race、vet、Windows/Linux build 与 bounded fuzz；
- SQLite/PostgreSQL/MySQL migration/contract；
- govulncheck、gosec、secret/artifact scan；
- container nonroot/read-only/probe/SIGTERM smoke；
- copy-keep-todo 与 copy-remove-todo；
- release snapshot、checksum、SBOM 与 signature verification。

Action 使用 immutable commit SHA；工具版本集中声明。CI 失败不能由 `continue-on-error`、环境绕过或仅记录 warning 隐藏。

### 9.3 Release

GoReleaser 生成 Windows/Linux archives、checksums 与 source-linked metadata；Syft 生成 SPDX JSON，Cosign keyless bundle/signature 由验证步骤反查 artifact digest。`v1.0.0-rc.1` 先完成隔离验收，最终 `v1.0.0` 只在全部标签通过且用户授权远端副作用后创建。

没有远端授权时执行 snapshot/local RC，输出到 ignored 临时 artifact 目录并完成本地验证；不得创建 tag、push、GitHub Release、GHCR 或外部 attestation。

## 10. 文件影响

预计修改范围如下；实施时以 `ONE-*` task 为边界，不因目录建议虚构空包：

- toolchain/governance：`go.mod`、`go.sum`、`.gitattributes`、验证脚本、architecture tests；
- runtime：`internal/kernel`、`internal/bootstrap`、`internal/composition`、`pkg/httpx`、配置 schema/example；
- API：`api/`、`internal/transport/http/`、Todo HTTP binding、旧 `module.Route` 调用方与测试；
- security：项目自有 security contract、JWT Adapter、Todo service/repository/model/CLI/HTTP；
- management/observability：`internal/module/ops`、process listener/registry wiring 与 tests；
- migration：`pkg/database/migrate`、`internal/module/migration`、Todo migration binding、`cmd/app db` 与数据库 startup gate；
- delivery：`Dockerfile`、`.dockerignore`、`.goreleaser.yaml`、`.github/workflows/`、copy/release Runbook；
- docs：根入口、配置/API/运行/部署/复制/迁移/安全/运维权威主题与 024 evidence。

任何新增公共 Go API、第三方依赖替换或真实目录边界偏离本节且改变调用方时，先更新研究与计划。

## 11. 失败语义

- config candidate、JWKS、listener、migration compatibility 或 required dependency 失败：保留旧 generation 或阻止首次 Ready，返回分类 error chain；
- pre-commit generation failure：补偿候选，旧代不受影响；
- post-commit old generation cleanup failure：degraded/cleanup-required，保留责任，不伪装 rollback；
- auth missing/invalid/expired/key unavailable：401 Problem；authenticated but denied：403 Problem；策略缺失：启动/构建 fail closed；
- migration lock/dirty/version incompatible：命令失败或 service not ready，不自动 repair/down；
- telemetry exporter failure：业务继续，drop/self-diagnostics 可见；
- audit sink failure：安全决策仍 fail closed 或按 operation 的已确认 policy 处理，不能静默吞掉；管理/高风险写操作默认 fail closed；
- artifact/SBOM/signature mismatch：release gate 失败，不发布。

## 12. 验证矩阵

| 层次 | 最小验证 |
| --- | --- |
| Static | gofmt、`go mod tidy -diff`、vet、generated clean diff、architecture/docs/metadata/link checks |
| Unit | generation state、ListenerHub admission/drain、strict protocol、Problem、policy、JWT/JWKS、schema compatibility |
| Concurrency | `go test -race ./...`、reload/request/JWKS/metrics registration 竞争、bounded fuzz |
| Integration | business+management、candidate reject、old drain、Todo owner、three DB migrations、exporter failure |
| Platform | Windows amd64 与 Linux amd64 原生 test/build/run/reload/shutdown |
| Container | nonroot/read-only、mount、probe、SIGTERM、image scan |
| Copy | keep/remove Todo 两隔离副本，无源路径/未跟踪依赖 |
| Release | snapshot/RC、archive/checksum/SBOM/signature/build-info/source commit 对应 |

本机当前没有 Docker 和可运行 WSL，因此容器/Linux runtime 在计划阶段没有被验证；实施时必须使用可用的本机环境或获得授权的 GitHub Actions，不能以 Windows 结果代替。

## 13. 授权边界与停止条件

一次后续确认可授权 `ONE-001..025` 的连续非文档施工、测试和检查点 commit。以下事项必须在该确认中单独允许，或执行前再次取得明确授权：

- 创建/启动/停止本地容器和临时数据库；
- 网络下载构建工具或依赖（正常 `go mod` 解析除外时仍应记录）；
- push、tag、GitHub Release、GHCR、外部 attestation；
- 真实部署、真实密钥、真实生产数据迁移。

命中 ADR-003 的重新确认触发器、发现无法隔离用户工作区修改、或任何必需验收环境连续不可用时停止，不降低竣工定义。
