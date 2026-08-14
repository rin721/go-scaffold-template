# R017：当前进程、CLI 与 Config 契约清点

> 状态：实施前历史快照，已由 [R021](../R021-foundation-closure-implementation/report.md) 替代。本文保留 `2daf47ad111141b27a1d8e100bb3d6e4cc1ea743` 的缺口证据，不代表当前实现。

## 1. 结论

当前架构不是“没有底层契约”，而是稳定契约与隐式行为交错：Kernel 资源平面的 Plan、Capability facade、Lease、默认文件发布和 reload candidate transaction 证据较强；Application/CLI/Config 控制面的模式、严格字段语义、单 Snapshot、运行监督与诊断还未闭合。推荐保留前者并在现有组合根上补齐后者，不以业务分层或新框架替代底层问题。

本报告是代码快照审计，不声称启动、listener 或外部资源已经运行。

## 2. 进程与 CLI 调用链

### 2.1 当前事实

- `cmd/app/main.go` 的 `main` 只调用 `os.Exit(runMain(...))`。`runMain` 先创建 signal context、baseline Logger 和 logging Manager；`process.run` 返回后再关闭 baseline，清理没有被最外层 `os.Exit` 跳过。
- `process.run` 总是创建 `FileSource -> EnvSource` Loader、Kernel 并调用 `composition.Compose`。`len(args)>0` 时构造 CLI 并在 Kernel Start 前执行；无参数时创建 Host 进入服务模式。
- CLI 分支没有调用 `Kernel.Start`，因此当前 `config init`、help/version/parse error 不启动 Database、Cache、Storage 或 HTTP；但它们仍依赖完整 Plan/Defaults/Capabilities composition 的构造成功。
- 标准流从 `runMain` 注入 `process`，再进入 `pkg/cli.RunWithIO`。CLI Adapter 没有必要直接占有 OS 全局流。
- `pkg/cli.CommandSpec` 能表达 name、aliases、args、flags、Run 和交互首页顺序，但没有 help group；`internal/kernel/cli.Contract` 返回有序命令集合，`NewApp` 拒绝 nil contract、空集合和顶层重复 name。
- `pkg/cli` 统一关闭 Cobra 自己的 usage/error 打印，help 写 stdout，parse error 归一为 UsageError；CommandError 保留 cause，CancelledError 映射 130。

### 2.2 已证实缺口

- 模式由“是否有参数”隐式决定，没有 Bootstrap/ApplicationCommand/Service 契约，也没有命令所需资源或副作用声明。
- top-level direct name 已校验，但 nested name/alias、GroupID、flag name/shorthand 与 inherited scope 冲突没有统一冻结检查。
- nil positional validator 接受任意参数；`Context.Get*` 缺失/类型不匹配返回零值，需要调用方记得 `IsFlagChanged` 才能区分。
- nil `context.Context` 会被替换为 Background，取消前置条件不够明确。
- version 支持存在但入口未设置版本；`ExitConfig=3` 没有默认配置失败的真实映射路径。
- CLI 目前没有声明目标文件、overwrite 或其他副作用，验收只能从具体命令实现推断。

## 3. 默认配置契约

### 3.1 已实现并应保留

- `DefaultContract.Defaults(ctx)` 由 capability owner 提供 ordered Object/Field/Value；聚合时校验空/重复 capability ID、无效/重叠 path、nil contract 和通用值域。
- 全部 Defaults、通用结构校验和 YAML/JSON 编码先在内存完成，失败不会触碰目标。
- 目标格式由 `.yaml/.yml/.json` 决定；返回绝对路径、格式和参与的 capability IDs。
- 新目录 `0700`，同目录临时文件 `0600`，完整写入后检查 short write、Sync、Close；默认 no-overwrite，force 显式替换；失败清理 temp，并 join 主错误与清理错误。
- Database 默认 SQLite 本地路径、Cache 默认 disabled/空 password、Storage 默认 local/空远程 secret、Logger 默认 stdout/stderr，没有把当前环境真实凭据写入模板。

### 3.2 不完整之处

- Generic Object validation 只验证可编码形状，不调用运行期 owner 的 typed decoder 和 semantic validator。
- 多数默认值来自同一 `pkg/* DefaultConfig()`，但这是实现惯例；生成器可省略运行期补齐字段，例如 logger 的部分 defaulted 字段，缺少 formal round-trip contract。
- 默认生成依赖完整 Compose 才取得 manager；结构上没有独立 Bootstrap composition。
- 当前写入机制的保证应按 OS/filesystem 表述；File Sync 不等于任意平台 crash durability，父目录持久化也没有独立契约。

## 4. Config Source、Binding 与 Snapshot

### 4.1 已实现事实

- `Source{Name, Load}` 输出 `map[string]any`，Loader 按注册顺序递归 merge，后者覆盖前者；生产顺序 File 后 Env，所以 Env 优先。
- FileSource 支持 YAML/JSON；EnvSource 通过 prefix 与 `__` 表达嵌套，空环境值仍作为显式输入。
- Snapshot 保存 raw、redacted、SHA-256 digest 与 provenance；Data/Section/Redacted/Provenance 返回副本。
- component Stage 通常先建立 typed `DefaultConfig()` 再 `DecodeSection`，因此缺失 section 通常采用 typed default。
- Config 校验分散在各 capability owner；资源连通性一般在 Build/Ready 阶段处理。

### 4.2 已证实缺口

- `DecodeSection` 开启 `WeaklyTypedInput` 且没有 `ErrorUnused`，未知字段静默忽略，空字符串和数值/布尔/字符串可被宽松转换。
- 未知字段、重复 YAML 字段、deprecated/removed 字段和 schema version 没有统一 contract。
- missing/zero/empty/disabled/default 语义由各 Config 实现隐式决定：数值 `0` 常被归一化，logger 用 pointer bool，Cache/Storage 用 enum disabled。
- Source 允许任意 `map[string]any` 值；Snapshot copy 只完整处理 JSON-like map/slice。自定义 Source 放入 pointer 或其他 mutable slice 时，不可变性没有被契约证明。
- redaction 依赖 key substring 启发式，owner 没有字段级 sensitivity metadata。
- built-in Source 的函数体没有完整取消点；nil Source 被 Loader 静默跳过，Source name 也没有非空唯一构造门禁。

## 5. Plan、Composition 与 Capability

- `internal/kernel/app.Plan` 使用显式 Add、typed Binding/Input、Replace、Freeze；duplicate component ID 和冻结后修改会失败。FrozenPlan 不做运行时 Resolver、reflection graph 或包扫描。
- `composition.Compose` 是唯一 production composition root，顺序注册 Logger、Clock、ID、Validator、Database、Cache、I18n、Storage；配置 Logger 是显式 replacement。
- Plan Freeze 只验证 component 层；Defaults/CLI/config path 的全局冲突在后续 manager/App 组装才暴露，仍在资源启动前，但不是单次统一冻结。
- Capabilities 以 Fixed/Direct/Configured/stable facade 暴露，当前 composition 结果是 broad struct；尚无业务调用方依赖完整 struct。目标应保持窄注入，不把它发展为巨型依赖对象。
- Logger Manager 保持 stable identity，baseline 在配置前可用，configured logger 成功后才替换；Database/Storage 通过 Lease 借用，Cache backend owner 与 typed Client owner 分离。

## 6. 启动、Reload、Stop 与资源

### 6.1 Kernel 资源平面

- RuntimeComponent 显式区分 Stage、Build、Start、Ready、PublishInitial、DiscardCandidate、BeginDrain、Commit、Resume、Rollback、StopPrevious、PrepareStop、StopCurrent、StopPending。
- Kernel Start 顺序 build/start/ready/publish；失败反序清理并合并错误。它是逐组件 publish，不是“所有 component ready 后一次原子发布”。
- Reload 先做 RestartRequired 预检，再准备候选、反向 drain、正向 commit、更新 Snapshot、resume、反向 cleanup previous；失败测试覆盖旧代继续可用与一次关闭。
- 当所有 component section 未变化时，Kernel 仍可接受整份新 Snapshot/digest；未来 application-only section 会产生“配置已接受、Application 状态未提交”的撕裂。
- committed cleanup failure 有 typed error，但没有 persistent degraded state、remediation 或后续 reload gate。
- Lease 明确 pending/serving/draining/stopped，drain 阻止新借用并等待 active uses；消费者不能释放共享 owner 资源。

### 6.2 Host/Supervisor

- Host 把 Kernel 作为第一 Participant，随后 application lifecycle，Watcher 是当前唯一长期 Task。
- Supervisor 顺序 Start；启动失败反序 Stop。Task 通过 errgroup 运行，第一项非 nil error 取消 siblings；所有 Task 返回后才 Stop Participants。
- nil Participant 被跳过；Participant/Task 空名和重复名没有完整构造校验。
- Task 提前返回 nil 不取消 siblings；忽略 context 的 Task 会让 Wait 无限等待，stop timeout 还未开始。
- Participant 只有 Start/Stop，没有 runtime error channel。若 HTTP Start 内部开 goroutine，serve failure 只能日志；若 Serve 是 Task 而 Shutdown 在 Participant.Stop，则先 Wait 后 Stop 形成互锁。
- terminal `Kernel.Stop` drain 失败当前 Resume 旧资源，但 Host 不会再次 Stop；这与终止后不得回到 serving 的进程语义冲突。

## 7. HTTP、Health、Database、Cache、Storage

| 能力 | 真实调用方与状态 | 结论 |
|---|---|---|
| HTTP | `pkg/httpx.Server.Start` 阻塞 `ListenAndServe`，只有 `Shutdown`；无 production composition/listener owner | 不完整，先补 lifecycle，不能先设计业务 Handler |
| Health | Registry/Snapshot 原语存在，无 production 注册/endpoint | 缺失进程 readiness/diagnostics；map 顺序与串行 check 也未冻结 |
| Database | composition + component + leased Access；GORM 在 Adapter 内，Ready ping | 保留；业务 Repository port 等真实需求 |
| Cache | explicit disabled、owned backend、调用方 typed Client | 保留；禁止旁路第二 Redis client |
| Storage | disabled/local/remote、borrowed Client、generation invalidation | 保留；敏感配置进入统一 Config contract |

## 8. 测试证据与缺口

当前测试已覆盖 default aggregation/file failure、CLI parse/I/O/error、Plan freeze、Kernel start/reload/rollback/cleanup、Lease 借用、Host/Watcher 和各 Adapter 的大量路径。仍缺少：

- CLI nested/alias/group/flag conflicts、mode/resource selection、version 和 ExitConfig 黑盒路径；
- default output -> strict runtime binder/validator round-trip；
- unknown/duplicate/deprecated config、Source identity/value domain、任意 Snapshot mutable input；
- Supervisor nil/duplicate ID、service runner nil return、uncooperative runner、runtime error 与 stop 总期限；
- HTTP bind/Serve/Shutdown/Close/Wait、active request drain 与 Host 互锁；
- single Application candidate、terminal drain no-resume、persistent degraded 和 diagnostics；
- production constructor/import/registration 全局治理。

## 9. 保留、补齐、优化或重设

| 判断 | 内容 |
|---|---|
| 保留 | 薄 main、显式 I/O、Plan/Freeze、single composition root、stable facade、Lease、Kernel candidate transaction、默认文件安全发布、Adapter owner |
| 补齐 | Bootstrap composition、strict section contract、default round-trip、single candidate coordinator、process state/diagnostics、HTTP lifecycle、governance |
| 优化 | CLI 完整冲突/退出/取消、Source canonical domain、Snapshot redaction/immutability、Supervisor completion/timeout、terminal drain |
| 重新设计 | 只针对 Supervisor 的 Run/Stop waiting protocol；不重写 Kernel，也不预设业务模块模式 |
| 拒绝 | Container、巨型 Context/Capabilities、无类型配置、隐藏 Provider、动态 lookup、第二套 composition/reload runtime |
