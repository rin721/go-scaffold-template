# 030 模块自有代码优先契约（module-owned code-first HTTP contract）

## 状态

- 当前阶段：已完成。
- 研究门禁：已通过，证据为 `R001`、`R002`、`R003`。
- 计划状态：已确认并实施完成。
- 当前授权：用户已确认 030 方案，授权本地实施、验证与聚焦提交；不授权 push、tag、Release、部署或外部写入。
- 本轮交付：研究、计划与本次实施的源码、生成物、依赖、门禁、测试与权威文档同步；未 push。
- 外部副作用：无。不启动服务、不写数据库、不 push、不 tag、不 release。

## 问题

当前仓库按 ADR-003（024）采用 **spec-first**：`api/openapi.yaml` 是 operation、schema、security、policy 和兼容性的唯一权威，`oapi-codegen` 从该文件生成 transport DTO、`StrictServerInterface`、Chi route binding 和 embedded spec，`openapi-inventory` 再从同一文件生成 operation inventory。模块的 HTTP binding 直接 import 生成的 `internal/transport/http/api` 包并实现窄 operation 接口，composition 用一个实现整份生成接口的静态 aggregate 收口。

这带来三点与用户要求的冲突：

1. **契约不归模块所有**：所有路由、DTO、policy 都写进全局 `api/openapi.yaml`；新增模块必须修改这个全局文件并重新生成整份代码，模块不能只声明并维护自己的路由契约。
2. **方向相反**：用户要求“契约文件由代码生成”，当前却是“由 openapi.yaml 生成代码”；authority 在 YAML，不在 Go 代码。
3. **第三方直接暴露**：`internal/transport/http/routes.go` 直接使用 chi、kin-openapi（openapi3/openapi3filter）与 nethttp-middleware；模块通过生成的 DTO 类型也间接依赖生成链；底层第三方库没有被项目自有契约吸收。

用户同时要求“先实现通用再拿来使用，先封装再使用，尽量保持不暴露底层第三方库”——即先建立项目自有的通用 HTTP 契约能力（typed contract DSL + 生成器 + 运行期 binder），再让模块只消费这份能力。

## 计划结论

采用单轨 **typed code-first + 模块自有契约**：

```text
internal/module/<name>/binding/http/contract  模块以项目自有 typed 声明描述自己的 operation
  -> pkg/httpx/contract                       项目自有契约 DSL、schema 构建与 bind 适配（内部封装第三方）
  -> internal/tools/contract-gen              从模块契约声明生成 api/openapi.yaml + operation_inventory.gen.go
  -> internal/transport/http                  从同一契约声明绑定路由、校验与错误呈现（第三方只在内部）
  -> internal/composition                     聚合全部模块契约与 handler，连接 Auth 与 Ops
```

代码（模块自有的 typed 声明）是唯一 authority；`api/openapi.yaml` 变成由代码生成的产物（仍纳入版本库并由 oasdiff 守卫兼容性）。oapi-codegen 生成链、`StrictServerInterface`、`HandlerWithOptions`、embedded spec 与 `nethttp-middleware` 单轨删除，不保留双轨。

## 阅读顺序

1. [R001 当前 HTTP 契约方向与模块耦合复核](research/R001-current-contract-direction/report.md)
2. [R002 typed code-first 生成路径与工具可行性](research/R002-code-first-generation-paths/report.md)
3. [R003 通用 HTTP 契约能力分层与迁移边界](research/R003-generic-contract-capability/report.md)
4. [需求](requirements.md)
5. [设计](design.md)
6. [任务与确认状态](tasks.md)

本记录是任务级计划，不替代 [HTTP API 契约](../../../api/README.md)、[应用模块开发指南](../../development/application-module-development.md) 或 [模块边界说明](../../../internal/module/README.md)。确认实施后必须同步这些当前权威文档。