# R003：数据、交付与 release 技术栈选型

## 1. 当前问题

当前项目用 GORM Schema 在 Service startup 执行 Todo migration，CI 只有 Ubuntu Go 与数据库 contract，没有 Windows runtime、容器、artifact、SBOM、签名或正式复制验收。一次性竣工必须冻结工具链并定义可复核的发布证据，不能把这些选择留给最后一轮。

## 2. Go 与平台基线

[Go 官方下载页](https://go.dev/dl/) 在本次研究时列出 `go1.26.5` 为稳定版本；[Go 1.26 release notes](https://go.dev/doc/go1.26) 延续 Go 1 compatibility。024 选择 Go 1.26.5，以满足当前 Prometheus、OTel、GoReleaser 与候选 jwx 的受支持范围；jwx v4 额外的实验开关问题由 [R004](../R004-jwx-jsonv2-reassessment/report.md) 重新仲裁。

正式支持矩阵：

| Artifact | Platform | 验收 |
| --- | --- | --- |
| binary archive | `windows/amd64` | test/build、service/CLI、SQLite 文件锁、copy acceptance |
| binary archive | `linux/amd64` | test/race/build、signal drain、filesystem permission、copy acceptance |
| OCI image | `linux/amd64` | nonroot/read-only、probe、migration job、SIGTERM、smoke |

不在首个正式 release 声称 macOS、arm64 或 Kubernetes 已支持。新增平台需要真实 runtime evidence，不用 cross-build 外推。

`.gitattributes` 固化文本 LF；Windows checkout 必须能直接通过 `go mod tidy -diff`。工具版本通过 `go.mod` tool dependencies、Action SHA/tag 与 release config 明确 pin，不使用运行期 `@latest`。

## 3. migration 选择

[`golang-migrate/migrate`](https://github.com/golang-migrate/migrate) v4 是受支持稳定线，v4.19.1 提供 PostgreSQL、MySQL 与 SQLite driver，CLI 支持 lock timeout，library 提供 graceful stop 和 driver-level lock 契约。

024 采用：

- `migrations/sqlite`、`migrations/postgres`、`migrations/mysql` 三套明确 SQL dialect；
- 同一 migration version 与业务语义在三套文件中保持一致；
- `embed.FS` 作为 release source，禁止运行期依赖工作目录中的松散 SQL；
- 独立 `db migrate up|version` command，读取与 Service 相同的严格 Database config；
- Service 只执行 current/dirty/expected version readiness；不自动 up/down；
- lock timeout、总 command deadline、dirty state、原始 error chain 与脱敏 DSN；
- production 不暴露默认 `down` 或静默 `force`，修复由 Runbook 和显式 repair 流程承担。

Todo 增加 owner subject 需要 expand/backfill/contract：先增加 nullable column/index，显式 backfill legacy owner，再收紧 not-null。存在旧行且没有 backfill subject 时 migration 必须失败，不能猜测 production owner。

## 4. 容器选择

[`distroless`](https://github.com/GoogleContainerTools/distroless) 官方提供 `static-debian13:nonroot`、CA certificates、tzdata、多架构 manifest 与 keyless signature。当前 Go/SQLite 栈可保持 `CGO_ENABLED=0`，因此选择该 runtime image。

实施要求：

- multi-stage build，builder 与 runtime image 都 pin digest；
- vector-form ENTRYPOINT；
- UID/GID nonroot；
- root filesystem read-only，`/tmp` 使用受控 tmpfs；
- SQLite/data/storage 使用显式 writable volume；
- business 与 management port 分离；
- healthcheck 不依赖 shell；由外部 container smoke 请求 management endpoint；
- image labels 写入 version/revision/source/licenses；
- build secret 不进入 layer、argument、environment 或日志。

容器不是必选部署平台，但它是本项目 release 的可重复运行与供应链验收载体。

## 5. release 工具链

### GoReleaser

[GoReleaser](https://goreleaser.com/customization/sign/sign/) v2.17.x 统一生成 Windows/Linux archive、checksum 与 release metadata，并能调用 Cosign 生成 bundle。配置只描述构建与打包，不替代项目测试或迁移验收。

### SBOM 与签名

- [Syft](https://github.com/anchore/syft) v1.x 对 binary/archive/image 生成 SPDX JSON SBOM；
- [Cosign](https://github.com/sigstore/cosign) 对 checksum blob 与 OCI digest 生成 keyless signature/bundle；
- consumer 必须实际执行 `cosign verify-blob`/`cosign verify`；只有生成没有验证不算门禁通过；
- GitHub artifact attestation 可在权限和 repository plan 支持时作为附加证据，不是唯一发布正确性来源。

### 版本

- 首个竣工候选以 `v1.0.0-rc.1` 验证；
- 两副本与全部 acceptance 通过后才允许 `v1.0.0`；
- release notes 包含支持平台、配置/DB migration、API breaking baseline、Todo 保留/删除、来源 commit、known limits 与 rollback；
- copy-owned 项目没有自动上游 merge，后续修复通过新 release、安全公告和人工迁移说明传播。

## 6. CI 与安全门禁

竣工 CI 至少包含：

1. format、tidy、test、race、vet、build；
2. Windows/Linux matrix 与 PostgreSQL/MySQL/SQLite contract；
3. OpenAPI validate/generate clean diff/oasdiff；
4. focused fuzz smoke；
5. [`govulncheck`](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) reachable vulnerability gate；
6. gosec、secret scan、tracked artifact allowlist；
7. container build、nonroot/read-only、business/management、SIGTERM、migration failure smoke；
8. release snapshot、checksum、SBOM、signature verification；
9. preserve-Todo 与 remove-Todo 两个 isolated copy acceptance。

Action 必须固定 major 或 commit SHA，permissions 最小化，fork PR 不获得 release credentials。release job 只由受保护 version tag 触发。

## 7. 环境与外部副作用

当前本机没有 Docker 和可运行 WSL，因此不能在本计划阶段声称 container/Linux runtime 已验证。未来一次性实施有三条合法路径：

1. 用户先提供本地 Docker/WSL；
2. 后续确认允许使用 GitHub Actions，push 后以远端 CI 取得证据；
3. 在另一明确授权的隔离 Linux/OCI 环境执行。

如果以上都没有，024 只能停在 `release-candidate-local`，不能声明正式竣工。远端 push/tag/Release/GHCR 也必须由后续确认明确授权。

## 8. 结论

研究门禁通过。技术栈、支持平台、migration 模型、容器边界和供应链证据已足够明确，可写入一个连续施工计划。当前环境缺口是未来验收条件，不阻止完成计划，但不能在实施时被 cross-build 或未运行的 workflow 冒充通过。
