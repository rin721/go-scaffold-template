# 需求：业务模块装配纯度与文档一致性（034）

## 1. 依据

- `R001`：接入一个 HTTP 业务模块需跨多个 composition 文件手改（`http_api.go`/`service.go`/`ops.go`/`todo.go`/`generation.go`），其中 `ops.go`/`service.go` 直接 import `todohttp.ModuleContract()`（跨模块反向读取 Todo 契约）；`contract-gen` 的 `registeredModules()` 是 composition 之外的注册点。
- `R002`：文档总体已随 031/032/033 更新，但「新增模块接入流程」与注册点说明、Ops 边界、门禁与 `architecture_test.go` 的关系需进一步核对/收口。

## 2. 目标

让「先通用，再使用」真正落地：业务模块完成后，接入应尽量收敛为「在 `internal/composition` 明确装配 + 依赖注入」这一清晰入口；消除跨模块反向读取契约、Todo 专属涤具、契约知识在各处重复；并让权威文档与实际能力一致。

## 3. 术语

- **装配纯度**：接入一个模块所需的改动应集中于 `internal/composition`（唯一合成根），不应要求在其他无关层多处注册/重复声明。
- **跨模块反向读取**：某模块为自身目的直接读取另一模块的契约包（如 `ops.go` 读 `todohttp.ModuleContract()`）。
- **契约知识收敛**：模块 HTTP 契约的 policy/operation 只应被「契约 owner + 唯一装配点 + transport/生成器」消费，不在多个 composition 文件重复硬编码。

## 4. 功能要求

### `REQ-001` 收敛 HTTP 契约接入点

- 用一个清晰的装配入口接收「模块 HTTP 契约 → 运行期 handler → policy/gate」，替代散布在 `http_api.go`（Todo 专属 `contractDispatcher`）、`service.go`（`operationPolicies` 硬编码）、`ops.go` 的多处 Todo 耦合。
- 新增 HTTP 业务模块时，不应再要求逐个手改多个 Todo 命名涤具；设计应对「模块 N HTTP 接入」提供统一接收方式（或多模块化），并明确其边界。

### `REQ-002` 消除跨模块反向读取 / 职责越界

- `composition/ops.go` 不得为生成 Observability operations 而 `import todohttp.ModuleContract()`；该职责应归属 composition 的装配处（观察对象是“完整 module 集”，而非具体 Todo 契约包）。
- `composition/service.go` 的 `operationPolicies()` 不再直接硬编码 `todohttp.ModuleContract()` 复制 policy；应由装配汇总处统一从各 HTTP 模块契约收集 policy。
- 判定哪些是「合理装配职责」（composition 本就唯一 know Kernel+module），哪些是「偏移」（跨模块取数、契约复制）。

### `REQ-003` 界定 contract-gen 注册职责并文档化

- 明确 `registeredModules()` 是 build-time 生成器的注册点（与运行组合分离，030 已允许），但要在 `api/README.md`/模块开发指南写清楚「新增 HTTP 模块需在此追加注册」，避免文档暗示只改 composition。
- 若倾向收敛，可将「模块契约汇总」提到一个明确位置供 runtime composition 与生成器共用，减少两套清单漂移。

### `REQ-004` 文档与实现一致性收口

- 逐条核对并更新：模块开发指南（新增模块接入/扩展流程、binding 契约清单）、`internal/module/README.md`（模块形态）、`api/README.md`（生成器注册点）、`docs/configuration/README.md`（i18n 聚合）、门禁说明 vs `architecture_test.go`。
- 消除「文档描述了能力但代码没有」与「代码已变但文档停在旧设计」，确保扩展接入说明与实际一致。

### `REQ-005` 保留既有契约红线

- 保留 031（handler/ + binding/http 分层）、032（pkg/kernel-app 配置边界）、033（统一 binding 契约清单 + i18n binding）已生效的设计与门禁；本任务只做装配纯度与文档收口，不做架构回退。

## 5. 质量要求

| 标准 | 可验收定义 |
| --- | --- |
| 装配收敛 | 新增 HTTP 模块接入点明确；不再要求手改多个 Todo 命名涤具 |
| 无跨模块反向读取 | `ops.go`/`service.go` 不再直接 import `todohttp.ModuleContract()` 复制契约 |
| 文档-实现一致 | 权威文档描述 == 当前实际能力（含注册点、扩展流程、门槛） |
| 可验证 | 门禁/测试通过；新增模块路径可用统一入口接入 |

## 6. 范围

### 包含

- 收敛 composition 的 HTTP 契约接入；消除 ops/service 对 Todo 契约包的反向读取；
- 界定并文档化 `registeredModules()` 注册职责（保留或收敛，需 design 决策）；
- 文档同步（开发指南扩展接入、模块 README、api/README 注册点、配置说明、门禁说明）；
- 保留既有的契约分层、i18n binding、pkg/kernel-app 边界与门禁。

### 不包含

- 改变公开 HTTP 契约、业务逻辑、`pkg/*` 通用契约；
- 引入动态注册 / Service Locator / 自动扫描；
- 重构 Kernel 生命周期 / config Scheduler；
- 重写文档体系；
- push、tag、Release、部署或数据库操作。

## 7. 验收标准

1. `ops.go`/`service.go` 不再直接 import `todohttp.ModuleContract()` 复制契约 policy；HTTP policy/operation 由唯一装配点汇总。
2. 新增 HTTP 模块的接入入口明确（一份设计 + 文档说明），不再依赖 Todo 专属涤具。
3. `registeredModules()`（或收敛后的汇总点）的职责与位置在权威文档说明，与实现一致。
4. 权威文档与当前执行逻辑、实际能力一致；扩展接入说明列明需改的文件/注册点。
5. 032/033 的 pkg/kernel-app 边界与模块契约分层门禁继续通过。
6. `go build ./...`、`go vet ./...`、`gofmt -l .`、`go test ./... -count=1`、`go test -race`（受影响）、`go mod tidy -diff`、`go generate ./...` 幂等、`git diff --check` 通过。

## 8. 确认要求

这是非纯文档实施计划：将修改 `internal/composition`、可能涉及 `contract-gen` 与权威文档/门禁。只有用户在本计划报告后的后续消息中明确确认 034 当前方案，才能开始实施。若实施中发现必须改变公开契约、模块边界、`pkg/*` 契约或 Kernel 生命周期，必须退回研究并重新确认。
