# 应用模块开发指南

本文是当前项目新增应用模块的权威开发入口。目录职责和允许依赖见 [`internal/module`](../../internal/module/README.md)，当前与暂缓的底层 Capability 见 [`pkg`](../../pkg/README.md)，Kernel App 组件形态和接入 API 见 [Kernel App 组件开发](../../internal/kernel/app/README.md)。

应用模块是由唯一 application composition root 显式选择的进程内纵向业务单元，不是 Go module、Kernel Component、动态插件或自动发现的 Provider 集合。普通 Model、Service、Repository、Adapter、Handler、binding 和 contribution 按业务名称收口，不进入 Kernel Plan。

新增业务能力必须先完整收口到 `internal/module/<name>`。这里的“完整”指 model、repo、service、handler、Adapter、binding、配置、migration/运行单元与 contribution 等真实职责都由该模块拥有，不要求为不存在的职责制造空目录。只有能力评估同时证明资源跨业务复用且由进程统一选择，才允许把底层资源提升到完整 Kernel Capability 链。

## 1. 开始条件

收到“新增应用模块”请求后，不先复制 Todo 目录，也不先创建空 Handler、Repository、配置或 CLI。研究阶段先写清：

- 业务能力名称、actor、触发条件、成功结果和不变量；
- 输入缺失、重复、冲突、取消和依赖失败的语义；
- 真实需要的 HTTP、Application CLI、后台运行单元或其他入口；
- 数据所有权、事务范围、一致性和迁移要求；
- 所有外部副作用和依赖能力；
- 验收示例与明确非目标。

如果这些问题不足以判断依赖、资源和失败边界，继续研究，不预建模块骨架。

## 2. 必答能力评估

每个新增模块的研究报告必须填写以下评估。字段必须出现；不适用时写明原因。即使结论为“不需要新能力”，也必须列出现有能力复用证据，不能用未提及表示已经检查。

| 维度 | 必答问题 | 交付结果 |
| --- | --- | --- |
| 用例 | 哪个 actor 触发什么行为，有哪些外部副作用 | 用例、约束与非目标 |
| 现有能力 | `pkg` 和 production composition 已有哪些能力可复用 | 能力、入口与代码证据 |
| 新能力 | 是否新增外部系统、SDK、协议或跨模块能力 | 有/无及理由 |
| 归属 | 是否只服务本模块；若拟升级底层，是否同时有跨业务消费者和进程统一选择证据 | 唯一语义 owner 与分类矩阵结论 |
| 资源 | 是否有连接、Client、listener、订阅、goroutine、缓存或派生句柄 | 构造和释放 owner |
| 运行 | 是否需要 Start、Ready、Health、Run、Stop、Wait | 运行 owner 和顺序 |
| 配置 | 是否有配置、Secret、Defaults、严格校验和变化分类 | section owner 与失败语义 |
| 出口 | 普通接口、稳定 facade 还是 Lease Access | 最小调用契约 |
| Reload | 新旧实例能否并存、排空和回滚，还是只能重启 | 适用策略与证据 |
| 契约适配 | 当前项目契约能否表达全部保证 | 适用、不适用或待研究 |
| 失败 | 取消、超时、重试、幂等、背压、降级和清理错误如何处理 | 可识别错误与诊断 |
| 日志 | 哪些生命周期、外部结果和状态变化必须记录；谁是唯一 owner；level、字段、脱敏和验证是什么 | 引用[开发日志规范](logging.md)的事件与验收清单 |
| 影响 | composition、Host、HTTP、CLI、配置、迁移和测试受何影响 | 文件与验收清单 |

能力评估的第一步是核对真实代码，而不是只读能力名称：

1. 从 [`composition.Capabilities`](../../internal/kernel/composition/composition.go) 确认当前进程实际选择并输出的稳定能力。
2. 从 [`pkg/README.md`](../../pkg/README.md) 确认项目能力契约、第三方边界和暂缓路线。
3. 追踪候选能力的构造位置、调用方、配置、资源 owner、停止和测试。
4. 只在现有能力确实不满足真实用例时提出新 Capability。

## 3. 能力归属决策

对每项依赖作出以下四类结论之一。

### 3.1 复用现有 Capability

composition root 从现有稳定能力取出模块真正需要的最小接口并显式注入。不得把完整 `Capabilities`、Kernel、Resolver 或共享可变容器传入模块。

### 3.2 模块专属 Adapter 或运行单元

只服务单个业务语义、没有跨模块稳定契约价值的第三方 SDK、Client 或协议转换，留在 `internal/module/<name>/adapter/<technology>`。Adapter 依赖并实现模块调用方定义的窄 port；第三方类型、错误、配置对象、Option、Client 和关闭权不得越过 Adapter package。模块的 Model、Service、binding、`Dependencies`、`Module` 与 contribution 只暴露标准库或项目自有类型，composition 不得穿透模块根导入其私有 Adapter。

模块专属 cache、goroutine、migration、消费者循环、索引同步或清理任务以有明确 owner 的 binding/contribution/Participant 接入应用；拥有第三方 SDK 或 goroutine 本身不构成 Kernel Capability 升级理由。

### 3.3 跨业务且由进程统一选择的底层 Capability

只有能力评估同时证明以下两项，底层资源才进入完整 `pkg/<name> -> internal/kernel/app/<name> -> internal/kernel/composition` 链：

1. 已有跨业务消费者或资源语义明确不属于任何单一业务模块；“未来可能复用”不算证据。
2. 实现、配置、资源 identity、生命周期或替换策略必须由进程 composition 统一选择，模块不能各自安全构造。

只满足跨业务复用但不需要进程选择的普通库，可以评估进入 `pkg`，但不得虚构 Kernel App 组件。只服务一个模块，即使拥有 SDK、Client、cache、连接或 goroutine，也继续收口在该模块并通过 contribution/Participant 管理生命周期。

业务专属与底层 Capability 是互斥分类。当前 Observability 同时覆盖 Auth/Todo 业务 HTTP 与 Ops management/diagnostics，且 registry/provider/exporter 由进程统一选择和治理，因此满足双条件；[027](../changes/027-business-module-third-party-isolation/README.md) 已将具体实现迁到 Kernel App，Ops 与 application composition 只消费项目自有契约。

### 3.4 证据不足

不能确定调用语义、资源 owner、一致性或失败保证时，保持研究状态。不得用占位 Adapter、空生命周期 Hook、虚构配置或 `TODO` 冒充接入完成。

## 4. 模块开发路径

能力评估通过后，按真实复杂度建立模块，不为了目录对称创建空层：

```text
internal/module/<capability>/
├── model/          # 业务状态、值与不变量
├── service/        # 用例与调用方拥有的窄 port
├── adapter/        # 仅模块专属第三方实现；不得向外泄漏技术类型
├── repo/           # 持久化 Adapter；仅在真实需要时建立
├── handler/        # 模块顶层 HTTP handler：Operations/Handler、DTO 映射、错误呈现、请求窄端口
├── middleware/     # 模块拥有的 HTTP 横切策略
├── binding/
│   ├── config/     # 模块配置 owner
│   ├── model/      # Schema/migration 等完成品
│   ├── http/       # 只做代码优先契约声明（ModuleContract）与运行期装箱（RuntimeHandlers）
│   └── cli/        # 模块自有 CLI Handler/已绑定命令 contract
└── module.go       # 纯内存局部装配与 contribution 输出
```

Todo 是当前完整示例，但只复制依赖方向和验证方法，不复制业务字段或无关入口：

```text
model <- service <- repo
   ↑         ↑
 handler  module.go <- internal/composition
   ↑
 binding/http   （契约声明 + RuntimeHandlers 装箱）
```

开发顺序：

1. 用 Model 表达业务状态与不变量；没有独立领域行为时不制造空领域层。
2. Service 定义用例和自己需要的 Repository、跨模块、Clock、ID 等窄 port。
3. 先用 fake port、固定时间和固定 ID 验证成功、冲突、依赖失败、取消与超时。
4. 实现实际需要的数据库、缓存、远程协议或其他 Adapter，并验证第三方错误转换、exported 类型、配置和资源边界；只有跨业务复用与进程统一选择两项均有证据时才走完整底层 Capability 路径。
5. 实现真实验收需要的 HTTP、CLI 或后台入口；HTTP 语义适配落在模块顶层 `handler/`（实现 `Operations`/`Handler`、DTO、错误呈现与 `ActorAccess`），`binding/http` 只声明代码优先契约（`pkg/httpx/contract.Module`）与 `RuntimeHandlers` 装箱；handler 不创建 Router、不加载 OpenAPI、不 import `binding/**` 或 `internal/transport/**`；不同入口复用同一 Service，不互相回环。
6. `module.go` 只做无 I/O、无 goroutine、无资源探测的局部装配，并返回窄 Handler/Service 与完成品 contribution。
7. `internal/composition` 显式选择模块、适配最小 Capability、连接跨模块 port、聚合模块基础契约与运行期 handler、合并 contribution 并建立 Host；`internal/tools/contract-gen` 从模块契约生成 `api/openapi.yaml` 与 operation inventory，`internal/transport/http` 只把完整契约绑定一次路由与校验。

HTTP 的固定构造顺序是 `模块顶层 typed Handler + binding 契约/装箱 → composition 聚合 contract + 运行期 handler → transport 一次绑定契约校验与路由 → application Router → Server`。最外层 Router 只拥有全局 middleware 和一次 API route tree 挂载；生成器从模块契约渲染 `api/openapi.yaml` 与 operation inventory。新增模块只增加自身 Handler/运行期 handler、契约声明、aggregate 转发与 composition 连接，不修改既有模块 Handler，不复制 method/path 或完整 Router，也不写第二份全局 OpenAPI。

### 4.1 统一 binding 契约清单（033）

业务模块按需提供以下 binding / 接入契约，每类的声明位置、接入方式、维护位置统一如下。仅按真实需要建立对应的 binding，不为了目录对称创建空层。

| Binding / 契约 | 声明位置 | 接入方式 | 维护位置 |
| --- | --- | --- | --- |
| HTTP binding | 模块顶层 `handler/`（Operations/Handler/DTO/ActorAccess/错误呈现）+ `binding/http`（`ModuleContract`/`RuntimeHandlers`） | 注册 `contract-gen` 的 `registeredModules()`；`internal/composition` 聚合；`internal/transport/http` 绑定 | `internal/module/<name>/handler` 与 `binding/http` |
| config binding | `binding/config` | composition 连接模块 Config | `internal/module/<name>/binding/config` |
| cli binding | `binding/cli` | cmd（command）装配 | `internal/module/<name>/binding/cli` |
| migration binding | `binding/migration` | composition / migrate 使用 | `internal/module/<name>/binding/migration` |
| i18n binding | `binding/i18n`（模块自有语言资源 + 窄契约，如 `MessageFiles()`/`fs.FS`/catalog） | composition 显式聚合进 Non-Essential I18n 装配，再按模块注入 `pkg/i18n.Translator` | `internal/module/<name>/binding/i18n` 与模块内语言资源 |
| middleware | `middleware/`（横切策略） | composition 挂载 | `internal/module/<name>/middleware` |

**新增业务模块必须接入的基础契约**：

- 若暴露 HTTP operation：必须提供 `handler/` + `binding/http` 的 `ModuleContract`/`RuntimeHandlers`，**并在两处接入**：`internal/composition` 负责运行时装配（policy 汇总、observability operations、route binding、依赖注入），`internal/tools/contract-gen` 的 `registeredModules()` 生成器注册点负责 build-time 渲染 `api/openapi.yaml` 与 operation inventory；新增模块不得只在 composition 装配而漏掉契约注册（否则 `go generate` 不渲染），也不得退化为手写固定路由。
- 若有用户可见翻译：必须提供 i18n binding（自有语言资源 + `binding/i18n`），经 composition/kernel 聚合后通过注入的 `pkg/i18n.Translator` 消费；不得绕过注入直接读 `pkg/i18n` 默认配置。
- `module.go` 只做纯内存装配并返回窄 Handler/Service 与 contribution；`internal/composition` 是唯一跨模块连接点。
- 保留 032 配置边界：`pkg/*` 只提供通用能力 + 基础默认；`kernel/app/*` 负责应用层默认与装配，不隐式依赖 `pkg/*.DefaultConfig()`。

## 5. 资源和运行 owner

一个功能可以同时包含底层共享资源和模块专属运行单元，必须分别确定 owner：

- 共享连接、连接池、可换代 Client 或稳定能力出口只有同时跨业务复用且由进程统一选择时才由 Kernel App 管理；
- 业务消费者、migration、索引任务和模块 Cleanup 由模块 contribution 声明并由 Host/Supervisor 管理；
- listener、Server 和进程级 runner 由 application owner 管理，模块不自行创建第二套 Server；
- 创建资源的一方负责关闭，借用者不获得共享资源 `Close` 权；
- 每个 goroutine 必须能指出 owner、取消来源和 Wait 位置；
- Stop、Wait、Close 同时失败时保留主错误与全部清理错误。

构造阶段只建立普通对象图，不能访问数据库、网络、打开 listener 或启动 goroutine。需要依赖已启动资源的探测、migration 和长期运行必须进入显式生命周期阶段。

## 6. Kernel App 形态选择

只有能力评估已经判定为“由当前进程选择并注入的底层 Capability”后，才进入本节。可编译 API 和完整约束以 [Kernel App 组件开发](../../internal/kernel/app/README.md) 为准。

| 真实能力特征 | 当前形态 | 输出与 Reload |
| --- | --- | --- |
| 代码固定且无资源生命周期 | `app.Value` | 普通项目接口，不进入运行节点 |
| 无运行期配置但需要 Start/Stop | `app.ManagedFixed` | 稳定 Lease facade，`NoReload` |
| 配置化且新旧实例可安全并存 | `app.ManagedConfigured` | 稳定 Lease facade，`KernelInstanceSwap` |
| 配置变化不能安全热换 | `app.ManagedConfigured` | 稳定 Lease facade，`RestartRequired` |
| 明确替换同一既有稳定 target | `app.ManagedConfiguredReplacement` | 复用 target 输出，`KernelInstanceSwap` |
| 影响当前同步 HTTP 模块对象图 | Application Generation | 完整重建 Router/Server，对新连接提交并排空旧代 |

分别回答以下问题，不能因为某个 API 已存在就机械套用：

- 构造参数由代码固定还是来自 typed Config；
- 是否真正需要 Defaults、CLI、Ready、Health、Start 或 Stop；
- 调用是否能被一次 Lease 回调完整包围，派生句柄是否可能逃逸；
- 候选与当前实例能否并存，旧实例何时允许排空；
- 失败时继续使用旧实例是否有明确价值；
- 配置改变的是底层资源，还是整个模块对象图、路由或订阅关系。

配置存在不等于能够热重载。当前同步 HTTP Service 已能把模块对象图、路由与底层能力放入完整 Application Generation；模块配置必须通过 composition factory 重建，而不是让 Kernel 单独接受 section。排他订阅、hijacked connection 或无法证明 admission/drain 的资源仍使用 `RestartRequired` 或先建立专用 Handoff，不能机械套用当前 HTTP 协议。

## 7. 当前契约不适用时

出现以下任一事实时，明确记录“当前契约不适用”，停止能力实现并建立独立研究或 ADR：

- 新旧资源不能并存，但需求要求不中断切换；
- 需要排他资源 Handoff、消费者组原子交接或监听端口移交；
- 需要资源自身 Native Atomic Reload；
- 需要提交后的观察期、自动回切或跨代结果比较；
- 需要当前前向 typed Input 无法表达的复杂资源依赖或多资源原子性；
- 无法确定借用对象、派生句柄、活跃请求或后台任务的排空边界；
- 失败、重试、幂等、补偿、一致性或恢复保证尚未定义。

升级流程：

1. 在当前模块研究中记录具体用例、缺口和受影响契约。
2. 比较 Application Generation、`RestartRequired`、模块级受管运行单元和专用 Handoff 四条路径。
3. 改变公共生命周期、依赖模型或资源切换语义时建立新的 `docs/changes/<seq-num-name>/`。
4. 难以逆转的长期决策使用 ADR。
5. 新方案确认前，模块任务保持待确认，不增加空 Hook、静默回退或临时兼容层。

## 8. 典型能力示例

以下示例只展示必须研究的问题和所有权拆分。项目当前没有选择或实现通用消息、邮件、搜索 Capability，真实任务必须重新研究技术选型和契约。

### 8.1 消息系统

共享连接、连接池或稳定 Publisher/Subscriber 出口只有同时存在跨业务消费者并由进程统一选择时才是底层 Capability；具体业务 Consumer、消息转换和订阅语义属于拥有用例的模块，其长期消费循环作为 Participant 交给 Host/Supervisor。

研究必须回答：

- at-most-once、at-least-once 或其他交付语义；
- ack/nack、幂等、排序、重试、死信和背压；
- 断线恢复、订阅就绪和停机排空；
- 消息处理与数据库提交的一致性或补偿；
- 配置变化是否要求消费者组排他 Handoff。

如果消费者组切换要求无损原子交接，不能把普通 `KernelInstanceSwap` 当作已经支持；应选择重启生效或建立新的生命周期设计。

### 8.2 邮件

无长期资源的同步 API 调用可以是窄项目 Adapter；持久连接、模板缓存、异步投递队列和重试 Worker 会引入资源与长期运行 owner。

研究必须区分：

- “发送请求已接受”与“邮件最终送达”；
- 模板、附件、地址和敏感字段的所有权；
- 幂等键、限流、可重试错误和永久失败；
- 同步失败、异步补偿和投递诊断；
- Client 连接、队列和 Worker 的关闭与排空。

### 8.3 搜索

共享搜索 Client 只有同时跨业务复用并由进程统一选择时才是底层 Capability；查询模型、索引文档转换、增量同步和重建任务属于拥有数据语义的模块。

研究必须回答：

- Database 与索引之间的一致性目标和延迟窗口；
- 写入失败、重复事件、乱序和补偿；
- 全量重建、断点恢复和流量切换；
- 索引 schema/version、别名和回滚；
- 查询不可用时失败还是可观测降级。

需要原子索引或别名切换时，必须验证具体搜索系统的原生保证；不能从 Kernel Swap 自动推导支持。

### 8.4 i18n 接入规范

i18n 是业务模块的正式 binding 契约（033）：业务模块按统一方式提供自身的 i18n 语言资源与 `binding/i18n`，而不是仅由底层 `kernel/app/i18n` 统一处理。业务模块统一通过注入的 `pkg/i18n.Translator` 消费翻译，不自行创建 Translator，不直接读取或依赖 `pkg/i18n` 的默认配置。

- **i18n binding 声明位置**：`internal/module/<name>/binding/i18n`（例如 `catalog.go`），暴露模块自有语言资源（如 `MessageFiles() []string`、`MessagesFS() fs.FS` 或静态 catalog），并在该目录下提供模块语言内容（如 `locales/messages.<lang>.yaml`，相对模块目录）。
- **接入方式**：`internal/composition` 显式聚合各业务模块的 i18n binding 语言资源到 Kernel I18n App（`internal/kernel/app/i18n`）或组装进 `i18n.messageFiles`，再按模块注入 `pkg/i18n.Translator`（如 Todo 的 `HTTPDependencies.Translator`）。Handler 只在呈现边界调用 `Translate`/`MustTranslate`。不引入动态注册/Service Locator，聚合由 composition 显式完成。
- **统一路径**：全局 `./locales` 仍是默认语言目录与聚合源（由 `internal/kernel/app/i18n` 集中声明 `LocalesDir`）；业务模块自有的 `binding/i18n/locales/` 语言资源可被聚合进同一 Translator。语言文件按 `messages.<lang>.yaml|yml|json` 命名（lang 为 BCP 47，如 `zh-CN`、`en`）。
- **维护位置**：新增或修改语言内容优先在业务模块自身的 i18n binding 语言资源中维护（`internal/module/<name>/binding/i18n/locales/messages.<lang>.yaml`），并在 `binding/i18n` 暴露、经 composition 聚合；全局 `./locales/messages.<lang>.yaml` 作为聚合/默认源。新增语言即新增对应消息文件并暴露。
- **消息 ID 约定**：使用 `<domain>.<type>.<key>` 命名，例如 `todo.error.todo_not_found`；同一模块的消息 ID 前缀统一。
- **禁止行为**：业务 handler 不得绕过注入的 Translator 直接调用 `pkg/i18n.New`，不得复用 `pkg/i18n.DefaultConfig()` 作为应用默认；不得在未提供 i18n binding 的情况下靠全局路径散落声明，不得在不同模块各自建立第二套未聚合的语言资源维护方式。

## 9. 研究报告最小输出

新增模块的 `research/Rxxx-<semantic-name>/report.md` 至少包含：

```text
## 用例与外部副作用
## 现有 Capability 复用清单
## 新 Capability 与第三方边界
## 资源、运行和配置 owner
## 生命周期与 Reload 分类
## 当前契约适配性
## 错误、一致性和诊断
## composition、Host、入口和迁移影响
## 结论、未知与升级决定
```

允许合并标题，但所有语义必须可检索。结论必须是以下之一：

- 现有能力足够，可以形成模块实施计划；
- 需要模块专属 Adapter/Participant，当前公共契约足够；
- 需要新的底层 Capability，先形成独立接入设计；
- 当前契约无法表达需求，停止计划并升级研究或 ADR；
- 证据不足，研究门禁未通过。

## 10. 验证与完成标准

模块最低验证按实际入口和资源选择，不为目录完整制造无关测试：

- Model/Service 成功、输入、冲突、依赖失败、取消和超时测试；
- Repository/远程 Adapter 的转换、错误链、第三方类型零泄漏、资源借用与清理合约；
- HTTP/CLI/后台入口的协议边界和同一 Service 复用；
- route、command、module ID、Participant owner 和 import 架构检查；
- startup、ready、运行失败、反向停止、超时和资源泄漏测试；
- 配置 strict binding、Secret、RestartRequired 或候选切换测试；
- 真实垂直切片验收及当前文档同步；
- 搜索确认没有旧入口、重复客户端、旁路构造或失效文档。

构建通过不能替代资源生命周期或产品验收。交付必须区分已通过、未执行和被外部环境阻断的验证。
