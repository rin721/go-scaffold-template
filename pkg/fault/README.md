# fault

`pkg/fault` 提供跨基础能力复用的错误分类、可重试语义和关闭错误聚合。

它基于 Go 标准库 `errors`，保留 `errors.Is`、`errors.As` 和原始错误链，不引入大而全异常框架。

## 推荐入口

- `New(code, message)`：构造没有底层原因的项目分类错误。
- `Wrap(err, code, op, retryable)`：为底层错误增加操作名、分类和可重试语义，同时保留错误链。
- `CodeOf(err)`：提取项目错误码；对 `context.Canceled` 和 `context.DeadlineExceeded` 做保守映射。
- `Retryable(err)`：判断错误是否被标记为可重试。
- `JoinClose(primary, label, closeErr)`：保留主错误和资源关闭错误。

## 基础使用示例

```go
package gateway

import (
	"context"
	"errors"

	"github.com/rin721/go-scaffold-template/pkg/fault"
)

func Call(ctx context.Context, query func(context.Context) error) error {
	if err := query(ctx); err != nil {
		return fault.Wrap(err, fault.CodeUnavailable, "gateway.query", true)
	}
	return nil
}

func Handle(err error) fault.Code {
	if errors.Is(err, context.DeadlineExceeded) {
		return fault.CodeOf(err)
	}
	if fault.Retryable(err) {
		return fault.CodeUnavailable
	}
	return fault.CodeOf(err)
}
```

## 关闭错误聚合

```go
func finish(primary error, closeErr error) error {
	return fault.JoinClose(primary, "database", closeErr)
}
```

## 边界说明

- `Code` 是跨基础能力复用的粗粒度分类，不替代业务领域错误码。
- `Retryable` 只表示调用方可以考虑重试；是否真的重试应由 `resilience` 策略或业务幂等性决定。
- 错误消息不得包含密码、Token、完整 DSN 等敏感值；需要记录配置时只记录脱敏后的上下文。
