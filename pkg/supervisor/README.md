# supervisor

`pkg/supervisor` 负责监督进程级 `Participant`、长期 `Task`、退出信号和优雅关闭。它不承担依赖注入或实例热替换，只协调调用方已经构造好的对象。

Participant 按登记顺序启动并按反向顺序停止。任一 Task 失败会取消其他 Task，随后进入停止阶段；多个停止错误使用 `errors.Join` 完整保留。

## 推荐入口

- `New(cfg, participants...)`：校验总预算/force 预留，返回 Supervisor 与 error，并登记已经构造完成的 Participant。
- `AddTask(name, run)`：在运行前登记长期 Task。
- `Run(ctx)`：顺序启动、等待 Task、传播失败、反向停止并聚合错误。
- `RunOperation(ctx, operation)`：顺序启动、执行一次 operation、反向停止并聚合 operation/cleanup 错误；只允许没有长期 Task 的 one-shot 流程。
- `Snapshot()`：返回 process state、ready、共享 shutdown budget 和按注册顺序排列的 typed responsibility units；不返回原始 error 文本、配置或用户对象。
- `SignalContext(parent)`：创建监听 `SIGINT` 和 `SIGTERM` 的 context。

## 基础示例

```go
package runtime

import (
	"context"
	"time"

	"github.com/rin721/go-scaffold-template/pkg/supervisor"
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
	process, err := supervisor.New(
		supervisor.Config{ShutdownTimeout: 5 * time.Second, ForceTimeout: time.Second},
		server,
	)
	if err != nil {
		return err
	}
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
- `RunOperation` 同样只能调用一次；nil operation、已经登记 Task 或重复运行都会失败。
- Participant 启动失败时，已启动项仍会按反向顺序停止，启动错误与停止错误同时返回。
- Task 失败会取消其他 Task；调用方主动取消 context 视为正常退出，但停止错误仍会返回。
- 没有 Task 时，`Run` 会持续等待 context 结束，而不是在 Participant 启动后立即退出。
- `ShutdownTimeout <= 0` 使用默认总预算 10 秒，`ForceTimeout <= 0` 使用默认预留 1 秒；force 预留必须小于总预算。
- Participant 共享同一组绝对 deadline，不会按组件重建完整 timeout。只有显式实现 `ForceStopper` 的 Participant 才进入 force 阶段；force policy 在注册时由真实接口冻结，普通 Participant 不会被推断为可强制停止。
- `Snapshot.Units` 以 `participant/task`、`start/ready/run/stop/force`、`pending/running/ready/stopped/forced/failed` 和 exit policy 结构化表达每项责任。Stop/Task 已返回 error 是 `failed`；只有 goroutine 在最终 deadline 仍未返回才是 `pending`；`forced` 不伪装为 graceful success。
- `Snapshot.Budget` 暴露同一次 shutdown 的 started/graceful/final deadline、当前 phase 和耗尽结果。只有所有已启动责任 clean stopped 且聚合错误为空，process 才报告 `stopped`。
- owner name 是诊断身份，不是任意描述文本：必须以小写字母或数字开头，只能包含小写字母、数字、点、下划线或连字符，最长 128 bytes；禁止把地址、配置值或凭据放入名称。
- one-shot CLI 使用 `RunOperation` 时不会为了凑长期 Task 启动空 goroutine；operation 结束后立即进入同一套有界反向停止。

## 与 Kernel 的边界

使用 Kernel 的长期 Service 应通过 `kernel.NewHost` 接入进程监督。Host 固定先启动 Kernel、再启动上层 Participant，并把可选配置监听登记为 Task；停止顺序自动反转。`Host.Diagnostics()` 把 Supervisor units 与 Kernel ownerships 组合成唯一 process view。Application CLI 可把 Coordinator 与 migration Participant 交给 `RunOperation`，但仍由 application composition root 明确拥有它们。

`pkg/supervisor` 不导入也不感知 `internal/kernel`。Kernel 的候选实例构造、租约排空、重载回滚和原子发布仍由 Kernel 自身负责，不能用 Supervisor 替代。
