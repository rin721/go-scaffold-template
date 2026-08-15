# 需求：底层闭环与成熟 Go HTTP API 脚手架门禁

## 1. 目标

定义两层停止线：

1. 当前底层何时形成配置、装配、资源、运行、重载、排空、停止、诊断和验证的真实闭环，从而允许后续业务模块进入详细设计；
2. 项目何时可以诚实声明为成熟 Go Server HTTP API 后端脚手架。

当前剩余范围与取舍由 [R008](research/R008-remaining-foundation-closure/report.md) 支撑；R002/R004 已被它取代并保留为三项 P0 实施前历史。外部实践继续由 [R003](research/R003-go-runtime-practices/report.md) 支撑，逐资源 Close/retry/force 语义由 [R005](research/R005-resource-finalization-policy/report.md) 支撑，R006/R007 保存 diagnostics/config 实施前快照；整体 HTTP 差距继续由 [R001](research/R001-current-readiness-reassessment/report.md) 与 [019-R002](../019-http-api-maturity-gap-assessment/research/R002-http-api-maturity-reference/report.md) 支撑。生命周期、统一 diagnostics 与配置确定性的施工级契约和证据分别由已实施的三个计划负责；剩余 Foundation requirement 由 [`FOUNDATION-CLOSURE-001`](plans/foundation-closure-001.md) 单轨负责。

## 2. 标签定义

- **Foundation-partial**：主链和正常路径存在，但至少一个失败/终止/诊断责任没有可验证终态。当前 synchronous HTTP/CLI profile 已越过该状态。
- **Foundation-closed**：配置、显式装配、资源 owner、start/ready/run/reload/drain/stop、错误、诊断和故障验证构成闭环。
- **Business-design-unlocked(profile)**：Foundation-closed 已通过，且具体业务需求完全落入已经验证的能力 profile；只解锁设计，不授权实现。
- **Copy-ready**：固定 release 可在支持平台完整复制、迁移 identity、保留/移除示例并独立演进。当前部分达到。
- **Production HTTP API-ready**：协议、安全、管理、遥测、数据演进、交付和兼容保证均有默认实现或明确 fail-closed 接入点，并由复制副本和部署验收证明。当前未达到。
- **Scenario-ready**：消息、调度、租户、搜索等特定能力由真实业务验收，不属于默认 baseline。

只有 Foundation-closed、Copy-ready 与 Production HTTP API-ready 同时通过，才允许使用“成熟 HTTP API 后端脚手架”标签。

## 3. Foundation-closed 必需要求

### 3.1 配置输入与一致性

- `FND-CONFIG-001`：File/Env/未来 Source 的同源和跨源 object/non-object 形状冲突必须确定性拒绝，不依赖枚举顺序；null 属于 non-object，不得与 object 静默改形状。
- `FND-CONFIG-002`：Loader 仍是 Coordinator 唯一调用路径；同一 immutable candidate 必须被全部 Kernel 和 application owner 严格校验后才能 Build。
- `FND-CONFIG-003`：unknown field、exact/case-fold duplicate key、空 path segment、跨类型弱转换、未知顶层 section、秘密脱敏和 provenance 的现有保证不得回退；配置错误不得输出原始 value。
- `FND-RECONCILIATION-001`：preflight-only `RestartRequired` 不得与 committed cleanup debt 共用永久 latch。后续候选只有在重新加载、全部 owner 校验且相对当前有效 generation 不再要求重启，并完整通过现有 reload 事务后才能清除 restart 状态；degraded/cleanup-required 仍 fail-closed。

### 3.2 DI、装配与边界

- `FND-ASSEMBLY-001`：`cmd/app -> internal/composition` 保持唯一 application composition root；Bootstrap、Service、CLI 的资源与长期任务边界明确。
- `FND-ASSEMBLY-002`：业务对象继续手工显式装配；Kernel Plan 保持 typed、forward-only、freeze-once，不引入 locator、扫描、反射容器或第二套生命周期 owner。
- `FND-ASSEMBLY-003`：业务 core 依赖使用方契约；资源只由 owner 构造并经 stable Access/Lease 暴露，调用方不得自行创建第二套 client/connection。

### 3.3 生命周期与资源清理

- `FND-LIFECYCLE-001`：terminal drain 立即阻断新借用、等待活跃借用，但 timeout 后不能把未完成清理伪装成 stopped。
- `FND-LIFECYCLE-002`：initial start compensation、candidate discard、previous-generation cleanup 和 terminal cleanup 失败时，必须保留 owner、generation、phase、实例责任和原始 error chain，直到明确 finalized/failed/forced 终态。
- `FND-LIFECYCLE-003`：`NoFinalization`、drain 后 terminal close、graceful shutdown、graceful-to-force、可重试 finalization 和 process forced exit 是不同场景策略；不得用一个万能 `Close(options)`、无限等待或默认强关活跃资源合并语义。
- `FND-LIFECYCLE-004`：一个 generation 只能成功终结一次，每个 attempt 至多执行一次；只有 Adapter 证明再次调用会真实补做安全步骤时才允许多个有编号 attempt。多项清理失败继续聚合，不能因前一项失败跳过可安全执行的后续清理。
- `FND-LIFECYCLE-005`：提交前 reload 保持 all-or-nothing；提交后的 cleanup failure 诚实 degraded，不能伪装 rollback，也不能丢失 cleanup debt。
- `FND-LIFECYCLE-006`：构造在分配资源后失败时，必须完成补偿或把残余 handle、owner、phase 和 error chain 显式转交生命周期治理；不得以 nil instance 加 error 遗忘部分成功资源。
- `FND-LIFECYCLE-007`：能力停止服务、finalization 被调用、物理句柄释放和 release verification 通过分别建模；重复 Close 返回 nil/`ErrClosed` 不能作为释放证明。
- `FND-LIFECYCLE-008`：统一层只治理 owner/generation/admission/drain/budget/state/diagnostics；Database、Redis、logger、storage、HTTP、watcher 等由窄 Adapter 选择场景策略。Go 实现使用策略多态，不使用同名重载或含义不清的 bool 参数。

### 3.4 Supervisor、readiness 与诊断

- `FND-RUNTIME-001`：长期工作只由 `Supervisor.Task` 或后续等价窄契约拥有，必须接收 context、可等待退出并有稳定 owner name。
- `FND-RUNTIME-002`：Participant 与 Task 的 start/ready/run/drain/stop 顺序共享 process-level 总预算；各层只消费剩余 budget。忽略 context 的责任方必须进入 pending diagnostics。
- `FND-DIAGNOSTICS-001`：安全快照至少表达 state、ready、generation/digest、phase、owner、scenario policy、attempt、pending units、restart/cleanup required、verification result 和 error type，不输出原始配置或凭据。
- `FND-DIAGNOSTICS-002`：只有全部责任已 finalized，或明确进入 failed/forced 终态，才可报告 stopped；错误日志只在决定策略的边界记录。
- `FND-DIAGNOSTICS-003`：运行中人工 retry/force 只允许经 owner 进程内受控 operation 执行；独立 CLI 进程只能作为未来 management transport client，并且必须按 owner/generation 鉴权、幂等和审计。

### 3.5 治理与验证

- `FND-GOV-001`：保留 production import graph、composition bypass、module core 与第三方边界检查；新增路径不得只靠文档约定。
- `FND-GOV-002`：建立正常、部分构造失败、ready 失败、reload reject、drain timeout、terminal close error、重复调用不重做 terminal attempt、显式 HTTP force、调用方最终释放、uncooperative participant/task 的确定性反向测试；当前无已证明 retryable 资源，因此不得用假 Adapter 冒充 retry 验收。
- `FND-GOV-003`：Service 与 CLI 对同一 participant/resource 的启动、反向停止、错误聚合和最终清理不变量一致。
- `FND-GOV-004`：验证至少包含状态机单元、Kernel/Supervisor/Host 集成、真实可关闭资源所有权、race/vet/build 和文档/架构 gate；未执行项如实报告。

## 4. 业务设计解锁要求

- `BIZ-UNLOCK-001`：第 3 节和 [acceptance.md](acceptance.md) 的 Foundation 阻断项全部通过并刷新 R002/R004。
- `BIZ-UNLOCK-002`：每个新模块先定义 actor、use case、outcome、error、data owner、transaction、config、resource、HTTP/CLI、background、ready/health 和 stop 需求。
- `BIZ-UNLOCK-003`：需求落入已验证 profile 才能进入 Handler/Service/Repo/Model 详细设计；新 Runner/Health/shared resource/reload policy 必须先完成独立底层研究与计划。
- `BIZ-UNLOCK-004`：模块不访问 Kernel/Container/第三方 client，不私起不可等待 goroutine，不通过全局状态或事件转发隐藏依赖。
- `BIZ-UNLOCK-005`：通过只授权设计；源码实现仍需该业务任务计划报告后的后续确认。

## 5. 成熟 HTTP API baseline

### 5.1 产品、版本与可移植性

- `BASE-001`：copy-owned 是唯一消费模型；复制指南明确 tracked baseline、identity 映射、Todo 保留/删除和独立 Git 历史。
- `BASE-002`：发布固定 tag/version、provenance、支持平台、兼容政策、安全公告和人工迁移说明。
- `BASE-003`：Windows 与 Linux 对 module、行尾、build、test、race、vet、config init 和 identity 扫描给出同一成功语义。
- `BASE-004`：仓库和副本不携带凭据、运行数据、本机缓存、`.git` 或未声明 writable path。

### 5.2 API authority 与协议

- `API-001`：只选择 spec-first 或 typed code-first 一条 authority；Router、OpenAPI、权限、inventory 和 contract tests 使用同一 operation identity。
- `API-002`：Operation 表达稳定 ID、method/path、版本/弃用、schema、错误、安全、reliability 和 observability metadata。
- `API-003`：建立 lint、生成/一致性校验、breaking/operational diff 与删除门禁；transport DTO 不进入领域 core。
- `PROTO-001`：严格定义 Content-Type/Accept、body/unknown/trailing、validation、HEAD/204 和 response commit。
- `PROTO-002`：业务错误、validation、panic、404、405 和 middleware failure 使用同一 machine-readable problem presenter。
- `EDGE-001`：trusted proxy、client IP、CORS/CSRF、request budget、header/body limits、rate/overload 有 owner、配置、安全默认和诊断。

### 5.3 安全、管理与观测

- `SEC-001`：每个 operation 显式 public/protected；遗漏政策在生成、构建或启动前失败。
- `SEC-002`：项目自有 Principal/Policy/Decision/Audit 边界不泄露第三方 claims；对象级授权使用真实资源事实。
- `SEC-003`：credential Adapter 只随真实 actor 选择；受保护 operation 缺 Adapter 时 fail closed。
- `MGMT-001`：startup/liveness/readiness 语义分离，允许命名依赖贡献，并有 timeout/cache/concurrency 与停止时序。
- `MGMT-002`：独立受控 management listener 暴露 probes、metrics、build info、脱敏 diagnostics；pprof 显式保护。
- `OBS-001`：入站、出站和后台工作传播 trace，日志关联 request/trace ID，指标只用低基数 operation/route identity。
- `OBS-002`：exporter failure 不阻断业务，但有有界队列、丢弃、自诊断和敏感属性 allowlist。

### 5.4 数据演进与交付

- `DATA-001`：production migration 使用 version/checksum/owner-lock/bounded step/durable result 与独立 command/job。
- `DATA-002`：定义 schema/app compatibility、expand/backfill/contract、失败前滚和最小权限；本地 auto-migrate 是显式 profile。
- `DELIVERY-001`：可重复 build metadata、最小非 root 容器、只读边界、probe/termination/resource 示例与 smoke。
- `DELIVERY-002`：CI 覆盖 test/race/vet/tidy/build、contract diff、fuzz smoke、`govulncheck`、secret/artifact scan。
- `DELIVERY-003`：release 产出 checksum、SBOM/签名、notes、兼容声明和 rollback Runbook，并在隔离环境验证。

## 6. 非目标

- 不恢复插件 Runtime，不引入运行时万能 Resolver、Fx/Wire 或通用 DAG；若未来真实规模推翻此判断，必须新研究和 ADR。
- 不删除已验证 reload 来绕过 cleanup bug，也不把所有配置字段改成热路径可变状态。
- 不开发 generator；copy-owned 决策保持有效。
- 不在没有真实 actor 时选择 JWT/OIDC/session/RBAC 产品。
- 不预装消息、调度、分布式锁、邮件、搜索、租户或特性开关。
- 不要求所有消费者采用 Kubernetes、某个云、APM 或 API gateway。

## 7. `FOUNDATION-CLOSURE-001` 验收

- R008 已检查既有 metadata，并明确 R002/R004 的 supersede 与其他研究的继续复用边界。
- `FND-RECONCILIATION-001` 只解除无副作用 preflight latch，不自动恢复 degraded、cleanup debt 或 terminal failure。
- requirements/design/acceptance/tasks 对唯一 `FOUNDATION-CLOSURE-001`、current-profile 标签和业务解锁条件一致。
- 实施只覆盖 `FCL-001..008`，不新增依赖、公共 API、外部服务、后台 recovery 或 HTTP 产品能力。
- 目标测试、full test/race/vet/build/cross-build、文档链接、metadata、Diff 和提交范围门禁通过；未执行平台或外部场景如实报告。
