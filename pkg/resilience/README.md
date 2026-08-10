# resilience

`pkg/resilience` 提供 retry、timeout 和轻量 circuit breaker 策略。

HTTP、数据库、缓存和对象存储应复用这里的策略，而不是各自散写重试与超时规则。

## 推荐入口

- `Do(ctx, RetryPolicy, operation)`：按最大次数和指数退避执行重试。
- `WithTimeout(ctx, TimeoutPolicy, operation)`：为单次操作创建受控超时。
- `NewBreaker(threshold)`：创建按连续失败次数打开的轻量熔断器。
- `Breaker.Reset()`：在外部确认恢复后重置熔断器。

## retry 示例

```go
package gateway

import (
	"context"
	"time"

	"github.com/rin721/go-scaffold2/pkg/fault"
	"github.com/rin721/go-scaffold2/pkg/resilience"
)

func Query(ctx context.Context, call func(context.Context) error) error {
	return resilience.Do(ctx, resilience.RetryPolicy{
		MaxAttempts: 3,
		InitialWait: 50 * time.Millisecond,
		MaxWait:     500 * time.Millisecond,
		Retryable:   fault.Retryable,
	}, call)
}
```

## timeout 示例

```go
err := resilience.WithTimeout(ctx, resilience.TimeoutPolicy{
	Timeout: 2 * time.Second,
}, func(ctx context.Context) error {
	return client.Ping(ctx)
})
```

## circuit breaker 示例

```go
breaker := resilience.NewBreaker(3)
err := breaker.Do(func() error {
	return callRemote()
})
```

## 策略语义

- `MaxAttempts <= 0` 时只执行一次，不会隐式无限重试。
- 未提供 `Retryable` 时默认所有错误都可重试；生产路径建议显式传入判断函数，并确认操作具备幂等性。
- `WithTimeout` 在 `Timeout <= 0` 时不创建额外 timeout，只沿用传入 context。
- `Breaker` 是进程内轻量状态，不提供半开探测、分布式共享、指标上报或自动恢复策略。

## 在业务代码中的推荐使用方式

应用入口或基础设施 adapter 负责选择策略参数，再把受控调用暴露给业务组件。业务代码不要在每个调用点散写 `time.Sleep`、无限 for retry 或忽略 `context` 的重试循环。
