# 任务：模块顶层 HTTP Handler 分责

## 1. 门禁状态

- 研究门禁：已通过，证据为 `R001`、`R002`。
- 当前计划状态：待确认。
- 当前授权：用户仅请求研究并提交修复方案；未授权实施。实施必须等待用户在计划报告后的后续消息中明确确认 031 当前方案。
- 实施前提：尚未满足；命中 design 第 12 节触发器后恢复待确认。
- 外部副作用：确认后的实施只修改仓库内源码、测试与文档目录；不 push、不 tag、不 release、不部署、不写外部数据库。

## 2. 任务清单

| ID | 工作量 | 依赖 | 任务 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| `RES-001` | M | 无 | 复核当前 binding/http 职责混叠与消费方 | R001 区分事实、推断与消费方清单 | 已完成 |
| `RES-002` | M | `RES-001` | 确定模块顶层 handler 分层目标、依赖方向与迁移边界 | R002 区分分层、顺序与剩余未知 | 已完成 |
| `PLAN-001` | M | `RES-001..002` | 冻结 031 需求、设计、文件影响、风险与验证 | README、requirements、design、tasks 完整且相互引用 | 已完成（待确认） |
| `MIG-001` | M | 用户确认 | 新建 `internal/module/todo/handler`，迁入 handler/DTO/测试并改包名 | handler 包独立编译；不 import binding/transport/第三方 HTTP 框架 | 待确认 |
| `MIG-002` | M | `MIG-001` | 更新 `module.go`、`composition/http_api.go`、`todo_authorization.go` 导入与引用 | 编译通过；`Operations`/`ActorAccess`/`NewHandler` 指向 handler 包 | 待确认 |
| `BIND-001` | M | `MIG-001` | `binding/http` 收敛为契约 + 装箱，`handlers.go` 依赖 handler.Operations | 无业务 handler 实现；`RuntimeHandlers`/`ModuleContract` 语义不变 | 待确认 |
| `CLEAN-001` | M | `MIG-002`、`BIND-001` | 删除旧 binding/http 中的业务 handler/dto 文件 | 零残留引用；无兼容 alias | 待确认 |
| `GOV-001` | M | `CLEAN-001` | 更新架构门禁规则与正反 fixture | 阻止 handler import binding/transport/第三方；binding 无业务 handler | 待确认 |
| `DOC-001` | M | `CLEAN-001` | 同步 api/README、模块指南、Todo README、internal/module/README 与变更索引 | 权威文档只描述单轨现行分层 | 待确认 |
| `VER-001` | L | 全部实施任务 | 执行生成/结构/协议/进程/完整 gate 与 oasdiff | requirements 第 7 节全部有直接证据；无旧轨残留 | 待确认 |

## 3. 实施顺序

```text
MIG-001 -> MIG-002 + BIND-001
  -> CLEAN-001 -> GOV-001 -> DOC-001 -> VER-001
```

handler 包未建成前不得迁移消费方；绑定收敛与消费方更新必须同轮完成，避免长期红灯。`CLEAN-001` 是单轨删除，不保留旧业务 handler 或兼容 wrapper。

## 4. 计划报告后的确认要求

1. 用户阅读研究结论、需求、设计与任务后，在后续消息中明确确认「031 当前方案」；
2. 确认后按任务 ID 顺序实施；范围外的新事实退回研究阶段；
3. 实施中的每个检查点提交只包含 031 范围文件；不 push、不 tag、不 release。

## 5. 停止条件

- 用户未确认当前计划；
- 命中 design 第 12 节重新确认触发器；
- 工作区出现无法可靠分离的用户修改；
- 生成、测试、race、vet、build、tidy 或架构门禁失败且无法在确认范围内修复；
- 为继续工作必须保留双轨、静态兼容 alias 或反向依赖。

停止时保留研究、计划与已确认任务范围，不以占位实现或降低门禁冒充完成。

## 6. 建议实施提交信息（确认后）

```text
refactor(module): move HTTP handler to module top-level layer
```