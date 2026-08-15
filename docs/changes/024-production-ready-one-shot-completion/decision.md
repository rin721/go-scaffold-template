# ADR-003：采用单轨生产就绪竣工方案

- 状态：已接受
- 日期：2026-08-15
- 依据：[R001](research/R001-current-one-shot-baseline/report.md)、[R002](research/R002-api-security-stack-selection/report.md)、[R003](research/R003-delivery-release-stack-selection/report.md)
- 取代范围：022 中尚未启动的 `PORTABILITY/API/MANAGEMENT/OBSERVABILITY/SECURITY/MIGRATION/DELIVERY/RELEASE/ACCEPTANCE` Program；023 若尚未形成已确认 commit，其施工 authority 同样由 024 单轨吸收。

## 决策

采用一个总 Program 连续完成生产就绪模板，不再逐 Program 等待确认。实施仍按可回滚的依赖检查点提交，最终以同一竣工矩阵验收，不允许检查点自行缩小最终目标。

### 1. 语言与平台

- 基线升级到 Go `1.26.5`，`go.mod`、CI、容器 builder 和 release toolchain 使用同一版本。
- 正式二进制支持 `windows/amd64` 与 `linux/amd64`；OCI image 首个正式范围为 `linux/amd64`。
- `.gitattributes` 固化 Go/module/YAML/Markdown 为 LF，Windows 与 Linux 的 `go mod tidy -diff` 必须同义通过。

### 2. 完整应用代际

- watched runtime configuration 采用不可变 `ApplicationGeneration`；同一 Snapshot 构造 Capabilities、Auth、Todo、Router、business HTTP、management route 与 exporter candidate。
- 进程级 `ListenerHub` 独占物理 listener；候选全部 Ready 后，在无 I/O 的 commit 点切换连接 admission。
- 在途请求固定旧代，旧代停止 admission 后排空；cleanup debt 保留 owner/generation 并 fail-closed。
- migration 与 release 配置不是 watched runtime state，不参与代际切换。

### 3. API authority

- 采用 **spec-first OpenAPI 3.0.3**，`api/openapi.yaml` 是 operation、schema、response、security 和兼容性的唯一事实源。
- 使用 `oapi-codegen v2.8.x` 的 strict Chi server 生成 transport DTO、server interface 与 route binding；生成代码提交到 Git，并由 `go generate + clean diff` 门禁验证。
- 选择 3.0.3 是因为当前稳定 `oapi-codegen` 对 3.1/3.2 仍不是正式支持；不得启用其实验 parser 冒充稳定 baseline。
- `operationId` 是 Router、权限、日志、trace、metrics、inventory 和 contract test 的统一低基数 identity。
- 使用 `oasdiff v1.22.x` 执行 breaking/changelog gate；当前手写 `module.Route` 与重复路径/权限事实在迁移完成后删除。

### 4. 协议与安全

- 所有公开错误使用 RFC 9457 `application/problem+json`；panic、404、405、validation、认证、授权、rate/overload 与未知错误同轨。
- project-owned `Principal`、`CredentialVerifier`、`Authorizer`、`Decision` 与 `AuditSink` 位于稳定应用边界；业务层不导入 JWT/JWK/OpenAPI/Chi 类型。
- production Adapter 使用 `github.com/lestrrat-go/jwx/v4` 校验 JWT/JWKS bearer token，强制 issuer、audience、允许算法、时间窗口与 key refresh；凭据缺失或 Adapter 未 Ready 时 protected operation fail closed。
- Todo 作为真实 actor 验收：记录 `owner_subject`，Service 在读取资源事实后执行对象级授权。现有数据通过显式 expand/backfill/contract migration，不猜测 production owner。
- development anonymous actor 只允许 `environment=development` 且业务 listener 为 loopback；production 配置禁止该模式。

### 5. 管理与观测

- 业务与 management listener 分离。management 提供 startup/live/ready、Prometheus metrics、build info 与脱敏 diagnostics；pprof 默认禁用，完整 diagnostics 受 management scope 保护。
- 使用 OpenTelemetry Go `1.44.x` 的稳定 trace API/SDK 与 OTLP/HTTP exporter；不引入仍为 Beta 的 OTel logs。
- 使用 `prometheus/client_golang v1.24.x` 暴露稳定 metrics。标签只允许 operation、method、status class、error class 与命名 dependency，不记录 raw path、subject、Todo ID 或异常文本。
- exporter failure 不阻断业务，但必须有有界队列、drop counter、shutdown flush budget 和自诊断。

### 6. 数据演进

- 使用 `github.com/golang-migrate/migrate/v4 v4.19.x` 与按 driver 分离、嵌入二进制的 versioned SQL。
- migration 由独立 `db migrate` command/job 拥有；Service 启动只检查 schema version/readiness，不再执行 GORM AutoMigrate 风格变更。
- SQLite、PostgreSQL、MySQL 各有 contract test；down migration 只用于测试和明确回滚 Runbook，不作为生产默认自动回退。

### 7. 交付与发布

- 使用 digest-pinned `gcr.io/distroless/static-debian13:nonroot` 运行 CGO-free binary，root filesystem 默认只读，SQLite/data path 显式挂载。
- GoReleaser `v2.17.x` 统一 binaries、archives、checksums 和 release metadata；Syft `v1.x` 生成 SPDX JSON SBOM；Cosign 生成 keyless bundle/signature。
- CI 增加生成物一致性、OpenAPI diff、test/race/vet/tidy/build、fuzz smoke、`govulncheck`、gosec、secret/artifact scan、数据库 contract、容器 smoke 与复制验收。
- 远端 push/tag/GitHub Release/GHCR/attestation 是外部副作用，只有后续一次性确认明确授权时执行；否则只生成和验证本地 release candidate。

## 不选择的方案

- 不选择 Huma code-first：会把第三方框架契约带入 Handler，并使当前项目边界和 API authority 同时迁移。
- 不选择 ogen：其静态 Router 会替换当前 Chi/runtime 边界，迁移面大于 strict Chi Adapter。
- 不手写 OpenAPI 或自研 schema generator；也不保留 Route/OpenAPI/权限三套清单。
- 不用启动时 AutoMigrate 作为 production migration。
- 不把 API key、固定 dev token、默认 public Todo 或仅网络隔离当成 production authentication。
- 不用 Docker `latest`、未固定 Action、未签名 artifact 或只有绿色 unit test 的结果声明 release-ready。

## 后果

该决策显著扩大一次施工的文件面和验证成本，但消除了 022 后续十二个 Program 与 023 reload 方案之间的重复迁移。代价由严格依赖顺序、检查点 commit、旧入口单轨删除和最终双副本验收控制。

## 重新确认触发器

以下任一事实出现时必须暂停并更新计划：

- 稳定版 `oapi-codegen` 无法表达当前 Todo 协议或 strict Chi 运行时；
- `ListenerHub` 原型无法在 Windows/Linux 证明 pending connection 无丢失；
- production actor 不是 JWT/JWKS bearer 可表达的 subject/scope 模型；
- 需要浏览器 cookie/session、mTLS、WebSocket、SSE、HTTP/3 或多租户数据隔离；
- migration 需要在线双写、跨库复制或不可逆生产数据操作；
- 首个 release 需要新增平台、真实云部署、私有 registry 或不同供应链系统；
- 需要重写 copy-owned 产品决策或发布 v2 public Go API。
