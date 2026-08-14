# health

`pkg/health` 提供健康检查注册表、超时控制、状态快照和错误聚合。

本包不依赖 HTTP。`/livez`、`/readyz` 或管理端点应由装配层或 `pkg/httpx` 适配暴露。

## 推荐入口

- `New(timeout)`：创建健康检查注册表，`timeout <= 0` 时使用默认 2 秒。
- `Register(name, check)`：注册命名检查，名称不能为空且不能重复。
- `Snapshot(ctx)`：执行所有检查并返回汇总状态和每项结果。
- `Snapshot.Error()`：聚合失败检查的错误，便于启动检查或 readiness gate 向上返回。
- `Degraded(message)`：返回降级但仍可运行的 `warn` 结果。

## 基础使用示例

```go
package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/rin721/go-scaffold-template/pkg/health"
)

func BuildRegistry(dbPing func(context.Context) error) (*health.Registry, error) {
	registry := health.New(2 * time.Second)
	if err := registry.Register("database", func(ctx context.Context) health.Result {
		if err := dbPing(ctx); err != nil {
			return health.Result{
				Kind:   health.KindReadiness,
				Status: health.StatusFail,
				Error:  err,
			}
		}
		return health.Result{Kind: health.KindReadiness, Status: health.StatusPass}
	}); err != nil {
		return nil, err
	}
	return registry, nil
}

func Check(ctx context.Context, registry *health.Registry) error {
	snapshot := registry.Snapshot(ctx)
	if snapshot.Status == health.StatusFail {
		return fmt.Errorf("health check failed: %w", snapshot.Error())
	}
	return nil
}
```

## 状态语义

- `pass`：检查通过。
- `warn`：组件降级但进程仍可运行，整体状态在没有失败项时为 `warn`。
- `fail`：检查失败，整体状态为 `fail`，失败错误可通过 `Snapshot.Error()` 聚合返回。

## 边界说明

- 本包只管理检查函数和快照，不负责 HTTP 路由、JSON 响应格式、指标上报或探针路径。
- 每个检查都会在 registry timeout 内运行，检查函数应尊重传入的 `context.Context`。
- 注册表可由应用入口创建后注入管理端点或启动检查流程，业务组件不要自行创建第二套健康检查 registry。
