# lifecycle

`pkg/lifecycle` 负责启动、停止、后台任务和信号上下文。它不承担依赖注入职责，只协调已经构造好的 `Participant`。

参与者按注册顺序启动，按反向顺序停止；停止阶段使用 `errors.Join` 保留多个关闭错误。

## 推荐入口

- `New(cfg, participants...)`：创建运行器并登记已经构造完成的参与者。
- `AddTask(name, run)`：在 `Start` 前注册后台任务。
- `Start(ctx)`：按注册顺序启动参与者，然后运行后台任务。
- `Stop(ctx)`：按反向顺序停止参与者，并聚合停止错误。
- `SignalContext(parent)`：创建监听 `SIGINT/SIGTERM` 的 context。

## Participant 示例

```go
package runtime

import (
	"context"
	"time"

	"github.com/rin721/go-scaffold2/pkg/lifecycle"
)

type Server struct{}

func (s *Server) Name() string { return "server" }
func (s *Server) Start(ctx context.Context) error {
	return nil
}
func (s *Server) Stop(ctx context.Context) error {
	return nil
}

func NewRunner(server *Server) *lifecycle.Runner {
	return lifecycle.New(lifecycle.Config{ShutdownTimeout: 5 * time.Second}, server)
}
```

## 后台任务示例

```go
runner := lifecycle.New(lifecycle.Config{ShutdownTimeout: 5 * time.Second})
if err := runner.AddTask("consumer", func(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}); err != nil {
	return err
}
```

## 运行边界

- `AddTask` 必须在 `Start` 前调用；运行器启动后再注册任务会返回错误。
- `Start` 会等待所有后台任务返回。常驻任务应在外层传入可取消的 context，通常配合 `SignalContext` 使用。
- `Stop` 会使用 `ShutdownTimeout` 限制停止阶段；多个参与者停止失败时会用 `errors.Join` 保留全部错误。
- 本包只协调生命周期，不创建依赖、不解析配置、不管理组件装配图。

## 推荐装配方式

不需要动态配置的应用仍可在入口直接构造具体对象，再把需要统一启动/停止的对象包装成 `Participant`。使用 kernel 时，将 kernel 作为第一个 Participant，把 `Kernel.Watch` 注册为后台 Task，再登记依赖基础能力的服务；Runner 的反向停止顺序会先停止上层服务，再停止 kernel 管理的资源。

本包仍然只协调已经构造好的 Participant 和 Task，不导入或感知 `internal/kernel`。业务组件不应自行监听进程信号或绕过 Runner 启动长期 goroutine。
