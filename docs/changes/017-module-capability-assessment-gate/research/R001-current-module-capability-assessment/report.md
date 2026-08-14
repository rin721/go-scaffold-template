# R001：当前应用模块能力评估缺口

## 1. 研究问题与范围

本研究回答：当用户只说“新增一个应用模块”时，当前 `AGENTS.md`、项目文档和代码边界是否足以让 AI Agent 稳定联想到以下问题，并把结论交付给用户：

- 现有 Capability 是否已经满足模块需求；
- 是否需要消息、邮件、搜索等新的底层能力；
- 新能力是否拥有连接、listener、goroutine、订阅或其他必须治理的资源；
- 资源应由模块、Host 还是 Kernel 管理启动、停止、重载和释放；
- 当前 Kernel 生命周期契约是否能真实表达该能力；
- 契约不适用时是否会停止实现并转入独立研究、设计或 ADR。

范围覆盖当前 HEAD `2a0b454969b90076e937eab37d250f39006dfd90` 的规则、文档导航、Kernel App 组件说明、真实 composition、应用模块说明、Todo 研究先例和架构测试。不选择消息中间件、邮件服务或搜索产品，也不修改运行时代码。

## 2. 方法与既有研究复用

研究先检索 `docs/**/research/**/metadata.yaml`，再打开与模块、Capability 和生命周期直接相关的报告：

- 012 R001 的初始能力事实已被后续研究替代，只作为历史定位；
- 012 R015 说明 package graph 能约束 import，但不能证明研究问题已经被回答；
- 014 R001 是首个真实模块的有效先例，它先盘点现有能力，再得出 Todo 不需新增底层能力的结论；
- 016 R001 只处理应用模块命名，不适用于能力或生命周期调整。

随后逐项核对 `AGENTS.md`、根 README、`docs/README.md`、`internal/module/README.md`、`pkg/README.md`、`internal/kernel/app/README.md`、真实 `Capabilities` 清单及 package graph 测试。研究只做静态分析，没有用另一个 Agent 进行提示词行为实验，因此以下“会不会联想到”是基于规则可发现性和强制输出要求的工程判断，不是模型概率测量。

## 3. 当前事实

### 3.1 通用规则已经覆盖必要工程底线

`AGENTS.md` 已要求：

- 接入通用能力或外部系统前，调查标准库、项目现有能力和成熟第三方技术栈；
- 第三方类型必须留在项目自有薄封装或 Adapter 后面；
- 封装统一处理配置、错误、取消、超时、资源释放和诊断；
- 资源创建者、goroutine owner、停止信号、等待和重载边界必须明确；
- 研究追踪真实 composition root、定义、调用方、错误语义和资源所有权；
- 依赖选择、模块边界或外部副作用实质变化时，返回研究并重新确认。

这些规则足以阻止一个认真执行的 Agent 把外部 Client 随意塞进 Service，但没有要求每个新增模块都交付一份“能力差距与生命周期适配性”结论。

### 3.2 项目已经提供完成判断所需的事实

当前项目入口明确区分两个平面：普通应用对象由 `internal/composition` 静态构造，当前进程选择的底层能力通过 `internal/kernel/app` 和 Kernel composition 显式进入 Plan。

真实 `composition.Capabilities` 已列出 Logger、Clock、ID Generator、Validator、Database、Cache、I18n 和 Storage。`pkg/README.md` 又明确把消息、后台任务、调度、分布式锁、认证、邮件、搜索、特性开关和观测采集列为等待真实场景的暂缓路线。因此 Agent 有能力判断“已有”与“尚无”，不需要猜测。

`internal/kernel/app/README.md` 已给出当前组件形态：

- 固定无资源能力使用 `app.Value`；
- 无运行期配置但需要 Start/Stop 的能力使用 `app.ManagedFixed`；
- 配置化资源按能否并存选择 `KernelInstanceSwap` 或 `RestartRequired`；
- 只有明确替换既有稳定 target 时使用 Replacement。

同一文档还明确：Native Atomic Reload、排他资源 Handoff 和切换观察期尚未实现。当前 Plan 是显式前向顺序和 typed Input，不是任意通用依赖 DAG。这些事实足以判断当前契约是否适用。

### 3.3 当前应用模块入口没有把判断变成必答输出

`internal/module/README.md` 当前要求新增模块复制 Todo 的依赖方向和验证方式，说明模块对象不进入 Kernel Plan，但没有要求在编码前列出：

- 模块所需能力清单；
- 现有能力复用证据；
- 新能力及外部系统清单；
- 资源 owner、生命周期和 Reload 分类；
- Kernel 契约适配性与缺口。

`docs/README.md` 当前直接链接底层能力库和 Kernel 文档，但没有“应用模块开发”主题入口。根 README 只在 Todo 学习示例后链接模块说明，新模块作者可能从 Todo 目录直接开始复制，而没有经过能力判定。

012 的“新模块开发黄金路径”包含最小 Capability、Participant、Cleanup、启动/停止和资源泄漏检查，但它属于历史变更记录，文件顶部仍保留当时的阻塞状态，不能承担当前权威操作指南。

### 3.4 自动门禁不能替代语义判断

当前 package graph 测试能拒绝模块核心导入 Kernel、HTTP、CLI 或 Database 技术边界，也能保护唯一 composition root。它不能判断：

- 邮件发送是否需要连接池或后台重试；
- 消费者是否需要 ack、幂等、背压、死信和排空；
- 搜索索引是否需要最终一致性或重建任务；
- 某个资源能否新旧并存，是否只能 `RestartRequired`；
- 当前 Kernel 是否缺少 Handoff、观察期回滚或其他生命周期语义。

这些问题必须由需求和资源事实驱动，不能可靠地由静态 import 测试推断。

## 4. 判断

### 4.1 Agent 当前会不会联想到

- 用户明确写出消息队列、邮件或搜索时：触发外部系统与第三方边界规则，联想到新 Capability 的可能性高。
- 需求隐含后台消费、连接池、异步重试或索引同步时：规则能提供线索，但是否继续追问取决于 Agent 是否主动打开 Kernel App 和能力清单，稳定性不足。
- 判断当前 Kernel 生命周期契约是否无法表达时：项目已有答案来源，但新模块入口没有强制导航和输出格式，容易被漏掉。

因此当前状态是“知识完备、入口分散、输出不强制”。不能把高概率行为描述为治理保证。

### 4.2 是否需要修改项目

需要一次小范围文档治理调整，但不需要修改 Kernel 运行时代码，也不应把项目特有的组件形态和目录规则全部塞进 `AGENTS.md`。

合理分工是：

1. `AGENTS.md` 只增加通用语境：新增模块、通用能力或外部系统时，必须按项目文档检查现有能力、边界、资源所有权、生命周期和基础契约适配性；项目没有可验证路径时停在研究/计划阶段。
2. 根 README 和 `docs/README.md` 把 Agent 导向当前项目的应用模块开发指南。
3. 项目指南拥有必答能力评估表、项目路径、Kernel 形态、升级条件和例子。
4. `internal/module/README.md`、`pkg/README.md`、`internal/kernel/app/README.md` 分别继续拥有目录边界、能力清单和组件 API，不复制第二套正文。

## 5. 建议的必答评估维度

每个新增模块研究必须显式填写，允许答案为“无”，但不允许省略：

1. 真实用例和外部副作用。
2. 现有 Capability 复用清单及证据。
3. 新 Capability、外部系统或第三方 SDK。
4. 模块专属 Adapter、跨模块通用能力与进程级底层能力的归属。
5. 连接、Client、listener、订阅、goroutine、缓存和派生句柄的唯一 owner。
6. Build、Start、Ready、Health、Run、Stop、Wait、Close 语义。
7. 配置来源、Secret、Defaults、严格校验和变化分类。
8. Direct、Lease 或稳定 facade 的出口需要。
9. `NoReload`、`KernelInstanceSwap`、`RestartRequired` 或 Replacement 的适用性。
10. 当前契约不支持的 Handoff、Native Reload、观察期回滚、复杂依赖或多资源原子性。
11. 超时、取消、重试、幂等、背压、降级、错误链和诊断。
12. 对 composition、Host、配置、CLI、HTTP、中间件、数据迁移和测试的影响。

## 6. 典型能力边界推断

### 消息系统

共享连接或连接池可能属于底层 Capability；具体业务消费者、Handler 和订阅语义属于应用模块，其长期循环作为有 owner 的 Participant 交给 Host/Supervisor。必须明确 ack、幂等、排序、重试、死信、背压、断线恢复和停机排空。若消费者组切换要求排他 Handoff，当前普通 Swap 不足，应选择重启生效或建立独立生命周期设计。

### 邮件

无长期资源的同步 API 调用可以是窄项目 Adapter；持久连接、模板缓存、异步投递队列和重试 Worker 会引入资源与长期任务。邮件“请求已接受”与“最终送达”必须区分，重试不能在没有幂等或诊断语义时隐藏在实现中。

### 搜索

搜索 Client 可能是共享底层能力；查询模型、索引文档转换、增量同步和重建任务属于拥有该数据语义的模块。必须明确数据库与索引的一致性、失败补偿、重建、别名切换和配置变更策略。需要原子索引切换时不能从当前 Kernel Swap 自动推导支持。

## 7. 局限、未知与刷新条件

- 没有进行多模型、多提示词的行为实验，因此不提供数值化遗漏概率。
- 没有真实的新消息、邮件或搜索需求，不能预先决定第三方实现、配置键、公开接口或最终 owner。
- 第二个应用模块或首个新底层 Capability 接入时，应按本方案刷新真实开发体验和遗漏点。
- 若 Kernel App 生命周期、Plan 依赖或 Host Participant 语义变化，本报告必须复核。

## 8. 对当前任务的影响与研究门禁

研究门禁通过。事实证明项目无需为了本问题修改 Kernel；需要的是通用 Agent 导航、项目级单一权威指南和新模块研究的必答能力评估。关键事实、推断、局限与非目标已经分离，足以形成文档实施计划。
