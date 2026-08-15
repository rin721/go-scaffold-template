# 需求：第三方封装与分轨装配

## 1. 依据

- `R001`：业务模块私有 JWT Adapter 当前没有泄漏 jwx 类型，方向正确；Ops 却把 Prometheus/OTel 具体类型暴露给 module/composition，并把进程级 Observability 错放为模块技术实现。
- 用户确认：新增业务能力先在 `internal/module/<name>` 完整收口；业务专属第三方在该模块内实现 Adapter 并封装；只有同时证明跨业务复用且由进程统一选择的底层资源，才进入完整 Kernel Capability 装配链。

## 2. 目标

建立一套可执行的第三方分轨规则，使业务开发者只依赖项目自有契约，同时允许具体技术在正确的实现叶子存在：

1. 新增业务能力的 model、repo、service、handler、Adapter、binding、配置、migration/运行单元和 contribution 按真实职责先收口到 `internal/module/<name>`；不为不存在的职责制造空目录。
2. 只服务该模块的第三方进入 `internal/module/<name>/adapter/<technology>`，第三方影子不得越过 Adapter package。
3. 只有经能力评估同时证明“跨业务复用”和“由进程统一选择”的底层资源，才进入 `pkg/<name> -> internal/kernel/app/<name> -> internal/kernel/composition`。
4. 拥有第三方 SDK、Client、cache、goroutine、配置或生命周期本身不构成 Kernel Capability 升级理由。
5. application composition 只连接项目自有模块/Capability 输出，不长期持有第三方或底层具体 Adapter 类型。
6. 当前 Ops Observability 经过真实消费者与进程选择证据满足双条件；Auth/JWT 只服务 Auth 模块，保留模块内 Adapter 并加固。

## 3. 分类判据

| 跨业务复用 | 进程统一选择 | 归属 |
| --- | --- | --- |
| 否 | 否或是 | 继续收口在 `internal/module/<name>`；需要生命周期时通过模块 contribution/Participant 管理 |
| 是 | 否 | 可以评估普通 `pkg` 复用库，但不得虚构 Kernel App 组件 |
| 是 | 是 | 才能进入完整 `pkg -> internal/kernel/app -> internal/kernel/composition` 底层链 |

能力评估必须列出真实消费者、选择位置、配置 owner、资源 owner 和生命周期证据。只有未来可能复用、技术看起来通用或需要 goroutine 都不算证明。

## 4. 范围

### 包含

- 冻结业务模块 Adapter 的第三方封装规则；
- 建立“跨业务复用且由进程统一选择”双条件的 `pkg -> kernel/app -> kernel/composition` 规则；
- 设计 Observability 项目契约和底层装配边界；
- 收口 Ops 对 Prometheus/OTel 具体类型的导出；
- 保持 Auth/JWT 模块内 Adapter 并加固泄漏门禁；
- 更新 package graph、AST 导出检查、正反 fixture、测试和当前权威文档的后续任务。

### 不包含

- 替换或升级 jwx、Prometheus、OpenTelemetry、OTLP 技术；
- 修改公开 HTTP API、metrics 格式、trace propagation、配置键或默认值；
- 修改 Auth policy、Todo 用例、数据库 schema 或 migration 行为；
- 新增动态 DI、运行时 Resolver、Service Locator 或全局可变 registry；
- push、tag、Release、部署或外部系统写入。

## 5. 验收标准

1. Auth/JWT 第三方类型只出现在 Auth Adapter 实现/测试，不进入 Model、Service、binding、模块导出签名或 composition。
2. `internal/module/ops/**` 的生产代码不再直接导入 Prometheus、OTel、OTLP 或相关具体 Adapter。
3. Ops `Dependencies`、`Module`、middleware 和 diagnostics 只使用标准库或项目自有契约。
4. application composition 不导入 Prometheus/OTel 第三方包，也不把具体 registry/provider 类型作为长期字段。
5. Observability 的项目契约不复制第三方 API，不导出第三方类型、Option、Config、错误或 Close 权。
6. Observability 底层实现明确 process-stable metrics 与 generation-owned telemetry 的构造、Ready、Start/Stop/flush、Reload 和失败语义。
7. 现有 Prometheus exposition、HTTP metrics/trace、OTLP drop/flush、diagnostics、Auth/JWKS 和 generation 行为保持。
8. 架构门禁能拒绝第三方 selector 泄漏、模块根暴露具体 Adapter、未满足双条件的能力越级进入 Kernel App，以及上层持有具体技术类型。
9. 旧 Ops 技术实现路径、旧导出字段和兼容 wrapper 零残留；文档只保留一套当前规则。
10. scope 对应的 Go test/race/vet/build/tidy、生成 clean diff、Markdown 链接和 `git diff --check` 通过。

## 6. 确认要求

本轮是纯文档设计修订。源码、测试、依赖声明、生成物和运行状态均未授权修改。只有用户在本计划报告之后的后续消息明确确认 027 当前方案，才能实施 `GOV-001..VER-001`。
