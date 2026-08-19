# HTTP API 契约

`api/openapi.yaml` 是公开 HTTP operation、路径、请求/响应 schema、security 与兼容性的产物，由项目自有生成器 **从各模块的代码优先（code-first）契约声明生成**。Go 代码是唯一权威；不再由 openapi.yaml 生成 Go 代码，也不再维护第二份手写路由/DTO 清单。

## 权威与生成

1. 每个业务模块分层声明自己的 HTTP 契约与 handler（031 分责）：模块顶层 `internal/module/<name>/handler` 实现窄 `Operations`/`Handler`、DTO 映射、错误呈现与 `ActorAccess`；`internal/module/<name>/binding/http` 以 `pkg/httpx/contract` 的 typed 类型声明 operation（method/path/operationId/policy/security 与 DTO schema），例如 `todo/binding/http/contract_module.go` 的 `ModuleContract()`，并提供 `RuntimeHandlers` 装箱。新增 HTTP 业务模块**除了在 `internal/composition` 装配**，还必须把其契约注册到 `internal/tools/contract-gen/main.go` 的 `registeredModules()`（build-time 生成器注册点、独立于运行图），否则 `go generate` 不会渲染该模块的 `api/openapi.yaml` 与 operation inventory。
2. 在仓库根目录执行：

   ```powershell
   go generate ./...
   ```

   这会运行 `internal/tools/contract-gen`，从所有已注册的模块契约渲染：

   - `api/openapi.yaml` —— 公开契约产物；
   - `internal/transport/http/api/operation_inventory.gen.go` —— operation identity 与 policy inventory。

3. 审阅生成 diff；`go generate` 后必须 clean diff（`git diff --exit-code -- api internal/transport/http/api`）。
4. 生成器输出必须与既有契约语义兼容：CI 用 `oasdiff breaking` 对照上一个已提交 `api/openapi.yaml` 基线，新增公共破坏必须先采用版本/弃用策略并记录决策，不能简单更新一份副本绕过。

## 运行期绑定

`internal/transport/http` 是唯一 route binding owner：它从聚合后的模块契约构建 OpenAPI 校验规范、一次绑定路由、执行 operation gate 与问题呈现。新增业务模块只扩展自身契约声明、runtime handlers 与 `internal/composition` 的聚合，不复制 Router、validator 或 method/path。

- 模块顶层 handler 使用模块自有 DTO（`internal/module/<name>/handler/dto.go`），不依赖全局生成包、不 import `binding/**` 或 `internal/transport/**`。
- 底层第三方库（kin-openapi、yaml、jsonschema）只存在于 `pkg/httpx/contract` 内部与 transport/生成器，不泄漏到业务模块。
