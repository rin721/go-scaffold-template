package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

// Participant 表示由运行时统一启动和停止的参与者。
type Participant interface {
	Name() string
	Start(context.Context) error
	Stop(context.Context) error
}

// Runner 管理一组生命周期参与者和后台任务。
type Runner struct {
	participants []Participant
	tasks        []Task
	timeout      time.Duration
	mu           sync.Mutex
	started      bool
}

// Task 表示一个受 context 控制的后台任务。
type Task struct {
	Name string
	Run  func(context.Context) error
}

// Config 配置生命周期运行器。
type Config struct {
	ShutdownTimeout time.Duration
}

// New 创建生命周期运行器。
func New(cfg Config, participants ...Participant) *Runner {
	timeout := cfg.ShutdownTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Runner{participants: append([]Participant(nil), participants...), timeout: timeout}
}

// AddTask 注册后台任务。必须在 Start 前完成注册。
func (r *Runner) AddTask(name string, run func(context.Context) error) error {
	if run == nil {
		return fmt.Errorf("lifecycle task %q run function is nil", name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return fmt.Errorf("lifecycle runner already started")
	}
	r.tasks = append(r.tasks, Task{Name: name, Run: run})
	return nil
}

// Start 按注册顺序启动参与者，并运行后台任务。
func (r *Runner) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return fmt.Errorf("lifecycle runner already started")
	}
	r.started = true
	tasks := append([]Task(nil), r.tasks...)
	participants := append([]Participant(nil), r.participants...)
	r.mu.Unlock()

	for _, participant := range participants {
		if participant == nil {
			continue
		}
		if err := participant.Start(ctx); err != nil {
			return fmt.Errorf("start participant %s: %w", participant.Name(), err)
		}
	}
	group, groupCtx := errgroup.WithContext(ctx)
	for _, task := range tasks {
		task := task
		group.Go(func() error {
			if err := task.Run(groupCtx); err != nil {
				return fmt.Errorf("run task %s: %w", task.Name, err)
			}
			return nil
		})
	}
	return group.Wait()
}

// Stop 按反向顺序停止参与者，并聚合所有停止错误。
func (r *Runner) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	stopCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var joined error
	for index := len(r.participants) - 1; index >= 0; index-- {
		participant := r.participants[index]
		if participant == nil {
			continue
		}
		if err := participant.Stop(stopCtx); err != nil {
			joined = errors.Join(joined, fmt.Errorf("stop participant %s: %w", participant.Name(), err))
		}
	}
	return joined
}

// SignalContext 创建监听 SIGINT/SIGTERM 的 context。
func SignalContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
}
