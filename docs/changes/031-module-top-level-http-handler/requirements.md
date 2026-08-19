# 需求：模块顶层 HTTP Handler 分责（031）

## 1. 依据

- `R001`：Todo 的 `binding/http` 当前同时承载 HTTP handler 实现、DTO/映射、代码优先契约声明与运行期装箱四类职责，handler 与绑定混在同一包，职责边界不清晰。
- `R002`：目标分层为「模块顶层 handler 层 + binding 只做绑定」，依赖方向保持单向，迁移按单轨执行。

## 2. 目标

把 Todo 模块的 HTTP handler 从 `internal/module/todo/binding/http` 迁移到模块顶层 `internal/module/todo/handler`，使：

```text
模块顶层 handler（HTTP 应用语义适配）
  -> binding/http（只做代码优先契约 + 运行期装箱）
  -> module.go / composition（装配两者）
```

每层只负责自己相应的职责，分工分明，便于代码阅读。

## 3. 术语

- **模块顶层 handler 层**：`internal/module/todo/handler`，负责 HTTP 语义到 UseCases 的适配（Operations/Handler/ActorAccess/DTO 映射/错误呈现）。
- **binding 层**：`internal/module/todo/binding/http`，只负责代码优先契约声明（`ModuleContract`）与运行期把 typed handler 装箱为 `contract.Handler`（`RuntimeHandlers`）。
- **装配层**：`module.go` 与 `internal/composition`，唯一同时连接 handler 与 binding 的位置。

## 4. 功能要求

### `REQ-001` handler 上移到模块顶层

HTTP handler 实现（`Operations` 接口、`Handler`、`NewHandler`、`ActorAccess`、DTO 与映射、`present`/`errorContract` 错误呈现）必须位于模块顶层 `internal/module/todo/handler`，不得再放在 `binding/http`。

### `REQ-002` binding 只做绑定

`internal/module/todo/binding/http` 只保留代码优先契约（`contract.go`、`contract_module.go` 的 `ModuleContract`）与运行期装箱（`handlers.go` 的 `RuntimeHandlers`），不得包含任何业务 handler 实现、DTO 映射或错误呈现。

### `REQ-003` 单向依赖

- `handler` 不得 import `binding/**`、`internal/transport/**`，不得导入 chi/kin-openapi/nethttp-middleware 等 HTTP 框架，不创建 Router、不加载 OpenAPI。
- `binding/http` 可 import `handler`（仅取 `Operations` 等类型），不得反向新增业务逻辑。
- `module.go` 与 `internal/composition` 是唯一同时连接 handler 与 binding 的位置。

### `REQ-004` 行为与契约不变

迁移不改公开 HTTP 行为、DTO 形态、operationId、policy、security、路由或 `api/openapi.yaml` 语义；`oasdiff` 无 ERR；现有 Todo/Auth/Ops/composition/transport/process tests 全部保持通过。

### `REQ-005` 单轨迁移

迁移完成后不保留旧的 `binding/http` handler 实现、兼容 alias、双 handler 或临时 wrapper；删除旧文件与失效文档入口。

### `REQ-006` 门禁与文档同步

架构门禁新增/修改：模块顶层 handler 位置与依赖方向、binding 只依赖 handler + contract、禁止 handler import binding/transport 与第三方 HTTP 框架。同步 `api/README.md`、`internal/module/README.md`、Todo README 与模块开发指南，使权威文档只描述单轨现行分层。

## 5. 质量要求

| 标准 | 可验收定义 |
| --- | --- |
| 简单 | handler、binding、装配各只有一处明确 owner，从目录名即可区分职责 |
| 明确 | `Operations`/`Handler`/`ActorAccess`/DTO 在 handler 包；`ModuleContract`/`RuntimeHandlers` 在 binding/http |
| 可读 | 读 module.go 或 composition 能看出 handler -> binding -> transport 的顺序 |
| 可验证 | 结构门禁、协议测试、进程测试、完整 gate 与 oasdiff 同时通过 |

## 6. 范围

### 包含

- 新建模块顶层 `handler` 包并迁移 handler/DTO/测试；
- `binding/http` 收敛为契约 + 装箱；
- 更新 `module.go`、composition、transport 测试、contract-gen 导入；
- 架构门禁、权威文档与变更文档同步。

### 不包含

- 改变公开 HTTP 行为、版本、路径或契约语义；
- 新增第二个真实业务模块或假业务示例；
- 动态 route registry、扫描、插件或 Service Locator；
- 修改 Kernel、Host、listener、配置 schema、migration、Database 或 module Contribution；
- 新增/升级第三方依赖；
- push、tag、Release、部署或数据库操作。

## 7. 验收标准

1. `internal/module/todo/handler` 存在且包含 `Operations`/`Handler`/`NewHandler`/`ActorAccess`/DTO 与错误呈现；`internal/module/todo/binding/http` 不再包含业务 handler 实现。
2. `handler` 不 import `binding/**`、`internal/transport/**` 或 chi/kin-openapi/nethttp-middleware；架构门禁正反 fixture 通过。
3. `binding/http` 只保留 `ModuleContract`/`RuntimeHandlers` 与契约 schema，不承载业务逻辑。
4. 所有消费方（module.go、composition、transport/routes_test、contract-gen）导入正确，无未初始化引用。
5. 公开 HTTP 行为与 `api/openapi.yaml` 语义不变（oasdiff 无 ERR）。
6. Todo/Auth/I18n/Ops/composition/transport/process 等现有测试全部通过。
7. `go build ./...`、`go vet ./...`、`gofmt -l .`、`go test ./... -count=1`、`go test -race`（受影响）、`go mod tidy -diff`、`go generate` 幂等与 `git diff --check` 通过。
8. `api/README.md`、`internal/module/README.md`、Todo README 与模块开发指南同步为「handler 顶层 + binding 只做绑定」单一现行说明。

## 8. 确认要求

这是非纯文档实施计划：将修改源码、测试与文档目录。只有用户在本计划报告之后的后续消息明确确认 031 当前方案，才能开始实施。若实施中发现必须改变公开契约、依赖、Kernel/Host、module Contribution 或路由生成策略，必须退回研究并重新确认。
