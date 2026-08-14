package kernel

import (
	"context"
	"fmt"
	"time"

	"github.com/rin721/go-scaffold-template/pkg/health"
	"github.com/rin721/go-scaffold-template/pkg/supervisor"
)

const configWatchTaskName = "kernel-config-watch"

// WatchOptions 显式启用并配置 Kernel 文件配置监听。
type WatchOptions struct {
	OnReloadError func(error)
}

// HostOptions 配置 Kernel Host 的进程监督和可选配置监听。
type HostOptions struct {
	ShutdownTimeout time.Duration
	Watch           *WatchOptions
	Runners         []supervisor.Task
}

// Host 使用 Supervisor 管理 Kernel、上层参与者和长期任务。
type Host struct {
	supervisor  *supervisor.Supervisor
	coordinator *Coordinator
	health      *health.Registry
}

// NewHost 创建尚未运行的 Kernel Host。
//
// Coordinator 固定为第一个 Participant，并在内部启动 Kernel。调用方传入的上层
// Participant 随后启动，并在停止时先于 Kernel 退出，避免访问已关闭的底层能力。
func NewHost(coordinator *Coordinator, options HostOptions, participants ...supervisor.Participant) (*Host, error) {
	if coordinator == nil || coordinator.runtime == nil {
		return nil, fmt.Errorf("kernel host coordinator is nil")
	}

	ordered := make([]supervisor.Participant, 0, len(participants)+1)
	ordered = append(ordered, coordinator)
	ordered = append(ordered, participants...)
	processSupervisor := supervisor.New(supervisor.Config{ShutdownTimeout: options.ShutdownTimeout}, ordered...)
	for index, runner := range options.Runners {
		if err := processSupervisor.AddRunner(runner); err != nil {
			return nil, fmt.Errorf("register host runner %d: %w", index, err)
		}
	}

	if options.Watch != nil {
		onReloadError := options.Watch.OnReloadError
		if onReloadError == nil {
			return nil, fmt.Errorf("kernel host reload error callback is nil")
		}
		if len(coordinator.loader.FilePaths()) == 0 {
			return nil, fmt.Errorf("kernel host watch requires a file config source")
		}
		watchReady := make(chan struct{})
		if err := processSupervisor.AddRunner(supervisor.Task{
			Name:  configWatchTaskName,
			Ready: watchReady,
			Run: func(ctx context.Context) error {
				return coordinator.watch(ctx, onReloadError, watchReady)
			},
		}); err != nil {
			return nil, fmt.Errorf("register kernel config watch task: %w", err)
		}
	}

	registry := health.New(2 * time.Second)
	host := &Host{supervisor: processSupervisor, coordinator: coordinator, health: registry}
	if err := registry.Register("process.liveness", host.liveness); err != nil {
		return nil, fmt.Errorf("register process liveness: %w", err)
	}
	if err := registry.Register("process.readiness", host.readiness); err != nil {
		return nil, fmt.Errorf("register process readiness: %w", err)
	}
	return host, nil
}

// Health 执行有界的进程 liveness/readiness 检查，不创建管理端点。
func (h *Host) Health(ctx context.Context) health.Snapshot {
	if h == nil || h.health == nil {
		return health.Snapshot{Status: health.StatusFail, Results: []health.Result{{Name: "process", Status: health.StatusFail, Message: "host is nil"}}}
	}
	return h.health.Snapshot(ctx)
}

func (h *Host) liveness(context.Context) health.Result {
	state := h.supervisor.Snapshot().State
	status := health.StatusPass
	if state == supervisor.StateFailed || state == supervisor.StateStopped {
		status = health.StatusFail
	}
	return health.Result{Kind: health.KindLiveness, Status: status, Message: string(state)}
}

func (h *Host) readiness(context.Context) health.Result {
	kernelState, processState := h.Diagnostics()
	status := health.StatusFail
	if kernelState.Ready && processState.Ready {
		status = health.StatusPass
	}
	return health.Result{Kind: health.KindReadiness, Status: status, Message: string(processState.State)}
}

// Run 委托 Supervisor 完整执行启动、任务等待和反向停止。
func (h *Host) Run(ctx context.Context) error {
	if h == nil || h.supervisor == nil {
		return fmt.Errorf("kernel host is nil")
	}
	return h.supervisor.Run(ctx)
}

// Ready 在所有 startup owner 和必需 runner 都进入运行态后关闭。
func (h *Host) Ready() <-chan struct{} {
	if h == nil || h.supervisor == nil {
		closed := make(chan struct{})
		close(closed)
		return closed
	}
	return h.supervisor.Ready()
}

// Diagnostics 返回配置协调与进程监督的安全状态快照。
func (h *Host) Diagnostics() (Diagnostics, supervisor.Snapshot) {
	if h == nil {
		return Diagnostics{State: LifecycleFailed}, supervisor.Snapshot{State: supervisor.StateFailed}
	}
	return h.coordinator.Diagnostics(), h.supervisor.Snapshot()
}
