# 任务：模块顶层 HTTP Handler 分责

## 1. 门禁状态

- 研究门禁：已通过，证据为 `R001`、`R002`。
- 当前计划状态：已完成。
- 当前授权：用户已确认 031 方案，授权本地实施、验证与聚焦提交；不授权 push、tag、release、部署或外部副作用。
- 实施前提：已满足；若命中 design 第 12 节触发器则恢复待确认。
- 外部副作用：无。只修改仓库内源码、测试与文档目录，不写外部数据库。

## 2. 任务清单

| ID | 工作量 | 依赖 | 任务 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| `RES-001` | M | 无 | 复核当前 binding/http 职责混叠与消费方 | R001 区分事实、推断与消费方清单 | 已完成 |
| `RES-002` | M | `RES-001` | 确定模块顶层 handler 分层目标、依赖方向与迁移边界 | R002 区分分层、顺序与剩余未知 | 已完成 |
| `PLAN-001` | M | `RES-001..002` | 冻结 031 需求、设计、文件影响、风险与验证 | README、requirements、design、tasks 完整且相互引用 | 已完成 |
| `MIG-001` | M | 用户确认 | 新建 `internal/module/todo/handler`，迁入 handler/DTO/测试并改包名 | handler 包独立编译；不 import binding/transport/第三方 HTTP 框架 | 已完成 |
| `MIG-002` | M | `MIG-001` | 更新 `module.go`、`composition/http_api.go`、`todo_authorization.go` 导入与引用 | 编译通过；`Operations`/`ActorAccess`/`NewHandler` 指向 handler 包 | 已完成 |
| `BIND-001` | M | `MIG-001` | `binding/http` 收敛为契约 + 装箱，`handlers.go` 依赖 `handler.Operations` | 无业务 handler 实现；`RuntimeHandlers`/`ModuleContract` 语义不变 | 已完成 |
| `CLEAN-001` | M | `MIG-002`、`BIND-001` | 删除旧 binding/http 中的业务 handler/dto 文件 | 零残留引用；无兼容 alias | 已完成 |
| `GOV-001` | M | `CLEAN-001` | 更新架构门禁规则与正反 fixture | 阻止 handler import binding/transport/第三方；binding 无业务 handler | 已完成 |
| `DOC-001` | M | `CLEAN-001` | 同步 api/README、模块指南、Todo README、internal/module/README 与变更索引 | 权威文档只描述单轨现行分层 | 已完成 |
| `VER-001` | L | 全部实施任务 | 执行生成/结构/协议/进程/完整 gate 与 oasdiff | requirements 第 7 节全部有直接证据；无旧轨残留 | 已完成 |

## 3. 实施顺序

```text
MIG-001 -> MIG-002 + BIND-001
  -> CLEAN-001 -> GOV-001 -> DOC-001 -> VER-001
```

handler 包未建成前不得迁移消费方；绑定收敛与消费方更新必须同轮完成，避免长期红灯。`CLEAN-001` 是单轨删除，不保留旧业务 handler 或兼容 wrapper。

## 4. 实施结果

1. 用户已在计划报告后的后续消息中确认「031 当前方案」。
2. 实施按任务 ID 顺序完成，见本轮提交与逐轮证据。
3. 实施提交只包含 031 范围文件；不 push、不 tag、不 release。

## 5. 逐轮证据

| 轮次 | 日期 | 完成任务 | 证据 | Commit | 剩余风险 |
| --- | --- | --- | --- | --- | --- |
| 1 | 2026-08-19 | `RES-001`、`RES-002`、`PLAN-001` | 研究结论、需求、设计、任务文档完成，HEAD `633d1a7` | 纯文档 | 未实施 |
| 2 | 2026-08-19 | `MIG-001..VER-001` | handler 上移、binding 收敛、消费方/门禁/文档同步；`go build ./...`、`go vet ./...`、`go test ./...`、affected race、`gofmt -l .`、`go mod tidy -diff`、`go generate` 幂等、`git diff --check` 全部通过 | 见本行所在提交 | 无真实第二个业务模块 |

## 6. 停止条件

- 命中 design 第 12 节重新确认触发器；
- 工作区出现无法可靠分离的用户修改；
- 生成、测试、race、vet、build、tidy 或架构门禁失败且无法在确认范围内修复；
- 为继续工作必须保留双轨、静态兼容 alias 或反向依赖。

## 7. 建议实施提交信息（已实施）

```text
refactor(module): move HTTP handler to module top-level layer
```