# 027 第三方封装与分轨装配

## 状态

- 当前阶段：实施与验证已完成。
- 研究门禁：已通过，证据为 `R001`。
- 计划状态：已确认。
- 本轮授权：实施 `CONTRACT-001..VER-001`，不得扩展到计划外能力。
- 外部副作用：不启动服务、不写数据库，不执行 push、tag、Release 或部署。

## 问题

当前设计只强调“模块专属第三方 Adapter 留在模块”，没有把业务能力完整收口范围和底层升级门禁写成同一条规则。结果是一方面容易只收口 Model/Service，却把 Handler、Adapter、binding 或 contribution 放到模块外；另一方面又可能仅因出现 SDK、Client、cache 或 goroutine，就把单模块能力误升为 Kernel Capability。Ops 的装配接口还直接暴露 `trace.Tracer`、具体 Prometheus Registry 和 OTel Provider。

第三方实现出现在业务模块私有 Adapter 内本身不违规；第三方类型、具体 Adapter 或技术配置越过模块/Capability 契约才违规。只有能力评估同时证明资源跨业务复用且由进程统一选择，才允许进入完整底层装配链。

## 计划结论

采用两条互斥轨道：

```text
新增业务能力
  internal/module/<name>/
    -> 收口 model、repo、service、handler、Adapter、binding 与 contribution
    -> 专属第三方进入 adapter/<technology>，只实现本模块定义的窄 port
    -> 第三方类型不越过 Adapter package

同时满足“跨业务复用 + 进程统一选择”的底层资源
  pkg/<capability>                         项目自有契约、错误与 facade/Access
    -> internal/kernel/app/<capability>    底层 Definition 与生命周期
    -> internal/kernel/composition         唯一实现选择与 Plan 装配
    -> internal/composition / module       只消费项目自有输出
```

业务专属 Adapter 的完整路径是 `internal/module/<name>/adapter/<technology>`；它不是无 owner 的全局 `internal/module/adapter`。拥有第三方 SDK、Client、cache 或 goroutine 本身不会改变轨道。

按当前事实，Auth 的 JWT/JWKS 与 audit 只服务 Auth 模块，可以继续留在模块内，但 `auth.Module` 不得暴露 jwx 类型或具体 Adapter。Prometheus、OpenTelemetry、OTLP exporter 和通用 HTTP observation 同时覆盖 Auth/Todo 业务 HTTP 与 Ops management/diagnostics，并由进程统一选择和治理，满足双条件，才进入 Observability 底层 Capability 计划。

源码已单轨迁移到项目自有 Observability 契约与底层 App；实现与验证证据见 `tasks.md`。

## 阅读顺序

1. [R001 当前第三方边界复核](research/R001-current-third-party-shadow/report.md)
2. [需求](requirements.md)
3. [设计](design.md)
4. [任务与确认状态](tasks.md)

本记录是任务级设计，不替代 [应用模块开发指南](../../development/application-module-development.md)、[底层能力库](../../../pkg/README.md) 或 [Kernel App 组件开发](../../../internal/kernel/app/README.md)。
