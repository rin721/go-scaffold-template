// Package supervisor 负责进程级参与者、长期任务、信号和优雅退出监督。
package supervisor

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

const defaultShutdownTimeout = 10 * time.Second

// Participant 表示由 Supervisor 统一启动和停止的参与者。
type Participant interface {
	Name() string
	Start(context.Context) error
	Stop(context.Context) error
}

// Task 表示一个由 Supervisor 拥有并受 context 控制的长期任务。
type Task struct {
	Name string
	Run  func(context.Context) error
}

// Config 配置进程监督器的关闭边界。
type Config struct {
	ShutdownTimeout time.Duration
}

type supervisorState uint8

const (
	supervisorCreated supervisorState = iota
	supervisorRunning
	supervisorFinished
)

// Supervisor 管理一组进程级参与者和长期任务。
type Supervisor struct {
	participants []Participant
	tasks        []Task
	timeout      time.Duration

	mu    sync.Mutex
	state supervisorState
}

// New 创建尚未运行的 Supervisor。
func New(cfg Config, participants ...Participant) *Supervisor {
	timeout := cfg.ShutdownTimeout
	if timeout <= 0 {
		timeout = defaultShutdownTimeout
	}
	return &Supervisor{
		participants: append([]Participant(nil), participants...),
		timeout:      timeout,
	}
}

// AddTask 注册长期任务。必须在 Run 前完成注册。
func (s *Supervisor) AddTask(name string, run func(context.Context) error) error {
	if run == nil {
		return fmt.Errorf("supervisor task %q run function is nil", name)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != supervisorCreated {
		return fmt.Errorf("supervisor already run")
	}
	s.tasks = append(s.tasks, Task{Name: name, Run: run})
	return nil
}

// Run 完整执行参与者启动、任务等待和反向停止。
//
// Run 只能调用一次。传入 context 被主动取消时视为正常退出；参与者、任务和
// 停止阶段的其他错误会保留原始错误链并向上返回。
func (s *Supervisor) Run(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("supervisor context is nil")
	}

	s.mu.Lock()
	if s.state != supervisorCreated {
		s.mu.Unlock()
		return fmt.Errorf("supervisor already run")
	}
	s.state = supervisorRunning
	participants := append([]Participant(nil), s.participants...)
	tasks := append([]Task(nil), s.tasks...)
	s.mu.Unlock()

	started := make([]Participant, 0, len(participants))
	for _, participant := range participants {
		if participant == nil {
			continue
		}
		if err := participant.Start(ctx); err != nil {
			stopErr := s.stopStarted(ctx, started)
			s.finish()
			return errors.Join(normalizeCancellation(ctx, fmt.Errorf("start participant %s: %w", participant.Name(), err)), stopErr)
		}
		started = append(started, participant)
	}

	runErr := runTasks(ctx, tasks)
	stopErr := s.stopStarted(ctx, started)
	s.finish()
	return errors.Join(normalizeCancellation(ctx, runErr), stopErr)
}

func runTasks(ctx context.Context, tasks []Task) error {
	if len(tasks) == 0 {
		<-ctx.Done()
		return ctx.Err()
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

func (s *Supervisor) stopStarted(parent context.Context, participants []Participant) error {
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(parent), s.timeout)
	defer cancel()

	var joined error
	for index := len(participants) - 1; index >= 0; index-- {
		participant := participants[index]
		if err := participant.Stop(stopCtx); err != nil {
			joined = errors.Join(joined, fmt.Errorf("stop participant %s: %w", participant.Name(), err))
		}
	}
	return joined
}

func (s *Supervisor) finish() {
	s.mu.Lock()
	s.state = supervisorFinished
	s.mu.Unlock()
}

func normalizeCancellation(ctx context.Context, err error) error {
	if ctx.Err() == context.Canceled && isOnlyCancellation(err) {
		return nil
	}
	return err
}

func isOnlyCancellation(err error) bool {
	if err == context.Canceled {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !isOnlyCancellation(child) {
				return false
			}
		}
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		child := wrapped.Unwrap()
		return child != nil && isOnlyCancellation(child)
	}
	return false
}

// SignalContext 创建监听 SIGINT/SIGTERM 的 context。
func SignalContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
}
