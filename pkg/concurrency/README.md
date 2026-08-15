# concurrency

`pkg/concurrency` 封装 singleflight 和固定并发 worker pool；内部可以使用 errgroup，但不把第三方 Group 类型交给调用方。

运行期 goroutine 必须受 context 控制，不能靠调用时序碰巧退出。

## 推荐入口

- `SingleFlight.Do(key, fn)`：合并相同 key 的并发加载，避免缓存击穿或重复远端请求。
- `NewPool(workers).Run(ctx, tasks)`：用固定并发度执行一组 `context` 感知任务。

## worker pool 示例

```go
package batch

import (
	"context"

	"github.com/rin721/go-scaffold-template/pkg/concurrency"
)

func Process(ctx context.Context, ids []string, handle func(context.Context, string) error) error {
	tasks := make([]func(context.Context) error, 0, len(ids))
	for _, id := range ids {
		id := id
		tasks = append(tasks, func(ctx context.Context) error {
			return handle(ctx, id)
		})
	}
	return concurrency.NewPool(4).Run(ctx, tasks)
}
```

## singleflight 示例

```go
package profile

import (
	"github.com/rin721/go-scaffold-template/pkg/concurrency"
)

type Loader struct {
	sf concurrency.SingleFlight
}

func (l *Loader) Load(key string, query func() (any, error)) (any, error) {
	value, err, _ := l.sf.Do(key, query)
	return value, err
}
```

## 并发边界

- `Pool.Run` 中任一任务返回错误后，关联 context 会被取消，后续任务应主动观察 `ctx.Done()`。
- `workers <= 0` 会退回到 1，避免无 worker 导致调用永久等待。
- `SingleFlight` 只合并同进程内相同 key 的同时调用，不提供跨进程锁或持久化缓存。
- 业务代码应把任务失败原样向上返回；不要在任务内部吞掉错误后让批处理看起来成功。
