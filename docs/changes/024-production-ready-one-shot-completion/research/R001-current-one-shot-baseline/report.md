# R001：当前一次性竣工基线

## 1. 研究问题与快照

本报告以 `e251b73518a457ec97c529d067ddfffe77be203a` 为代码基线，回答 Foundation 闭环之后，“一次性竣工”还必须迁移哪些真实代码边界、为什么不能把 022 的十二个 Program 随意并行实现，以及当前环境能否直接证明最终 release。

方法是从 `cmd/app -> internal/composition -> module/http binding -> httpx/health/database -> workflow/release assets` 追踪 authority、运行 owner、错误协议、认证、观测、迁移、构建和验收。并行未跟踪的 023 仅做只读检查，没有修改、暂存或提交。

## 2. 已确认事实

### 2.1 Foundation 已完成，但完成范围有限

022 当前权威结论是 `Foundation-closed(current synchronous HTTP/CLI profile)`。配置候选、显式装配、资源 owner、reload、drain、stop、typed diagnostics 与故障测试已经闭环；这不等于 HTTP product、release 或新运行 profile 已完成。

### 2.2 当前 HTTP 仍有多份人工事实

- `internal/module.Route` 只持有 method、path、Handler 与 middleware，没有 operation ID、schema、response、security 或兼容元数据。
- `internal/composition.applicationRouter` 手工安装模块 Route 和全局 middleware。
- `pkg/httpx.DefaultErrorHandler` 输出自有 `{error,message}` JSON；`BindJSON` 没有 strict unknown/trailing/empty-body 契约。
- 仓库没有 OpenAPI、生成器、operation inventory 或 breaking diff gate。

因此当前 Router、handler DTO、错误码、文档和未来权限清单会成为多份 authority，不能在此基础上直接叠加 Swagger 注释或另一套 registry。

### 2.3 安全和 edge 只有局部工具，不是产品政策

- 已有 RequestID、Recovery、SecureHeaders、简单 CORS、BodyLimit 与全局 token bucket。
- 没有 Principal、CredentialVerifier、Authorizer、对象级资源事实、Audit、protected/public operation 分类或 fail-closed startup。
- 限流没有 actor/route key、容量诊断、`Retry-After`、代理信任或过载并发政策。
- Todo 没有 owner subject，所有 HTTP operation 当前公开。

局部 middleware 可以复用实现思想，但不能被描述为 `SEC-001..003` 或 `EDGE-001` 已通过。

### 2.4 Health 与 diagnostics 没有管理传输

`Host` 已有 in-process `Health()` 和唯一 `ProcessDiagnostics`，却没有独立 management listener、startup/live/ready endpoints、metrics、build info、保护后的 diagnostics 或 pprof policy。将这些端点挂到业务 Router 会混淆暴露面和生命周期 owner。

### 2.5 migration 仍属于服务启动副作用

Todo `Migrator.Start` 通过 `database.Client.Migrate` 在业务 ready 前执行 additive schema 变更。当前没有 version/checksum、dirty state、跨进程 lock、独立 command/job、expand/backfill/contract 或 schema/app compatibility gate。

这满足本地示例启动，不满足 production migration；竣工必须把 migration 从 Service startup 单轨迁出。

### 2.6 CI 和 release 资产不完整

当前唯一 workflow 已覆盖 Ubuntu test/race/vet/tidy/CGO-free build，以及 PostgreSQL/MySQL database contract。仓库没有：

- Windows job 与两平台同义 manifest；
- generated/OpenAPI/breaking gate；
- fuzz smoke、`govulncheck`、gosec、secret/artifact scan；
- Dockerfile、container smoke、build metadata；
- release config、checksum、SBOM、签名、attestation 与 rollback Runbook。

### 2.7 当前环境不足以本地证明最终竣工

- 本机是 Go `1.25.7 windows/amd64`；
- `docker` command 不存在；
- WSL 没有可运行 distro；
- `gh 2.93.0` 可用，remote 是 `git@github.com:rin721/go-scaffold-template.git`；
- 当前分支 `main` 相对 `origin/main` ahead 2。

所以计划阶段可以完成代码事实研究；将来若不安装 Docker/WSL，就必须通过经授权的 GitHub Actions 获得 Linux runtime、container 和 release evidence。cross-build 不能替代 runtime。

## 3. 并行 023 对单轨方案的影响

当前 worktree 有未跟踪 `023-full-configuration-seamless-reload/` 草案。其研究指出 HTTP/Todo/Cache 位于当前 Kernel reload 事务不同层级，完整配置无感生效需要不可变 Application Generation 与进程级 ListenerHub。

这与 024 有直接依赖：如果先实现 API/Auth/Management，再单独实现 023，会重复改造 Router、HTTP Server、Host、配置 owner 和 diagnostics。正确顺序是：

1. 024 实施开始先确定 023 是否已有确认 commit；
2. 已实施则以其 commit 为基线，不重复施工；
3. 未实施则由 024 吸收等价要求，023 不再拥有第二套施工 authority；
4. 任何路径都先完成 Application Generation，再迁移产品 HTTP 能力。

024 不修改或暂存该未跟踪目录，因为它属于其他工作。

## 4. 事实与推断分离

### 事实

- Foundation current profile 已通过；Production HTTP API-ready 未通过。
- 当前没有 OpenAPI/auth/management transport/OTel/versioned migration/release assets。
- 当前迁移、Route 和错误协议的真实位置已由代码确认。
- 当前 CI 与本机环境不足以证明双平台/container/release。

### 推断

- 一次性竣工应使用一个总计划，否则各 Program 会在 Router、config、Host、CI 和 docs 上反复迁移。
- 运行时代际迁移必须位于 API/Auth/Management 之前，才能让新增配置和 listener 只迁移一次。
- 最终验收需要外部 CI 或新增本地容器环境；是否执行远端发布仍需当前指令授权。

## 5. 结论

研究门禁通过。剩余工作不是一个小型 HTTP middleware 任务，而是一个有确定依赖的产品化迁移。它可以在一次后续确认后连续完成，但必须保留内部检查点、单轨删除和最终总验收，不能以“一个大 commit”替代工程闭环。
