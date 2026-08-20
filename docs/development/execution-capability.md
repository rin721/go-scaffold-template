# 业务模块接入 execution 能力（幂等 / 重试 / 执行记录）

> 权威文档：本文是「应用模块如何接入 `execution` 能力」的唯一现行入口。实现与契约以
> `pkg/execution`、`internal/kernel/app/execution`、`internal/kernel/composition` 及业务模块代码为准。

## 1. 业务模块应该拿到什么

业务模块**不直接构造** backend、不感知恢复治理、不读 `Capabilities` 全集，也不反向依赖
`kernel/app/execution` 的具体实现。它只消费一个由调用方定义、经 composition 注入的**最小稳定契约**，
核心是：

```go
// pkg/execution.OperationExecutor 是底层能力输出的稳定执行契约。
type OperationExecutor interface {
    Execute(ctx context.Context, exec Execution) (Result, error)
}
```

模块应在自己的 `service` 层定义一个**窄 port**（由使用方定义抽象；Adapter 由 composition 提供并依赖该契约），
而不是把 `Capabilities.Execution` 或 `pkg/execution.OperationExecutor` 直接塞进业务代码深处。

## 2. 执行入口与关键类型

来自 `pkg/execution`（权威 godoc）：

```go
type Execution struct {
    Key        Key                 // 幂等键 = 业务域 + 业务/请求 ID
    Policy     resilience.RetryPolicy // 直接给策略（未用命名策略时）
    Timeout    time.Duration        // 0 = 不额外超时
    Trigger    string               // 低敏触发者/来源，写入执行记录
    Operation  Operation            // func(ctx) (any, error) 业务执行体
    PolicyName string               // 引用配置里按模块声明的命名策略（推荐）
}

type Result struct {
    Status    Status // completed | running | failed
    Duplicate bool   // 重复提交且已完成 = true，不再执行 Operation
    RecordID  string
}
```

- 幂等：同 `Key` 重复提交 → `Result.Duplicate=true`，`Operation` 不执行、不重试。
- 重试：对可重试错误按策略退避；重试次数、间隔在策略中治理。
- 执行记录：自动落记录，并携带经 `context` 传递的低敏全链路追踪标识。

## 3. 按模块声明命名策略（策略隔离）

在应用配置中按模块名核对这些策略，多个模块互不绑定同一套固定参数：

```yaml
execution:
  driver: memory            # 当前 memory backend（自带降级 + 恢复 + 异步记录 + 观测）
  policies:
    todo:
      retryMaxAttempts: 3
      retryInitialWaitMs: 50
      retryMaxWaitMs: 500
      timeoutMs: 2000
```

策略配置字段：`retryMaxAttempts` / `retryInitialWaitMs` / `retryMaxWaitMs` / `timeoutMs`。
未知 `PolicyName` 会让 `Execute` 返回可识别错误，**不会静默回退**。

## 4. 在业务代码中使用

```go
// executor 是经 composition 注入的模块自有执行 port
res, err := executor.Execute(ctx, pkgexecution.Execution{
    Key:        pkgexecution.Key("todo:complete:" + command.ID), // 幂等键
    PolicyName: "todo",
    Operation: func(ctx context.Context) (any, error) {
        return s.repository.Save(ctx, todo) // 业务执行体
    },
})
// err != nil：执行治理失败，按错误语义处理（重试耗尽 / backend 失败 / 取消 / 超时）
// res.Duplicate==true：重复提交已完成，不重跑
```

如需写全链路追踪：在进入前 `ctx = pkgexecution.WithTrace(ctx, traceID)`。

## 5. 观测

- `Access.Recovery()`：恢复治理快照（状态 / 缓冲 / 丢弃 / 状态变化次数）。
- `Access.Health()`：Healthy=pass，Degraded/Recovering=warn。
- 状态变化自动输出日志（Degraded=Warn，Recovering/Healthy=Info），异步记录失败输出 Warn。

## 6. 边界（必须明确）

- 本地 memory 降级只是**单实例容错手段**，不提供与分布式 Cache 相同强度的幂等/跨进程一次语义；
  多实例下不保证跨实例去重一致。
- 真实外部主存储（Cache=Redis / 数据库）尚未接入：当前主存储为 memory，恢复/回放/去重语义已通过
  故障注入在装配层端到端验证；接入真实主存储属 `docs/changes/035` 的 `NEXT-002`。
- 业务模块对一次业务操作应自行选择合适幂等键并明确其生命周期（何时算重复、何时过期）。

## 相关入口

- 契约：`pkg/execution`（`OperationExecutor` / `Execution` / `Result` / `WithTrace`）
- 组件：`internal/kernel/app/execution`（`Access.Execute` / `Recovery` / `Health`）
- 装配：`internal/kernel/composition` → `Capabilities.Execution`
- 接入示例：`docs/changes/036-business-module-execution-adoption/design.md`（Todo 落地）
