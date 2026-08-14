# 实施前底层契约清单与状态矩阵

## 1. 用途与状态

本文以 `2daf47ad111141b27a1d8e100bb3d6e4cc1ea743` 为实施前基线，记录当时从进程入口到资源停止的契约与缺口；实施后结论见 [R021](../research/R021-foundation-closure-implementation/report.md)。契约不等于 Go interface：函数、类型、注册顺序、状态机、配置格式、错误分类、资源所有权和可验证不变量都属于契约。

状态只使用以下术语：

| 状态 | 含义 |
|---|---|
| 已实现 | 有明确代码、真实调用方和相应测试证据，当前语义可以作为兼容基线 |
| 隐式存在 | 行为由调用顺序、默认值或实现细节形成，但没有独立契约或治理 |
| 不完整 | 主干已实现，但关键状态、失败路径或跨边界责任没有闭合 |
| 缺失 | 当前没有实现或真实调用方 |
| 目标设计 | 本变更推荐补充的最小契约，尚未实现 |
| 尚未确认 | 需求或取舍不足，不能冻结公共 API |

同一契约可以同时具有“当前状态”和“目标状态”。表中的“决策”只允许保留、补齐、优化或重新设计；没有证据的问题不创建新抽象。

## 2. 总览

| ID | 契约与语义所有者 | 当前定义与真实调用方 | 当前状态 | 决策与关键缺口 |
|---|---|---|---|---|
| `BOOT-001` | 进程入口与退出；`cmd/app` | `main -> runMain -> process.run -> execute` | 已实现 | 保留薄 `main`、signal、baseline Logger、统一退出码；补诊断映射 |
| `BOOT-002` | 运行模式选择；Application owner | `len(args)>0` 选 CLI，否则 Service | 隐式存在 | 优化为显式 Bootstrap/Application/Service 模式；当前参数数量不等于资源需求 |
| `CLI-001` | 命令定义、注册与冲突；`pkg/cli`、`internal/kernel/cli` | `CommandSpec`、`App.AddCommand`、`Contract.Commands` | 不完整 | 保留项目封装；补完整 command path/group/alias/flag 冲突预检 |
| `CLI-002` | 解析、执行、I/O 与退出；`pkg/cli`、`cmd/app` | Cobra Adapter、`RunWithIO`、typed errors | 不完整 | 保留显式 I/O 和错误链；禁止 nil context 静默替换，补版本和 config exit 语义 |
| `CLI-003` | 命令资源与副作用声明；Application owner | 无；CLI 分支仍构造完整 Kernel/Plan | 缺失 | 目标设计：命令声明运行模式、最小依赖和副作用，不传全量 Capabilities |
| `CLI-004` | 默认配置生成；Config owner | `config init -> DefaultManager.Generate` | 不完整 | 保留安全写入；补 owner typed round-trip 校验和平台保证边界 |
| `CFG-001` | Source、加载与优先级；Config owner | `Source`、`Loader`；`FileSource -> EnvSource` | 不完整 | 保留显式有序覆盖；补 Source 身份、值域、取消和冲突规则 |
| `CFG-002` | Default 注册与聚合；配置节 owner | `DefaultContract`、ordered `Binding`、`DefaultManager` | 不完整 | 保留分节 owner；把默认投影与同一 typed binding/validation contract 关联 |
| `CFG-003` | Binding/Decode；配置节 owner | `Snapshot.Section`、`DecodeSection`、各组件 `Stage` | 不完整 | 优化为 strict typed binding；当前未知字段忽略且 weak conversion 开启 |
| `CFG-004` | Validation 阶段；配置节/资源 owner | 分散在 default object、typed Config、Build/Ready | 隐式存在 | 补结构、字段、跨字段、资源探测四阶段和错误分类 |
| `CFG-005` | 不可变 Snapshot、摘要与来源；Config owner | `Snapshot`、`Digest`、`Provenance`、`Redacted` | 不完整 | 保留；限制规范值域并深拷贝，记录顺序/来源，禁止敏感值诊断 |
| `CFG-006` | 缺失、零值、空值、禁用和默认；配置节 owner | 各 `pkg/* Config` 的零值归一化、枚举或指针 | 隐式存在 | 按字段明确语义；不建巨型 Config，也不统一成任意 Map |
| `CFG-007` | Schema、废弃和版本演进；Config owner + section owner | 无统一 schema/evolution contract | 缺失 | 目标设计：严格字段集合、deprecated 策略；仅真实迁移需要时引入版本 |
| `CMP-001` | 显式 Plan 与 Composition root；`internal/kernel/composition` | `Plan.Add/Replace/Freeze`、`Compose` | 已实现 | 保留唯一装配入口、顺序和 typed input；补注册表全局校验时机 |
| `CAP-001` | Capability 暴露；`internal/kernel/app` 与 `pkg/*` | Fixed/Direct/Configured、stable facade | 已实现 | 保留；普通业务对象不得进入 Kernel，也不得传全量 Capabilities |
| `RES-001` | 资源创建、所有权与借用；组件 owner | `RuntimeComponent`、Lease、Access/Client | 已实现 | 保留 owner 关闭、调用方借用；HTTP 与未来 runner 需遵循同一规则 |
| `RES-002` | 构造、探测、就绪、回滚；组件 owner | `Build/Start/Ready/...` | 不完整 | Reload 较完整；首次启动逐组件发布，不是跨组件原子发布 |
| `LIFE-001` | Host/Participant；Application owner | Kernel + application Participant、Watcher Task | 不完整 | 补非空唯一 ID、运行错误、状态和终止语义 |
| `LIFE-002` | Supervisor 与 runner；`pkg/supervisor` | 顺序 Start、errgroup Run、反序 Stop | 不完整 | 重新设计等待顺序；nil 提前完成、不合作 runner、共享超时未闭合 |
| `LIFE-003` | 进程状态与 readiness；Application owner | `pkg/health.Registry` 未接生产 | 缺失 | 目标设计：starting/running/draining/stopped/failed/degraded 与 reason |
| `REL-001` | Reload/RestartRequired；Kernel + Application owner | `Kernel.Reload`、component mode、Watcher | 不完整 | 保留 candidate transaction；补 application 节预检与单候选协调 |
| `REL-002` | Reload、终止排空与 degraded；Application owner | reload drain 可 Resume；Stop 失败也 Resume | 不完整 | 区分可回滚 reload 与不可逆 terminal drain；cleanup failure 持久可见 |
| `OBS-001` | Logger、Error 与敏感信息；`cmd/app`、logging/error owner | baseline + stable Manager、typed/wrapped errors | 不完整 | 保留 baseline；补阶段、owner、generation、退出分类，绝不记录 Secret |
| `OBS-002` | Diagnostics/Health；Application owner | Registry 原语、reload callback 日志 | 缺失 | 补状态快照和最后失败；不把 readiness 等同资源 candidate Ready |
| `ADP-HTTP` | HTTP 接入边界；HTTP owner | `pkg/httpx` 原语，无 production composition | 不完整 | 补 listener/Serve/Shutdown/Close/Wait 唯一 owner 与监督 |
| `ADP-DB` | Database 接入边界；Database owner | typed Config、GORM Adapter、leased Access | 已实现 | 保留项目契约与 ping Ready；业务 Repository 需求出现后再定义窄 port |
| `ADP-CACHE` | Cache 接入边界；Cache owner | explicit disabled、backend owner、typed Client | 已实现 | 保留；调用方 client 归模块，不暴露第二套 backend 构造 |
| `ADP-STORAGE` | Storage 接入边界；Storage owner | disabled/local/remote、borrowed Client | 已实现 | 保留；安全 placeholder 与租约失效语义继续作为门禁 |

证据入口：进程/CLI/Config 见 [R017](../research/R017-current-contract-inventory/report.md)，生命周期与 Adapter 见 [R011](../research/R011-current-foundation-closure-audit/report.md)，外部比较见 [R018](../research/R018-cli-default-contracts/report.md)、[R019](../research/R019-config-contracts/report.md) 和 R012-R014。

## 3. Bootstrap 与 CLI 契约卡

### 3.1 `BOOT-001` / `BOOT-002`

| 维度 | 当前事实 | 缺口与目标 |
|---|---|---|
| 输入/输出/依赖 | 输入 `args`、stdin/out/err、OS signal；输出整数 exit code；依赖 baseline Logger、Loader、Kernel、Composition | 模式必须先于重资源构造确定，并成为可测试专用类型 |
| 前置/后置/状态 | `runMain` 建 signal context 和 baseline；所有 defer 在返回 exit code 前运行，只有最外层 `main` 调 `os.Exit` | 建立 `Bootstrap -> ApplicationCommand -> Service` 显式状态，不以 `len(args)` 隐含推断 |
| 缺失/零值/默认 | 无参数固定进入 Service；CLI `Version` 当前未注入 | 缺失版本应在组装期报错或明确禁用，不输出空版本成功 |
| 副作用/所有权 | 入口拥有 signal、baseline Logger 和标准流；Service 可创建资源，CLI 分支不 `Kernel.Start` | Bootstrap help/version/生成配置不得创建 DB/Cache/HTTP；普通命令只获得声明的窄依赖 |
| 错误/取消/清理 | `execute` 写 stderr 并通过 typed CLI error 映射；baseline 最终关闭 | 诊断需区分 parse/config/start/runtime/stop；取消保留 `context.Canceled` |
| 并发/生命周期 | signal context 贯穿 CLI/Service | Service 才进入长期监督；one-shot 命令完成不触发 Service 状态机 |
| 证据 | `cmd/app/main.go`、`cmd/app/main_test.go` | 增加模式选择、无资源 Bootstrap、版本与退出码测试 |

### 3.2 `CLI-001` / `CLI-002` / `CLI-003`

| 维度 | 当前事实 | 缺口与目标 |
|---|---|---|
| 输入/输出/依赖 | `CommandSpec` 包含 name/aliases/args/flags/run 与交互首页顺序；没有 help group 契约；`Context` 暴露 I/O、flags、args | 注册记录还需稳定 command path/group、mode、required capabilities、side-effect class |
| 前置/后置/状态 | 顶层空名/重名被拒绝；`NewApp` 聚合有序 contract | 冻结前校验完整树的 path/name/alias/group/flag/shorthand 冲突；校验成功后定义不可变 |
| 缺失/零值/默认 | nil `Args` 接受任意参数；`Get*` 缺失/类型不匹配返回零值；nil ctx 被替换为 Background | 需要显式 positional policy；调用方用 `IsFlagChanged` 或 typed option 区分；nil ctx 应拒绝 |
| 副作用/所有权 | I/O 可由入口注入；当前命令没有副作用声明 | 文件写入须声明目标与 overwrite policy；禁止隐藏网络/资源启动 |
| 错误/取消/清理 | Usage=2、Command=1、Internal=1、Cancelled=130；Config=3 未被真实路径使用 | 明确 help/version=0、parse=2、config contract=3、cancel=130；保留原始 cause |
| 并发/生命周期 | CLI 同步执行；signal 可取消 | Bootstrap 命令不得启动 Supervisor；Application command 是否存在尚未确认 |
| 证据 | `pkg/cli/*`、`internal/kernel/cli/*` 及测试 | 增加冲突矩阵、I/O、nil context、资源选择和副作用验收 |

### 3.3 `CLI-004`

| 维度 | 当前事实 | 缺口与目标 |
|---|---|---|
| 输入/输出/依赖 | `config init --output --force` 调 ordered `DefaultManager`；YAML/JSON 由扩展名选择 | 生成只依赖 config registrations，不依赖 Kernel install 或资源 Capability |
| 前置/后置/状态 | 全部 default object 在内存校验、编码成功后才写；成功返回绝对路径、格式和 capability IDs | 写前必须让每个 section 用运行期同一 binder/semantic validator 做 typed round-trip |
| 缺失/零值/默认 | 默认值由各 capability 声明；敏感字段为空或安全 placeholder | 每字段 missing/empty/disabled/default 语义必须来自 owner；禁止真实凭据和隐藏 fallback |
| 副作用/所有权 | 目录新建 `0700`、临时文件 `0600`；默认不覆盖；force 显式替换；管理器拥有临时文件清理 | 文档只承诺已验证平台/文件系统范围；补父目录持久化边界说明，不泛称跨平台 crash-atomic |
| 错误/取消/清理 | 取消、短写、Sync/Close/replace/cleanup error 保留；失败删除 temp | config contract error 映射 ExitConfig；任何候选失败不得触碰目标文件 |
| 并发/生命周期 | one-shot，无 goroutine；无 force 使用 exclusive link 防覆盖 | 并发生成同目标必须恰有一个成功；force 竞争语义需要明确测试或标为不支持 |
| 证据 | `internal/kernel/config/default*.go`、`internal/kernel/cli/config_contract.go` 与测试 | R018；新增 default-to-strict-bind round-trip 门禁 |

## 4. Config 契约卡

### 4.1 `CFG-001` / `CFG-005`

| 维度 | 当前事实 | 缺口与目标 |
|---|---|---|
| 输入/输出/依赖 | Source 输出 `map[string]any`；Loader 按注册顺序 merge；入口为 File 后 Env，所以 Env 优先 | Source 需要非空唯一 name、明确 priority/order、受限 canonical value domain |
| 前置/后置/状态 | 每次 Load 产生 Snapshot、SHA-256 digest、provenance、redacted copy | 只有全部 source/结构检查成功才产生候选；失败不改变当前 Snapshot |
| 缺失/零值/默认 | nil source 被跳过；缺失 section 返回空 Snapshot | nil/空名应在构造期失败；缺失 section 的默认行为由 section owner 明示 |
| 副作用/所有权 | File/Env 读取外部状态；Snapshot 试图深拷贝 map/slice | `Source` 必须响应取消；任意 pointer/自定义 mutable value 不得进入 Snapshot |
| 错误/取消/清理 | source error 加名称上下文；redaction 按敏感 key 启发式处理 | 敏感字段元数据由 owner 声明，启发式仅做保底；错误不得包含原始 secret 值 |
| 并发/生命周期 | Snapshot 对常见 JSON-like tree 可安全复制；Loader 本身无运行状态 | 同一启动/reload 仅加载一次，并把同一不可变候选交给所有 owner |
| 证据 | `internal/kernel/config/source.go`、`loader.go`、`snapshot.go`、测试 | 增加 mutable value、source identity/priority、取消和 redaction 测试 |

### 4.2 `CFG-002` / `CFG-003` / `CFG-004` / `CFG-006` / `CFG-007`

| 维度 | 当前事实 | 缺口与目标 |
|---|---|---|
| 输入/输出/依赖 | DefaultContract 生成通用 Object；运行期各 component 用 `DecodeSection` 写入 typed Config | 每个 section 注册把 path、default projection、strict binder、semantic validator、change classifier、sensitivity 关联为同一 owner contract |
| 前置/后置/状态 | 默认对象只做通用值域校验；typed Config 在 Stage/Build 各自归一化和校验 | 阶段固定为 source syntax/merge -> strict bind -> 字段/跨字段校验 -> resource probe -> commit |
| 缺失/零值/默认 | 多数 decoder 先填 `DefaultConfig`；数值 `0` 常被归一化成默认；logger 用 pointer bool；Cache/Storage 用 enum disabled | 每个字段明确 missing、explicit zero、empty、disabled、default；需要区分时用 pointer/optional/enum，不用任意 map |
| 未知/废弃/版本 | `mapstructure` 开 WeaklyTypedInput，未开 ErrorUnused；没有 deprecated/version 规则 | 未知字段和 YAML 重复字段默认失败；deprecated 只允许有 owner、截止条件和诊断；真实迁移前不虚构版本层 |
| 副作用/所有权 | decode/字段校验应无副作用；资源连通性在 Build/Ready | validator 不拥有资源；probe 创建的候选由 component owner 关闭或提交 |
| 错误/取消/清理 | 当前 decode error 保留路径有限；资源错误由 component 包装 | 错误携带 source/section/field/stage/category，敏感值脱敏，保留原始 cause/cancel/timeout |
| 并发/生命周期 | typed candidate 在发布后按不可变使用 | defaults 与 runtime load 必须走同一 binder/validator；reload 分类由 owner 显式返回 |
| 证据 | `internal/kernel/config/defaults.go`、`decode.go`、各 `internal/kernel/app/*_component.go` | R019；需要 strict/round-trip/unknown/duplicate/deprecated 测试 |

最小注册单元是“配置节契约”，不要求每个 Config 都创建 interface。具体 Go API 尚未确认，但必须能表达以下关系，且不得演化成巨型配置结构：

```text
SectionID + ConfigPath
  -> Defaults projection
  -> Strict typed binding
  -> Semantic validation
  -> Change classification
  -> Sensitivity metadata
```

## 5. Composition、Capability 与资源契约卡

### 5.1 `CMP-001` / `CAP-001`

| 维度 | 当前事实 | 缺口与目标 |
|---|---|---|
| 输入/输出/依赖 | composition 按显式顺序 Add/Replace/Freeze；typed Binding/Input；输出 Kernel、Capabilities、Defaults、可选 CLI | Application coordinator 只协调候选和 mode，不成为第二个 DI/Resolver |
| 前置/后置/状态 | duplicate component ID、冻结后修改失败；Compose 是唯一生产入口 | 默认/CLI/配置节冲突也应在冻结/启动副作用前统一失败 |
| 缺失/零值/默认 | 可选 Logger 替换、可选 CLI；Capability 组合显式 | 不把 nil capability 当 disabled；disabled 由 owner 的专用状态表示 |
| 副作用/所有权 | Plan 定义阶段应无外部副作用；资源在 RuntimeComponent 构造 | `Compose` 不探测资源；普通业务构造函数不得查询容器或拿全量 Capabilities |
| 错误/取消/清理 | 注册错误在 Compose 返回；构造/启动错误在 Kernel | 错误标识 component/phase；冻结前静态治理所有 ID/依赖/path 冲突 |
| 并发/生命周期 | FrozenPlan 只读；stable facade 支持代际切换 | Application graph 在真实用例出现后用普通构造函数显式建图，不进入 Kernel Plan |
| 证据 | `internal/kernel/app/plan.go`、`internal/kernel/composition/composition.go` 与测试 | 保留当前主轴，不新增 Container、Provider、扫描或运行时 lookup |

### 5.2 `RES-001` / `RES-002`

| 维度 | 当前事实 | 缺口与目标 |
|---|---|---|
| 输入/输出/依赖 | RuntimeComponent 接收 section candidate，输出受 owner 管理的资源；消费者通过 Lease/Access 借用 | HTTP listener、server、runner 也必须有唯一 owner、停止信号和 Wait |
| 前置/后置/状态 | pending -> serving -> draining -> stopped；reload 候选完整后换代 | initial Start 当前逐组件 Ready/Publish，不得声称跨组件原子；需要明确对外不可见前提或改为批量发布 |
| 缺失/零值/默认 | disabled 是 Cache/Storage 显式模式 | 资源缺失、disabled 和 unavailable 必须是不同状态/错误 |
| 副作用/所有权 | component owner 关闭 DB/backend/storage；借用者不能 Close 共享资源 | 任何探测失败、启动失败、旧代清理失败都能定位 owner 与剩余资源 |
| 错误/取消/清理 | 启动失败反序清理并 join；Lease drain 等 active use | 完整进程回滚需包含 non-Kernel owner；cleanup error 持久诊断而非只返回一次 |
| 并发/生命周期 | Lease 阻止 drain 后新借用，等待 active use | 借用必须在 callback/lease scope 内；不得缓存 generation-specific raw client |
| 证据 | `internal/kernel/app/runtime_component.go`、Lease 和各 component 测试 | 增加首次发布可见性、跨 owner 回滚和 degraded 测试 |

## 6. Host、Supervisor、Reload 与诊断契约卡

### 6.1 `LIFE-001` / `LIFE-002` / `LIFE-003`

| 维度 | 当前事实 | 缺口与目标 |
|---|---|---|
| 输入/输出/依赖 | Participant Start/Stop；Task Run；Host 注册 Kernel、application lifecycle、Watcher | supervised unit 需要非空唯一 ID、blocking Run/Wait、runtime error、stop/wait owner |
| 前置/后置/状态 | 顺序 Start；Task 全返回后反序 Stop；启动失败反序 Stop | Ready 仅在所有必需单元已运行；任一关键 runner error 或意外 nil 返回触发取消和终止 |
| 缺失/零值/默认 | nil Participant 被跳过；空/重复 Task 名允许；nil completion 无分类 | 构造期拒绝 nil/空/重复；one-shot 与 service runner completion policy 显式区分 |
| 副作用/所有权 | Supervisor 拥有 task goroutine；Participant 自己拥有资源 | owner 必须能发起停止并 Wait；不合作 runner 进入结构化超时诊断 |
| 错误/取消/清理 | 首个非 nil task error 取消 siblings；共享 stop timeout；join Stop errors | 错误保留 start/runtime/stop/wait phase；总期限及 owner 子预算可验证，不能无限 Wait |
| 并发/生命周期 | errgroup 等所有 task；Watcher 合作取消 | 目标状态机见 [runtime-state-machine.md](../design/runtime-state-machine.md) |
| 证据 | `pkg/supervisor/supervisor.go`、`internal/kernel/host.go` 和测试、R012/R013 | 必须补 nil-return、runtime failure、uncooperative runner、readiness/diagnostics 测试 |

### 6.2 `REL-001` / `REL-002`

| 维度 | 当前事实 | 缺口与目标 |
|---|---|---|
| 输入/输出/依赖 | Watcher 串行触发 Kernel.Reload；component 声明 RestartRequired 或 swap | Loader 由候选协调者唯一调用，application 与 Kernel 对同一 candidate 预检 |
| 前置/后置/状态 | RestartRequired 在 Kernel 副作用前失败；候选失败保持旧代；commit 后更新 snapshot | application-only 变化不得被 Kernel 单独吞掉 digest；所有 owner 同意后才进入副作用 |
| 缺失/零值/默认 | 无变化仍可能更新整份 Snapshot；没有 application section | change classifier 明确 unchanged/live-replace/restart-required/not-reloadable |
| 副作用/所有权 | prepare candidate 可创建资源，失败由 component 丢弃；commit 后清理旧代 | 每个候选资源只有一个 owner；cleanup 失败记录 generation/owner 并进入 degraded |
| 错误/取消/清理 | reload drain 失败可 Rollback/Resume；Stop drain 失败当前也 Resume | reload 可恢复旧代；terminal drain 一旦开始不得回到 ready/serving，错误仍继续 best-effort cleanup |
| 并发/生命周期 | Kernel 串行 operation；Watcher 串行 reload | reload 与 stop 互斥；degraded 后默认拒绝继续 reload，直至重启或明确处置 |
| 证据 | `internal/kernel/kernel.go`、`watch.go` 及 reload/stop tests、R014 | 增加 single-load、application preflight、terminal drain 和 degraded gate 测试 |

### 6.3 `OBS-001` / `OBS-002`

| 维度 | 当前事实 | 缺口与目标 |
|---|---|---|
| 输入/输出/依赖 | baseline Logger 在配置前可用；configured Logger 成功构造后替换；error 保留 cause | Diagnostics 由 Application owner 聚合，不从可选 public adapter 推断基础日志 |
| 前置/后置/状态 | baseline 最终由入口关闭；Manager 是 stable facade | 状态快照至少含 process state、ready reason、generation/digest、last reload/cleanup |
| 缺失/零值/默认 | health Registry 没有 production caller | 缺失 checker 与 disabled capability 分开；无诊断不能解释为 healthy |
| 副作用/所有权 | 决策边界记录一次错误；reload callback 当前仅日志 | diagnostics 只读；不得输出原始配置、Token、DSN、密码或 secret diff |
| 错误/取消/清理 | 多错误 join；typed CommittedCleanupError | 诊断应保存结构化 category/owner/phase/time，不保存敏感 cause payload |
| 并发/生命周期 | Logger facade 可并发；health Registry 顺序/顺序不稳定 | Diagnostics snapshot 并发安全且代际一致；readiness 在 drain/failure 前置变 false |
| 证据 | `internal/kernel/logging`、`pkg/health`、Kernel error tests | 增加状态转换、脱敏、generation 一致性与 last-error 测试 |

## 7. Adapter 边界

| ID | 定义/调用方 | 资源与错误语义 | 当前判断 |
|---|---|---|---|
| `ADP-HTTP` | `pkg/httpx` 只有 Router/Middleware/Server 原语，无 production caller | `ListenAndServe` 阻塞，`Shutdown` 不等于 owner 已 Wait；未预绑定 listener，端口错误不能作为同步 Start 失败 | 不完整；先补生命周期 Adapter，不设计业务 Handler |
| `ADP-DB` | composition 注册 Database component，消费者借 `database.Access` | owner 构造、ping、换代和 Close；消费者不可关闭共享 GORM DB | 已实现并保留；Repository port 等真实业务需求出现 |
| `ADP-CACHE` | composition 注册 Cache component，模块可基于 Access 建 typed Client | disabled 显式，backend owner 关闭；typed client 归调用模块 | 已实现并保留；禁止绕过项目边界再建 Redis client |
| `ADP-STORAGE` | composition 注册 Storage component，消费者借 Client | disabled/local/remote，借用随 Lease 代际失效，manager 负责清理 | 已实现并保留；安全默认值与敏感字段规则需纳入 Config contract |

HTTP、Database、Cache、Storage 的共同不变量是：第三方具体类型不成为跨模块公共契约；创建者是唯一释放者；借用者只能在有效 lease/scope 内使用；配置、取消、超时、资源探测与清理错误由 Adapter owner 统一处理。

## 8. 当前稳定、隐式与缺失结论

- 稳定并应保持：薄进程入口、显式 Plan/Freeze、单一 composition root、typed Binding/Input、stable Capability facade、Lease、Kernel reload candidate transaction、baseline Logger、DB/Cache/Storage owner 边界、默认配置的安全临时文件写入。
- 隐式且需显式化：`args` 驱动模式、CLI 资源需求、配置 precedence 身份、缺失/零值/默认规则、validation stages、首次启动发布可见性、Host readiness 和终止 drain。
- 关键未闭环：严格 Config schema/binding、default-to-runtime round trip、单一 application candidate、CLI 完整冲突与副作用治理、Supervisor runtime completion、HTTP lifecycle、进程状态/诊断、cleanup degraded 和自动化架构门禁。
- 当前架构不需要整体替换。建议保留 Kernel 资源平面，补齐薄 Application 协调与治理；任何 Container、全量 Capabilities、无类型 Map、隐藏 Provider、运行时 lookup 或第二套装配体系都不在目标内。
