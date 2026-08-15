# 设计：保留现有架构的底层加固与 HTTP API 成熟化路线

## 1. 设计结论

当前架构不需要容器级重构。目标是在原有主链上补齐失败终态，再继续 HTTP API 产品治理：

```text
File/Env config
  -> Loader + all owner validation
  -> application composition root
       -> forward-only Kernel Plan -> stable Access/Lease
       -> local application modules
  -> Coordinator + Host + Supervisor
  -> start -> ready -> run
  -> reload: prepare -> drain -> commit -> previous cleanup
  -> stop admission -> graceful drain
       -> finalized
       -> cleanup-pending/failed/forced (owner + generation + policy)
  -> diagnostics + verification
  -> business-design unlock by proven profile
  -> API authority / protocol / security / management / delivery
```

`cleanup-pending`、terminal-failed 与 forced 的生命周期最小语义已经由 [`FOUNDATION-LIFECYCLE-001`](plans/foundation-lifecycle-001.md) 实施；跨 Kernel/Supervisor diagnostics 数据源和实施前缺口由 [R006](research/R006-unified-runtime-diagnostics/report.md) 复核，施工级契约与结果由已实施的 [`FOUNDATION-DIAGNOSTICS-001`](plans/foundation-diagnostics-001.md) 唯一负责。[R005](research/R005-resource-finalization-policy/report.md) 保留逐资源研究快照，本设计不复制施工正文。

## 2. 继续保留的当前边界

- `cmd/app` 只做模式选择、信号 context、baseline logger 和顶层错误退出。
- `internal/composition` 是业务对象、HTTP、CLI、Host 和未来场景 task 的唯一 application composition root。
- `internal/kernel/app` 管共享底层资源的构造、ready、stable access 和 generation；业务模块不进入 Kernel Plan。
- Kernel Plan 保持 typed、forward-only、freeze-once；不引入扫描、locator、反射 DI 或动态插件图。
- `pkg/supervisor` 管 Participant 与长期 Task；Kernel 管资源代际。二者按 owner 层次组合，不合并为万能生命周期。
- `internal/module` 保持局部纯内存装配与最小完成品 Contribution。能力不够时先研究，不在模块内部绕过。
- `pkg/httpx` 隔离 chi/net/http；业务 Service 不依赖 transport、ORM、telemetry SDK 或身份 SDK。
- copy-owned 项目拥有全部源码，不依赖源模板 Runtime，也不允许 generator 覆盖消费者修改。

## 3. Phase F0：清理责任与真实终态

### 3.1 已确认的设计输入

[R005](research/R005-resource-finalization-policy/report.md) 已确认：

- `Close` 幂等不等于失败后可重试。Database、Redis、logger、fsnotify 当前都是 terminal attempt，重复 Close 不会可靠补做失败步骤。
- HTTP 独有 `Shutdown -> Close` 的 graceful-to-force 路径，但 force 会中断 active connection，且不覆盖 hijacked connection。
- 当前 Kernel StorageManager 的 local/S3 client 不拥有需关闭的底层句柄，应归类为 no-finalization；Clock/ID/Validator/I18n 等纯值同样不需要 Close。
- 最后一个 active Lease 释放后，仍持有 generation 的 owner 必须保留责任；冻结计划选择由后续同 owner `Stop(ctx)` 继续收敛，不创建无独立预算的后台队列。只有未来资源 Adapter 明确证明 retryable 时，才允许另立变更新增有界重试。
- 当前 CLI 是独立进程，不能直接清理服务进程内资源；未来 CLI 只能调用 owner 进程内受控 management operation。
- Supervisor/composition 应拥有一个总 shutdown budget，各层只消费剩余预算；冻结的代码默认是总计 10 秒、最后 1 秒保留给显式 force。第二次信号和 deployment grace period 与该默认值的部署关系仍待部署研究。

### 3.2 必须满足的不变量

- 终止开始后不恢复 admission。
- active use 未结束时不默认强关。
- cleanup error 不清空仍负有责任的实例引用。
- state 不能从“仍需 cleanup”直接跳到 stopped。
- cleanup owner、generation、phase、scenario policy、attempt、retryability/force policy 和 error type 可诊断。
- 一个 generation 只能成功终结一次；每个 attempt 至多执行一次。只有经 Adapter 证明的 retryable 策略允许多个有编号 attempt；多资源 cleanup 尽最大安全努力并聚合错误。
- 业务能力不可用、Close 已调用、物理句柄已释放和释放验证通过必须分别表达。
- 构造在分配资源后失败时，补偿必须成功，或把残余 handle 和责任显式转交生命周期 owner；不能只返回 nil instance 加 error 后遗忘资源。

### 3.3 可选实现路线

| 路线 | 优点 | 风险 | 判断 |
| --- | --- | --- | --- |
| Stop 超时立即 force close | 有界、实现直观 | 可能破坏活跃 request/transaction | 不能作为通用默认 |
| Stop 超时标 stopped 并遗忘 | 简单 | 当前 bug；资源和 goroutine 无 owner | 拒绝 |
| 无限等待全部释放 | 保证最终 cleanup | 破坏部署终止预算 | 拒绝通用默认 |
| 一个通用 `Close(options)` | 表面 API 少 | bool/枚举组合掩盖资源差异，容易误用 force/retry | 拒绝 |
| 场景化策略 + 统一 owner/state 引擎 | 不同资源语义明确，同时复用治理与诊断 | 需要逐资源 Adapter 和契约校验 | **推荐** |

### 3.4 场景化关闭策略

统一引擎不直接定义一个万能 `Close()`，而是编排以下互斥场景：

| 场景策略 | 适用对象 | 操作语义 | 当前资源 |
| --- | --- | --- | --- |
| `NoFinalization` | 纯值或没有独占句柄的 Adapter | 只完成 owner 状态转换，不调用空 Close | Clock、ID、Validator、I18n、Todo migrator、当前 local/S3 StorageManager |
| `DrainThenTerminalClose` | 关闭动作会把对象置为终态，错误后不能可靠重做 | 先关闭 admission 并排空，再执行一次 terminal attempt；失败保留责任与诊断 | Database、Redis、configured/baseline logger、fsnotify watcher |
| `GracefulShutdown` | 协议自身提供可等待的优雅停止 | 使用总预算的剩余 context，等待运行循环确认结束 | 无需 force 的 Participant/Task |
| `GracefulThenForce` | 协议明确提供有损 force，且产品政策允许 | graceful 失败后单独记录 force decision/result | HTTP Server；hijacked connection 需另有 owner |
| `RetryableFinalize` | Adapter 能证明再次执行会重做未完成安全步骤 | owner 保留引用，执行有界、有编号、有退避的 retry | 当前没有已证明实例，不能为未来场景预设启用 |

这些名称是上层场景分类，不会机械地变成一组公共接口。当前计划只为已证明对象实现：Kernel 的“无 finalizer/一次 terminal finalizer”，以及 Supervisor 的 graceful Stop/HTTP 可选 ForceStop；`RetryableFinalize` 当前没有实例，因此只保留扩展门禁，不预建 retry framework。具体标识符以 [`FOUNDATION-LIFECYCLE-001`](plans/foundation-lifecycle-001.md) 第 4、5 节为唯一权威。

### 3.5 目标状态机

```text
serving
  -> drain-pending
       -> finalizing
            -> finalized
            -> cleanup-pending -> finalizing   only retryable policy
            -> terminal-failed
            -> force-pending -> forced | force-failed
```

- 从 `drain-pending` 起拒绝新借用，但不能据此宣称物理资源已释放。
- caller deadline 到期只结束该次等待，不得自动丢弃 owner；总 process budget 与最终退出政策另行决定。
- terminal-failed 不是成功，也不允许 raw Close retry；它表示已无安全重做能力，但清理结果必须持续可诊断。
- `finalized`、`forced` 和 `terminal-failed` 是不同终态，readiness、退出码和审计不得合并。

### 3.6 构造、运维 operation 与验证

- Database 的网络 Ping 应保留在 Kernel candidate `Ready`，避免构造器分配 pool 后 Ping 失败、Close 又失败却只能返回 nil instance 的隐藏窗口。
- 多资源构造必须在 ownership transfer 前聚合补偿；若补偿可能失败且需要持续 owner，构造契约必须能显式转交 cleanup debt，不能依赖 `errors.Join` 后丢引用。
- 最后一个 Lease 释放只唤醒同一 drain 等待，不创建脱离总预算的后台 finalizer。caller timeout 后由同一 owner 的后续 `Stop(ctx)` 继续收敛；未来 `FinalizePending(owner, generation)` 也只能调用 owner 进程内受控 operation，CLI 只是其鉴权 transport client，不直接操作内存实例。
- release verification 按资源定义：数据库连接/文件所有权、Redis pool/goroutine、logger sink 文件、HTTP listener/Serve done、fsnotify channel/handle。第二次 Close 无错误不构成验证。

## 4. Phase F1：诊断、Supervisor 与配置确定性

### 4.1 结构化诊断

当前实现证据入口为 [`FOUNDATION-DIAGNOSTICS-001`](plans/foundation-diagnostics-001.md)。它冻结并已实现 Host 单一 process snapshot、Kernel/Supervisor typed responsibility ledger、budget、terminal classification、release verification 和并发读取门禁；本节只保留 Program 级目标。

在不泄露配置内容的前提下，目标诊断统一回答：

- 当前 state/phase 和 ready；
- current generation/digest/provenance；
- restart required 与 cleanup required 是否成立；
- pending owner 是 capability、participant 还是 task；
- 对应 generation、首次发生时间、error type 与最终 policy。

日志只在 Coordinator/Supervisor/顶层 process 等决定策略的边界记录；底层继续只返回带链错误。

### 4.2 Supervisor

- Participant Stop 与 Task Run 都必须有稳定 name 和同一总 shutdown budget。
- uncooperative Task/Participant 都进入 PendingUnits；返回不等于声称 goroutine 已消失。
- 不尝试实现 Go 不支持的 goroutine 强杀。必要的 force 行为只能由具体资源/协议 owner 提供，或交给进程监督器退出。

### 4.3 EnvSource

`setNested` 应与 Loader 的结构冲突原则一致：同一 EnvSource 的 scalar/object、object/scalar、空 path segment 和大小写碰撞必须确定性拒绝。具体实现应复用一个有错误返回的结构写入规则，避免为 Env 单独形成第二套语义。

## 5. Phase F2：故障验收与 Foundation 复核

测试必须先证明失败语义再声明闭环：

1. active Lease -> Stop timeout -> new Use rejected -> active Use releases -> finalization succeeds once；
2. current/candidate/previous generation 的 cleanup error 保留 owner 和 error chain；
3. 当前 terminal finalizer 失败后不重复 raw Close；测试证明 cached terminal-failed 与 future retryable 扩展门禁明确分离；
4. uncooperative participant/task 均出现在 diagnostics；
5. Service cancellation、runner failure、watcher failure、CLI operation failure 的 reverse Stop 一致；
6. EnvSource shape collision 在 Build 前失败；
7. 真实可关闭资源证明旧/当前 generation 不残留文件锁或连接 owner。

完成后按 [acceptance.md](acceptance.md) 逐门复核。只有 Foundation-closed 通过，才进入 Phase B。

## 6. Phase B：按 profile 解锁业务模块设计

### 6.1 当前已证明 profile

- 同步 HTTP request/response；
- 一次性 CLI operation；
- 启动阶段同步 Participant，例如 migration；
- 通过项目自有契约消费现有共享 capability；
- 模块局部纯内存构造 Handler/Service/Repo/Adapter。

### 6.2 未证明 profile

- 后台 consumer、scheduler、stream/long polling；
- 异步 warm-up 与独立 ready；
- 模块动态 health contribution；
- 新共享 client/resource 及其 reload；
- 多模块事务、事件或跨模块内部调用。

新模块需求落在未证明 profile 时，先建立独立 foundation capability research/change。不要提前把 `Contribution` 扩成包含任意 hook、map 或 `any` 的万能协议，也不要让 Participant.Start 私起不可等待 goroutine。

## 7. Phase 0：可重复 copy baseline

Foundation 加固后，建立不改变产品语义的复制与平台可重复性：

- 明确行尾政策，使 Windows/Linux `go mod tidy -diff` 同义；
- 固化 validation manifest 和支持平台；
- 建立正式复制指南、identity 清单和 release provenance；
- 保留 Todo 与删除 Todo 两条验收路径。

## 8. Phase 1：API authority 决策

只比较两条路线：

1. spec-first：OpenAPI 为 authority，生成 transport DTO/server contract/client/contract tests；
2. typed code-first：项目自有 typed Operation 为 authority，生成 OpenAPI、route catalog、政策矩阵和 tests。

用 Todo 全部 operation 做隔离原型，比较 nullable/enum/error/security、生成稳定性、breaking diff、IDE、client 和 copy identity。ADR 单轨选择，替换当前 Route 权威，不保留手写 Route、OpenAPI 和权限清单三套事实。

## 9. Phase 2：协议与 edge policy

以 Operation 为输入建立 strict decode/validation、统一 problem、404/405/panic/middleware 同轨错误、request budget、trusted proxy、CORS/CSRF、rate/overload，以及分页/条件更新/幂等的显式 opt-in 契约。Middleware 只执行已编译政策，不运行期任意查询 registry。

## 10. Phase 3：管理面与可观测性

由 composition 拥有 health registry，Kernel capability、application participant 和真实 module 在 listener 前贡献命名 Check。独立受控 management listener 提供 startup/live/ready、metrics、build info 和脱敏 diagnostics；pprof 只显式启用并保护。

项目自有窄观测契约表达 operation/status/duration/bytes/dependency/error class；OpenTelemetry 留在 Adapter。日志、trace、metrics 共享低基数 operation identity。

## 11. Phase 4：安全政策

Operation 显式 public/protected；遗漏分类 fail closed。真实 actor 出现后再研究 Credential Adapter、Principal、Policy/Decision、对象级授权和 Audit。第三方 Token/claims 不进入业务契约。

## 12. Phase 5：数据演进、交付与 release

- production migration 使用独立 versioned command/job、checksum、lock、bounded step 和 durable result；
- build 注入 version/commit/time，容器非 root、只读优先；
- CI 增加 contract diff、fuzz smoke、`govulncheck`、secret/artifact scan；
- release 产出 checksum、SBOM/签名、兼容声明和 rollback Runbook；
- 两个正式 release 隔离副本在 Windows/Linux 和部署场景完成验收。

## 13. 依赖顺序

```text
FOUNDATION-LIFECYCLE-001
        -> FOUNDATION-DIAGNOSTICS-001
        -> FOUNDATION-CONFIG-001
        -> FOUNDATION-ACCEPTANCE-001
        -> BUSINESS-DESIGN-UNLOCK

FOUNDATION-ACCEPTANCE-001 -> MANAGEMENT-001 -> OBSERVABILITY-001
FOUNDATION-ACCEPTANCE-001 -> API-AUTHORITY-001 -> API-CONTRACT-001
PORTABILITY-001 -------------------------------> RELEASE-001
API-CONTRACT-001 -> PROTOCOL-001 / EDGE-001 / SECURITY-001
MIGRATION-001 ---------------------------------> DELIVERY-001
all baseline ---------------------------------> ACCEPTANCE-001
```

`FOUNDATION-CONFIG-001` 可在生命周期研究期间并行研究，但实现与最终验收必须与 Foundation 状态一致。业务详细设计不能与 Foundation P0 并行越过解锁门。

## 14. 关键风险与控制

| 风险 | 后果 | 控制 |
| --- | --- | --- |
| 用新 DI 容器掩盖 cleanup bug | 迁移面扩大，失败语义仍未定义 | 保留 composition，只修 owner/state/test |
| Stop timeout 直接强关 | 活跃请求或事务损坏 | 两阶段 drain，资源专用 force policy |
| cleanup error 后只记日志 | 引用和责任丢失 | pending cleanup state + structured diagnostics |
| 把 Close 幂等当成 retryable | 重复调用不补做释放，却误报恢复 | 默认 terminal attempt；retry 必须由 Adapter 证明并显式选择 |
| 用一个 `Close(options)` 覆盖所有场景 | no-op、graceful、force、retry 语义互相污染 | 统一状态治理 + 互斥场景策略 |
| 无限等待 finalization | 部署无法终止 | graceful budget 与 process exit policy 分离 |
| 业务模块私起 goroutine | Supervisor 无法等待和诊断 | profile gate；先扩展窄 Runner 契约 |
| 提前扩成万能 Contribution | 抽象无真实调用方 | 真实需求先做 capability assessment |
| 直接实现 Swagger 注解 | Router/文档/权限多权威 | API authority 原型和 ADR 先行 |
| 一次提交全部 P0 | 难审查、难回滚 | 022 内独立施工计划、稳定任务 ID、逐项确认 |

## 15. 未决项

1. 部署平台 SIGTERM 总预算、第二次信号和非零退出政策；
2. HTTP hijacked connection 是否进入 baseline，以及由谁治理；
3. 外部 PostgreSQL/MySQL/Redis/fsnotify close error 的真实物理释放结果；
4. sticky RestartRequired 是否允许有效配置恢复后解除；
5. 第一个新业务模块是否需要 Runner/Health/new resource；
6. API authority 的 spec-first/typed code-first 选择；
7. management listener、认证、migration artifact、支持平台和签名方案。

R005 已消解“当前资源应选择哪类关闭策略”的研究未知；生命周期契约已经在 [`FOUNDATION-LIFECYCLE-001`](plans/foundation-lifecycle-001.md) 冻结，第 1 至 3 项的真实结果仍需其故障验证或部署研究证明。第 5 项决定是否需要扩展 module contract；后续项分别阻塞 HTTP 成熟化阶段。022 不替它们虚构答案。
