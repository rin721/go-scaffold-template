# 设计：Go 化的 Cordis 启发插件内核

> **已废除。** 本设计已被用户终止，不是当前架构方案，也不得作为后续实施授权。当前方向见 [019 HTTP API 成熟度缺口评估](../019-http-api-maturity-gap-assessment/README.md)。下文仅保留历史证据。

## 1. 设计结论

目标架构定义为：

> **显式 compiled-in Catalog + 声明式 Profile + typed capability graph + instance-owned reversible Effects + 现有 Kernel/Host 生命周期执行。**

它学习 Cordis 的时空可组合，不复制 Cordis 的动态 Context：

- 空间维度由 typed capability token、图编译、拓扑启动/停止和 provider identity 表达；
- 时间维度由每个插件实例的 Effect ledger、幂等 cleanup、反向排空和候选事务表达；
- Go 业务代码继续使用普通构造注入，不查询容器；
- 第一版插件集合在编译期固定，Profile 只选择已编译 Definition。

## 2. 不可约微内核

“一切皆插件”不能包括解释插件的 Runtime 本身。微内核只包含：

```text
Catalog -> Profile Compiler -> Capability Graph -> Frozen Plan
                                      |
                                      v
                         Instance Lifecycle + Effect Ledger
                                      |
                                      v
                         Kernel / Coordinator / Host execution
```

微内核不拥有 Todo、HTTP 业务路由、数据库实现、CLI 命令或其他产品功能。它只解释声明、管理状态、回收 Effect 和输出诊断。

## 3. 插件的正确粒度

一个对象只有同时满足以下一项以上才值得成为插件：

- 可由 Profile 选择启用/禁用；
- 有独立 typed 配置；
- 提供或消费跨包 Capability；
- 产生 Route、Command、Runner、Health、监听器等注册副作用；
- 拥有需要 Start/Ready/Drain/Stop 的资源；
- 有真实替换、隔离或多实例需求。

因此：

- Database provider、HTTP surface、Todo feature module 是插件候选；
- Todo Service、Repository 实现、Handler 是 Todo 插件内部普通对象；
- `pkg/clock`、`pkg/httpx` 等库不是插件，封装它们的 Definition 才是插件；
- Bundle/Profile 是组合数据，不是插件；
- integration binding 可以是小插件，用于连接两个独立 Capability 并消除反向依赖。

## 4. 核心契约

下面是目标语义示意，不是已实现 API，实施时必须以 Go 编译器验证后的最小类型为准。

### 4.1 Identity 与 Manifest

```go
type PluginID string      // Definition 身份，例如 project.database.gorm
type InstanceID string    // Profile row 身份，例如 primary-database

type Manifest struct {
    ID          PluginID
    Version     Version
    Multi       bool
    Provides    []CapabilityRef
    Requires    []RequirementRef
    ConfigPaths []string
}
```

`PluginID` 必须 owner-qualified；`InstanceID` 是 reconciliation key。`Version` 首版只治理插件契约 major 和实现版本诊断，不承诺独立二进制兼容。

### 4.2 Typed Capability Token

```go
type Key[T any] struct { /* owner 私有身份 + CapabilityRef */ }
```

Token 由能力契约 owner 定义并导出，consumer 只能引用该 token。Compiler 内部可以用 token 的不可伪造身份索引异构节点，但 type erasure 不越过内部边界。Manifest 中的 `CapabilityRef` 用于配置、版本、图和诊断；真正向 Builder 传值仍由泛型闭包保证。

不采用以下 API：

```go
ctx.Get("database")
ctx.Resolve(reflect.TypeFor[Database]())
map[string]any{"database": db}
```

### 4.3 Definition 与 typed dependencies

目标 Definition 延伸当前 `app.Definition[O]`，把“前向 Binding”提升为“对 Capability Key 的声明”，由 Compiler 决定顺序：

```text
Definition[O]
  Manifest
  Config Binding/Decoder
  Requires[D]       -- 只把声明过的 Key 解码成 typed D
  Build(ctx, C, D)  -- 返回未发布实例
  Expose/Provide    -- 发布一个 typed Capability 或 broker contribution
  Lifecycle         -- Start/Ready/Drain/Stop
  Apply Effects     -- 注册 Route/Command/Health/Runner 等可撤销贡献
```

插件 Builder 直接收到 `D`，没有 Context lookup。当前 `DependencySet/Values.Resolve` 的“只允许声明输入”约束应复用；Compiler 在启动前把无序 Requirement 绑定成私有 typed closures。

### 4.4 Capability cardinality

| 类型 | 语义 | 例子 | 冲突规则 |
| --- | --- | --- | --- |
| `Exclusive[T]` | 一个 scope 只能有一个 provider | Database、Clock | 多 provider 在编译期失败 |
| `Contribute[T]` | 多实例向 owner-owned 集合贡献元素 | Route、Command、Health check | owner 规范化后检查重复 |
| `Broker[T]` | 稳定入口管理多个后端 | 动态路由表、provider pool | broker 唯一，成员可增删 |

不能用“最后注册覆盖”模拟替换。替换必须由 Profile 明确选择 exclusive provider，或由 broker 明确调度成员。

## 5. Catalog、Bundle 与 Profile

### 5.1 Catalog

Catalog 是当前二进制可用 Definition 的显式清单。推荐由唯一 composition 包调用普通函数加入：

```text
Catalog
  + builtin logger/database/cache/... definitions
  + http service surface
  + todo feature
```

后续如果多发行版重复清单成本出现，可以生成 `catalog_gen.go`，但生成结果仍是显式静态导入；禁止 `init`、文件系统扫描或运行期 Go package discovery。

### 5.2 Bundle

Bundle 只返回有序 Profile rows 和默认选择，不执行插件代码。典型 bundle：

- `foundation`：Logger、Clock、ID、Validation、Database、Cache、I18n、Storage；
- `todo`：Todo feature、migration contribution；
- `service-surface`：HTTP Router broker、HTTP Server、process health；
- `bootstrap-surface`：config/default CLI；
- `application-cli-surface`：Todo one-shot commands。

这里的名称只表示目标语义，最终包名与公开类型在实施任务中定稿。

### 5.3 Profile

Profile 从 Bundle 和显式 row 形成一个不可变期望装配：

```yaml
profile: service
bundles: [foundation, todo, service-surface]
plugins:
  - id: primary-database
    use: project.database.gorm
    configPath: database
  - id: todo
    use: project.module.todo
    configPath: todo
```

示例只描述目标数据模型，不确认新增配置文件或具体 key。第一阶段优先把 shipped Profile 定义为代码内不可变数据，继续复用同一 `config.Snapshot` 作为插件 typed config 来源，避免 Bootstrap 产生“先读取 Profile 才知道如何读取 Profile”的循环。外部 Profile 文件是后续独立能力。

覆盖规则必须简单且可解释：按 `InstanceID` 替换整行，最终声明生成 digest；row 顺序只用于人类展示和同级稳定 tie-break，不参与依赖满足。

## 6. Graph Compiler

Compiler 是纯阶段，不创建资源。推荐流水线：

```text
Catalog + Profile + config schema inventory
  -> resolve rows to Definitions
  -> validate identity/version/config ownership
  -> bind typed Requirements to providers
  -> validate cardinality and contribution conflicts
  -> detect cycles and illegal phase edges
  -> stable topological sort
  -> emit Frozen Plan + digest + explanation graph
```

排序 key 至少包含 lifecycle phase、拓扑层、Profile row order、InstanceID，确保相同输入产生相同结果。错误使用项目自有类型区分 unknown plugin、missing capability、ambiguous provider、cycle、version mismatch 和 invalid config，并保留底层原因链。

Compiler 不自动选择“看起来匹配”的实现。一个 exclusive Capability 存在多个候选时，Profile 必须明确 provider。

## 7. Plugin Instance 与 Effect Ledger

每个 Profile row 编译为一个 Instance；Instance 拥有：

- Manifest 与 InstanceID；
- resolved provider identity snapshot；
- validated config 与 digest；
- lifecycle state 与 generation；
- 已发布 capability；
- 带 label 的 Effect ledger；
- 最后错误、未完成 cleanup 和 degraded 诊断。

Effect API 的语义是“setup 与 cleanup 同地声明”：

```text
Effect(label, setup)
  setup 成功 -> 返回幂等 Cleanup
  后续 setup 失败 -> 反向清理已成功 Effect
  Instance unload -> LIFO 执行全部 Cleanup
  多个 Cleanup 失败 -> errors.Join，保留全部原因
```

需要纳入 Effect 的动作：

- 向 Router broker 注册 Route；
- 向 CLI owner 注册 Command；
- 向 Health registry 注册 checker；
- 向 Supervisor 注册 Participant/Runner；
- 注册配置 owner、observer 或 framework lifecycle listener；
- 向 broker 加入 provider。

不纳入可逆 Effect 的动作：业务数据库写入、已发送消息、已发送邮件、外部 API 调用。它们需要业务幂等、outbox 或补偿，不能在 unload 时盲目“回滚”。

## 8. 生命周期与依赖顺序

### 8.1 第一阶段：启动期 Profile

```text
Declared -> Pending -> Preparing -> Active
                                  -> Failed
Active -> Draining -> Inactive
```

1. Compiler 完成全图验证；
2. 按拓扑正序 Build/Start/Ready/Apply；
3. 任一失败按已激活实例反向回收 Effect 与资源；
4. 所有必要实例和 runner ready 后进程 ready；
5. 终止时先撤销 readiness，按反向拓扑停 dependents，再停 providers。

现有 Kernel 的资源候选事务继续管理同一插件实例内部的配置换代。Profile 结构、provider 选择和依赖边变化全部返回 RestartRequired。

### 8.2 后续：live reconciliation

只有独立任务确认后才扩展：

```text
compile candidate graph
  -> diff by InstanceID + provider identity + config digest
  -> prepare new providers
  -> mark leaving providers unavailable
  -> drain dependents in reverse topology
  -> commit capability switch
  -> reactivate dependents in forward topology
  -> cleanup old effects/resources
```

候选失败保留旧图；commit 后旧代 cleanup 失败进入 degraded，而不是撤销已经对外可见的新代或外部 emission。这部分复用当前 Coordinator 的单候选和 CommittedCleanupError 语义。

## 9. Scope、隔离与策略

Cordis 的 child Context/isolate/intercept 很强，但当前项目只有单进程、单应用 scope。第一阶段只实现 root/application scope，防止用 scope 隐藏本应显式的模块依赖。

未来真实需求出现时再增加：

- tenant/request/job 等有明确生命周期的 child scope；
- 同一 Capability 在不同 scope 的独立 provider；
- capability policy/interceptor 元数据。

权限不能只靠注入声明。任何不可信插件都必须放到进程/Wasm/容器 sandbox，并通过收窄协议访问 Host Capability。

## 10. 事件与扩展点

不复制 Cordis 的通用 Events 到业务层。Go 默认使用：

- 直接 typed interface 调用：有明确单一消费者/提供者；
- contribution/broker：多提供者集合；
- typed framework hook：确有观察、串行处理或 waterfall 语义时，由事件 owner 定义专用类型和分发保证；
- 业务消息：只有真实异步语义出现后独立设计交付、幂等和顺序。

事件不得用来绕过依赖环、隐藏错误或建立无业务语义转发。

## 11. 当前到目标的映射

| 当前实现 | 可复用 | 目标变化 |
| --- | --- | --- |
| `app.Definition[O]` | 无副作用声明、typed output、配置 owner | 增加 Manifest、Capability Key、实例身份和无序 Requirement |
| `Plan/Binding/Input` | 私有 typed binding、Freeze | Catalog/Profile 编译图，不再人工保证 earlier order |
| `RuntimeComponent` | Build/Start/Ready/Drain/Commit/Stop | 归入 Instance lifecycle，增加 Effect ledger 和依赖方退出顺序 |
| `Kernel/Coordinator` | 单候选、资源换代、degraded | 成为插件 Runtime 的资源执行层，不成为第二容器 |
| `Host/Supervisor` | participant/runner owner、反向停止 | 接收 owner-aware contribution，并按 graph phase 排序 |
| `module.Contribution` | Route/Participant 完成品与冲突校验 | 单轨迁移为 Route/Runner broker Effect；删除旧静态聚合路径 |
| `internal/composition` | 唯一 composition root | 只建立 Catalog、选择 Profile 和启动 Runtime，不逐项手工装配 |
| Bootstrap/Service/Application CLI | 正确的资源模式边界 | 变为三个 shipped Profile/surface bundle |

## 12. 迁移阶段

### Phase 0：决策与契约

- 新 ADR 冻结“compiled-in plugin、typed token、无 Resolver、启动期 Profile”的边界；
- 定义 Manifest/ID/Capability/version/cardinality；
- 定义错误与诊断模型；
- 不迁移现有运行路径。

### Phase 1：纯 Graph Compiler

- Catalog、Profile、Bundle、Requirement 和 deterministic graph；
- 只使用无副作用 fake plugins 验证缺失、重复、环、版本和稳定排序；
- 提供 validate/dump/explain 只读入口；
- 现有应用仍走旧路径，直到下一阶段单轨切换。

### Phase 2：底层能力适配与切换

- 将现有 Kernel App Definition 接到新 Definition/Catalog；
- Profile 产生当前同序 Frozen Plan；
- 保持 Kernel/Coordinator/Lease 不变；
- 验证等价后删除旧手工 `Compose` 清单。

### Phase 3：应用 contribution 与 Todo

- 建立 Route、Command、Health、Participant/Runner 的 stable owner/broker；
- Todo feature 作为粗粒度插件，内部对象仍普通构造；
- HTTP/CLI surface 通过 Profile 选择；
- 删除旧 `module.Contribution` 聚合和硬编码 Todo composition。

### Phase 4：Effect 与治理闭环

- 插件实例 Effect ledger、LIFO cleanup、失败聚合、状态/graph/effect 诊断；
- package graph、禁止 Resolver/scan/init、完整旧符号搜索；
- 同步当前权威文档与脚手架示例。

### Phase 5：按真实需求决定是否动态化

先交付并使用至少两个真实插件/Profile。只有出现不停机装配变更验收后，才研究 live reconciliation、scope 或 broker rolling update；第三方插件另立 ADR 与威胁模型。

## 13. 主要风险与控制

| 风险 | 结果 | 控制 |
| --- | --- | --- |
| Plugin 退化为 DI 容器 | 依赖隐藏、边界穿透 | typed token + 构造注入；Context 不提供 Get |
| 每个对象都插件化 | 配置和认知爆炸 | 只插件化可选择/有生命周期/有贡献的粗粒度单元 |
| 双 Runtime 长期并存 | 生命周期和配置撕裂 | 分阶段验证，阶段内适配，最终单轨删除旧路径 |
| 动态装载先于真实需求 | 安全和兼容成本失控 | 第一版 compiled-in；live/external 独立门禁 |
| Effect 被误解为业务回滚 | 外部状态损坏 | 文档与类型区分 Cleanup、Compensation、Emission |
| string key 接口漂移 | 运行期崩溃 | owner token + namespace + major version + compile gate |
| provider 切换中断依赖方 | use-after-close | stable Access/broker 或反向拓扑 drain；无证据则 RestartRequired |

## 14. 文件影响预估

以下只是确认后的预估，不是本轮已授权修改清单：

- 新增插件契约、Catalog、Profile、graph compiler、instance/effect runtime 包；
- 调整 `internal/kernel/app` 与 `internal/kernel/composition`；
- 调整 `internal/composition`、`internal/module`、Todo、HTTP/CLI/health contribution；
- 增加 ADR、当前架构主题文档、开发指南和架构测试；
- 可能调整配置示例与 CLI 只读命令；
- 不修改 `pkg/*` 的通用库定位，除非具体 broker 需要项目自有窄契约且另有证据。

## 15. 验证策略

- Graph compiler table tests、fuzz/property tests：确定性、环、缺失、重复、版本、cardinality；
- Effect tests：部分启动失败、LIFO、幂等、异步 cleanup、`errors.Join`、owner 泄漏；
- Lifecycle tests：正序激活、反序退出、provider/dependent 顺序、取消和超时；
- Profile golden：Bootstrap/Service/Application CLI 的 plan 与资源边界；
- 现有 Kernel reload、Todo HTTP/CLI、Supervisor、config tests 全量回归；
- `go test ./...`、`go test -race ./...`、`go vet ./...`、Linux build、`git diff --check`；
- 搜索拒绝 `init` registry、scan、公开 Resolver、旧 Compose/Contribution 和 Go `plugin`。

## 16. 未决项

以下事项不阻塞确认 Phase 0/1，但在对应实施阶段前必须定稿：

1. 新包最终命名以及是否在现有 `internal/kernel/app` 内演进；
2. `Key[T]` 的不可伪造身份和内部 type erasure 具体实现；
3. contribution set 与 stable broker 的最小公共契约；
4. shipped Profile 是 Go 数据、嵌入资源还是独立文件；第一版推荐 Go 数据；
5. `validate-profile/dump-plan/explain` 进入现有 CLI 的具体命令树；
6. 012 哪些决策由新 ADR 精确替代，哪些继续有效。
