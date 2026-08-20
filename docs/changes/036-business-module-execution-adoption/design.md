# 036 设计方案：业务模块接入 execution（Todo 落地）

引用支撑研究：R001。

## 1. 纯文档交付

新增 `docs/development/execution-capability.md`（唯一现行权威接入文档）：声明式命名策略配置、
`OperationExecutor.Execution/Result` 用法、错误语义、`WithTrace` 全链路、观测与多实例边界；
在 `docs/development/application-module-development.md` 引入口链接。纯文档可按门禁直接完成。

## 2. Todo 接入：Service 窄 port

在 `internal/module/todo/service` 定义模块自有执行 port（由使用方定义抽象，Adapter 依赖并满足它）：

```go
// Executor 是 Todo 用例经 execution 能力执行的关键写操作时使用的窄 port。
// 提供幂等（按 key 去重，重复完成返回 Duplicate）、按策略重试与执行记录。
type Executor interface {
    Execute(context.Context, pkgexecution.Execution) (pkgexecution.Result, error)
}
```

`Service` 新增该依赖（`service.New(..., executor Executor, ...)` 或 `WithExecutor`），
在 `Complete`（及可选的 `Create`）对关键写操作（如 `repository.Save`）包装执行：

```go
res, err := s.executor.Execute(ctx, pkgexecution.Execution{
    Key:        pkgexecution.Key("todo:complete:" + id),
    PolicyName: "todo",
    Operation: func(ctx context.Context) (any, error) {
        return s.repository.Save(ctx, todo)
    },
})
```

语义：
- `res.Duplicate==true`：该幂等键已完成，不重跑写操作，直接返回已完成对象。
- 可重试仓库错误按 `todo` 策略退避重试；重试耗尽 / backend 失败保留错误链向上导出。
- 执行记录自动落盘（异步），并经 `WithTrace` 携带追踪标识。

## 3. composition 注入 Adapter

`internal/composition/todo.go#prepareTodo` 从 `capabilities.Execution`（`executionapp.Access`）构造窄 port
Adapter（类型适配，不泄漏 `executionapp.Access` 之外的实现），经 `todo.Dependencies.Executor` 注入 `todo.New`；
`todo.Module` 透出到 `Service`。无反向依赖、无循环。

## 4. 配置

`execution.policies.todo`（命名策略 `TODO-001` 应用默认）：
`retryMaxAttempts: 3` / `retryInitialWaitMs: 50` / `retryMaxWaitMs: 500` / `timeoutMs: 2000`。
`Execution.PolicyName="todo"` 由 `executionapp.Access.Execute` 解析填充 `Policy`/`Timeout`（未知名报错，不静默回退）。

## 5. 文件影响（非文档部分）

| 文件 | 动作 |
| --- | --- |
| `internal/module/todo/service/service.go` | 新增 `Executor` 窄 port；注入到 `Service`；`Complete`(或 Create) 包装执行 |
| `internal/module/todo/service/service_test.go` | 覆盖重复提交只执行一次、可重试按策略重试、executor 失败错误链 |
| `internal/module/todo/module.go` | `Dependencies` 增 `Executor`；传递到 `service.New` |
| `internal/composition/todo.go` | 用 `capabilities.Execution` 构造 Adapter 并注入 |
| `internal/composition/service_test.go` 等 | 装配/门禁更新 |
| 配置文件 / 默认配置 | `execution.policies.todo`（若 application 默认配置含 execution 节） |

## 6. 失败与边界语义

- 重复提交：返回已完成（Duplicate），不重跑；调用方按 `Result.Duplicate` 处理。
- 重试：仅可重试错误（`fault.Retryable`）；`MaxAttempts` 有上限；超时走 `Timeout`。
- backend 失败：返回可区分错误，不静默成功；记录失败保留原错误（异步日志 Warn）。
- 多实例边界：memory 主存储不等价于分布式 Cache 幂等；明确记录。

## 7. 验证方案

- 单元：Service 窄 port 覆盖（重复只执行一次、重试、executor 失败错误链）。
- 装配：`todo.New` 注入 Executor 成功；composition 装配后 `Capabilities.Execution` 可用。
- 门禁：`go build/vet/gofmt ./...`、受影响 `go test -race`、`go mod tidy -diff`、`git diff --check`。

## 8. 未决 / 待确认

- 落地哪个用例：优先 `Complete`（已有业务幂等，叠加执行治理最自然）。是否同时包 `Create` 由确认决定。
- 是否在默认配置文件加入 `execution.policies.todo`（涉及应用默认配置改动，需确认）。
