# Composition、Capability 与运行生命周期责任图

## 1. 设计目标

本文把定义、配置、资源和进程状态连接为一条可验证运行链路。目标不是再建一个 Application Container，而是让每个阶段都有唯一执行者、明确输入输出、失败语义与诊断，并保留现有 Kernel 资源平面。

状态：**已实施设计记录**。实施快照见 [R021](../research/R021-foundation-closure-implementation/report.md)，实施前 Supervisor/HTTP/Reload 缺口见 [current-facts-and-gaps.md](../requirements/current-facts-and-gaps.md)，详细契约 ID 见 [foundation-contract-catalog.md](../requirements/foundation-contract-catalog.md)。

## 2. 全链路责任图

| 阶段 | 唯一执行者 | 输入 | 成功输出/状态 | 失败不变量与验证 |
|---|---|---|---|---|
| 定义 | capability/Adapter/module owner | 项目需求、第三方边界 | typed Config、窄 capability、lifecycle unit 定义 | 无 runtime 副作用；API/owner 静态审阅 |
| 注册 | composition root | 显式 definitions/section/commands | 有序 registry/Plan | 空/重复/冲突在副作用前失败 |
| 冻结 | Plan/registry owner | 完整注册集合 | immutable Plan、command/config registries | 冻结后修改失败；顺序确定 |
| 配置加载 | Application config coordinator | ordered Sources、ctx | single immutable candidate/digest | 任一 source 失败不改变 current |
| 绑定 | section owner contract | candidate section | typed Config | unknown/type/duplicate/missing 错误定位且脱敏 |
| 语义校验 | section owner | typed Config | validated candidate | 无资源副作用；跨字段不变量测试 |
| 变化预检 | Application + Kernel owners | current/candidate typed config | change class 与 execution plan | RestartRequired 在构造前返回 |
| 构造 | resource/component owner | validated typed Config、显式依赖 | pending candidate | 构造资源由 owner 清理；无 publish |
| 资源探测 | resource/component owner | pending candidate、ctx | ready candidate | timeout/cancel 保留；失败反序清理 |
| 启动 | Supervisor | ordered participants/runners | started units | 启动失败反序 stop/wait，不进入 ready |
| 就绪 | Application state owner | all required ready/run acknowledgements | process `ready/running` | listener/runner 未运行时不得 ready |
| 运行 | Supervisor | blocking runners | runtime completion/error | 意外 nil 或 error 触发全局取消 |
| 重载 | Application config coordinator + Kernel | single candidate | new generation 或 RestartRequired | 旧代在 commit 前保持 serving |
| 排空 | process/lifecycle owner | reload/terminal intent | no new work, active work drained | reload 可 rollback；terminal 不 Resume |
| 停止 | Supervisor + each owner | bounded ctx | owners stopped | 继续尝试全部 owner，保留所有错误 |
| 清理 | resource owner | current/previous/pending resources | each resource closed once | cleanup error 结构化；committed failure -> degraded |
| 诊断 | Application state owner | lifecycle events/errors/digest | immutable redacted snapshot | generation 一致、无 secret、状态序列可测试 |

## 3. 装配边界

```text
cmd/app
  -> select Mode
  -> Bootstrap composition
       command registry + config registrations + Default manager
  -> OR Service composition
       single Config candidate coordinator
       Kernel Plan                 # Logger/DB/Cache/Storage 等资源平面
       explicit Application graph  # 仅真实对象，普通构造函数
       supervised runtime units    # HTTP/Watcher/后台 runner
       state/diagnostics owner
```

不变量：

- production composition root 只有一个；Bootstrap/Service 是同一根按 mode 选择的显式分支，不是第二套注册体系。
- Kernel Plan 只治理需要稳定 façade、Lease、换代或统一资源生命周期的基础 capability；Handler/Service/Repository/Model 不进入 Plan。
- Application graph 只有真实用例出现后才用普通 Go 构造函数建立；调用方定义窄 port，Adapter 实现 port。
- 不允许 Container、Resolver、全量 Capabilities 参数、包扫描、隐藏 Provider、`init` 自注册或运行时随意 lookup。
- 一个资源只能有一个 construction/close owner；消费者拿 lease/borrowed handle，不得调用共享资源 Close。

## 4. Process 状态机

### 4.1 状态

| 状态 | 含义 | readiness | 允许事件 |
|---|---|---:|---|
| `new` | 尚未冻结装配 | false | compose |
| `starting` | 加载候选、构造、启动、探测中 | false | ready、start-failed、cancel |
| `running` | 所有 required unit 已启动并持续运行 | true | reload、runtime-failed、signal |
| `reloading` | 新候选准备/换代中，旧代仍可服务 | 保持旧状态，commit 窗口按协议控制 | reload-commit、reload-rejected、reload-rollback |
| `draining` | 拒绝新工作，等待 active work | false | drained、drain-timeout |
| `stopping` | 有界 stop/wait/cleanup | false | stopped、stop-failed |
| `degraded` | 已 commit 新代但旧代 cleanup 等不可回滚步骤失败 | false 或按已确认运维策略；当前建议 false | terminal restart/explicit remediation |
| `failed` | startup/runtime/terminal failure 已确定 | false | bounded cleanup -> stopped/failed exit |
| `stopped` | 所有可停止 owner 已完成尝试 | false | 无 |

`ready` 是由状态和 required unit 条件推导的结果，不是可独立设置的布尔值。`Kernel RuntimeComponent.Ready` 只说明候选资源可发布，不能直接把进程置为 ready。

### 4.2 启动事件序列

```text
new
  -> compose/freeze
  -> starting
     -> load candidate once
     -> strict bind/validate/classify
     -> construct/probe Kernel candidates
     -> pre-bind HTTP listener and prepare runners
     -> start participants in order
     -> start supervised runners
     -> receive required running acknowledgements
  -> running + ready=true
```

任一失败：先 `ready=false`，取消未完成工作，反序 stop/wait/cleanup 已拥有资源，合并主错误与清理错误，进入 `failed` 并返回确定退出分类。当前 Kernel 首次启动逐组件 PublishInitial；实施前必须选择并证明：在 `Host.Run` 返回 ready 前无外部 borrower，或调整为“全部候选 ready 后统一发布”。此项 **尚未确认**，不得在文档中声称已原子启动。

### 4.3 运行监督

每个长期 runner 必须：

- `Run(ctx)` 阻塞到错误、取消或正常终止协议成立；内部 goroutine 也由该 runner 等待。
- 声明 completion policy：service runner 的 nil 提前返回是异常；明确 one-shot task 才允许 nil 完成。
- 运行错误进入 Supervisor，不只记录日志；Supervisor 取消 siblings 并驱动 drain/stop。
- owner 提供有界 Stop/Close，并在返回前或显式 Wait 中确认 goroutine 退出。
- 未响应 context 的 runner 在 deadline 后形成 owner/phase diagnostics；Supervisor 不无限卡在“先等 Run 全返再 Stop”。

推荐 Supervisor 协议是 `Start -> Run/Wait -> RequestStop -> StopAndWait` 的显式阶段，不要求采用 R013 项目的具体接口。

## 5. HTTP lifecycle

HTTP 是基础 Adapter，不等同业务 Handler：

```text
validated HTTP Config
  -> net.Listen                 # 同步暴露端口占用
  -> construct http.Server
  -> register as supervised runner
  -> Serve(pre-bound listener) # blocking
  -> running acknowledgement
  -> readiness may become true

terminal/reload intent
  -> readiness false
  -> reject/stop new work as configured
  -> Shutdown(ctx)              # wait active HTTP requests
  -> Close fallback if timeout
  -> Wait Serve return
  -> close listener exactly once
```

- listener、server、serve goroutine 由同一 HTTP lifecycle owner 管理。
- `http.ErrServerClosed` 只有在 owner 已发起正常 shutdown/close 时才归一化为 nil；运行期意外返回仍是 failure。
- hijacked connections 不由 `Shutdown` 自动管理；真实 WebSocket/upgrade 需求出现后必须注册并验收，否则明确不支持。
- HTTP 不直接持有 Database/Cache/Storage 全量能力；middleware/handler 仅获得窄依赖。
- 管理端口/业务端口是否分离、read/write/idle/shutdown timeout 的具体值仍 **尚未确认**，由运行验收决定。

## 6. Reload 状态机

```text
running
  -> reloading: load candidate once
     -> source/bind/validate failure -----------------> running(old)
     -> RestartRequired/NotReloadable ----------------> running(old) + diagnostic
     -> prepare/probe failure ------------------------> running(old)
     -> begin reload drain failure -> rollback/resume -> running(old)
     -> commit all owners
        -> update current snapshot/generation once
        -> resume/admit new work
        -> cleanup previous
           -> success -------------------------------> running(new)
           -> failure -------------------------------> degraded(new)
```

reload 与 terminal stop 使用不同 intent。只有 reload drain 允许 rollback/resume；terminal signal/runtime failure 进入 draining 后不再恢复 serving。

Application coordinator 只负责 single candidate、所有 owner preflight 与 commit 顺序；Kernel 继续执行底层 component candidate transaction。应用配置变化若不能参与同一事务，必须 `RestartRequired`，不能让 Kernel 单独接受整个 digest。

## 7. 终止状态机

```text
running | reloading | degraded | failed
  -> readiness=false
  -> draining(terminal)
     -> reject new work
     -> request runners stop/cancel
     -> drain active uses/requests within total deadline
  -> stopping
     -> reverse owner Stop
     -> Wait all goroutines
     -> close current/pending/previous resources exactly once
     -> join main + stop + wait + cleanup errors
  -> stopped / failed exit
```

规则：

- stop 总期限从 terminal intent 开始计算，不能等不合作 Task 后才启动。
- 每个 owner 获得剩余预算或已设计的子预算；某个 owner 超时不阻止后续 owner 获得 stop 尝试。
- terminal drain 失败也继续 best-effort Stop/Close，绝不 Resume。
- cancel、deadline、runtime error、stop error 和 cleanup error 保持可识别；日志只在进程决策边界记录一次。
- `os.Exit` 只发生在所有可执行 defer/close 已返回之后。

## 8. Diagnostics 契约

只读状态快照至少包含：

```text
process state + since
ready boolean + reasons
current generation + redacted digest/provenance
required unit states
last startup/reload/runtime/stop failure category
committed cleanup/degraded owner and generation
restart-required reason
```

不得包含 raw config、password、Token、Secret、private key、完整 DSN 或第三方 client。Diagnostics 使用状态事件更新，不通过解析日志反推。日志包含 stable owner/component ID、phase、generation 和 error category，原始 cause 只在安全边界保留。

## 9. 错误与资源规则

1. 错误上下文按 `owner -> phase -> operation -> cause` 组织，`errors.Is/As` 可识别 cancel/timeout/config/restart/cleanup。
2. 构造失败由构造 owner 清理 pending；启动成功后的资源由 Supervisor/participant owner 停止；借用者永不关闭共享资源。
3. 多项 cleanup 使用 `errors.Join` 或等价 typed aggregation，保留主错误；禁止只返回最后一个错误。
4. goroutine 必须能列出 owner、cancel source 和 Wait 位置；无法回答任一项即不允许注册。
5. 任何 stable facade 切换都与 generation/digest 一致；不得缓存跨代 raw client。
6. 只有真正决定继续、重试、退出或降级的边界记录错误，避免逐层重复日志。

## 10. 自动化治理门禁

| 门禁 | 自动证据 |
|---|---|
| 注册 | 空/重复 component、section、command、participant、runner ID 和路径/flag 冲突测试 |
| 依赖 | package graph 禁止业务反向导入 composition/internal Adapter；禁止 Container/Resolver/全量 Capabilities |
| 构造 | production resource constructor 只从 composition/lifecycle owner 可达；搜索旁路 `gorm.Open`、Redis/HTTP/storage client 构造 |
| 配置 | strict unknown/duplicate/type/default round-trip/sensitivity/Snapshot immutability 测试 |
| 生命周期 | startup/runtime error/nil early/cancel/uncooperative/timeout/reverse stop/event sequence 测试 |
| HTTP | bind failure、Serve failure、active request drain、Shutdown timeout、Wait/Close exactly once 测试 |
| Reload | single load/snapshot、RestartRequired no-side-effect、candidate rollback、degraded gate、stop/reload mutual exclusion 测试 |
| 诊断 | state/readiness/generation/last error/degraded 转换与脱敏测试 |
| 文档 | 权威文档 API/路径链接、旧符号/旧配置键搜索、`git diff --check` |
| 运行证据 | 仅在真实执行后记录 listener、signal、reload 与 shutdown 证据；未运行不得声称通过 |

## 11. 业务详细设计解锁条件

以下条件全部满足并有实现、测试和必要运行证据后，才允许带着首个真实用例恢复 Handler/Service/Repository/Model 设计：

- Bootstrap/Service mode 与 CLI/Config 契约验收通过；默认配置和运行期 binding 不漂移。
- 同一启动/reload 只有一个 candidate/digest，所有 owner 预检一致。
- Supervisor 对运行错误、nil 提前完成、不合作 runner、signal 和总期限有确定序列。
- HTTP listener、Serve、readiness、drain、Shutdown/Close/Wait 的 owner 和证据闭合。
- reload 与 terminal drain 分离，cleanup degraded 可见且限制后续动作。
- 资源/goroutine owner、错误聚合、敏感信息和 diagnostics 门禁通过。
- package/registration/composition/lifecycle 自动化门禁进入常规验证。
- 首个真实业务用例、数据所有权、外部协议和验收明确。

通过基础条件不等于批准业务实现；真实用例仍需在 012 中更新设计并再次确认。底层门禁未通过时，不继续扩写假设性的业务接口。
