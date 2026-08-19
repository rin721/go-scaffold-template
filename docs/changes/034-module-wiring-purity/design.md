# 设计：业务模块装配纯度与文档一致性（034）

## 1. 设计结论

把「先通用，再使用」落成可执行状态：让模块的 HTTP 契约接入、policy 汇总、生成器注册收敛到清晰入口，消除 composition 内 Todo 专属涤具与跨模块反向读取，并让权威文档与实际能力一致。保留 031/032/033 已生效的分层与边界。

## 2. 现状问题（R001/R002 依据）

- HTTP 契约接入散落：`http_api.go`（Todo 专属 `contractDispatcher`）、`service.go`（`operationPolicies` 硬编码 `todohttp.ModuleContract()`）、`ops.go`（import `todohttp.ModuleContract()` 推导 observability ops）、`todo.go`（CLI）、`generation.go`（长驻装配），外加 `contract-gen/main.go registeredModules()`。
- 结果：新增第二个 HTTP 模块会触发多处复制；`ops.go`/`service.go` 对 Todo 契约包的反向读取是明确的职责偏移。
- 文档：已随 031/032/033 更新，但「新增模块接入流程」与注册点说明、Ops 边界、门禁-测试关系需收口。

## 3. 目标装配结构（设计）

### 3.1 收敛「模块 HTTP 契约 → 运行期 handler → policy/gate」接入点

新设/整理一个 composition 内统一接收点，例如：

- `internal/composition/http_module.go`：定义窄接口 `HTTPModuleContracts()` / 一个聚合器，接收一组「契约 `contract.Module` + 运行期 handlers」描述。
- `contractDispatcher` 泛化：不再要求 `todoOperations todohandler.Operations`，改为接收「契约 + handlers 映射」或按模块分组；`newContractDispatcher` 改为通用构造。
- policy 汇总：`operationPolicies()` 改为从“已接入的全部 HTTP 模块契约”汇总，而非只 `todohttp.ModuleContract()`；Observability 的 ops operations 归 composition 装配，不反向读 Todo 契约。

### 3.2 权衡：是否合并 contract-gen 注册

- 方案 A（默认，低风险）：保留 `registeredModules()` 作为 build-time 生成器注册点（030 既定允许），但在 `api/README.md` + 模块开发指南写明确「新增 HTTP 模块需追加注册」；不做运行时耦合。
- 方案 B（可选，更大改动）：把「模块契约汇总清单」提到一个 composition 可复用的持有点（如 `internal/composition/modules.go` 的 `httpModuleDescriptors()`），供 contract-gen（`registeredModules` 改从该处）与运行组合并复用，减少两套清单漂移。
- 实施时先做 A 并把 B 作为 design 记录，若用户要求再落地 B。

### 3.3 消除反向读取

- `ops.go`：不再 import `todohttp.ModuleContract()`；Observability operation inventory 由 composition 的装配汇总提供（基于“已接入 HTTP 模块契约”的通用枚举），或由 Ops 模块接收已注入的 operation 集（沿用现有 `dependencies.Operations []pkgobservability.Operation`）。
- `service.go operationPolicies()`：改为从装配汇总的 HTTP 模块契约集合收集 policy，不写死 Todo。

### 3.4 文档一致

同步：模块开发指南（新增模块接入步骤、`registeredModules` 注册点、装配入口）、`internal/module/README.md`、`api/README.md`（生成器注册）、`docs/configuration/README.md`（i18n 聚合一致性）、门禁说明 vs `architecture_test.go`。确保“文档描述能力 == 当前实现”。

## 4. 文件影响（预计划）

- 修改 `internal/composition/http_api.go`（泛化 dispatcher）、`service.go`（policy 汇总）、`ops.go`（去 Todo 依赖）、可能新增 `http_module.go`；
- 修改 `internal/tools/contract-gen/main.go`（若落地 A 仅文档；若 B 则复用汇总）；
- 同步权威文档：模块开发指南、模块 README、api/README、配置说明、变更索引；
- 若加门禁：`architecture_test.go` 增加「composition 不得反向 import 具体模块契约包 (ops/service)」反例（若现有门禁覆盖物理 package，注意 local package 无法通过 import 表达，需审慎或仅文档约束）。

## 5. 失败语义

- 若通用 policy/contract 接口过泛化导致可读性下降，保留现有窄接口，仅在 composition 聚合层抽象。
- 若 contract-gen 汇总复用引入 build/runtime 耦合，退回方案 A（保留独立注册点 + 文档）。

## 6. 验证矩阵

- 定向单元：`operationPolicies` 从汇总契约生成；`contractDispatcher` 通用化；`ops.go` 无 Todo 依赖。
- 集成：Todo/Auth/Ops/Migration tests、composition/generation 集成、management 接受测试通过。
- 门禁：pkg/kernel-app 边界、模块 handler 依赖、contract 分层（031/032/033）继续通过。
- 文档：权威文档与实现一致（无漂移清单）。
- 完整：`gofmt -l .`、`go generate` 幂等、`go test ./...`、`go test -race`（受影响）、`go vet`、`go build ./cmd/app`、`git diff --check`。

## 7. 单轨迁移顺序

1. 先收敛 `contractDispatcher` / policy 汇总（HTTP 契约接入统一入口）。
2. `ops.go` 去 Todo 依赖；`service.go operationPolicies` 改从汇总集合。
3. 处理 `registeredModules()` 说明（A：文档；B：复用汇总）。
4. 同步权威文档与门禁说明。
5. 全量验证后提交（只提交 034 范围；不 push）。

不保留双套装配、旧 Todo 专属涤具或跨模块反向读取。

## 8. 重新确认触发器

- 必须改变公开 HTTP 契约、模块边界、`pkg/*` 契约或 Kernel 生命周期；
- 必须引入动态注册 / Service Locator / 自动扫描；
- 文档-实现仍然无法收口的本质性分歧；
- 用户希望保留 `ops.go` 读取 Todo 契约等现状而不调整。
