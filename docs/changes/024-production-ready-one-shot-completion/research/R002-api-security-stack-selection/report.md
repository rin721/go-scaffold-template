# R002：API、安全与观测技术栈选型

## 1. 研究问题

022 要求先选择唯一 API authority，再建立协议、安全、管理与观测。024 不能把这项选择留到实施中途，因此本报告比较稳定工具与当前 Chi/显式组合边界的适配性，并冻结可直接施工的技术栈。

研究只使用官方规范、官方源码、官方文档和 release metadata。HTTP/OpenAPI/Problem Details/健康语义继续复用有效的 [019-R002](../../../019-http-api-maturity-gap-assessment/research/R002-http-api-maturity-reference/report.md)。

## 2. API authority 方案比较

| 方案 | 当前事实 | 项目影响 | 结论 |
| --- | --- | --- | --- |
| Huma typed code-first | 官方支持 OpenAPI 3.1 和多种 Router | Handler/输入输出直接采用 Huma 契约，第三方框架进入模块 transport API | 不选 |
| ogen spec-first | 生成静态 Router、校验、client/server，热路径无 reflection/`any` | 会替换现有 Chi 与 `pkg/httpx` Router，模块迁移和 middleware 重写面最大 | 不选 |
| oapi-codegen strict Chi | OpenAPI 3.0 stable；生成 strict server、DTO、Chi binding；保留业务实现 interface | 可保留 Chi、显式 composition 和 project-owned use case，只替换 transport authority | **选择** |
| 自研 typed Operation generator | 可以完全贴合现有类型 | 重复造 schema/parser/generator/breaking diff，维护与安全责任不可接受 | 拒绝 |

[`oapi-codegen`](https://github.com/oapi-codegen/oapi-codegen) 官方说明 stable 主线支持 OpenAPI 3.0，3.1/3.2 仍等待或位于实验 parser；strict server 生成 typed request/response interface，但请求验证仍需官方配套 middleware。v2.8 还强化 spec validation，并建议把生成代码提交 Git、审阅生成 diff，降低恶意或错误 spec 生成代码的风险。

因此冻结：

- `api/openapi.yaml` 使用 OpenAPI `3.0.3`；
- `oapi-codegen v2.8.x` 生成 strict Chi server、models 和 embedded spec；
- 官方 net/http validation middleware 在业务 Handler 前执行 schema/security requirement 解析；
- 生成代码只位于 transport adapter，不进入 Service/Repo/Model；
- spec 是 trusted repository input，禁止运行期加载远端 spec；
- `go generate` 后必须 clean diff，生成文件与工具版本一同审阅。

选择 3.0.3 是稳定性取舍，不声称已经使用最新 OpenAPI 3.2。上游正式支持 3.1/3.2 时再研究单轨迁移，不在本任务启用实验 parser。

## 3. Operation 与兼容性

每个 operation 必须在 spec 中声明稳定 `operationId`。该 ID 同时驱动：

- generated server method；
- public/protected 与 scope；
- error/problem response；
- access log、trace span、metric labels；
- inventory、contract test 和 deprecation；
- release breaking/changelog。

[`oasdiff`](https://github.com/oasdiff/oasdiff/blob/main/docs/BREAKING-CHANGES.md) 官方命令可以在 CI 中对两个 spec 执行 `breaking`/`changelog` 并按严重级别失败。024 选择 v1.22.x，第一份生产 spec 作为兼容 baseline；后续变更默认阻断 `ERR`，`WARN` 必须由有截止日期的兼容记录显式批准。

旧 `module.Route`、手写路径安装和另一个权限清单必须在迁移完成后删除；不能让 generated Router 与旧 Route 双轨长期存在。

## 4. 协议选择

- 使用 [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457.html) `application/problem+json`，项目定义稳定 problem type URI、code、status、title、detail、instance、request/trace ID 和 validation extensions。
- strict decode 明确处理 Content-Type、Accept、空 body、body size、unknown fields、trailing values、query/path/header 参数和重复值。
- 404、405、panic、schema validation、authentication、authorization、rate limit、overload 和 dependency failure 全部经过同一 presenter。
- response commit 后的 encode/stream error 只记录诊断，不能再伪造第二个 problem response。
- DTO 由 generator 拥有，business command/result 继续由模块拥有；Adapter 做显式转换。

## 5. 身份与授权选择

> 2026-08-15 复核：本节的安全契约继续有效；`jwx v4` 版本选择因其强制 `GOEXPERIMENT=jsonv2` 已由 [R004](../R004-jwx-jsonv2-reassessment/report.md) 取代，用户已确认方案 A `jwx v3.2.0`。

项目不能只放一个解析 token 的 middleware。024 冻结以下项目自有语义：

```text
Credential -> CredentialVerifier -> Principal
Principal + Action + ResourceFact -> Authorizer -> Decision
Decision + Outcome -> AuditSink
```

- `Principal` 只暴露 subject、actor kind、scopes 和认证时间，不暴露第三方 claims map。
- `CredentialVerifier` 负责 bearer extraction、签名、issuer、audience、允许算法、expiry/not-before 与 JWKS refresh。
- `Authorizer` 由业务使用方需求驱动；Todo Service 读取资源后以 owner subject 做对象级决定。
- `AuditSink` 记录 operation、actor subject 的受控标识、resource type/id 的策略化表达、decision 和 outcome；不得记录 token 或任意 claims。

[`lestrrat-go/jwx`](https://github.com/lestrrat-go/jwx) 提供 JWT/JWK/JWS 与 JWKS cache 能力；第三方类型只在 `internal/module/auth/adapter/jwt` 内出现。原报告只识别到 v4 要求 Go 1.26，遗漏其强制实验性 `GOEXPERIMENT=jsonv2` 的条件，因此不得继续使用本段作为 v4 已确认依据。具体 module owner 由 [R005](../R005-security-module-ownership/report.md) 修订。

production 必须配置 issuer、audience、JWKS URL 和算法 allowlist。development anonymous actor 只是显式本地 profile：只允许 development 环境与 loopback listener，production 配置在 Build 前拒绝。

## 6. Management 与 observability

### 6.1 Management

- 独立 listener，不复用 business Router；
- `/startupz`、`/livez`、`/readyz` 语义分离；
- `/metrics` 使用 Prometheus exposition；
- `/build` 只暴露 version/commit/build time/dirty flag；
- `/diagnostics` 返回脱敏 `ProcessDiagnostics` 子集并要求 management scope；
- pprof 默认不注册，显式启用时同样受保护。

### 6.2 Traces、metrics 与 logs

[`opentelemetry-go`](https://github.com/open-telemetry/opentelemetry-go) 当前 traces/metrics 为 stable，logs 为 Beta。024 只引入稳定 trace SDK 与 OTLP/HTTP exporter；现有 zap 日志通过 context 关联 request/trace ID，不引入 OTel logs。

[`prometheus/client_golang`](https://github.com/prometheus/client_golang) v1.24.x 支持 Go 1.25/1.26。metrics 使用专用 Registry，禁止 default global registry；labels 只允许低基数 operation/status/error/dependency，遵循 Prometheus 对高基数标签的官方警告。

exporter failure 不影响业务返回，但 queue、drop、retry、flush deadline 和最后 error type 必须进入 diagnostics。禁止把 raw path、Todo ID、subject、token、query 或 error string 作为 metric/span attribute。

## 7. 适用边界与局限

该选择适用于当前 JSON REST、Chi、同步 HTTP/CLI、JWT/JWKS 和单体 copy-owned template。它不覆盖 cookie/session、OAuth login redirect、opaque token introspection、mTLS、WebSocket、SSE、HTTP/3 或多租户隔离；出现这些需求必须重新研究。

研究门禁通过。API、安全、管理和观测的技术选择已经足够具体，可进入同一个 024 施工计划，不需要实施中途再次比较框架。
