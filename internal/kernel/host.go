package kernel

import (
	"context"
	"fmt"
	"time"

	"github.com/rin721/go-scaffold2/pkg/supervisor"
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
}

// Host 使用 Supervisor 管理 Kernel、上层参与者和长期任务。
type Host struct {
	supervisor *supervisor.Supervisor
}

// NewHost 创建尚未运行的 Kernel Host。
//
// Kernel 固定为第一个 Participant。调用方传入的上层 Participant 随后启动，
// 并在停止时先于 Kernel 退出，从而避免上层服务访问已关闭的底层能力。
func NewHost(runtime *Kernel, options HostOptions, participants ...supervisor.Participant) (*Host, error) {
	if runtime == nil {
		return nil, fmt.Errorf("kernel host runtime is nil")
	}

	ordered := make([]supervisor.Participant, 0, len(participants)+1)
	ordered = append(ordered, runtime)
	ordered = append(ordered, participants...)
	processSupervisor := supervisor.New(supervisor.Config{ShutdownTimeout: options.ShutdownTimeout}, ordered...)

	if options.Watch != nil {
		onReloadError := options.Watch.OnReloadError
		if onReloadError == nil {
			return nil, fmt.Errorf("kernel host reload error callback is nil")
		}
		if len(runtime.loader.FilePaths()) == 0 {
			return nil, fmt.Errorf("kernel host watch requires a file config source")
		}
		if err := processSupervisor.AddTask(configWatchTaskName, func(ctx context.Context) error {
			return runtime.Watch(ctx, onReloadError)
		}); err != nil {
			return nil, fmt.Errorf("register kernel config watch task: %w", err)
		}
	}

	return &Host{supervisor: processSupervisor}, nil
}

// Run 委托 Supervisor 完整执行启动、任务等待和反向停止。
func (h *Host) Run(ctx context.Context) error {
	if h == nil || h.supervisor == nil {
		return fmt.Errorf("kernel host is nil")
	}
	return h.supervisor.Run(ctx)
}

var _ supervisor.Participant = (*Kernel)(nil)
