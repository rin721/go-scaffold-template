# R003 通用 HTTP 契约能力分层与迁移边界

## 1. 研究问题

“模块以自有 typed 契约声明路由 + 由代码生成 openapi.yaml + 运行期一次性绑定”要成为项目自己的能力，需要回答三个问题：

1. 通用能力（声明类型、schema 生成、YAML 渲染）放在哪一层，才能满足“先实现通用再拿来使用”和“不暴露底层第三方库”？
2. 模块、transport、composition 各消费什么，才能让模块只拥有自己的契约、不再依赖全局生成包？
3. 从现状（spec-first 单轨）迁移到目标（code-first 单轨）时，哪些门禁与文档必须同步，顺序是什么？

## 2. 方法与范围

- 只使用仓库内当前代码、目录约定与架构门禁作为证据。
- 目标是确定分层与迁移边界，不编写实现。

## 3. 证据（事实）

### 3.1 现有分层

- `pkg/`：只存放面向应用模块开放的通用底层能力；第三方库必须被项目自有接口/adapter 隔离；`pkg/httpx` 已封装 `net/http + chi`，业务代码不直接依赖 chi 类型。
- `internal/transport/http`：application 协议层，唯一 owner：OpenAPI validator、strict middleware、生成 route binding；当前它直接使用 kin-openapi/nethttp-middleware（见 R001）。
- `internal/composition`：唯一同时知道 Kernel Capability 与应用模块的位置；`strictAPIServer` 是唯一满足完整生成接口的位置。
- `internal/module/<name>/binding/http`：模块拥有的 operation Handler、DTO 映射与错误呈现；被架构门禁禁止导入 chi/kin-openapi/nethttp-middleware。

### 3.2 用户要求的三条主线

- 契约模块化：每个模块单独管理自己的路由契约。
- 方向反转为 code-first：契约文件由代码生成。
- 通用能力先行 + 封装：先实现通用再拿来使用，第三方不泄漏到业务侧。

### 3.3 现有门禁（architecture_test.go）

- 模块 HTTP binding 禁止导入 chi / kin-openapi / nethttp-middleware。
- 生成 `HandlerWithOptions` 只在 transport 出现一次；完整 `StrictServerInterface` 断言只在 composition 出现一次。
- module core（model/service）禁止导入 pkg/httpx 等边界；`cmd/app` 只允许 import composition。
- 这些门禁以 AST 检查和 `go list` 依赖图实现，可扩展为新规则。

## 4. 推断：目标分层

### 4.1 通用契约能力（pkg 级项目自有能力）

新增项目自有契约能力，例如 `pkg/httpx/contract`（或独立 `pkg/contract`，名字按真实语义定）：

- 定义模块可引用、不含第三方类型的声明类型：`Operation`、`OperationID`、`Security`、`Policy`、`Route`、`DTOSchema` 等；DTO 仍由模块以普通 Go struct 定义，schema 由生成器经 invopop/jsonschema 派生。
- 提供从 []模块声明 -> openapi3.T 的渲染函数（内部使用 kin-openapi + yaml.v3），输出 `api/openapi.yaml` 与 `operation_inventory.gen.go`。
- 提供运行期 binder 需要的统一 operation 表（ID/method/path/policy/security），供 transport 与 composition 复用，避免第二份事实源。
- 这是“先实现通用再拿来使用”的落点：模块与 transport 都依赖这套项目自有能力，第三方实现只存在于此能力内部。

### 4.2 模块（internal/module/<name>/binding/http）

- 模块在 `binding/http` 声明自己的契约值（例如 `var Contract = contract.Module{...}`），只引用 `pkg/httpx/contract` 与自己的 DTO 类型。
- 模块继续实现窄 operation Handler（以项目自有 Operation 类型或接口为签名），不创建 Router、不加载 OpenAPI、不调用生成 binding。
- 模块不再 import `internal/transport/http/api` 生成包。

### 4.3 transport（internal/transport/http）

- 只做一次通用绑定：接收聚合后的契约声明表 + 模块 Handler 适配器，构建 openapi3.T，安装 validator、strict 边界、operation policy、404/405 与问题呈现，绑定路由。
- 第三方（chi/kin-openapi/yaml.v3）只存在于 transport 与 pkg 能力内部；不再有 per-module 绑定。

### 4.4 composition

- 聚合各模块契约声明与 Handler 适配器，交给 transport 绑定；连接 Auth 到 operation policy；连接 Ops 到 operation inventory。
- 删除完整 `StrictServerInterface` 静态 aggregate（它是因为 spec-first 生成完整接口才存在的），改为对契约表的显式聚合。

## 5. 迁移边界（单轨顺序）

1. **通用能力先行**：先在 pkg 新建契约能力 + 生成器（`internal/tools/contract-gen` 或并入现有 openapi-inventory 改名），输出与当前 `api/openapi.yaml` 等价的 YAML，跑 golden 对比证明等价。
2. **Todo 单轨迁移**：Todo 改为声明契约 + 窄 Handler 适配器；composition 聚合；transport 改一次通用绑定。
3. **删除旧链**：删除 oapi-codegen 生成链（`api.gen.go`、`oapi-codegen.yaml`、generate.go 中的 oapi-codegen 指令）、`nethttp-middleware` 依赖、完整 StrictServerInterface 断言、per-module 旧 Handler 实现。
4. **门禁与文档同步**：更新 architecture_test 规则（禁止模块导入生成包/第三方；transport 仍是唯一绑定点；composition 仍是唯一聚合点），更新 `api/README.md`、应用模块开发指南、Todo README 与根 README 相关段落。
5. **全量验证**：`go generate` clean diff、test/race/vet/build/tidy、contract tests、oasdiff breaking、架构门禁、进程测试。

## 6. 适用与不适用场景

- 适用：当前单模块到多模块的 HTTP 演进；模块契约独立演进；第三方封装与统一绑定。
- 不适用：不新增第二个虚构业务模块来演示；不引入动态注册表；不改 Kernel/Host/listener 语义；不做运行期 spec 远端加载。

## 7. 局限与剩余未知

- 契约能力放在 `pkg/httpx` 子包还是独立 `pkg/contract` 需按“能力语义归属”最终定名；本报告只确立“pkg 层项目自有能力”这一边界。
- 生成器从 Go struct 到 JSON Schema 的等价性（enum/nullable/format/pattern/additionalProperties）需男原型验证；若出现不可表达语义，退回研究阶段。
- 完整 StrictServerInterface 删除后，composition 聚合的编译期完整性如何保证（缺少新模块 operation 时在哪一位置失败）需在设计阶段定义（预期：契约表聚合处编译期失败或显式注册失败）。

## 8. 对 030 的影响

- 030 的 requirements/design/tasks 必须按本报告的分层与迁移顺序编写；通用能力先行是本任务的核心约束。
- 设计阶段必须回答“删除完整接口后编译期完整性的替代机制”，并把它写进任务清单。
- 研究门禁通过：分层与迁移边界已确定，足以形成计划。