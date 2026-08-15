# 任务：生产就绪模板一次性竣工

## 1. 门禁状态

- 024 研究门禁：C1-C3 证据见 `R001..R004`；C4-C6 module ownership 复核见 `R005..R006` 并已通过。
- 024 计划状态：C1-C7 已完成；C8 的两个隔离副本、Windows quality/security 与 local RC 已完成，Linux 原生 runtime、容器、PostgreSQL/MySQL 和远端 CI 仍未执行。R004 的 `jwx v3.2.0` 选择继续有效；Go 1.26.5 已由 R007 的安全证据单轨替换为 1.26.6。
- 当前标签：`Foundation-closed(current synchronous HTTP/CLI profile)`；Production HTTP API-ready 未通过。
- 当前授权：连续实施 `ONE-001..025`、本地与 CI 验证、检查点提交，以及临时本地容器/测试数据库。
- 当前禁止：push、tag、GitHub Release、GHCR、外部 attestation、真实部署与真实数据迁移。

## 2. 单轨任务账本

| ID | 工作量 | 依赖 | 内容 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| `ONE-001` | S | 用户确认 | 仲裁 023/024 与工作区 authority，固定 HEAD、用户修改和实施范围 | 只有 024 为当前施工 authority；可复用成果有 Commit/证据映射；用户修改不混入 | 已完成（C2） |
| `ONE-002` | M | `ONE-001` | 升级 Go 1.26.6、固定 LF 与 Windows/Linux validation manifest | go.mod/CI/builder/release 同版；两平台 tidy 语义一致；无 CRLF 漂移 | 实现完成；Windows 动态通过，Linux 待 C8 |
| `ONE-003` | M | `ONE-002` | 固定新增依赖、工具、Action、许可证与安全基线 | 版本与 checksum/commit SHA 可追踪；无未使用依赖；选型符合 ADR-003 | 已完成（C7；本地工具按官方 checksum 校验） |
| `ONE-004` | L | `ONE-003` | 建立稳定 file candidate、全 owner Validate/Build 与补偿账本 | 候选读取稳定；任何副作用前完成严格校验；部分构造无资源遗忘 | 已完成（C2/Windows） |
| `ONE-005` | XL | `ONE-004` | 实现 typed immutable `ApplicationGeneration` 与完整资源 owner | 同 Snapshot 构造现有七节及 auth/management/observability；无 locator/全局可变资源 | 进行中（七节基础完成） |
| `ONE-006` | XL | `ONE-005` | 实现 process-level `ListenerHub` 和 request/connection generation lease | Windows/Linux admission、切换、pending connection、drain、shutdown 测试通过 | 进行中（Windows 契约完成） |
| `ONE-007` | L | `ONE-006` | 迁移 reload/Host/Service/CLI 并删除旧 restart/server swap authority | candidate reject 保旧代；commit 后 cleanup debt 可诊断；旧路径/符号/文档无残留 | 已完成（C2 当前 profile） |
| `ONE-008` | L | `ONE-007` | 建立 OpenAPI 3.0.3 spec、oapi-codegen strict Chi 与 operation inventory | Todo contract 完整；生成物可重复；每个 operation security/Problem/operationId 完整 | 已完成（C3） |
| `ONE-009` | XL | `ONE-008` | 迁移 strict handler/Router/DTO mapping，删除 `module.Route` | 所有 HTTP 调用方只走生成 binding；core 无 transport/第三方类型；旧 authority 搜索为零 | 已完成（C3） |
| `ONE-010` | L | `ONE-009` | 实现 RFC 9457 presenter 与 strict request/response protocol | validation/404/405/panic/middleware/auth/dependency 统一；协议负向 contract 通过 | 已完成（C3；auth 场景随 C4 接入） |
| `ONE-011` | L | `ONE-010` | 完成 trusted proxy、budget、limits、CORS、rate/overload | 安全默认、429/503/Retry-After、取消与低敏诊断测试通过 | 已完成（C3） |
| `ONE-012` | XL | `ONE-008`、`ONE-005` | 在 `internal/module/auth` 收口安全契约、配置、middleware、operation policy、audit 与 jwx JWT/JWKS generation participant | issuer/audience/alg/time/key refresh/fail-closed 测试通过；无第三方类型泄漏；无顶层分散 security/adapter 包 | 已完成（C4，jwx v3.2.0） |
| `ONE-013` | XL | `ONE-010`、`ONE-012`、`ONE-016` | Todo 定义 actor/对象授权/audit 窄 port；composition 连接 Auth module；完成 `owner_subject` 与 CLI actor | expand/backfill/contract 完成；跨 actor 拒绝；HTTP/CLI 同 policy；敏感信息不泄露 | 已完成（C4） |
| `ONE-014` | L | `ONE-007`、`ONE-012` | 在 `internal/module/ops` 实现 management use cases/HTTP binding 与独立 process-owned listener | startup/live/ready 分离；management budget/scope 生效；pprof 默认不存在；无分散 management package | 已完成（C5） |
| `ONE-015` | XL | `ONE-009`、`ONE-014` | 在 Ops module 接入 OTel/Prometheus Adapter、middleware 与 generation contribution；process 保持 registry identity | 低基数/脱敏、传播、drop/flush/self-diagnostics、重复 generation 注册测试通过 | 已完成（C5） |
| `ONE-016` | XL | `ONE-003`、`ONE-005` | 以 `pkg/database/migrate` + `internal/module/migration` + Todo-owned migration set 建立三 driver SQL 与独立 CLI | fresh/incremental/idempotent/lock/dirty/version contract 通过；Todo owner migration 可执行；移除 Todo 无 SQL 残留 | 实现完成；SQLite 动态通过，PostgreSQL/MySQL 待 C8/CI |
| `ONE-017` | L | `ONE-016` | Todo migration binding 提供 schema compatibility，删除 startup AutoMigrate/Schema authority | service 不改 schema；too-old/too-new/dirty fail closed；旧接口/依赖/测试无残留 | 已完成（C6） |
| `ONE-018` | L | `ONE-002`、`ONE-014`、`ONE-017` | 可重复 build、build info、distroless nonroot image 与 runtime smoke | linux/amd64 image digest pin、read-only/mount/probe/SIGTERM/无 shell 验收通过 | 实现完成；image runtime 待 C8/CI |
| `ONE-019` | XL | `ONE-011`、`ONE-013`、`ONE-015`、`ONE-017`、`ONE-018` | 建立 generation、quality、security、DB、container CI | pinned Actions；所有 gate fail closed；无 continue-on-error/环境绕过 | 实现完成；Windows 本地门禁通过，未触发远端 CI |
| `ONE-020` | L | `ONE-018`、`ONE-019` | GoReleaser、Syft、Cosign、checksum 与 local RC 验证 | Windows/Linux artifact、SPDX SBOM、bundle/signature 与 build/source digest 对应 | 已完成（C7 local RC） |
| `ONE-021` | M | `ONE-020` | 完成 copy-owned 指南、identity manifest、migration/rollback/security Runbook | 干净环境可按文档执行；不依赖本机路径、未跟踪文件或隐式 secret | 已完成（C7） |
| `ONE-022` | L | `ONE-021` | 创建“保留 Todo”隔离副本并全门禁验收 | identity 迁移、三 DB、两平台/容器/API/security/release smoke 通过 | 部分完成：Windows quality/security/local RC 通过；Linux/容器/PostgreSQL/MySQL 未执行 |
| `ONE-023` | XL | `ONE-021` | 创建“移除 Todo”隔离副本并全门禁验收 | Todo 代码/路由/spec/config/migration/docs/dependency 无残留，模板仍完整通过 | 部分完成：当前实现残留扫描与 Windows quality/security/local RC 通过；Linux/容器未执行 |
| `ONE-024` | XL | `ONE-022`、`ONE-023` | 执行双平台、容器和完整失败矩阵总验收 | `ACC-REQ-001..006` 全有真实证据；未执行项不标通过 | 部分完成：Windows 本地矩阵通过；环境相关门禁未执行 |
| `ONE-025` | L | `ONE-024` | 同步唯一权威文档、清理旧轨、汇总 Commit 与发布结论 | 三标签一致；024 无残留 diff；local RC 或明确授权后的 remote release 可验证 | 进行中：未通过标签已同步；依赖总验收才能完成 |

## 3. 检查点与提交边界

| 检查点 | 任务 | 推荐 Conventional Commit | 必须通过后才能继续 |
| --- | --- | --- | --- |
| C1 | `ONE-001..003` | `build: align production toolchain` | tidy/build/governance/dependency audit |
| C2 | `ONE-004..007` | `feat(runtime): switch immutable application generations` | generation/listener/reload/race/failure tests |
| C3 | `ONE-008..011` | `feat(api): establish strict OpenAPI transport` | generated clean diff、contract、oasdiff、protocol/edge tests |
| C4 | `ONE-012..013` | `feat(security): enforce actor authorization` | JWT/JWKS、policy、Todo ownership/audit、migration compatibility |
| C5 | `ONE-014..015` | `feat(ops): add management observability` | probes/diagnostics/trace/metrics/exporter failure/race |
| C6 | `ONE-016..017` | `feat(database): adopt versioned migrations` | three-driver migration + schema readiness |
| C7 | `ONE-018..021` | `build(release): add verifiable delivery pipeline` | container/CI/local RC/checksum/SBOM/signature/docs |
| C8 | `ONE-022..025` | `docs(release): close production template acceptance` | two copies/two platforms/full failure matrix/authority/diff |

推荐 message 只表达 scope；实际提交前由 Conventional Commit 门禁根据真实 diff 校准。不得 squash 掉会妨碍回滚和证据定位的施工检查点，也不得把用户范围外文件带入。

## 4. 精确验证清单

### 4.1 每个检查点

```powershell
gofmt -l .
go mod tidy -diff
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/app
git diff --check
```

在 Linux 原生环境执行同义命令；`go test -race` 受平台支持限制时必须在 linux/amd64 CI 真实执行，不能删除该 gate。

### 4.2 生成、API 与安全

```powershell
go generate ./...
git diff --exit-code
govulncheck ./...
gosec ./...
go test ./... -run 'Contract|Protocol|Problem|JWT|JWKS|Authorization|Audit'
```

另执行固定版本 OpenAPI lint、`oasdiff breaking`、secret/artifact scan 与 bounded fuzz；命令最终落入仓库验证脚本/CI，不能依赖 Agent 临时记忆。

### 4.3 数据、容器与发布

- SQLite/PostgreSQL/MySQL：fresh up、incremental、重复 up、lock timeout、dirty、too-old/too-new、Todo expand/backfill/contract；
- container：nonroot、read-only、显式 writable mount、business/management probe、reload、SIGTERM、无凭据、image scan；
- release：GoReleaser snapshot/RC、archive smoke、checksum、SPDX SBOM、Cosign bundle/signature verify、build info/source Commit；
- copy：保留 Todo 与移除 Todo 两个隔离目录分别运行 Windows/Linux/DB/container/API/security/release gate。

### 4.4 文档与提交

- 检查 `docs/**/research/**/metadata.yaml` 必需字段、status/snapshot/refresh/supersedes；
- 检查 Markdown 相对链接，排除 fenced code 与 inline code；
- 搜索旧 Route、AutoMigrate、restart-only、旧标签、旧配置键、旧依赖与绝对路径；
- `git diff --check`、完整 diff、staged diff、staged file list 和 post-commit status；
- 只在所有 024 验收完成后将状态改为完成。

## 5. 逐轮证据

| 轮次 | 日期 | 完成任务 | 证据 | Commit | 剩余风险 |
| --- | --- | --- | --- | --- | --- |
| 1 | 2026-08-15 | `RES-001..003`、024 requirements/design/tasks | HEAD `e251b73518a457ec97c529d067ddfffe77be203a`；代码/配置/CI/平台审计；官方主源；ADR-003 | `97d081c` | 本机无 Docker/可运行 WSL；远端发布未授权 |
| 2 | 2026-08-15 | C1：Go 1.26.5、LF policy、Action SHA | Windows `go mod tidy -diff`、`go test ./... -count=1`、`go vet ./...`、`go build ./cmd/app`、`git diff --check` 通过 | `d587a2f` | builder/release 与 Linux 同义门禁在后续检查点补齐 |
| 3 | 2026-08-15 | C2 基础：`ONE-001/004/007`，`ONE-005/006` 当前七节与 Windows 契约 | stable file、七节单改、Todo/HTTP 同进程生效、资源复用、Schema/bind reject、watcher 恢复、旧请求固定、graceful/force 与 targeted race 通过 | `56ce851` | auth/management/observability 将在后续任务进入同一 generation；Linux runtime 留待 C8 真验收 |
| 4 | 2026-08-15 | C3：`ONE-008..011` 与 C2 diagnostics/listener 加固 | OpenAPI 3.0.3、oapi-codegen v2.8.0、oasdiff v1.22.0、strict Chi、operation inventory、RFC 9457、旧 Route 零引用；生成 hash 不变，Windows 全量 test/race/vet/build、protocol/edge contract、oasdiff self baseline、Diff 检查通过 | `86c2aca` | JWT/JWKS、operation authorization/audit 与真实 base breaking diff 在 C4/C7 接入；Linux runtime 留待 C8 |
| 5 | 2026-08-15 | C4 + C6 implementation：`ONE-012/013/017` 完成，`ONE-016` 三 driver 实现 | Auth module + jwx v3.2.0、OpenAPI operation policy、Todo actor/owner/跨主体隐藏、独立 `db migrate`、三 driver checksummed SQL、显式 legacy owner completion、startup read-only exact-version gate；Windows `go test ./...` 通过 | `2752110` | 本机无 Docker 且无可运行 WSL，PostgreSQL/MySQL 动态 migration/lock/dirty gate 必须由 C7 CI 真验收 |
| 6 | 2026-08-15 | C5：`ONE-014..015` | Ops module、独立 management listener、startup/live/ready、scope-protected diagnostics、build info、稳定 Prometheus registry、OTel 1.44 trace/OTLP HTTP、有界 drop/flush/self-diagnostics；业务/管理面隔离与重复 generation 测试通过 | `411ff86` | 外部 OTLP backend 未配置；exporter failure 使用受控 fake 动态验证 |
| 7 | 2026-08-15 | C7 实现与 Windows 本地证据：`ONE-002/003/018..021` | Go 1.26.6；Windows 全量 test/race/vet/CGO-free build、生成 diff、artifact allowlist 通过；`govulncheck` 可达漏洞 0、`gosec` 0 issue、`gitleaks` 0 leak；GoReleaser v2.17.1 配置通过，Windows/Linux amd64 archives、SPDX SBOM、SHA-256 checksum、本地 Cosign bundle/signature 生成并反向验证 | 见本行所在检查点提交 | 本机无 Docker 且无可运行 WSL；Linux runtime、container、PostgreSQL/MySQL 与远端 CI 未执行，必须在 C8 保持未通过状态 |
| 8 | 2026-08-15 | C8 Windows 隔离复制证据：`ONE-022/023` 本地部分与 `ONE-024/025` 状态同步 | source `3339305`、archive SHA-256 `bd005829...655e`；保留 Todo commit `21a5005` 与移除 Todo commit `11c9954` 均通过 quality/race/vet/CGO-free、安全扫描、Windows archive CLI smoke 和 source-commit 对应 local RC；临时 Cosign 私钥已删除；见 R008 | 见本行所在检查点提交 | Linux 原生 runtime、container、PostgreSQL/MySQL、远端 CI/keyless attestation 未执行；三项竣工标签不能升级 |

后续每个检查点必须补充命令、平台、退出码、artifact/digest、Commit 与未通过项。只有最终总验收可以把任务状态改为完成。

## 6. 当前授权边界

用户已经确认连续实施全部任务并允许临时本地容器与测试数据库。施工无需在检查点间重复确认；只有命中 ADR-003 的实质变更触发器才退回研究。push、tag、GitHub Release、GHCR、外部 attestation、真实部署与真实数据迁移保持禁止。未获远端发布授权时，施工终点是验证通过的 local `v1.0.0-rc.1` 与 clean 024 worktree。

## 7. 停止条件

- 用户尚未在本计划报告之后确认；
- 命中 ADR-003 重新确认触发器；
- 发现 023/用户修改无法可靠隔离；
- 必需 Linux/container/DB/release 验收环境不可获得；
- 任何安全、migration、artifact 或签名 gate 失败。

停止时保留已通过检查点和完整错误证据，不绕过 gate，不把未通过项改写为风险后宣布竣工。
