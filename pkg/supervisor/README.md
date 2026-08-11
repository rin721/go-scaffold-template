# supervisor

`pkg/supervisor` 负责监督进程级 `Participant`、长期 `Task`、退出信号和优雅关闭。它不承担依赖注入或实例热替换，只协调调用方已经构造好的对象。

Participant 按登记顺序启动并按反向顺序停止。任一 Task 失败会取消其他 Task，随后进入停止阶段；多个停止错误使用 `errors.Join` 完整保留。

## 推荐入口

- `New(cfg, participants...)`：创建 Supervisor 并登记已经构造完成的 Participant。
- `AddTask(name, run)`：在运行前登记长期 Task。
- `Run(ctx)`：顺序启动、等待 Task、传播失败、反向停止并聚合错误。
- `SignalContext(parent)`：创建监听 `SIGINT` 和 `SIGTERM` 的 context。

## 基础示例

```go
package runtime

import (
	"context"
	"time"

	"github.com/rin721/go-scaffold2/pkg/supervisor"
)

type Server struct{}

func (s *Server) Name() string { return "server" }
func (s *Server) Start(ctx context.Context) error {
	return nil
}
func (s *Server) Stop(ctx context.Context) error {
	return nil
}

func Run(server *Server) error {
	process := supervisor.New(
		supervisor.Config{ShutdownTimeout: 5 * time.Second},
		server,
	)
	if err := process.AddTask("consumer", func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}); err != nil {
		return err
	}
	ctx, cancel := supervisor.SignalContext(context.Background())
	defer cancel()
	return process.Run(ctx)
}
```

## 运行语义

- `Run` 只能调用一次，且不接受 nil context；开始运行后不能再调用 `AddTask`。
- Participant 启动失败时，已启动项仍会按反向顺序停止，启动错误与停止错误同时返回。
- Task 失败会取消其他 Task；调用方主动取消 context 视为正常退出，但停止错误仍会返回。
- 没有 Task 时，`Run` 会持续等待 context 结束，而不是在 Participant 启动后立即退出。
- `ShutdownTimeout <= 0` 使用默认的 10 秒停止期限。

## 与 Kernel 的边界

使用 Kernel 的应用应通过 `kernel.NewHost` 接入进程监督。Host 固定先启动 Kernel、再启动上层 Participant，并把可选配置监听登记为 Task；停止顺序自动反转。

`pkg/supervisor` 不导入也不感知 `internal/kernel`。Kernel 的候选实例构造、租约排空、重载回滚和原子发布仍由 Kernel 自身负责，不能用 Supervisor 替代。
