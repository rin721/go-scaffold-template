# 026 Handler-first HTTP 路由绑定

## 状态

- 当前阶段：已完成。
- 研究门禁：已通过，证据为 `R001`、`R002`。
- 计划状态：已确认并实施完成。
- 当前授权：用户已在计划报告后的后续消息中明确要求“实施方案”，授权 `GOV-001` 至 `VER-001` 的本地实施、验证和聚焦提交；不授权 push、tag、Release、部署或外部写入。
- 外部副作用：本计划不需要启动服务、写数据库、push、tag、Release 或部署。

## 问题

当前 Todo HTTP 链路对单模块可以工作，但模块内 `httpbinding.New` 同时构造 operation Handler、OpenAPI validator、strict middleware、Chi Router 和整份生成 route binding。该结构隐藏了“先构造 Handler、再统一绑定路由”的顺序，并把“只有一个业务 HTTP 模块”写进了对象图：OpenAPI 一旦增加第二个模块的 operation，Todo Handler 会被整份 `api.StrictServerInterface` 扩张影响，复制现有做法还会重复绑定整份路由。

## 计划结论

采用单轨的 Handler-first 链路：

```text
Module UseCases
  -> module-owned operation Handler
  -> application-owned static strict API aggregate
  -> one generated OpenAPI route binding
  -> one application Router
  -> Server / Listener
```

模块只实现自己拥有的 operation，不创建 Router，不加载 OpenAPI，不返回 `net/http.Handler`。应用 composition 显式聚合各模块 Handler；协议 binding 只执行一次规范校验、strict middleware 和生成 route 安装；最外层 Router 只安装全局 middleware 并挂载一个命名清楚的 API routes Handler。

这次计划不新增第二个虚构业务模块，不改变公开 OpenAPI，也不引入动态注册表、运行时 Resolver 或多份手写路由表。

## 实施结果

- Todo HTTP binding 已收敛为只依赖 UseCases、Translator 与 `ActorAccess` 的窄 operation `Handler`，不再创建 Router、加载 OpenAPI 或满足完整应用接口。
- `internal/composition` 通过静态 `strictAPIServer` 聚合模块 Operations，并把 operation policy 与 Todo Actor/Object 授权拆成三个边界清楚的职责。
- `internal/transport/http` 成为 OpenAPI validator、strict middleware 与 generated route binding 的唯一 owner；application Router 只安装全局 middleware 并挂载 `apiRoutes`。
- architecture test 会拒绝模块 HTTP binding 导入 Chi/validator、调用生成 route binding、多处生产 binding 或模块拥有完整接口断言。
- OpenAPI、生成代码、依赖和现有 HTTP 行为保持不变；完整验证证据见 [任务账本](tasks.md)。

## 阅读顺序

1. [R001 当前链路与新增模块摩擦](research/R001-current-handler-route-coupling/report.md)
2. [R002 oapi-codegen 分区能力与方案比较](research/R002-oapi-codegen-route-partitioning/report.md)
3. [需求](requirements.md)
4. [设计](design.md)
5. [任务与确认状态](tasks.md)

本记录是任务级计划，不替代 [应用模块开发指南](../../development/application-module-development.md)、[HTTP API 契约](../../../api/README.md) 或 [模块边界说明](../../../internal/module/README.md)。实施确认后必须同步这些当前权威文档。
