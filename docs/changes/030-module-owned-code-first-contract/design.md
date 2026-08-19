# 设计：模块自有代码优先 HTTP 契约（030）

## 1. 设计结论

采用“模块 typed 契约声明（authority） -> 生成器渲染 openapi.yaml + inventory -> transport 单一 binder -> composition 聚合”的单轨结构：

```text
internal/module/todo/binding/http
  契约声明（contract.Module）+ 窄 Operation Handler

pkg 通用契约能力（例如 pkg/httpx/contract）
  声明类型 + schema 派生 + OpenAPI 渲染（内部使用成熟库）

internal/tools/contract-gen（或改名后的 openapi-inventory）
  从模块声明生成 api/openapi.yaml + operation_inventory.gen.go

internal/transport/http
  一次构建 spec -> 校验 + policy + strict 边界 + 404/405 -> 路由绑定

internal/composition
  聚合模块契约与 Handler 适配器 -> 交给 transport -> 连接 Auth/Ops
```

代码是唯一 authority；`api/openapi.yaml` 与 inventory 是生成产物（纳入 Git）；oapi-codegen 生成链与第三方直连单轨删除。

## 2. 为什么不是现有结构

现有结构以 `api/openapi.yaml` 为唯一 authority，oapi-codegen 生成 DTO/接口/路由；模块直接依赖生成包；transport 直接使用 chi/kin-openapi/nethttp-middleware。这使新模块必须修改全局 YAML 并全局重生成，契约不归模块，第三方暴露（R001 事实）。本设计把 authority 反转为模块自有代码，并让通用能力先行（R003 顺序）。

## 3. 模块自有契约

模块在 `binding/http` 声明自己的 operation 集合。契约类型全部来自 pkg 通用能力（项目自有，不含第三方类型）。精确命名在实施时按 Go 语义微调，职责不可扩张：

```go
// 模块示例（实现细节可在实施中调整，语义以此为准）
var Contract = contract.Module{
    Name: "Todo",
    Operations: []contract.Operation{
        {
            ID:       "createTodo",
            Method:   contract.MethodPost,
            Path:     "/api/v1/todos",
            Security: contract.Bearer(),
            Policy:   contract.Policy{Scope: "todos:write", Action: "todo.create"},
            Request:  contract.DTO[CreateTodoRequest](),
            Responses: contract.Responses{
                201: contract.DTO[Todo](),
                // 错误响应引用通用 Problem 契约
            },
        },
        // listTodos / getTodo / completeTodo 同样声明
    },
}
```

DTO 仍为模块拥有的普通 Go struct（带 json tag），schema 由生成器经成熟库派生；模块不写第二份 YAML。

## 4. 通用契约能力（pkg 层）

新增项目自有契约能力，负责：

1. 声明类型：Operation/OperationID/Security/Policy/Method/Route/DTOSchema 等，只含项目自有类型。
2. schema 派生：把模块 DTO struct 转成 JSON Schema（内部封装 invopop/jsonschema，配置 required/additionalProperties 等以匹配当前契约语义）。
3. 渲染：把模块声明 -> openapi3.T（内部使用 kin-openapi），再序列化为 YAML（yaml.v3）。
4. 运行期 operation 表：输出统一低基数 operation 表（ID/method/path/policy/security），供 transport 绑定、Auth policy、Ops 观测复用，避免第二份事实源。

该能力是“先实现通用再拿来使用”的落点：模块与 transport 都依赖它，第三方实现只存在于其内部。

## 5. 生成器

- 位置：`internal/tools/contract-gen`（可沿用 `openapi-inventory` 的目录与维护路径，改名以表达新职责）。
- 输入：模块契约声明清单（compostion 或独立注册文件显式列出所有模块）。
- 输出：`api/openapi.yaml`、`operation_inventory.gen.go`。
- 校验：生成期校验 operationId 唯一性、policy 完整性、security 一致性（沿用当前 openapi-inventory 的规则）。
- 接入：`go generate` 指令更新为运行该生成器；`Verify-Quality.ps1/sh` 的 clean diff 路径仍对 `api` 与 `internal/transport/http/api` 生效。
- 首次迁移：生成器输出与当前 `api/openapi.yaml` 人工逐项对比（enum/nullable/format/pattern/additionalProperties/security/policy），差异必须经过审阅并由 golden 测试固化。

## 6. transport 单一 binder

`internal/transport/http` 改为从聚合后的模块声明绑定一次：

1. 校验聚合声明非空、operation 表完整；
2. 构建一份 openapi3.T（使用 pkg 契约能力渲染，或加载生成的 YAML；倾向后者保持单一产物事实）；
3. 安装 404/405、单 JSON document 与 OpenAPI 请求校验（kin-openapi 封装仍留在 transport 或能力内部）；
4. 安装 strict 边界、operation metadata（operationId/language/policy）与问题呈现；
5. 用同一张 operation 表绑定路由，返回命名的 `apiRoutes` http.Handler。

它不包含 Todo 业务、模块判断或手写 method/path；公开 method/path 只来自模块契约声明。

## 7. composition 聚合

composition 聚合全部模块的契约声明与 Handler 适配器，生成一张 operation 表交给 transport；连接 Auth 到 operation policy（沿用现有 OperationGate/operationAuthorizer 模式），连接 Ops 到 operation inventory（沿用 `opsOperations` 的读取方式，改从新表读取）。删除完整 `StrictServerInterface` 静态 aggregate；替换为“契约表 + 适配器表”的显式聚合。

编译期完整性替代机制：新模块 operation 未在 composition 注册时，在聚合点显式注册失败（构造期报错）或登记表类型要求每个模块贡献固定字段；实现时选择明确一种并在任务中固化，不能靠运行时静默忽略。

## 8. 文件影响

预计实施范围：

- 新增 pkg 通用契约能力（pkg/httpx/contract 或按语义定名）及测试；
- 新增/改造 `internal/tools/contract-gen` 与 golden 测试；
- 修改 `internal/module/todo/binding/http`（契约声明、Handler 适配器、测试）；
- 修改 `internal/transport/http`（生成指令、单一 binder、测试）；
- 修改 `internal/composition`（聚合、Auth/Ops 连接、generation 构造顺序、测试）；
- 修改 `internal/kernel/composition/architecture_test.go`（门禁规则与 fixture）；
- 修改 `api/oapi-codegen.yaml`（删除）与 `go.mod`（删除 oapi-codegen/nethttp-middleware 工具与依赖，提升 invopop/jsonschema 为 direct）；
- 同步 `api/README.md`、应用模块开发指南、Todo README、`internal/module/README.md`、根 README 相关段落；
- 迁移首轮生成 `api/openapi.yaml` 与 `operation_inventory.gen.go`。

## 9. 失败语义

- 生成器发现 operationId 重复/policy 不完整：生成失败，不产出文件。
- 生成输出与 golden 不一致：CI clean diff 失败，阻止提交。
- 模块 handler 依赖 nil：模块构造失败，候选 generation abort，旧代保留。
- operation 未在 composition 注册：聚合构造失败，不安装路由，不静默返回 501。
- 无 Principal：保持 401；policy 拒绝：保持 403；未知 Auth 错误保留原因链并脱敏。
- request/response/DTO/I18n/业务 fault：保持当前状态码、Problem code 与取消/超时语义。
- 多个错误同时发生：继续遵循现有 generation abort 与 cleanup error join 规则。

## 10. 架构门禁

新增/修改结构检查规则（表达通用规则，不只搜 Todo 文件名）：

1. 模块 HTTP binding 不得导入 chi、kin-openapi、nethttp-middleware、jsonschema 或 `internal/transport/http/api`。
2. `internal/transport/http` 的手写代码不得导入 `internal/module/*`。
3. 完整 StrictServerInterface 断言与 `HandlerWithOptions` 调用在 production 代码中为零。
4. 契约声明只允许出现在模块 `binding/http`；composition 是唯一聚合位置；transport 是唯一绑定位置。
5. 既有 module owner、cmd、Kernel 与 pkg 依赖方向继续有效。

结构测试应包含正反 fixture；禁止用别名、路径白名单或注释约定代替可执行门禁。

## 11. 验证矩阵

- 定向结构：模块契约声明、生成器 golden、composition 聚合、transport 单一绑定。
- 协议行为：现有 Todo contract tests 全部迁移并通过；401/403/对象隐藏/I18n/脱敏不变；invalid JSON/未知字段/Content-Type/404/405 不变；operation ID/policy/日志/trace/metrics/审计不变；`cmd/app` HTTP/CLI process tests 通过。
- 完整门禁：`gofmt -l .`、`go generate ./...`、`git diff --exit-code -- api internal/transport/http/api`、`go mod tidy -diff`、`go test ./...`、`go test -race ./...`、`go vet ./...`、`go build ./cmd/app`、`git diff --check`、oasdiff breaking。

`go generate` 会写工作区，只能在用户确认后的实施阶段执行。验证后审阅完整 diff、staged diff 与 staged file list，只提交 030 范围。

## 12. 单轨迁移顺序

1. 通用能力先行：pkg 契约能力 + 生成器 + golden 对比，证明与当前 openapi.yaml 等价后再动模块。
2. Todo 迁移：声明契约 + Handler 适配器；composition 聚合；transport 改单一 binder。
3. 删除旧链：oapi-codegen 配置/工具/生成物、`nethttp-middleware` 依赖、完整 StrictServerInterface 断言、旧 Handler 形态。
4. 更新门禁与文档，执行全量验证。
5. 审阅完整 diff 后提交（只提交 030 范围；不 push）。

不保留旧构造器 wrapper、兼容 alias、双路由表或 feature flag。

## 13. 重新确认触发器

出现以下任一事实时退回研究和待确认：

- 必须改变公开 OpenAPI、生成器配置/版本或兼容性基线；
- schema 派生无法等价表达当前 nullable/enum/format/pattern/additionalProperties 语义且无法在确认范围内解决；
- 必须按 tag/spec 分包或引入多份完整 route binding；
- 必须修改 Kernel/Host、listener、配置 schema、migration 或 module Contribution；
- 必须新增第三方生成框架或保留任何旧轨并行。