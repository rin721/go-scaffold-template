# R001 当前第三方影子、封装泄漏与装配层级复核

## 1. 研究问题

本报告回答三个问题：新增业务能力哪些职责必须先收口到 `internal/module/<name>`；业务模块使用第三方库时怎样在模块内封装；什么证据足以让资源升级到完整底层 Capability 装配链。

## 2. 用户确认的边界

本任务采用用户本轮明确给出的三条原则：

1. 新增业务能力先按 `internal/module/<name>` 收口 model、repo、service、handler、Adapter、binding、配置/运行职责与 contribution。
2. 只服务一个业务模块的第三方库进入该模块的 `adapter/<technology>`，封装后只服务本模块，不暴露第三方类型。
3. 只有能力评估同时证明资源跨业务复用且由进程统一选择，才进入 `pkg/<name> -> internal/kernel/app/<name> -> internal/kernel/composition`；SDK、Client、cache 或 goroutine 本身不是升级理由。

因此，判断标准不是“仓库中完全不能出现第三方技术名”，而是业务纵向切片是否完整收口、第三方是否处于正确 owner 的实现叶子、是否越过项目自有契约，以及完整底层链是否有双条件证据。

## 3. 方法与范围

1. 从 HEAD `80bb57fd5acbc280208c536e4ab9c6dcda53e15d` 检查 `internal/module/**` 的生产 import。
2. 追踪 Auth、Ops 的模块构造、导出签名、composition 调用方、配置和生命周期 owner。
3. 复核 `pkg`、Kernel App 与 production composition 的现有能力模型。
4. 检查 package graph 和导出类型门禁实际覆盖范围。

本轮不修改源码、测试实现、依赖、配置或生成物，也不重新选择 JWT、metrics 或 tracing 技术。

## 4. 当前事实

### 4.1 业务模块私有 Adapter 不等于违规

`internal/module/auth/adapter/jwt` 直接使用 jwx、httprc 和 singleflight，但它对 Auth Service 实现的是项目自有 `service.CredentialVerifier`：输入是 `model.Credential`，输出是 `model.Principal`，错误转换为 Auth 语义；JWKS cache 只通过项目自有 `supervisor.Participant` 暴露生命周期。jwx 类型没有进入 Auth Service/Model 或 `auth.Module` 输出。

这条路径符合“业务模块专属第三方在模块内 Adapter 封装”的方向。目录中看见 `jwt` 是实现选择可见，不是契约污染。真正需要补强的是可执行门禁，确保未来不会把 `jwk.Key`、`jwt.Token` 或具体 `Verifier` 变成模块调用协议。

### 4.2 Ops 已发生真实泄漏

当前 Ops 不只是私有实现中使用第三方：

- `ops.Dependencies.Metrics` 暴露 `*prometheusadapter.Registry`；
- `ops.Module.Telemetry` 暴露 `*oteladapter.Provider`；
- `middleware.HTTP` 直接接收 `trace.Tracer` 与具体 Prometheus Registry；
- `Provider.Tracer()` 直接返回 OTel `trace.Tracer`；
- `internal/composition` 必须导入 Ops 内部 Prometheus 具体包，并长期持有其具体类型。

这些虽不是仓库外 public API，却是 application composition 与 Ops 之间的 exported 装配协议，已经违反“项目自有契约隔离第三方”的约束。

### 4.3 Ops Observability 不是业务模块专属技术

Prometheus registry、HTTP metrics、OTel provider、OTLP exporter、trace propagation 和通用请求 observation 不只服务 Ops management 用例。当前事实分别证明双条件：

- Auth/Todo 业务 HTTP 与 Ops management/diagnostics 是不同模块消费者，构成跨业务复用；
- Prometheus registry identity 由 `applicationGenerationFactory` 跨 generation 持有；
- OTel provider/exporter 是 generation-owned 资源并参与 Start/Stop/flush；
- 实现、配置、registry/provider identity 与切换由进程 composition 统一选择和治理。

因此，Observability 不是因为拥有 exporter/goroutine 才下沉，而是因为真实满足“跨业务复用 + 进程统一选择”。把具体实现放在 `internal/module/ops/adapter` 会让“management 用例 owner”同时冒充“进程底层观测能力 owner”；它应先形成项目自有 Capability，再从底层注入 Ops/application。

### 4.4 Migration 对照说明

当前 migration 已部分符合分轨：通用 `golang-migrate` 技术封装位于 `pkg/database/migrate`，Todo schema、compatibility 与 completion 留在 Todo 的 `binding/migration`，`internal/module/migration` 只编排显式命令。业务 schema 语义与非业务 engine 没有混成一个第三方 Adapter。

027 不移动 Todo migration，也不重新设计 migration engine；它作为本次分轨规则的正向对照。

### 4.5 当前门禁缺口

`internal/kernel/composition/architecture_test.go` 会阻止 module core 导入 Kernel/HTTP/CLI/Database、跨模块 owner import 和模块 HTTP binding 使用 Chi/OpenAPI validator，但不会检查：

- 业务模块 Adapter 的 exported 声明是否泄漏第三方 selector；
- 模块根 `Dependencies`/`Module` 是否暴露模块内具体 Adapter；
- 未满足双条件的能力是否仅因技术复杂度越级进入 `pkg -> kernel/app -> kernel/composition`；
- `internal/composition` 是否长期持有本应被 Capability 隐藏的第三方/具体 Adapter 类型。

`pkg/boundary_test.go` 已有第三方导出类型检查，但只覆盖 `pkg` 且名单不是通用 import-origin 判断。现有测试通过不能证明本任务边界成立。

## 5. 目标判断

采用两条互斥轨道。

### 5.1 业务模块专属第三方

- 新增业务能力先在 `internal/module/<name>` 收口 Model、repo、Service、Handler、Adapter、binding、配置/迁移/运行单元和 contribution；这些是职责集合，不要求制造空目录。
- 物理位置保留在 `internal/module/<name>/adapter/<technology>`。
- Adapter 依赖并实现模块调用方定义的窄 port；第三方类型、错误、配置对象和关闭权不得越过 Adapter package。
- 模块的 Model、Service、binding、`Dependencies`、`Module` 和 Contribution 只暴露标准库或项目自有类型。
- composition 可以调用模块根构造器，但不导入模块内部 Adapter；具体选择由模块根依据已校验的模块配置完成。

Auth/JWT 保持这条轨道，不迁到顶层通用 Adapter，也不升级为 Kernel Capability。

### 5.2 底层 Capability 双条件门禁

| 跨业务复用 | 进程统一选择 | 结果 |
| --- | --- | --- |
| 否 | 任意 | 继续归属业务模块；资源生命周期通过模块 contribution/Participant 管理 |
| 是 | 否 | 可以评估普通 `pkg` 复用库，但不进入 Kernel App |
| 是 | 是 | 才进入完整 `pkg -> internal/kernel/app -> internal/kernel/composition` 链 |

完整底层链中，`pkg/<capability>` 定义项目自有最小契约、错误、配置语义和 facade/Access；`internal/kernel/app/<capability>` 声明构造、typed config、依赖、Ready/Start/finalizer、Reload 和资源 owner；`internal/kernel/composition` 是唯一实现选择与 Plan 装配位置。上层只消费 project-owned 输出。

Prometheus、OTel、OTLP 和通用 HTTP observation 进入 Observability Capability 轨；Ops 只消费项目自有 metrics handler、HTTP observer 和低敏 diagnostics 契约。

## 6. Observability 目标边界

目标结构按职责表达，不要求为目录对称制造空包：

```text
pkg/observability/
  project-owned HTTP observation、metrics endpoint、diagnostics 契约
  不出现 prometheus、otel、sdktrace、trace.Tracer 等导出类型

internal/kernel/app/observability/
  Prometheus/OTel/OTLP 的具体构造与配置翻译
  process-stable metrics identity 与 generation lifecycle 的明确 owner
  输出稳定 facade/Access，不暴露 Close

internal/kernel/composition/
  把 Observability Definition 加入底层 Plan 并输出 Capability

internal/composition/
  把项目自有 Observability 输出连接到 application Router 与 Ops

internal/module/ops/
  只保留 management、probe、build、diagnostics 用例及协议 binding
  不导入 Prometheus/OTel，不导出具体 Adapter
```

当前 Prometheus registry 跨 generation 稳定，而 OTel exporter 随 generation 构造。实施设计必须保持这个不变量；如果一个 Kernel Definition 无法同时正确表达两种寿命，应拆成稳定 Metrics 与 generation-owned Telemetry 两个窄组件，不能用全局变量、裸实例或先停旧实例绕过。

## 7. 资源、配置与失败语义

- 不新增或升级第三方依赖，不改变 Prometheus exposition、OTLP/HTTP、低基数标签、drop/flush 和诊断行为。
- 创建 registry/provider/exporter 的底层实现负责关闭或终结；消费者没有 Close 权。
- 所有配置继续严格绑定、验证和脱敏；第三方 Config/Option 不进入 `pkg` 或模块契约。
- 候选构造、Ready、提交、撤回、旧代排空和 cleanup debt 语义保持；失败保留原因链但对外诊断只暴露低敏类型。
- 不把 Observability 做成万能事件总线或照抄 OTel API 的 Wrapper；只封装当前 HTTP observation、metrics endpoint 与 diagnostics 实际需要。

## 8. 适用性、局限与未知

本结论适用于当前单体进程和显式 composition。业务模块 Adapter 只要仍专属于该模块就留在模块；被多个业务消费或由进程统一选择中的任一事实只触发重新评估，必须两项同时成立才升级到完整底层链。

本轮没有实施 Observability Capability。稳定 Metrics 与 generation Telemetry 应用一个还是两个 Kernel Definition，需要在实施前按现有 Plan/Generation 生命周期做定向验证；027 设计将其列为必须保持的分裂点，而不是允许实施者自行泄漏具体类型。

## 9. 对当前任务的影响与研究门禁

027 必须修订当前模块开发指南、`pkg` 与 Kernel App 说明，冻结完整业务收口清单和双条件决策表；后续源码迁移要收口 Ops 导出协议、建立底层 Observability Capability、删除旧技术实现路径并补 package graph/AST 门禁。

当前 import、导出签名、进程调用链、资源 owner、正向 migration 对照和门禁缺口均有可复核证据；剩余组件拆分细节不妨碍形成带重新确认触发器的计划，研究门禁通过。
