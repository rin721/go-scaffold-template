# 041 日志体系补齐与治理设计

## 总体策略

沿用现有观测链：

```text
pkg/logger
  -> internal/kernel/logging.Manager baseline/replacement
  -> composition 显式注入
  -> 各 owner 只记录自己负责的事件
  -> Health / Diagnostics / Metrics / Trace / Execution Record 保持各自职责
```

本任务只补齐 owner 边界、字段、级别和测试门禁，不引入新的日志框架或跨层事件总线。

## 日志级别

| 场景 | Debug | Info | Warn | Error |
| --- | --- | --- | --- | --- |
| migration one-shot | phase start、config loaded | status/up completed | compatible=false、dirty/too old 等可操作异常结果 | operation failed、close failed |
| execution async/recovery | queue state 诊断（如需要） | recovered/healthy | degraded、async persistence failed、overflow alert | 组件启动失败或 stop cleanup debt，由上层 owner 记录 |
| messaging consumer | 可选的 sampled duplicate/ack 诊断 | admission opened/closed、provider ready | defer/retry/decode reject、optional route unavailable | dead-letter、required route unavailable、不可恢复 provider failed |
| scheduler | task start/completed、no-op | scheduler start/stop | coordination degraded、task failed | task fatal，交给 generation/supervisor 终结 |
| management | 不记录每次成功轮询 | 低频状态恢复（如实现状态缓存） | readiness warn、diagnostics 部分不可用 | management operation failed、readiness fail 由状态变化 owner 记录 |

健康路径不为了覆盖级别伪造 Warn/Error；取消和正常关停不记 Error。

## 结构化字段

稳定字段由语义 owner 就近定义，优先使用已有名称：

- 通用：`owner`、`phase`、`operation`、`generation`、`outcome`、`error_type`、`cause_type`。
- migration：`current_version`、`target_version`、`dirty`、`empty`、`compatible`。
- execution：`state`、`from`、`buffered`、`dropped`、`overflow`。
- messaging：`provider`、`driver`、`consumer`、`producer`、`route`、`contract`、`message_id`、`trace_id`、`delivery_count`、`disposition`。
- scheduler：`task`、`state`、`trigger`、`attempt`。
- management：`probe`、`status`、`status_class`、`component`。

禁止字段：密码、Token、Secret、完整 DSN、Authorization、Cookie、Broker URI、完整 URL/query、payload/body、headers 全量、subject、配置快照、原始错误文本和未经审查的任意 map。

## 文件影响

预计修改：

- `internal/kernel/app/execution/execution.go` 与测试：修复 async error 日志字段。
- `internal/composition/migration.go`、`internal/module/migration/service.go` 或 CLI binding 测试：增加 one-shot operation 日志。
- `internal/kernel/app/messaging/messaging.go`、`internal/kernel/app/messaging/rabbitmq/rabbitmq.go` 与测试：增加 Consumer disposition、decode reject、admission/provider generation 字段。
- `internal/module/ops/binding/http/handler.go` 或 Ops service/binding 测试：增加 management 异常 outcome 日志，避免成功轮询噪声。
- `internal/kernel/app/schedule/schedule_test.go`：补日志断言，必要时微调字段。
- `internal/kernel/composition/architecture_test.go`：扩展禁止模式，守住低敏日志。
- `docs/development/logging.md`、`docs/development/execution-capability.md`、`docs/development/scheduled-task-capability.md`、`docs/development/messaging-capability.md`、`docs/operations/migration-and-rollback.md`、`docs/operations/messaging.md`、`docs/operations/scheduled-tasks.md`。
- `docs/changes/041-logging-observability-governance/**` 与 `docs/changes/README.md`。

具体实现时若发现需要改变公共接口、配置格式、模块边界、RabbitMQ topology、execution backend 或数据库迁移语义，必须回到计划并重新确认。

## 实施细节

### Execution

将 `AsyncConfig.OnError` 的日志从原始错误文本改为分类字段。优先复用 `errors.Is` 和稳定错误类型，必要时增加包内 `executionErrorType` helper。日志示例：

```text
level=warn msg="execution record persistence failed"
owner=execution phase=async-record error_type=execution_buffer_overflow cause_type=...
```

### Migration

在 application composition 的 one-shot 边界记录 operation start/completed/failed。`status` 和 `up` 返回前记录低敏状态字段；失败只记录错误类型和 phase，完整错误链继续向 CLI 边界返回，由 stderr 输出人类可读错误。

### Messaging

Consumer runtime 只记录非成功或需要 operator 关注的 disposition：

- `DispositionDeferUncounted`：Warn，动作是 defer，原因通常是 execution backend、running lease 或上游取消。
- `DispositionRetryCounted`：Warn，动作是 broker retry，记录 delivery_count。
- `DispositionDeadLetter`：Error，动作是 dead-letter，记录 error_type、consumer、route、message_id、trace_id。
- decode/contract reject：Warn，不记录 payload 或 headers。

Provider/admission 日志增加 `generation`、`enabled` 或 `desired` 字段。若需要 Provider 拿到 generation，只在 `internal/kernel/app/messaging` 的内部依赖结构中传递，不暴露给业务 `pkg/messaging`。

### Management

management 成功探针可能高频，不逐次 Info。只在异常 outcome 或状态变化时记录低敏事件；若实现状态变化缓存会引入并发状态，需要保持 owner 明确并用测试覆盖。

### Scheduler

保留当前任务级日志策略，补测试保护。只有发现字段缺失影响关联时才微调，不扩大为每次 tick 或 skip 的日志。

## 验证方案

1. `logger.TestLogger` 单元测试断言新增事件的 level、message、字段、数量和顺序。
2. 敏感信息测试使用包含 DSN、URI、payload、subject、token-like 字符串的错误或消息，确认日志不包含原文。
3. Messaging 使用 fake Provider/Consumer 测试 disposition；RabbitMQ adapter 用单元 fixture 测试 decode reject 和 provider state 字段，不要求本轮连接真实 Broker。
4. Migration 使用临时配置和 SQLite 测试 status/up 日志；进程 smoke 只使用临时目录与 loopback。
5. 架构搜索门禁覆盖 direct zap/global logger、production Noop、`logger.String("error", err.Error())`、`fmt.Print` 运行日志。
6. 执行范围匹配的 Go 与 Markdown 验证。

