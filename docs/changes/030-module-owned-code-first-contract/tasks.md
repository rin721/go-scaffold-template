# 任务：模块自有代码优先 HTTP 契约

## 1. 门禁状态

- 研究门禁：已通过，证据为 `R001`、`R002`、`R003`。
- 当前计划状态：已完成。
- 当前授权：用户已确认 030 方案，授权本地实施、验证与聚焦提交；不授权 push、tag、release、部署或外部副作用。
- 实施前提：已满足；若命中 design 第 13 节触发器则恢复待确认。
- 外部副作用：无。只修改仓库内源码、生成物、依赖声明与测试。

## 2. 任务清单

| ID | 工作量 | 依赖 | 任务 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| `RES-001` | M | 无 | 复核当前 spec-first 契约方向、模块耦合、第三方暴露与既有 ADR 决策 | R001 区分事实、推断与权限来源 | 已完成 |
| `RES-002` | M | `RES-001` | 核实 typed code-first 工具链（invopop/jsonschema、kin-openapi、yaml.v3）可行性 | R002 使用已固定版本与本地模块缓存，说明采用/拒绝路径 | 已完成 |
| `RES-003` | M | `RES-001`、`RES-002` | 确定通用契约能力分层、迁移边界与门禁方向 | R003 区分分层、顺序与剩余未知 | 已完成 |
| `PLAN-001` | M | `RES-001..003` | 冻结 030 需求、设计、文件影响、风险与验证 | README、requirements、design、tasks 完整且相互引用 | 已完成 |
| `GEN-001` | L | 用户确认 | 建立 pkg 层通用契约能力与生成器 | 生成器输出与当前 `api/openapi.yaml` 语义等价（oasdiff 无 ERR）；operationId/policy 校验完整 | 已完成 |
| `GEN-002` | M | `GEN-001` | 生成 `api/openapi.yaml` 与 `operation_inventory.gen.go` 并接清理 diff 门禁 | go:generate 幂等 + clean diff；首次迁移差异逐项审阅 | 已完成 |
| `TODO-001` | L | `GEN-001` | Todo 迁移为模块自有契约、DTO 与 typed Handler 适配器 | 不再 import 生成包；窄 `Operations` 使用模块自有 DTO | 已完成 |
| `BIND-001` | L | `GEN-001` | transport 改为单一契约 binder | 一套 spec 校验/policy/404/405/问题呈现；每代一次 | 已完成 |
| `COMP-001` | M | `TODO-001`、`BIND-001` | composition 聚合契约与运行期 handler 并连接 Auth/Ops | 删除完整 `StrictServerInterface` 断言；注册缺失构造期失败 | 已完成 |
| `REM-001` | M | `COMP-001` | 删除 oapi-codegen 生成链、nethttp-middleware 依赖与旧生成物 | 零引用 `api.gen.go`、`api/oapi-codegen.yaml`、`StrictServerInterface`、`HandlerWithOptions`、`GetSwagger` | 已完成 |
| `GOV-001` | M | `REM-001` | 更新架构门禁规则与正反 fixture | 阻止模块/第三方泄漏、双绑定、生成包引用；允许 contract-gen 生成期 import 模块契约 | 已完成 |
| `DOC-001` | M | `REM-001` | 同步 `api/README.md`、模块指南、Todo README、`pkg/README.md` 与变更索引 | 权威文档只描述 code-first 单一现行说明 | 已完成 |
| `VER-001` | L | 全部实施任务 | 执行生成/结构/协议/进程/完整 gate 与 oasdiff | requirements 第 7 节全部有直接证据；无旧轨残留 | 已完成 |

## 3. 实施顺序

```text
GEN-001 -> GEN-002
  -> TODO-001 + BIND-001 -> COMP-001
  -> REM-001 -> GOV-001 -> DOC-001 -> VER-001
```

通用能力先行必须成立：`GEN-001` 未完成且未 golden 等价前，不得迁移 Todo。`TODO-001..REM-001` 是单轨替换，不保留旧构造器、旧生成物、旧完整接口断言或兼容 wrapper。

## 4. 确认与实施结果

1. 用户已在计划报告后的后续消息中确认“030 当前方案”。
2. 实施按任务 ID 顺序完成，见下方逐轮证据。
3. 实施提交只包含 030 范围文件；不 push、不 tag、不 release。

## 5. 逐轮证据

| 轮次 | 日期 | 完成任务 | 证据 | Commit | 剩余风险 |
| --- | --- | --- | --- | --- | --- |
| 1 | 2026-08-19 | `RES-001..003`、`PLAN-001` | 研究结论、需求、设计、任务文档完成，HEAD `c68377b` | 纯文档 | 未实施 |
| 2 | 2026-08-19 | `GEN-001`、`GEN-002`、`TODO-001`、`BIND-001`、`COMP-001`、`REM-001`、`GOV-001`、`DOC-001`、`VER-001` | 代码优先运行时迁移完成：`go build ./...`、`go vet ./...`、`go test ./... -count=1`、affected race、`go mod tidy -diff`、`gofmt -l .`、`go generate` 幂等、`oasdiff breaking` 无 ERR、架构门禁全部通过 | 见本次提交 | 无真实第二个业务模块 |

## 6. 停止条件

- 命中 design 第 13 节重新确认触发器；
- 工作区出现无法可靠分离的用户修改；
- 生成、测试、race、vet、build、tidy 或架构门禁失败且无法在确认范围内修复；
- 为继续工作必须保留双轨、动态注册、手写 YAML 或第三方直连。

## 7. 建议实施提交信息（已实施）

```text
refactor(http): generate contract from module-owned typed declarations
```