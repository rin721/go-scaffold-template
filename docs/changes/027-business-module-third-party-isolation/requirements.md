# 需求：第三方封装与分轨装配

## 1. 依据

- `R001`：业务模块私有 JWT Adapter 当前没有泄漏 jwx 类型，方向正确；Ops 却把 Prometheus/OTel 具体类型暴露给 module/composition，并把进程级 Observability 错放为模块技术实现。
- 用户确认：业务模块使用的第三方库在模块内实现 Adapter 并封装；非业务第三方库尽量先封装为项目能力，再从底层装配。

## 2. 目标

建立一套可执行的第三方分轨规则，使业务开发者只依赖项目自有契约，同时允许具体技术在正确的实现叶子存在：

1. 业务模块专属第三方留在模块内 Adapter，第三方影子不得越过 Adapter package。
2. 非业务、跨模块或进程级第三方先形成 `pkg` 项目契约，再经 Kernel App 与底层 composition 装配。
3. application composition 只连接项目自有模块/Capability 输出，不长期持有第三方或底层具体 Adapter 类型。
4. 当前 Ops Observability 按第二条迁移；Auth/JWT 按第一条保留并加固。

## 3. 分类判据

| 问题 | 是 | 否 |
| --- | --- | --- |
| 是否只服务一个业务模块的真实语义，且没有跨模块稳定价值 | 模块内 Adapter 候选 | 继续下一问 |
| 是否被多个模块/入口消费，或由进程统一选择配置与资源 | 底层 Capability 候选 | 保持局部普通实现 |
| 是否拥有连接、Client、goroutine、registry、exporter 或终结动作 | 必须明确生命周期 owner | 可使用普通构造 |
| 当前项目自有契约能否完整表达所需保证 | 形成实施计划 | 返回研究，不泄漏第三方类型绕过 |

拥有第三方 SDK 或 goroutine 本身不会自动升级为底层 Capability；“业务专属”与“进程复用/选择”共同决定轨道。

## 4. 范围

### 包含

- 冻结业务模块 Adapter 的第三方封装规则；
- 建立非业务第三方 `pkg -> kernel/app -> kernel/composition` 规则；
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
8. 架构门禁能拒绝第三方 selector 泄漏、模块根暴露具体 Adapter、非业务实现绕过底层 Capability 和上层持有具体技术类型。
9. 旧 Ops 技术实现路径、旧导出字段和兼容 wrapper 零残留；文档只保留一套当前规则。
10. scope 对应的 Go test/race/vet/build/tidy、生成 clean diff、Markdown 链接和 `git diff --check` 通过。

## 6. 确认要求

本轮是纯文档设计修订。源码、测试、依赖声明、生成物和运行状态均未授权修改。只有用户在本计划报告之后的后续消息明确确认 027 当前方案，才能实施 `GOV-001..VER-001`。
