# R001：当前 HTTP API 脚手架成熟就绪度复核

## 1. 研究问题与口径

本报告回答：完成 copy-owned 产品形态和 repository identity 迁移后，当前项目是否已经达到“复制后即可作为成熟 Go Server HTTP API 后端起点”的程度。

成熟度按可验证保证判定，不按包数量判定：一个能力只有同时拥有唯一 owner、稳定契约、配置和安全默认、失败与生命周期语义、诊断及验收证据，才算模板基线已经提供。存在工具函数、README 意向或单元测试，不等于 production composition 已获得该保证。

外部基线复用仍有效的 [019-R002](../../../019-http-api-maturity-gap-assessment/research/R002-http-api-maturity-reference/report.md)。本轮重点复核当前实现快照，不重复抄录 RFC、OpenAPI、OpenTelemetry、Kubernetes、OWASP 与 Go 官方资料。

## 2. 快照、方法与验证

- 代码快照：`main@fa349ab95a66ab641304dd9bfb31993d65cc04a6`。
- 环境：Go 1.25.7、Windows/amd64、CGO enabled。
- 路径：`README -> cmd/app -> internal/composition -> internal/module -> pkg/httpx -> Kernel/Host -> migration -> CI/release assets`。
- 搜索：OpenAPI/Swagger、Problem Details、Principal/authn/authz、management probes、metrics/trace、pprof、容器、部署、release、SBOM 和 vulnerability gate。
- 动态验证：`go test ./...`、`go test -race ./...`、`go vet ./...`、`go build ./...` 通过。
- 可移植性验证：`go mod tidy -diff` 返回 1；`git ls-files --eol` 显示 `go.mod` 与 `go.sum` 为 index LF、working tree CRLF，diff 是 241 行行尾归一化，不是依赖内容变化。

没有启动长期服务、连接远程 Database/Redis/S3、执行容器或部署，也没有修改依赖文件。

## 3. 019 之后真正改变了什么

### 3.1 已解决

- 020 已把产品形态单轨确定为 copy-owned source scaffold，并在忽略目录验证保留 Todo 和移除 Todo 的两个 Windows 副本。
- 021 已把 remote、Go module/import、应用品牌和当前文档统一为 `go-scaffold-template`。
- 当前 `go test`、race、vet 和 build 在 canonical identity 下通过。

### 3.2 尚未改变

从 019 快照到当前 HEAD，生产 Go 文件的实质变化是 module/import/应用 identity 迁移，没有新增 HTTP contract、安全、management、telemetry、migration 或 delivery 行为。因此 019 的主要 API 缺口仍然成立；只有“产品形态尚未决定”已经由 020 关闭。

## 4. 代码事实与成熟度判定

| 平面 | 当前事实 | 判定 | 阻塞成熟标签的原因 |
| --- | --- | --- | --- |
| 进程与资源 | Coordinator、Kernel、Supervisor、HTTP listener ready、反向 Stop、degraded/restart-required 已实现 | 通过 | 无；这是应保留的底座 |
| 模块边界 | Contribution 验证 module/route/participant 唯一性，Todo 有 HTTP/CLI/DB 闭环 | 通过 | 只有一个真实业务模块，跨模块政策体验尚未证明 |
| Product/identity | copy-owned ADR、两个隔离副本和 canonical module 已完成 | 部分通过 | Linux、正式复制指南、release/tag/provenance 未完成 |
| API authority | `module.Route` 只有 Method、Path、Handler、Middlewares | 未通过 | 无 operation ID/schema/security/version/deprecation、OpenAPI 和兼容 diff |
| 请求/错误协议 | `BindJSON` 与 `{error,message}` 可用，Todo 手工映射业务错误 | 未通过 | 严格 JSON、统一 problem、404/405、validation details 和提交边界未统一 |
| Edge policy | production 只启用 Recovery、RequestID、AccessLog、SecureHeaders | 未通过 | CORS/BodyLimit/RateLimit 只是局部工具；无 trusted proxy、request budget、route policy |
| 身份与授权 | production 无 Principal、authentication、authorization 或 audit | 未通过 | 无显式 public/protected policy 和对象级授权闭环 |
| 管理面 | Host 内有 liveness/readiness Snapshot，注释明确不创建管理端点 | 未通过 | 编排系统不可消费；无 startup、dependency health、diagnostics/build info 或访问边界 |
| 可观测性 | 有结构化日志和 request ID | 未通过 | 无 trace、metrics、propagation、低基数 route identity、SLO 或 exporter owner |
| 数据演进 | Todo 在 Start 中执行 additive `Migrate` | 未通过 | 无 version/checksum/lock、独立 migrate job、backfill、expand-contract 和多副本语义 |
| Quality/delivery | CI 有 test/race/vet/tidy/build 及 PostgreSQL/MySQL contract | 部分通过 | 无 contract/fuzz/vuln/container/supply-chain/release/smoke；Windows tidy 行尾失败 |

## 5. 为什么“测试全绿”仍不能推出成熟

当前测试充分证明现有代码在声明范围内可靠，但它们没有可测试的 API authority、认证策略、外部 probe、telemetry、版本化 migration 或部署产物。测试只能证明已经定义的契约，不能替未定义的契约作保证。

同理，Todo 能在本地 SQLite 中完成增删改查，证明垂直切片真实可运行；它不能证明多副本部署、公开 API 兼容、零停机 schema 演进、安全边界或 release 回滚已经解决。

## 6. 当前可用边界

### 6.1 可以使用

- 学习显式 composition、配置、生命周期、资源租约和模块结构；
- 快速验证内部原型或受控单体服务；
- 由有能力补齐安全、协议和交付门禁的团队，作为新项目的基础源码；
- 在不声称 production-ready 的前提下复用 Todo 或按 020 清单移除示例。

### 6.2 不应直接使用

- 复制后直接发布公网 API；
- 把 `/api/v1`、中间件函数或全绿单测当成 API 兼容和安全保证；
- 多副本生产环境中让每个实例自动执行 schema migration；
- 需要 Kubernetes/负载均衡器 probe、SLO、审计、身份授权或供应链证明的交付；
- 承诺 Linux、容器、release 或上游安全迁移已经验证。

## 7. 实现计划影响

1. 020 已关闭产品形态决策，不再研究 generator 或外部 Runtime library。
2. 第一条关键路径仍是 API authority：先比较 spec-first 与 typed code-first，再单轨建立 Operation/OpenAPI/compatibility。
3. Windows 行尾可重复性是独立且低耦合的交付缺口，可与 API authority 研究并行，但必须单独计划和确认。
4. 错误、edge policy、身份政策与 telemetry 都应消费同一个 operation identity；不能先各建一份 metadata。
5. management 和 versioned migration 可以独立研究，但其 production 实施必须与 deployment/release 验收汇合。
6. 成熟标签必须由两个独立复制副本、Windows/Linux、可部署产物和失败场景共同验收，不能在当前仓库单测通过后提前宣布。

## 8. 局限与刷新条件

- 本轮没有做容量、渗透、漏洞、容器、Linux、远端服务或真实部署测试。
- 只有 Todo 一个真实领域，认证 actor、租户和出站 API 仍不存在，不能据此选择具体产品。
- 当前没有 template release/tag，无法验证消费者从正式版本复制和追踪安全公告的流程。
- Route、错误、安全、management、migration、delivery 或首个 release 发生变化时必须刷新本报告。

## 9. 研究结论

研究门禁通过。当前项目的进程底座已经明显高于示例仓库，但整体成熟度被 API 产品治理和生产交付的系统性缺口阻塞。结论不是“推倒重来”，而是保留现有 composition/lifecycle 边界，按单一 API authority 驱动协议、安全、管理、观测、迁移与 release 门禁逐层闭环。
