# R001 当前 HTTP 契约方向与模块耦合复核

## 1. 研究问题

当前 HTTP 契约由谁管理、朝哪个方向生成，模块对契约的依赖边界在哪里，第三方库暴露到什么程度，以及既有决策是否锁死了该方向？这决定了 030 修复方案改哪些文件、以什么顺序迁移、以及需要什么授权。

## 2. 方法与范围

- 只使用仓库内代码、生成物、测试、依赖清单与已确认决策作为证据；外部工具能力使用已固定版本和本地模块缓存核实。
- 复核对象：`api/`、`internal/transport/http/`、`internal/composition/`、`internal/module/todo/`、`internal/kernel/composition/architecture_test.go`、020/024 的决策与研究记录。

## 3. 证据（事实）

### 3.1 authority 与生成方向

- `api/openapi.yaml` 是 OpenAPI 3.0.3 规范，包含全部 4 个 Todo operation、路径、schema、security 与 `x-policy`；`api/README.md` 明确写入“`openapi.yaml` 是公开 HTTP operation、路径、请求/响应 schema、security 与兼容性的唯一权威”。
- `api/oapi-codegen.yaml` 固定生成 options：`models + chi-server + strict-server + embedded-spec`，`skip-prune: true`。
- `internal/transport/http/api/generate.go` 声明两条 go:generate：

  ```text
  go:generate go tool oapi-codegen --config ../../../../api/oapi-codegen.yaml ../../../../api/openapi.yaml
  go:generate go run ../../../tools/openapi-inventory -input ../../../../api/openapi.yaml -output operation_inventory.gen.go -package api
  ```

- 生成物：`api.gen.go`（ServerInterface、StrictServerInterface、DTO、HandlerWithOptions、embedded spec + GetSpec/GetSwagger）与 `operation_inventory.gen.go`（OperationID 常量、Operation 表、Operations()、OperationForStrictName）。
- 结论（事实）：**方向是 spec-first——YAML 是权威，代码由 YAML 生成**，与用户要求的“契约文件由代码生成”相反。

### 3.2 模块对契约的依赖

- `internal/module/todo/binding/http/handler.go` 的 `Operations` 接口使用生成类型：`api.ListTodosRequestObject`、`api.ListTodosResponseObject` 等；模块实现 `api.CreateTodo201JSONResponse` 等生成 DTO，并 import `internal/transport/http/api`。
- `internal/composition/http_api.go` 的 `strictAPIServer` 实现**整份**生成接口 `api.StrictServerInterface`，只转发到 `todohttp.Operations`；`var _ api.StrictServerInterface = (*strictAPIServer)(nil)`。
- `internal/composition/service.go` 与 `ops.go` 通过 `httpapi.Operations()` 读取 operation identity 生成 Auth policy 与 Observability operation 列表。
- 结论（事实）：模块的 HTTP 契约事实（method/path、DTO、policy、security）全部来自全局 YAML + 生成包；模块不拥有、不单独管理自己的路由契约；composition 用完整接口聚合掩盖了“契约由谁拥有”的问题。

### 3.3 第三方暴露

- `internal/transport/http/routes.go` 直接 import：`github.com/getkin/kin-openapi/openapi3`、`openapi3filter`、`github.com/go-chi/chi/v5`、`github.com/oapi-codegen/nethttp-middleware`。
- `internal/tools/openapi-inventory/main.go` 直接 import `kin-openapi/openapi3`。
- 生成的 `api.gen.go` 引用了 kin-openapi 与 chi 类型（embedded spec、runtime、ChiServerOptions）。
- 架构测试（`architecture_test.go`）已禁止模块 HTTP binding 导入 chi/kin-openapi/nethttp-middleware，并强制：生成 `HandlerWithOptions` 只允许出现在 `internal/transport/http` 一次、完整 `StrictServerInterface` 断言只允许出现在 `internal/composition` 一次。
- 结论（事实）：对业务模块有封装门禁，但 transport 与生成物本身直接面向第三方；项目自有契约层（类似 pkg/httpx 对 chi 的封装）尚未覆盖 OpenAPI 生成与校验。

### 3.4 既有决策

- ADR-003（`docs/changes/024-production-ready-one-shot-completion/decision.md` 第 3 节）明确选择 **spec-first OpenAPI 3.0.3**：oapi-codegen 生成 strict Chi server 与 route binding，生成代码纳入 Git，go generate + clean diff 门禁；`oasdiff v1.22.x` 做 breaking/changelog gate。
- 024 R002 比较过 Huma code-first / ogen spec-first / oapi-codegen strict Chi / 自研 typed Operation generator，结论：Huma 会把第三方框架契约带入 Handler 且 authority 同时迁移，不选；ogen 会替换 Chi 边界，不选；自研 generator 维护与安全责任不可接受，拒绝；最终选 spec-first oapi-codegen。
- 019/022 把“spec-first 还是 typed code-first”列为 API authority 待决项，并定义 typed code-first 为“项目自有 typed Operation 为 authority，生成 OpenAPI、route catalog、政策矩阵和 tests”；024 在 ADR-003 中实际关闭为 spec-first。
- 结论（事实）：spec-first 是**已实施、已确认、不可反向断言为“从未选择”**的决策；030 反转它必须显式声明取代 ADR-003 的权威选择，并提供用户在本任务中的明确授权记录。

## 4. 推断

1. 当前结构对单个 Todo 模块“能工作”，但它把“只有一个业务 HTTP 模块”写进了对象图：新增模块仍然要修改全局 YAML、重生成，且模块 HTTP 契约无法独立演进、独立测试、独立发布。
2. “先实现通用再拿来使用”的要求意味着：不是先改 Todo 的特例，而是先落地项目自有的通用 HTTP 契约能力（typed 声明 + 生成器 + 运行期 binder），再让 Todo 作为第一个消费者；能力落地前模块仍可以保持现状运行。
3. “不暴露底层第三方库”意味着：openapi.yaml 生成、spec 校验、路由绑定这些第三方能力要收进项目自有封装（类似 pkg/httpx 已对 chi 做的事），模块与 composition 只消费项目自有类型。

## 5. 适用与不适用场景

- 适用：当前单模块 HTTP 契约方向反转、模块自有契约声明、通用契约能力先行、第三方封装。
- 不适用：不改变公开 HTTP 行为；不做动态注册表；不新增第二个虚构业务模块；不扩展 Kernel/Host/listener 语义。

## 6. 局限与剩余未知

- 未验证“in-tree 工具能否完整表达当前 openapi.yaml 的全部语义（enum/nullable/pattern/format/security 组合）”——由 R002 用本地模块缓存核实基础能力，最终等价性仍需生成器原型校验。
- oapi-codegen 生成代码的若干行为（如 strict decode 细节、embedded spec 序列化顺序）一旦移除，运行期行为必须以现有 routes_test.go 与 contract tests 为基线重新证明。
- 本次为只读复核，未运行测试或生成命令。

## 7. 对 030 的影响

- 计划必须以“取代 ADR-003 authority 选择”为前置事实，并写入授权记录。
- 迁移顺序必须是“通用能力先行 -> Todo 单轨迁移 -> 删除旧生成链与第三方直连”，不能只改 Todo 特例。
- 架构门禁、`api/README.md`、应用模块开发指南、Todo README 都要随实施批量同步。