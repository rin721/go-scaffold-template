# R008：C8 隔离复制验收

## 研究问题

固定 C7 后的 tracked source commit，分别创建“保留 Todo”和“移除 Todo”两个独立项目时，能否完成真实 identity migration、模块边界验证、Windows 全量门禁和 source-commit 可追踪 local RC？哪些 Production-ready 门禁仍不能由当前环境证明？

## 固定输入

- source commit：`3339305783a82ca2a36bd27c4961adeb435eb3ff`；
- source archive：`tmp/scaffold-copy-validation-024-source.tar`；
- archive SHA-256：`bd0058296692e96b19837e91d4e78ff2808cc8f791bd4f3a39b7a1366eb4655e`；
- 两个副本都位于被 Git 忽略的 `tmp/scaffold-copy-validation-024/`，各自重新初始化 Git，不继承 source `.git`、remote、缓存、配置、数据或构建物；
- 目标 remote 使用不可写示例地址，只为本地 GoReleaser 解析目标仓库 identity 和 commit；没有 fetch、push、tag、release 或外部写入。

## 保留 Todo 副本

- module：`example.com/acme/task-service`；应用与 binary：`task-service`；环境前缀：`TASK_SERVICE_`；
- target commit：`21a50051efdc063fff89c6a3feb02a4a880481f5`；
- Todo、Auth、Ops、migration、strict OpenAPI transport 与 composition 均保留，业务能力仍由 `internal/module/*` 拥有；
- `Verify-Quality.ps1` 完整通过，包括 gofmt、tidy diff、生成 clean diff、test、race、vet、CGO-free build 和 artifact allowlist；
- `govulncheck` 可达漏洞 0，`gosec` issue 0，`gitleaks` leak 0；
- local RC `1.0.0-rc.1-local.21a5005` 生成 Windows/Linux amd64 archive、SPDX JSON SBOM、SHA-256 checksum、Cosign bundle/signature 并反向验证；metadata 的完整 commit 与 target commit 一致；
- Windows archive 中的 `task-service.exe --help` 成功，且保留 `config`、`db`、`todo` 命令。

## 移除 Todo 副本

- module：`example.com/acme/platform-service`；应用与 binary：`platform-service`；环境前缀：`PLATFORM_SERVICE_`；
- target commit：`11c9954fdbf48c3a18dd067a3f5355909438a373`；
- 删除 Todo module、Todo transport/spec、CLI、配置、migration set、composition、测试与当前说明；OpenAPI 当前路径为空；Auth 与 Ops 仍作为应用模块保留；
- 当前源码与当前文档扫描无 Todo 业务残留；`docs/changes` 中的历史 Todo 证据按 copy-owned 历史保留，不作为当前入口；
- `Verify-Quality.ps1` 完整通过，且 `govulncheck` 可达漏洞 0、`gosec` issue 0、`gitleaks` leak 0；
- local RC `1.0.0-rc.1-local.11c9954` 生成并验证同义 archive、SBOM、checksum 和签名证据；metadata 的完整 commit 与 target commit 一致；
- Windows archive 中的 `platform-service.exe --help` 成功，只保留 `config` 命令，没有 `db` 或 `todo` 入口。

## 过程中发现并修正的复制约束

GoReleaser 在没有 target remote 时无法从 Git commit 形成完整 repository metadata；把 target owner/name 硬编码进源配置会把源身份泄漏到副本。单轨规则因此是：副本不继承 source remote，完成 identity migration commit 后配置目标项目自己的 remote，再制作需要 commit 映射的 local RC。复制指南已同步该顺序。

两个副本并行执行 Windows race 时曾因系统盘临时链接空间不足失败；错误来自 linker `No space left on device`，不是源码或测试失败。将 `GOCACHE`、`GOTMPDIR`、`TEMP` 和 `TMP` 定向到工作盘后串行重跑，两份完整门禁均以退出码 0 结束。该失败不被删除或改写为首次通过。

## 未通过项与结论

本机没有 Docker，也没有可运行 WSL/Linux 与 PostgreSQL/MySQL 测试实例；用户同时禁止 push 和外部系统操作。因此下列门禁仍未执行：Linux 原生 runtime、OCI nonroot/read-only/SIGTERM/image scan、PostgreSQL/MySQL migration contract、远端 CI 与 keyless external attestation。

所以 C8 已证明 `ACC-REQ-005` 的 Windows quality/security/local archive 路径和 `ACC-REQ-006` 的 local RC 路径，但没有证明完整的 `ACC-REQ-001..006`。当前标签继续保持 `Foundation-closed(current synchronous HTTP/CLI profile)`；`Copy-ready(windows/amd64 + linux/amd64)` 与 `Production HTTP API-ready` 仍不得标记通过。
