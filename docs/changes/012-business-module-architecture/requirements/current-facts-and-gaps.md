# 当前事实与能力缺口

## 1. 证据快照

- 复核日期：2026-08-14。
- 代码基线：`2daf47ad111141b27a1d8e100bb3d6e4cc1ea743`。
- 范围：`cmd/app`、`internal/kernel`、`internal/kernel/composition`、`internal/kernel/app`、`pkg/supervisor`、`pkg/httpx`、`pkg/health` 及相关测试。
- 本文是静态取证，不声称 listener、远程依赖或产品验收已经运行；详细源码证据见 [R011](../research/R011-current-foundation-closure-audit/report.md)。

## 2. 当前真实调用链

### 2.1 配置输入与入口

`cmd/app/main.go` 先创建 baseline Logger，再组合 `FileSource -> EnvSource` Loader、Kernel 和 `composition.Compose`。有参数时 CLI 在 Kernel Start 前运行；无参数时 Host 包含 Kernel 和只记录日志的 `applicationLifecycle`，可选配置 Watcher 是当前唯一长期 Task。默认进程没有 HTTP listener 或业务图。

Kernel 自己在 `Start`/`Reload` 内调用 Loader。该边界对仅有 Kernel 配置的当前实现一致，但未来 application 若另读 HTTP/模块配置会产生双读取；若不另读，application owner 又无从参与候选预检。

### 2.2 显式装配与资源创建

`internal/kernel/app.Plan` 使用显式 Add、typed Binding/Input、Replace 和 Freeze。composition 依序建立 Logger、Clock、ID、Validator、Database、Cache、I18n、Storage，安装 stable Capabilities；没有反射对象图、运行时 Resolver、包扫描或 `init` 自注册。

RuntimeComponent 支持 Stage、Build、Start、Ready、PublishInitial，以及 reload 的 candidate discard、BeginDrain、Commit、Resume、Rollback、StopPrevious。构造和资源 owner 可从 composition 与 component definition 定位。

### 2.3 启动、Ready 与发布

Kernel Start 在一个 operation timeout 内顺序执行 load、build/start/ready 和 publish；任一失败按反序清理并保留主错误/清理错误。当前 `Ready` 只回答“候选资源是否可发布”，不回答 HTTP 是否监听、后台 runner 是否存活或进程是否正在排空。

Host 把 Kernel 作为第一个 Participant；Supervisor 按注册顺序 Start，启动失败时反序 Stop 已启动项。没有非空/重复 Participant/Task 名称的构造门禁，nil Participant 会被跳过。

### 2.4 运行监督

Supervisor 使用 `errgroup.WithContext` 运行 Task，并在所有 Task 返回后才反序 Stop Participant。第一项非 nil error 会取消兄弟；Task 提前返回 nil 不会立即取消兄弟。忽略 context 的 Task 会让 Wait 无限等待，当前 ShutdownTimeout 尚未开始计算。

Participant 只有 Start/Stop，没有运行期错误回传。未来 HTTP 若在 Start 内自行 goroutine，serve failure 无法交给 process owner；若 Serve 作为 Task 而 Shutdown 在 Participant.Stop，则“先 Wait Task、后 Stop Participant”形成确定性互锁。

Watcher 当前正确响应 context、串行调用 Reload，并在基础 watcher 错误时结束 Task；OnReady reconciliation 关闭启动/开始监听之间的窗口。它不能证明任意未来 Task 都合作退出。

### 2.5 重载与一致性

Kernel Reload 先对 RestartRequired 做无副作用预检，再准备全部候选；旧代仍可借用。之后反向 drain、正向 commit、更新 snapshot、resume，并反向清理旧代。候选失败或 drain 失败保持旧代，当前测试覆盖顺序、回滚、旧实例可用、每代关闭一次和 RestartRequired 无副作用。

局限：

- application-owned 配置尚不能加入同一预检事务。
- 当 Kernel component section 未变化时，未知/未来应用节变化可导致 Kernel 更新整份 snapshot digest，而没有 application 状态提交。
- committed cleanup failure 返回 typed error，但旧 handle 被清除；没有持久 degraded 状态、重试/重启策略或后续 reload 限制。
- `NativeAtomicReload`、`ComponentHandoff`、切换后观测与自动回滚在当前文档中明确未实现；现有组件没有证据要求立即补这些高复杂机制。

### 2.6 排空与停止

Lease 有 pending/serving/draining/stopped 状态，drain 阻止新借用并等待 active use，适合 reload 回滚。Kernel Stop 若 drain 失败会 Resume 并返回错误；在独立可重试 Stop 语义中合理，但 Host 已进入终止流程且不会再次 Stop，此时重新 serving 与进程状态冲突。

Supervisor 以一个全局 stop context 依次调用所有 Participant.Stop；靠前 owner 可耗尽预算，后续 Kernel 收到已过期 context。它会继续尝试后续 Stop 并合并错误，但没有未清理 owner 的结构化诊断。

### 2.7 诊断与验证

`pkg/health` 已有 liveness/readiness/startup Kind、Registry 和 Snapshot，但生产 composition 没有注册或暴露它。当前实现也没有进程 lifecycle state、ready reason、generation/digest、last reload/cleanup 或 degraded 视图。Registry 本身串行执行 checker，map 顺序不确定；是否需要并行/稳定排序应由真实运维接口验收决定。

Kernel/Host/Supervisor 测试覆盖大量局部失败路径，但缺少：HTTP Stop/Wait 互锁、Task nil 提前完成、不合作 runner、Participant runtime failure、非法/重复 ID、终止 drain 不 Resume、diagnostic state 和 package import boundary。

## 3. 当前 Capability 与边界

| 能力 | 已实现形态 | 当前结论 |
|---|---|---|
| Logger | baseline + 可替换 stable Manager | 保留；诊断状态仍需 owner |
| Clock / ID / Validator | Fixed + Direct | 保留；HTTP middleware 不应绕过显式注入 |
| Database | Configured + leased Access | 局部闭环；业务 Adapter 尚不存在 |
| Cache | Configured backend + typed Client | backend owner 清晰；未来 typed Client 需模块 owner |
| I18n | stable Translator facade | 资源换代存在；业务 message ownership 未定义 |
| Storage | borrowed Client / managed Manager | 局部闭环；业务使用尚无证据 |
| HTTP | 项目 Router/Middleware/阻塞 Server 原语 | 没有 listener/composition/lifecycle，未闭环 |
| Health | Registry 原语 | 未接入生产，不能声称 readiness 已实现 |

## 4. 十一项门禁当前判定

| 门禁 | 判定 | 主要理由 |
|---|---|---|
| 事实 | 部分满足 | Kernel 证据强，全进程入口缺实现/运行证据 |
| 目标 | 部分满足 | 现有资源事务有真实目标；业务细节过早 |
| 边界 | 部分满足 | Kernel/Adapter 清晰，application owner 缺失 |
| 装配 | 部分满足 | 当前 Plan 清晰，HTTP/runner/应用配置未统一 |
| 生命周期 | 不满足 | Supervisor、HTTP、终止 drain 未闭合 |
| 一致性 | 部分满足 | Kernel 内强，整份 snapshot 与 degraded 不完整 |
| 错误 | 部分满足 | 原因链较强，运行/cleanup/terminal 语义不足 |
| 治理 | 不满足 | 局部测试存在，架构/全链路门禁缺失 |
| 演进 | 满足方向 | 可在当前边界上渐进单轨补齐 |
| 复杂度 | 推荐路线满足 | 薄协调层收益可对应本地缺口 |
| 业务延伸 | 不满足 | 前置关键门禁未完成，必须阻塞 |

## 5. 事实、推断与目标边界

| 类别 | 示例 | 使用方式 |
|---|---|---|
| 已实现 | Kernel candidate transaction、Lease、Watcher reconciliation | 可作为兼容基线 |
| 已证实缺口 | HTTP/Supervisor 互锁、无 runtime error channel、无生产 readiness | 必须先设计并实施验证 |
| 研究推断 | 薄 coordinator 优于第二个 runtime | 有本地问题和外部主源共同支持 |
| 待确认目标 | 单候选协调、state diagnostics、terminal drain | 不得描述成已实现 API |
| 延后问题 | Handler/Service/Repository/Model 形态 | 基础门禁和真实用例后再决定 |
