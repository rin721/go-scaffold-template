# 027 第三方封装与分轨装配

## 状态

- 当前阶段：研究与计划已完成，非文档实施待确认。
- 研究门禁：已通过，证据为 `R001`。
- 计划状态：待确认。
- 本轮授权：只修订研究、需求、设计、任务和当前权威文档；不修改源码、测试实现、依赖或生成物。
- 外部副作用：不启动服务、不写数据库，不执行 push、tag、Release 或部署。

## 问题

当前设计只强调“模块专属第三方 Adapter 留在模块”，没有同时冻结另一半规则：非业务、可复用或进程级第三方能力应先形成项目自有封装并在底层装配。结果是 Auth/JWT 与 Ops/OTel/Prometheus 都被放入 `internal/module`，且 Ops 的装配接口直接暴露 `trace.Tracer`、具体 Prometheus Registry 和 OTel Provider。

第三方实现出现在业务模块私有 Adapter 内本身不违规；第三方类型、具体 Adapter 或技术配置越过模块/Capability 契约才违规。非业务第三方能力如果继续在上层临时拼装，也违反底层能力统一装配方向。

## 计划结论

采用两条互斥轨道：

```text
业务模块专属第三方
  internal/module/<name>/adapter/<technology>
    -> 实现模块自己定义的窄 port
    -> 第三方类型不越过 Adapter package

非业务、跨模块或进程级第三方
  pkg/<capability>                         项目自有契约、错误与 facade/Access
    -> internal/kernel/app/<capability>    底层 Definition 与生命周期
    -> internal/kernel/composition         唯一实现选择与 Plan 装配
    -> internal/composition / module       只消费项目自有输出
```

按当前事实，Auth 的 JWT/JWKS 与 audit 是 Auth 业务/应用语义专属 Adapter，可以继续留在模块内，但 `auth.Module` 不得暴露 jwx 类型或具体 Adapter。Prometheus、OpenTelemetry、OTLP exporter 和通用 HTTP observation 属于非业务、进程级 Observability Capability，应从 Ops 模块技术实现中抽离，经过项目自有契约封装并在底层装配；Ops 只保留 management/diagnostics 用例语义并消费稳定输出。

当前源码尚未迁移。实施、门禁和测试属于非文档任务，只有用户在本计划报告后的后续消息明确确认 027 当前方案后才能开始。

## 阅读顺序

1. [R001 当前第三方边界复核](research/R001-current-third-party-shadow/report.md)
2. [需求](requirements.md)
3. [设计](design.md)
4. [任务与确认状态](tasks.md)

本记录是任务级设计，不替代 [应用模块开发指南](../../development/application-module-development.md)、[底层能力库](../../../pkg/README.md) 或 [Kernel App 组件开发](../../../internal/kernel/app/README.md)。
