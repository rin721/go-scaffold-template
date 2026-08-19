# 031 模块顶层 HTTP Handler 分责

## 状态

- 当前阶段：已完成。
- 研究门禁：已通过，证据为 `R001`、`R002`。
- 计划状态：已确认并实施完成。
- 当前授权：用户已确认 031 方案，授权本地实施、验证与聚焦提交；不授权 push、tag、Release、部署或外部写入。
- 本轮交付：031 的源码、测试、架构门禁与权威文档同步；未 push。
- 外部副作用：无。不启动服务、不写数据库、不 push、不 tag、不 release。

## 问题

030 之后，Todo 模块的 `internal/module/todo/binding/http` 同时承载了四类职责：

- 运行期 HTTP handler（`Operations` 接口、`Handler` 实现、`ActorAccess` 端口、`present`/`errorContract` 错误呈现）——这是应用层 HTTP 语义适配；
- 模块自有 HTTP DTO 与 `model ↔ DTO` 映射（`dto.go`）；
- 代码优先契约声明（`contract.go`、`contract_module.go`）；
- 把 typed handler 装箱为 `contract.Handler` 的运行期绑定（`handlers.go` `RuntimeHandlers`）。

把 handler 实现放在 `binding/http`，与“binding 只负责协议绑定”职责混在同一目录，破坏了“单一职责、分层清晰、便于阅读”。handlers.go（绑定）还要 import 同包 handler 的类型，职责边界可读性差。

## 计划结论

采用单轨「模块顶层 handler 层 + binding 只做绑定」：

```text
internal/module/todo/
├── model/
├── service/
├── repo/
├── handler/                  # 模块顶层 HTTP handler 层（应用语义适配）
│   ├── dto.go                # HTTP DTO 与 model↔DTO 映射
│   ├── handler.go            # Operations 接口 + Handler 实现 + ActorAccess + 错误呈现
│   └── handler_test.go
├── binding/
│   ├── config/  cli/  migration/
│   └── http/                 # 只做代码优先契约声明与运行期装箱
│       ├── contract.go
│       ├── contract_module.go
│       └── handlers.go       # RuntimeHandlers：把 handler.Operations 装箱为 contract.Handler
└── module.go
```

- `internal/module/todo/handler`：模块顶层 HTTP handler，只负责 HTTP 语义到 UseCases 的适配、DTO 映射、actor 读取与错误呈现；不 import 任何 `binding/**`，不创建 Router、不加载 OpenAPI。
- `internal/module/todo/binding/http`：只负责契约（routes/policies/schemas）与运行期装箱（`RuntimeHandlers`）；依赖 `handler.Operations` 类型，但不再内嵌业务 handler 实现。
- 依赖方向：`handler` 不看 `binding/http`；`binding/http` 看 `handler`；`module.go` 与 `internal/composition` 同时看两者并装配。

## 阅读顺序

1. [R001 当前 binding/http 职责混叠复核](research/R001-current-binding-http-responsibility-mix/report.md)
2. [R002 模块顶层 handler 分层目标与边界](research/R002-module-top-level-handler-layering/report.md)
3. [需求](requirements.md)
4. [设计](design.md)
5. [任务与确认状态](tasks.md)

本记录是任务级计划，不替代 [应用模块开发指南](../../development/application-module-development.md)、[模块边界说明](../../../internal/module/README.md) 或 [HTTP API 契约](../../../api/README.md)。确认实施后必须同步这些当前权威文档。
