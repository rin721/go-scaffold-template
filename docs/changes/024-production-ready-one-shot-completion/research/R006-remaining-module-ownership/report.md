# R006：C5-C6 新增能力归属复核

## 1. 问题

R005 已确认 Auth 应按业务语义收口，而不是因为拥有第三方 Client/goroutine 就升级为 Kernel Capability。继续检查 C5/C6 后发现，原设计仍使用“management transport、OTel/Prom Adapter、migration contract/Adapter”等功能性名称，没有冻结唯一 package owner；若直接实施，会重复出现顶层 `internal/adapter/*`、`internal/migration`、composition glue 持续膨胀的问题。

## 2. 归属结论

### 2.1 C5：Ops application module

Management 与 observability 合并为一个 `internal/module/ops`，因为它们共同服务进程运维 actor、readiness/diagnostics、metrics/trace 与 build information：

```text
internal/module/ops/
├── model/                 probe、diagnostic、build view
├── service/               startup/live/ready/build/diagnostics 用例与窄 port
├── adapter/
│   ├── otel/              OTel provider/exporter
│   └── prometheus/        collector/registry adapter
├── middleware/            trace、metrics、operation correlation
├── binding/
│   ├── config/            management + observability 配置
│   └── http/              management handler/router
└── module.go              局部装配与 generation contribution
```

`pkg/health`、`pkg/logger`、`pkg/httpx` 继续作为既有底层能力被复用，不复制进 Ops。业务和 management 的物理 listener、稳定 Prometheus registry identity、generation admission 与 build-time ldflags 是进程 owner；composition 可以创建/持有这些完成品并注入 Ops，但不得实现 probe 规则、指标标签或 exporter 策略。

### 2.2 C6：Migration application module + Todo-owned SQL

Migration 分成两个有证据的 owner，而不是一个无语义的顶层 Adapter：

```text
pkg/database/migrate/                    # 跨模块、无业务 schema 的 golang-migrate 薄封装
internal/module/migration/               # db status/version/up CLI 用例、配置和 binding
internal/module/todo/binding/migration/  # Todo 三 driver SQL、版本清单、compatibility
```

- `pkg/database/migrate` 只处理 source/driver、lock、version/dirty、错误转换和关闭，不知道 Todo、CLI、配置文件或 service readiness。
- `internal/module/migration` 是显式命令 application module，组合一个或多个模块提供的 migration set；不参与 watched `ApplicationGeneration`，也不在 service 启动时自动执行 up/down/repair。
- Todo 表、`owner_subject` expand/backfill/contract、兼容窗口与 readiness 规则属于 Todo 模块。移除 Todo 的复制副本可以删除一个纵向目录及其 composition 选择，不需要搜索根 `migrations/` 的 Todo 残留。
- service generation 只调用 Todo 提供的 compatibility port；它不导入 `golang-migrate` 类型，也不通过旧 `database.Client.Migrate` 修改 schema。

### 2.3 C7-C8：不是业务模块的资产

Dockerfile、CI、GoReleaser、Syft/Cosign 配置、复制脚本和 release metadata 是仓库级交付资产，本来就不应塞进 `internal/module`。它们必须通过 manifest/script 明确 owner，并只消费构建产物或执行验证，不包含业务运行规则。

## 3. 统一边界表

| 能力 | 唯一语义 owner | 资源/lifecycle owner | composition 允许做什么 |
| --- | --- | --- | --- |
| Auth/JWT/Audit | `internal/module/auth` | Auth generation contribution | 注入 Clock/Logger、连接 Todo port、合并 contribution |
| Todo ownership | `internal/module/todo` | Todo generation/module participant | 注入 Database/Auth adapter、连接 HTTP/CLI |
| Management/OTel/Prom | `internal/module/ops` | Ops generation contribution；process 持有 listener/registry identity | 注入 diagnostics/Auth/Logger，连接 business/management middleware |
| Migration command | `internal/module/migration` | invocation Supervisor | 注入 Database migration engine 与模块 migration sets |
| Todo SQL/readiness | `internal/module/todo/binding/migration` | migrate command / generation readiness probe | 选择 migration set，连接 compatibility port |
| Delivery/release | 根级 manifest、workflow、script | CI/local release process | 构建、扫描、验收，不承载 runtime 业务规则 |

## 4. 门禁结论

R005/R006 共同修订 C4-C6 的 package owner，功能目标、`jwx v3.2.0`、OTel/Prometheus/golang-migrate 选型与 C7-C8 外部副作用禁令不变。因为 module boundary 与 migration asset placement 都发生实质变化，C4-C8 恢复非文档实施前需要用户对修订后的整体 ownership 明确确认。
