# 迁移建议

> 本章保留研究阶段的迁移建议。阶段 A、C 及阶段 D 的简单能力部分已由 006 实施；观察期、Native Reload、Handoff 等后续状态机调整仍须建立新的 `docs/changes/<seq-name>/` 任务并再次确认。

## 1. 迁移目标

目标不是推倒当前 Kernel，而是保留已经验证的资源治理能力，简化组件作者的开发入口，并补足最新确认的策略和观察期语义。

建议保留：

- `pkg` 不依赖 Kernel 的边界；
- 显式 composition 和固定启用清单；
- typed 配置和默认配置契约；
- 基线 Logger 与配置化 Logger 的所有权分离；
- stable Access、旧租约排空、候选失败保留旧实例；
- Supervisor 顺序启动、反向停止和 Watch Task；
- 完整错误链、超时与敏感信息脱敏。

建议收敛或扩展：

- `internal/kernel/capability` 重命名/迁移到开发者更易理解的 `internal/kernel/app`；
- Clock、ID Generator、Validator 等稳定能力也通过 `kernel/app` 和 composition 显式选择，但使用 Direct 输出且不强制 `Access.Use`；
- 增加仅服务底层组件的有序 typed `Binding/Input` 计划，禁止前向引用和运行期 Resolve；
- composition 不再手工搬运 Access、Defaults、CLI 契约；
- Definition 由一个组件入口组合可选小契约；
- 为每个组件增加显式 Reload Policy；
- 将 Ready/Health 接入候选准备和观察期；
- 成功切换后暂存上一代，观察通过后再清理；
- 增加回切、后续配置合并和排他资源表达。

## 2. 分阶段实施

### 阶段 A：冻结语义和建立黄金路径

先建立一个独立变更任务，固定：

- `Fixed/Configured`、`Direct/Leased` 和五种 Reload Policy 的名称、组合约束；
- typed `Binding/Input` 的 Component ID 唯一性、同计划约束、严格登记顺序和原子安装语义；
- Ready、Health、观察期和回切错误语义；
- 代际上限、Watch 合并和进程关闭优先级；
- 组件定义入口的最小 API；
- `internal/kernel/app` 与 composition 的目标依赖方向；
- 当前 Logger、Database 分别选择的策略及证据。

该阶段应先用设计和状态机测试场景消除歧义，不先批量移动目录。

### 阶段 B：扩展 Kernel 状态机

在保持现有公开行为可验证的基础上，实现：

- 策略分派；
- Ready 门禁；
- 当前代、候选代、上一代的明确所有权；
- 观察计时和 Health 采样；
- 排空新租约后的自动回切；
- 观察成功后的异步旧代清理；
- 最新配置合并队列；
- RestartRequired 和只读诊断结果。

状态机必须用确定性 fake clock、fake instance 和事件日志测试，不依赖真实 sleep。Go race 测试覆盖 Use、Reload、Health、回切和 Stop 并发。

### 阶段 C：迁移组件 API 与目录

采用单轨迁移：

1. 建立新的组件定义契约；
2. 同一任务迁移 Logger、Database、Clock、ID Generator、Validator、composition、测试和文档；
3. 删除旧 `capability` 路径、旧 Definition 接口和失效说明；
4. 搜索旧 import、符号、配置和目录引用归零；
5. 不保留 alias、legacy 目录或并行兼容入口。

目录改名不应早于新契约稳定，否则只是把复杂度从 `capability` 搬到 `app`。

### 阶段 D：用简单能力和真实资源组件分别验收

先用 Clock、ID Generator、Validator 验证 Fixed + Direct + NoReload 的轻量路径，再选择一个真实、边界清晰的新资源能力验证 Configured、生命周期和重载路径：

```text
pkg 封装 -> kernel/app 组件 -> composition typed 绑定 -> Kernel/Host -> 组件边界测试
```

候选应有明确使用场景和资源语义，能够验证生命周期与至少一种 Reload Policy，但不得为了验证框架而引入 HTTP 或业务分层。具体能力必须在后续任务中依据真实底层需求选择。

## 3. Logger 与 Database 的初步策略判断

以下是研究阶段判断，不是最终实现决策：

### Logger

当前 Logger Capability 使用 `logging.Manager` 在提交点替换委托目标，适合继续作为特殊稳定代理。是否归类为 `NativeAtomicReload` 或 `KernelInstanceSwap + Activation`，取决于后续契约是否把 manager 视为底层实现的一部分。无论命名如何，都应保留：

- 配置加载前的基线 Logger；
- 候选失败不改变当前委托；
- 新 Logger 发布后旧 Resource 才进入回收；
- 业务无法取得 Resource.Close。

### Database

当前 Database 使用 `KernelInstanceSwap` 的核心语义：新连接池 Build/Ping，旧租约排空后替换。若增加观察期，必须先验证：

- 两代连接池资源预算；
- 事务和查询派生对象全部被租约覆盖；
- Health 检查不会对数据库造成过载；
- 回切时上一代连接仍保持可用；
- 观察期内旧连接池不被服务端空闲策略提前判死。

证据不足时，应该将部分配置标记为 RestartRequired，而不是承诺完整无感回切。

## 4. 不建议的捷径

- 不用代码生成掩盖尚未稳定的组件契约。
- 不引入 Fx/Dig 来替换资源代际状态机。
- 不让 `internal/kernel/app` 组件通过 `init` 自动注册。
- 不建立万能 `Component`，也不通过大量 nil Hook 模拟可选行为。
- 不因所有能力统一进入 composition，就把 Direct 能力强制包装成 `Access.Use`。
- 不把 typed Binding 扩展成可在运行期任意查询的容器。
- 不为尚未建设的业务层新增 Kernel Handle、Resolver 或 Capabilities 使用规则。
- 不允许策略失败时静默退化成停旧启新。
- 不因担心迁移成本永久保留 `capability` 和 `app` 两套目录。
- 不把观察期旧实例长期保存为隐形备用池。

## 5. 后续变更的测试矩阵

新的实施任务至少覆盖：

| 场景 | 预期结果 |
| --- | --- |
| Fixed + Direct 组件构造 | 注入普通项目接口，不建立配置监听、租约或空生命周期 |
| 零值/跨计划 Binding 或重复能力 | 冻结装配计划失败，不启动任何组件 |
| Decode/Validate 失败 | 当前实例继续服务，不开始 drain |
| Build/Start/Ready 失败 | 恢复旧入口，候选完整清理 |
| 旧租约排空超时 | 不切换，恢复旧入口 |
| 原子切换成功 | 新调用进入新代，旧代进入观察保留 |
| 观察 Health 失败 | 排空新租约，原子回切上一代 |
| 回切时上一代也不可用 | 返回明确致命状态，不伪装恢复成功 |
| 观察期通过 | 确认新代，异步清理上一代 |
| 旧代清理失败 | 新代继续服务，输出 cleanup warning |
| 观察期多次变化 | 只处理最新配置，不产生第三个保留代 |
| Shutdown 与 Reload 并发 | 停止新事务，业务先停，所有已拥有实例被清理 |
| Native Reload 失败 | 底层旧状态继续有效，摘要不提交 |
| RestartRequired 变化 | 当前实例不变，诊断明确要求重启 |
| 排他资源无 Handoff | 注册或重载策略校验拒绝无感换代 |

## 6. 完成判定

只有满足以下条件才能宣布架构收敛完成：

- 新组件作者能从单一指南完成接入，无需理解 Kernel 私有状态机；
- Clock、ID Generator、Validator 的轻量路径，以及 Logger、Database 和至少一个新增资源组件完成真实验证；
- 旧目录、旧 API、旧测试和旧文档已单轨删除；
- 全部重载策略都有确定错误、超时、资源和诊断语义；
- Race、失败注入、连续变化和进程关闭测试通过；
- 文档准确区分直接能力、租约 Access、排他 Handoff 和 RestartRequired；
- 没有为了验收底层装配而提前引入 HTTP 或业务分层。

在此之前，本报告只能作为目标和迁移依据，不能把 `internal/kernel/app` 或观察期回滚写进根 README 的“已实现能力”。
