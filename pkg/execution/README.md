# execution

`pkg/execution` 定义幂等、失败重试、执行记录和恢复治理的项目自有契约。业务模块只依赖 `OperationExecutor`、`Execution`、`Key`、`Result`、`Record` 等稳定类型；backend、恢复循环、异步记录和配置策略由装配方选择并注入。

## 推荐入口

- 业务接入流程见 [业务模块接入 execution 能力](../../docs/development/execution-capability.md)。
- Kernel App 装配形态见 [Kernel App 组件开发](../../internal/kernel/app/README.md)。
- 配置节 ownership 见 [配置说明](../../docs/configuration/README.md)。

## 基础使用示例

```go
result, err := executor.Execute(ctx, execution.Execution{
	Key:       execution.Key("todo.cleanup:2026-08-20"),
	Trigger:   "scheduler",
	LeaseTTL:  time.Minute,
	Policy:    retryPolicy,
	Operation: func(ctx context.Context) (any, error) {
		return nil, service.Cleanup(ctx)
	},
})
if err != nil {
	return err
}
if result.Duplicate {
	return nil
}
```

业务代码不要自行创建第二套 Store 或 Executor 来绕过注入；需要不同策略时，通过模块配置声明命名策略并由 Kernel App 解析。

## 资源边界

- `NewExecutor` 只组合给定 `Store`，不创建跨进程存储。
- `NewMemoryStore` 仅进程内可见，不提供分布式幂等保证。
- `RecoveringStore` 主存储不可用时降级到本地 Store，使用有界缓冲、退避探测、可用性验证、回放和原子切回；`Start` 启动的 goroutine 必须由拥有者调用 `Stop`。
- `AsyncRecorder` 只把 `Record` 异步化，`Claim`、`IsCompleted`、`Complete` 和 `Release` 仍同步执行以保护幂等语义；拥有者必须调用 `Shutdown` 排空队列。
- `WithTrace`/`TraceFrom` 只传递低敏 trace/span 标识，不保存请求 body、凭据或完整 URL。

## 错误语义

错误保留原始原因链：

- `ErrEmptyKey`、`ErrNilOperation`：调用契约错误。
- `ErrAlreadyRunning`：同一 key 已有运行占用。
- `ErrRetryExhausted`：重试耗尽并保留最后失败原因。
- `ErrBackend`：Store 操作失败并保留 backend 原因。
- `ErrBufferOverflow`：降级或异步队列按告警策略溢出。

业务模块应根据业务场景决定返回、重试、告警或补偿，不在下层重复记录完整错误链。

## 边界说明

本包不定义消息 Outbox/Inbox、分布式锁、事务边界、数据库/Redis 具体实现或调度触发器。多实例语义取决于装配方注入的 Store；当前业务接入说明以 [execution 能力文档](../../docs/development/execution-capability.md) 为准。
