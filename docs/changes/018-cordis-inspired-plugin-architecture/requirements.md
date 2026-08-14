# 需求：Cordis 启发的积木式插件架构

> **已废除。** 本需求不再生效，不得据此修改源码、配置、依赖或运行状态。当前方向见 [019 HTTP API 成熟度缺口评估](../019-http-api-maturity-gap-assessment/README.md)。下文仅保留历史证据。

## 1. 产品目标

让使用者能从当前二进制已编译的插件 Catalog 中选择 Profile/Bundle，确定地组装后端服务、Bootstrap CLI 或 Application CLI；每个插件显式声明身份、配置、能力依赖、能力输出、生命周期和可逆贡献，Runtime 在任何副作用前完成图验证，并提供可解释的装配与诊断。

本目标由 [R001](research/R001-current-composition-baseline/report.md) 的当前事实和 [R002](research/R002-deepseek-harness-cordis/report.md) 的外部机制支撑。

## 2. 需求

### 2.1 架构与边界

- `ARCH-001`：保留一个不可约微内核，只负责 Catalog、Profile 编译、依赖图、插件实例生命周期、Effect 回收、配置协调和诊断；它不包含业务能力。
- `ARCH-002`：`pkg/*` 继续是不感知插件 Runtime 的通用库；第三方实现继续通过项目 Adapter 封装。
- `ARCH-003`：“Everything is a Plugin”只覆盖可独立选择、配置、贡献、启停或替换的产品能力，不把每个 Service、Repository、DTO 或纯函数都变成插件。
- `ARCH-004`：业务核心只接收构造参数中的窄接口，不导入 Runtime、Context、Catalog 或完整 Capabilities。

### 2.2 插件定义与能力契约

- `PLG-001`：每个插件 Definition 必须有 owner-qualified ID、版本、显示名、配置契约、提供能力、必需/可选依赖和实例化入口。
- `PLG-002`：插件实例 ID 与插件类型 ID 分离；同一 Definition 可以按不同实例配置出现，是否允许多实例由 Manifest 明确。
- `PLG-003`：Capability 使用 owner 定义的 typed token，并带 namespace 与 major contract version；禁止只用字符串或 `reflect.Type` 建立业务依赖。
- `PLG-004`：内部异构节点允许在 compiler/runtime 边界私有 type erasure，但普通插件和业务代码不得获得 `any` Store、按名 Get 或运行期 Resolver。
- `PLG-005`：能力 cardinality 必须显式区分 exclusive、contribution set 与 stable broker，不能依靠后注册覆盖。

### 2.3 Catalog、Bundle 与 Profile

- `CMP-001`：Catalog 只接收显式代码注册的 compiled-in Definition，不扫描 package、不依赖 `init`、不从文件路径加载 Go 代码。
- `CMP-002`：Bundle 是有序的 Profile row 集合；Profile 是命名装配；row 是稳定插件实例。覆盖顺序只影响最终声明，不替代依赖顺序。
- `CMP-003`：Graph Compiler 在副作用前拒绝未知插件、重复实例、重复 exclusive provider、缺失依赖、不兼容版本、依赖环、非法 scope、配置冲突和确定性冲突。
- `CMP-004`：同一 Catalog 与 Profile 必须产生确定的 Frozen Plan；启动/停止顺序来自依赖拓扑和显式 phase，而不是 map 或文件偶然顺序。
- `CMP-005`：提供 `validate-profile`、`dump-plan` 和 `explain` 级别的只读诊断能力，输出必须可脱敏、可测试且不启动资源。

### 2.4 生命周期与 Effect

- `LIFE-001`：每个插件实例拥有状态、配置快照、依赖快照、Effect ledger、诊断与子 contribution。
- `LIFE-002`：插件至少区分 Pending、Preparing、Active、Draining、Inactive、Failed；状态变更有唯一 owner。
- `LIFE-003`：插件产生的 Route、Command、Health checker、Participant/Runner、监听器与能力发布必须通过 owner-aware registration 建立，并返回幂等 cleanup；Runtime 按 LIFO 回收并保留全部清理错误。
- `LIFE-004`：provider 停止提供后，依赖方必须先退出或排空，provider 才能回收资源；循环等待在编译阶段拒绝。
- `LIFE-005`：外部持久写入、消息发送和其他 emission 不得伪装成可逆 Effect；需要 outbox、幂等或补偿时由业务契约明确。
- `LIFE-006`：第一阶段只保证启动期 Profile；装配结构变化一律 RestartRequired。现有配置化资源换代继续由 Kernel 事务治理。

### 2.5 迁移与单轨

- `MIG-001`：复用当前 Definition/Plan/Kernel/Coordinator/Host 的已验证语义，不并行建立第二个长期容器或生命周期框架。
- `MIG-002`：迁移顺序为底层能力、应用 contribution broker、Todo、HTTP/CLI surface、入口；每阶段有兼容窗口只限同一未发布实现分支，最终删除旧 Compose/Contribution 路径。
- `MIG-003`：目标实现必须保留 Bootstrap 无资源副作用、Service/Application CLI 模式差异、稳定 Lease/facade、配置单候选和反向停止。
- `MIG-004`：012 的固定手工装配决策只有在新 ADR 获确认并且替代门禁通过后才失效；扫描、Service Locator、全局可变 Registry 与 Go `plugin` 仍保持禁止。

## 3. 非目标

- 本轮不加载未编译进二进制的 Go 代码。
- 本轮不提供第三方插件市场、签名、下载、升级、权限或沙箱。
- 本轮不实现 package HMR、运行期任意 mount/unmount、跨进程透明调用或多租户隔离 scope。
- 不建立通用事件总线、万能 Context、巨型 Capabilities 或按字符串查找服务。
- 不把数据库业务数据回滚等同于插件 Effect cleanup。
- 不预选 Wasm、gRPC 或 HashiCorp go-plugin 作为外部插件协议。

## 4. 验收标准

### 4.1 第一阶段架构验收

- 给定 Catalog/Profile，可以在不创建 Kernel、listener、连接或 goroutine 的情况下验证并输出完整 Plan。
- 缺失依赖、重复 provider、版本不兼容和环路都有稳定错误类型、完整原因链和确定诊断。
- 插件 Build 只获得声明过的 typed dependencies；测试证明不能查询未声明能力。
- 相同输入重复编译得到同序 Plan 和同 digest。
- Bootstrap、Service、Application CLI 三种 Profile 的资源边界与当前行为一致。

### 4.2 迁移验收

- 当前 Logger、Clock、ID、Validator、Database、Cache、I18n、Storage、Todo、HTTP 与 CLI 均由 Profile 选择，入口不再逐项硬编码装配。
- Route、Command、Participant/Runner、Config Binding、Health contribution 均有实例 owner、冲突校验和 cleanup 证据。
- 旧 Compose/Contribution 装配路径、旧配置键、旧测试和失效文档没有残留。
- 当前 Todo HTTP/CLI 验收、配置重载、错误链、race、vet 和架构门禁保持通过。

### 4.3 后续 live reconciliation 门禁

只有出现明确不停机增删或 provider 切换用例，并证明 broker、依赖排空、外部副作用和失败恢复语义后，才建立独立任务扩展：

- candidate graph 与 current graph 的稳定 diff；
- provider 退出先于 dependents cleanup 的反向拓扑；
- candidate 失败保持 current graph；
- commit 后 cleanup 失败进入 degraded，不假装自动回滚外部 emission；
- 动态结果与从最终 Profile 冷启动结果等价。
