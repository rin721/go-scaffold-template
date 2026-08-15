# 需求：生产就绪模板一次性竣工

## 1. 目标与依据

本变更把 022 中尚未启动的 portability、API、协议、edge、安全、management、observability、migration、delivery、release 与 acceptance 工作，以及工作区 023 的完整配置代际目标，合并成一个依赖有序、一次确认后连续实施的单轨计划。

当前事实由 [R001](research/R001-current-one-shot-baseline/report.md) 支撑，API/安全选型由 [R002](research/R002-api-security-stack-selection/report.md) 支撑，交付/发布选型由 [R003](research/R003-delivery-release-stack-selection/report.md) 支撑；JWT/JWKS 依赖版本复核见 [R004](research/R004-jwx-jsonv2-reassessment/report.md)，C4-C6 模块归属复核见 [R005](research/R005-security-module-ownership/report.md) 与 [R006](research/R006-remaining-module-ownership/report.md)。施工级决策以 [ADR-003](decision.md) 为准。

本计划形成不等于非文档实施已经获准。只有用户在计划报告之后明确确认 `ONE-001..025`，才能修改源码、配置、测试、依赖、CI 或运行环境。

## 2. 竣工标签

- `ONE-REQ-001`：实施前继续保持 `Foundation-closed(current synchronous HTTP/CLI profile)`，不得提前改写为 production-ready。
- `ONE-REQ-002`：只有 Foundation、Copy-ready 与 Production HTTP API 三组验收全部通过，才能标记 `Foundation-closed(current production profile)`、`Copy-ready(windows/amd64 + linux/amd64)` 与 `Production HTTP API-ready`。
- `ONE-REQ-003`：“一次性”表示一次计划确认、连续施工、检查点提交和一次最终总验收；不表示单一巨大 commit，也不允许检查点降低最终完成定义。
- `ONE-REQ-004`：任何未执行的平台、数据库、容器、签名或外部发布验证都必须明确列为未通过，不得用 cross-build、mock 或本地文件生成替代真实验收。
- `ONE-REQ-005`：最终工作树只允许保留用户既有的范围外修改；024 自身不得留下过渡入口、旧 authority、兼容分支、TODO 或未归属生成物。
- `ONE-REQ-006`：新增业务能力必须先按 `internal/module/<name>` 收口 model、service、Adapter、binding 与 contribution；只有经能力评估证明是跨业务复用且由进程统一选择的底层资源，才进入 `pkg/<name> -> internal/kernel/app/<name> -> internal/kernel/composition`。拥有第三方 SDK、Client、cache 或 goroutine 本身不是升级为 Kernel Capability 的理由。

## 3. Application Generation 与无感切换

- `GEN-REQ-001`：所有 watched runtime section 必须在同一 immutable `ApplicationGeneration` 中由同一配置 Snapshot 构造、Validate、Build、Start 与 Ready；当前资源与新增 `auth`、`management`、`observability` 不得各自独立热改。
- `GEN-REQ-002`：进程级 `ListenerHub` 是业务与 management 物理 listener 的唯一 owner；候选未完全 Ready 前不得接收新连接，commit 点不得执行 I/O。
- `GEN-REQ-003`：每个请求在 admission 时固定 generation，处理中不得跨代查询可变全局资源；旧代停止 admission 后必须 drain，并保留 owner、generation、pending unit 与 cleanup debt 诊断。
- `GEN-REQ-004`：候选失败必须关闭候选并继续服务旧代；commit 后旧代清理失败必须进入 degraded/cleanup-required，不能伪装 rollback 或 success。
- `GEN-REQ-005`：配置 watcher、Host、ListenerHub、稳定 Prometheus registry 与进程信号属于 process baseline；数据库、缓存、存储、logger、i18n、Todo、JWT/JWKS verifier、Router、HTTP handler、OTel providers 与代际 route set 属于 generation owner。
- `GEN-REQ-006`：migration、release、build toolchain 和供应链配置不是 watched runtime state；其变化必须显式命令或重新部署。
- `GEN-REQ-007`：完整迁移后删除 restart-required 旧路径、手工 server swap、第二套 resource owner 与重复配置 authority；不得让 023 与 024 同时成为施工 authority。

## 4. API authority、协议与边缘策略

- `API-REQ-001`：`api/openapi.yaml` 使用 OpenAPI 3.0.3，并成为 operation、schema、response、security 与兼容性的唯一 public HTTP authority。
- `API-REQ-002`：`oapi-codegen v2.8.x` strict Chi 生成 transport DTO、server interface 与 route binding；生成物提交 Git，并由 `go generate` 后 clean diff 门禁约束。
- `API-REQ-003`：每个 operation 必须有稳定 `operationId`，Router、授权、日志、trace、metrics、inventory 与 contract test 复用同一 identity；缺失或重复在生成/构建前失败。
- `API-REQ-004`：使用 `oasdiff v1.22.x` 建立 breaking 与 changelog gate；公共破坏只能通过明确版本/弃用策略和 ADR，不得静默发布。
- `API-REQ-005`：生成 DTO 不进入 module core；strict handler Adapter 负责 transport/use-case 映射、错误转换和响应提交。
- `API-REQ-006`：现有 `module.Route`、手工路由清单、重复权限清单与旧 HTTP schema 在全部调用方迁移后删除。
- `PROTO-REQ-001`：所有公开失败统一为 RFC 9457 `application/problem+json`，覆盖 validation、认证、授权、404、405、panic、rate/overload、dependency 与未知错误。
- `PROTO-REQ-002`：JSON 请求严格校验 Content-Type、body size、unknown field、trailing token 与单值语义；错误不得泄露内部类型、SQL、DSN、token 或堆栈。
- `PROTO-REQ-003`：Accept、HEAD、204/304、响应已提交、request cancellation 与 deadline 的语义必须由 contract/integration tests 固定。
- `EDGE-REQ-001`：trusted proxy 与 client IP 必须显式配置；默认不信任转发 Header。
- `EDGE-REQ-002`：业务与 management listener 分别拥有 header/body、timeout、concurrency 与 request budget；所有预算可取消且有上界。
- `EDGE-REQ-003`：Bearer-only API 不使用浏览器 cookie/session，因而 CSRF 明确不适用；若启用 cookie/session 必须触发重新研究。CORS 默认拒绝跨域，只允许显式 origin/method/header。
- `EDGE-REQ-004`：保留进程级有界 limiter/overload protection，429/503 与 `Retry-After` 语义稳定；跨副本全局配额由外部 gateway/专用系统负责，不伪装为当前能力。

## 5. 认证、授权与审计

- `SEC-REQ-001`：`internal/module/auth` 是认证授权唯一业务 owner，收口 `Principal`、`CredentialVerifier`、`Authorizer`、`Decision`、`AuditSink`、配置、JWT Adapter、middleware 与 generation contribution；Todo 定义调用方拥有的窄跨模块 port，两个 module core 都不导入 JWT/JWK、OpenAPI 或 Chi 类型。
- `SEC-REQ-002`：每个 operation 明确 public 或 protected，并声明 scope/action；遗漏策略必须在生成、构建或启动前 fail closed。
- `SEC-REQ-003`：production HTTP 使用内部 `jwx v3.2.0` JWT/JWKS Adapter，强制 issuer、audience、允许算法、时间窗口、key refresh budget 与敏感诊断脱敏；verifier 未 Ready 时 protected operation fail closed。`GOEXPERIMENT=jsonv2` 不得成为隐式构建条件。
- `SEC-REQ-004`：Todo 保存 `owner_subject`，在读取真实资源事实后执行对象级授权；list/get/update/delete 不得只依赖请求路径中的 ID 或调用方输入。
- `SEC-REQ-005`：现有 Todo 数据通过显式 expand/backfill/contract migration 获得 owner；无法确定 owner 时停在 migration gate，不猜测 production subject。
- `SEC-REQ-006`：development anonymous actor 只允许 `environment=development` 且业务 listener 为 loopback；production 禁止。Application CLI 必须显式 `--subject`，把本机 operator 作为信任边界，但仍执行同一 Authorizer 与 AuditSink。
- `SEC-REQ-007`：认证失败、授权拒绝、策略缺失、JWKS 刷新和管理操作产生低敏审计结果；日志不得记录原始 token、完整 claims 或对象内容。

## 6. Management 与可观测性

- `OPS-REQ-001`：独立 management listener 提供 `/startupz`、`/livez`、`/readyz`、`/metrics`、`/build` 与脱敏 `/diagnostics`；业务 listener 不暴露这些入口。
- `OPS-REQ-008`：management 与 observability 以 `internal/module/ops` 为唯一应用 owner，收口 use cases、HTTP binding、OTel/Prometheus Adapter、配置、middleware 与 contribution；物理 listener、稳定 registry identity 与 build ldflags 仍由进程 owner 持有。
- `OPS-REQ-002`：startup/liveness/readiness 语义分离；readiness 汇总 generation、认证 verifier、数据库及必要 exporter 状态，停止 admission 前先变为 not ready。
- `OPS-REQ-003`：完整 diagnostics 只对 management scope 开放；pprof 默认禁用，启用时必须有独立配置、认证、预算与审计。
- `OPS-REQ-004`：使用 OpenTelemetry Go `1.44.x` stable trace 与 OTLP/HTTP exporter；不引入仍为 Beta 的 OTel logs。日志通过 request ID/trace ID 关联。
- `OPS-REQ-005`：使用 `prometheus/client_golang v1.24.x` 和进程级稳定 registry；指标只允许 operation、method、status class、error class 与命名 dependency 等低基数标签。
- `OPS-REQ-006`：exporter failure 不阻断业务，但必须有有界队列、drop counter、shutdown flush budget、自诊断和敏感属性 allowlist。
- `OPS-REQ-007`：build info 至少包含 version、commit、build time、Go version 与 dirty state；不得包含路径、凭据或 runner identity。

## 7. 数据迁移

- `DATA-REQ-001`：使用 `golang-migrate v4.19.x` 与按 SQLite/PostgreSQL/MySQL 分离、嵌入二进制的 versioned SQL；版本、checksum、dirty state 与 lock timeout 可诊断。
- `DATA-REQ-006`：migration command 归 `internal/module/migration`，通用 engine 归 `pkg/database/migrate`，Todo 三 driver SQL、owner expand/backfill/contract 与 compatibility 归 `internal/module/todo/binding/migration`；不得建立顶层无归属 `internal/adapter/migration` 或根级 Todo SQL authority。
- `DATA-REQ-002`：独立 `db migrate` command/job 是 migration owner；service 启动只校验 schema compatibility/readiness，不再执行 `database.Client.Migrate` 或 AutoMigrate 风格 schema 变更。
- `DATA-REQ-003`：每个 driver 均验证 fresh up、incremental up、重复执行、dirty/lock/too-new/too-old 和兼容窗口；down 只用于测试和明确 Runbook，不自动用于生产回滚。
- `DATA-REQ-004`：Todo `owner_subject` 采用 expand/backfill/contract，旧应用与新 schema 的兼容窗口、失败前滚和回滚边界必须写入 migration Runbook。
- `DATA-REQ-005`：迁移账号与运行账号权限分离；错误保留原始原因但不输出 DSN、密码或完整 SQL 参数。

## 8. 可移植交付与发布

- `REL-REQ-001`：基线升级为 Go `1.26.5`，`go.mod`、CI、容器 builder 与 release toolchain 同版；`.gitattributes` 固化 Go/module/YAML/Markdown 为 LF。
- `REL-REQ-002`：支持 `windows/amd64` 与 `linux/amd64` 二进制；首个 OCI image 只承诺 `linux/amd64`。两个平台必须执行同义 tidy/test/race/vet/build 和 CLI/config smoke。
- `REL-REQ-003`：使用 digest-pinned `distroless/static-debian13:nonroot`，CGO-free、非 root、默认只读 root filesystem；SQLite/data/config/cert path 必须显式挂载并检查权限。
- `REL-REQ-004`：CI 固定 Action/tool 版本，执行 generation clean diff、OpenAPI lint/diff、test/race/vet/tidy/build、bounded fuzz、`govulncheck`、gosec、secret/artifact scan、三数据库 contract、容器 smoke 与复制验收。
- `REL-REQ-005`：GoReleaser `v2.17.x` 生成 binaries、archives、checksums 与 metadata；Syft `v1.x` 生成 SPDX JSON SBOM；Cosign 生成并验证 keyless bundle/signature。
- `REL-REQ-006`：复制指南明确 tracked baseline、module/repository/config/brand identity、Todo 保留或移除、Git 历史、writable path 与 secret 注入，不复制 `.git`、缓存、构建物或运行数据。
- `REL-REQ-007`：最终验收必须创建两个隔离副本：一个保留 Todo，一个完全删除 Todo；两者不得依赖源工作区绝对路径或未跟踪文件。
- `REL-REQ-008`：第一候选版本为 `v1.0.0-rc.1`，通过 acceptance 后才允许 `v1.0.0`。没有明确外部授权时只生成本地 release candidate，不 push、不 tag、不创建 GitHub Release/GHCR/attestation。
- `REL-REQ-009`：正式 release 必须附兼容声明、迁移说明、checksum、SBOM、签名验证、rollback/forward-fix Runbook 与已知限制。

## 9. 最终验收

- `ACC-REQ-001`：Windows 与 Linux 原生 runtime 均完成启动、业务/management probe、reload、graceful shutdown 与失败恢复；cross-build 不计入 runtime 通过。
- `ACC-REQ-002`：SQLite、PostgreSQL、MySQL migration 和 Todo contract 在隔离实例通过；外部数据库不可用时该项保持未通过。
- `ACC-REQ-003`：协议、认证、对象授权、rate/overload、JWKS 不可用、旧代 drain、candidate reject、cleanup debt 与敏感信息负向场景通过。
- `ACC-REQ-004`：容器以 nonroot/read-only 启动，probe、SIGTERM、writable mount、无 shell/无凭据和镜像扫描通过。
- `ACC-REQ-005`：两个复制副本在干净环境独立 tidy/test/race/vet/build/smoke；移除 Todo 副本无路由、配置、migration、文档或依赖残留。
- `ACC-REQ-006`：release candidate 的 archive、checksum、SBOM、signature/bundle、build info 与源 Commit 可互相验证；若未获远端授权，只能标记 local RC verified。
- `ACC-REQ-007`：权威文档、OpenAPI、生成物、配置示例、Runbook、CI 和当前代码一致，024 任务证据逐项关联 Commit，最终工作树无 024 残留变更。

## 10. 非目标与重新确认线

- 不引入运行时 DI 容器、service locator、反射扫描、插件 Runtime 或项目 generator。
- 不预装消息、调度、邮件、搜索、租户、分布式锁或特性开关。
- 不承诺 WebSocket、SSE、HTTP/3、hijacked connection、多进程热升级、Kubernetes 或真实云部署。
- 不选择具体云厂商、OIDC provider、API gateway、APM backend 或私有 registry。
- 若真实 actor 需要 cookie/session、mTLS、多租户隔离，或迁移需要在线双写/跨库复制，必须返回研究阶段并重新确认。
- push、tag、GitHub Release、GHCR、外部 attestation、真实部署和真实数据迁移始终需要同一后续确认消息中的明确授权。
