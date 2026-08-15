# R004：jwx v4 与 jsonv2 生产基线复核

## 1. 触发事实

C4 实施前复核 `github.com/lestrrat-go/jwx/v4 v4.2.0` 的官方源码与实际 module 后确认：v4 不只要求 Go 1.26，还要求所有 `go build`、`go test`、`go run` 与 `go generate` 调用设置 `GOEXPERIMENT=jsonv2`。未设置时 Go 会因 `encoding/json/v2` build constraints 排除全部实现而失败。

R002 只记录“v4 要求 Go 1.26”，遗漏了这个全局实验开关。当前仓库 C1、C2、C3，现有 GitHub Actions、后续容器/release 计划以及 copy-owned 使用说明均未声明 `GOEXPERIMENT=jsonv2`。因此直接 `go get jwx/v4` 会实质改变整个模板的构建和复制契约，不是 C4 内部实现细节。

## 2. 当前证据

- jwx v4.2.0 官方 `jwx.go` Requirements 明确要求 Go 1.26+ 和每次命令设置 `GOEXPERIMENT=jsonv2`。
- 官方仓库 README 同样把 `GOEXPERIMENT=jsonv2` 列为 v4 requirement。
- 当前官方 Security Policy 把 v4 标记为 `preview`，同时明确 v3 仍在受支持版本中；v3.2.0 tag 自带的 Security Policy 也声明 v3 接收安全更新。
- 本机 Go 1.26.5 的 `go env GOEXPERIMENT` 为空；C1-C3 全部门禁均在无实验开关下通过。
- jwx v3.2.0 的官方 module 只要求 Go 1.25，源码与 module metadata 没有 `encoding/json/v2` 或 `GOEXPERIMENT` 要求。
- v3.2.0 的 `jwk.Cache` 已提供显式 `Register`、`Ready`、`Refresh` 与 `Shutdown`，并允许固定 HTTP client、响应体上限和受信 URL；能够满足 generation owner、初始 Ready、有界 refresh 与资源释放契约。默认 URL allow-all 不能直接采用，Adapter 必须从受控配置注册唯一 JWKS URL，并用自有 HTTP transport 限制 scheme、redirect、解析后的目标地址、超时与 body size。
- v3 的 `jwt.WithKeySet` 默认要求 token 与 JWK 的 `kid`、`alg` 匹配，且不会只信任 token header 推导算法；`WithIssuer`、`WithAudience`、`WithRequiredClaim`、`WithClock` 与 `WithAcceptableSkew` 足以实现当前 issuer/audience/subject/exp/nbf/iat 契约。Adapter 仍必须在进入 jwx 前校验配置的算法 allowlist，并禁止 infer/default-key 放宽选项。

以上结论只改变 JWT/JWKS Adapter 的第三方 major version，不改变 project-owned `Principal`、`CredentialVerifier`、`Authorizer`、`Decision`、`AuditSink`、issuer/audience/algorithm/time/key refresh/fail-closed 等安全语义。

## 3. 选项

### A. 改用 jwx v3.2.0（推荐）

- 保留成熟 jwx JWT/JWK/JWS 与受控 JWKS cache；第三方类型继续只位于内部 Adapter。
- v3 当前仍获得官方安全支持，而 v4 在当前安全策略中标记为 preview；选择 v3 不是降级到失去维护的旧版本。
- 保持普通 Go 1.26.5 命令、Windows/Linux、容器、release 与复制指南的现有契约。
- 代价是修改 ADR-003/R002 中已经冻结的 major version；未来 jsonv2 稳定且 v4 不再要求实验开关后再单轨升级。

### B. 保留 jwx v4.2.0 并采用 jsonv2 实验工具链

- 在所有本地命令、GitHub Actions、容器 builder、GoReleaser 和复制指南统一设置 `GOEXPERIMENT=jsonv2`。
- 必须补充整个仓库 JSON 行为、依赖兼容、两平台与复制副本回归，且生产标签要明确依赖实验特性。
- 这扩大 C1/C7/C8 范围，并提高复制使用者遗漏环境变量导致不可构建的风险，不推荐作为 production-ready 模板基线。

### C. 自研 JWT/JWKS

违反成熟技术优先和安全边界要求，拒绝。

## 4. 研究结论与门禁

用户于 2026-08-15 明确确认方案 A：使用 `jwx v3.2.0` 并继续实施 C4-C8。该依赖版本结论继续有效；随后 C4 因 package ownership 偏离当前 module 架构再次暂停，修订边界与确认门禁见 [R005](../R005-security-module-ownership/report.md)。方案 B 不进入当前构建、CI、容器、release 或复制契约。

本报告取代 R002 第 5 节中“直接选择 jwx v4”的版本结论，R002 的 OpenAPI、协议、安全契约和观测结论仍有效。
