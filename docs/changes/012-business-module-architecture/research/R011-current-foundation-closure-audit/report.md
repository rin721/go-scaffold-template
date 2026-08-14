# R011：当前底层能力闭环审计

## 1. 问题与方法

以 `2daf47a` 为代码基线，从 `cmd/app` 沿实际调用链读取配置、Plan、Kernel、Host、Supervisor、Watcher、HTTP、Health、Lease 和测试。本文区分“局部闭环”“全进程闭环”和“目标设计”，不把包存在或单元测试通过等同于产品可用。

## 2. 完整链路事实

| 环节 | 当前代码事实 | 当前意图 | 闭环判定 |
|---|---|---|---|
| 配置输入 | `cmd/app` 构造 Loader；`Kernel.Start/Reload` 自行调用 Loader | Kernel 独占候选配置事务 | Kernel 内成立；未来 application 配置会导致重复加载或所有权冲突 |
| 依赖装配 | `Plan` 显式 Add/Binding/Input/Replace/Freeze，composition 顺序可审查 | 避免隐藏解析与循环 | 当前 Capability 图满足；全应用对象图尚不存在 |
| 资源创建 | RuntimeComponent 分离 Build/Start/Ready/Publish，失败反向清理 | 候选不污染服务代 | Kernel 局部满足 |
| 启动与就绪 | Kernel 顺序启动；`Ready` 只在发布前检查 | 资源发布前阻断坏候选 | 资源就绪满足；没有进程级 Service readiness |
| 运行监督 | Supervisor 启动 Participant，再用 `errgroup` 等待 Task | 任一后台失败触发退出 | 部分成立；Participant 无运行期错误通道，Task 正常提前返回不触发退出 |
| 重载 | 预检 RestartRequired，候选准备，反向 drain，提交后清理旧代 | 旧代持续服务，候选失败回滚 | 已覆盖当前 Kernel sections；整份 application snapshot 未闭合 |
| 一致性 | Lease 阻止新借用并等待 active use；快照在 Kernel 提交后更新 | 避免换代期间资源逃逸 | Kernel 内较强；未知应用节变化仍可更新 digest，形成未来撕裂风险 |
| 排空与停止 | Reload 排空失败可 Resume；进程 Stop 复用同一语义 | 可取消、可回滚 | 终止路径不完整：排空超时时恢复 serving，但 Host 已在退出且不会重试 |
| 异常与清理 | 启动错误与回滚错误合并；提交后旧代清理错误返回 | 保留主错误与清理错误 | 原因链较强；cleanup 失败后旧句柄被清除，未持久化 degraded 状态，也未限制后续 reload |
| 诊断 | Logger 和 `pkg/health.Registry` 存在 | 提供基础诊断原语 | Health 未接入生产；无 lifecycle state、generation、last reload/cleanup 状态 |
| 验证治理 | Kernel/Host/Supervisor 有多组生命周期测试 | 固化顺序、回滚与换代 | 缺少全链路、HTTP interlock、运行期失败、非法/重复注册和 import 边界门禁 |

## 3. 已满足的关键不变量

- baseline Logger 在配置前存在，替换只有在候选构造成功后发生。
- Kernel 依赖图显式且冻结，当前没有反射 Resolver、扫描或 `init` 注册。
- 配置组件采用候选先建、旧代后排空/清理；候选失败保持旧代。
- Database、Storage 等通过稳定 facade/Lease 限制共享对象逃逸，owner 可定位。
- RestartRequired 在候选资源产生副作用前预检；顺序、回滚、drain timeout、cleanup error 已有定向测试。

这些事实证明现有 Kernel 不是需要推翻的原型，而是可继续演进的兼容基线。

## 4. 未闭合问题及影响

### 4.1 Supervisor 与 HTTP 会形成停止互锁

Supervisor 先等待所有 Task 结束，之后才反向调用 Participant.Stop。`http.Server.Serve` 类任务通常要等 `Shutdown` 才返回；若 Shutdown 位于 Participant.Stop，Supervisor 将永远到不了 Stop。现有 ShutdownTimeout 只包围 Stop，无法约束忽略 context 的 Task。Participant 若自行起 goroutine，又没有把运行期 serve error 回传给 Supervisor 的契约。

### 4.2 正常提前返回不是成功的统一语义

`errgroup.WithContext` 只在非 nil error 或 Wait 返回时取消 context。Service mode 中一个关键 Task 提前返回 nil 应触发进程终止，但当前不会取消仍在运行的兄弟 Task。Bootstrap/Application CLI 的 one-shot 完成又确实可以是成功，因此运行模式和完成语义必须显式区分。

### 4.3 Reload 与终止排空不能共用同一失败策略

reload 排空超时后 Resume 旧代是正确回滚；进程终止时 Resume 会重新接受工作，而外层 Supervisor 已进入退出路径。目标必须区分“可回滚换代”和“终止排空”，终止失败应保持 not-ready/不再接单，保留诊断并以失败退出，不能伪装恢复运行。

### 4.4 提交后清理失败没有持久状态

`CommittedCleanupError` 表示新代已经服务、旧代清理失败，不能回滚成普通失败。当前错误虽向上返回，但旧句柄随后被清除，没有 degraded/restart-required 状态，也没有阻断再次 reload。继续换代可能累积不可追踪的资源问题。

### 4.5 Ready 不是服务就绪

RuntimeComponent.Ready 是候选发布门禁，不代表 HTTP 已监听、所有运行单元仍存活、进程未排空或最近一次 cleanup 正常。`pkg/health` 目前只是未接入生产的库，不能据此声称 readiness/liveness 已实现。

## 5. 推荐

保留 Kernel Plan、stable facade、Lease 和候选事务；在其上新增薄的 application 配置/生命周期协调层，并局部强化 Supervisor、httpx、health 与治理测试。不要把业务对象塞入 Kernel，也不要引入第二个 DI/生命周期容器。

证据强度：Kernel 局部结论为高（源码与测试直接证明）；进程级闭环缺失为高（调用顺序可构造确定性互锁）；真实网络排空和运维诊断效果为中（尚无生产入口，需实现后运行验证）。

## 6. 验证与未决

后续必须用确定性测试证明：端口预绑定失败同步返回、运行期 serve error 触发全局取消、正常提前退出的模式语义、忽略 context 的 runner 仍被停止上限约束、终止排空不 Resume、cleanup degraded 阻断后续 reload，以及 readiness 状态转换。具体公开接口名、管理端点暴露方式和停止预算分配仍待实施方案确认，不在本文虚构。
