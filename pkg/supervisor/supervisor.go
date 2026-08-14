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
	// Ready 在 runner 已经取得其长期运行责任后关闭；nil 表示启动 goroutine 即可。
	Ready <-chan struct{}
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

// ProcessState 是 Supervisor 对外暴露的进程生命周期状态。
type ProcessState string

const (
	StateNew      ProcessState = "new"
	StateStarting ProcessState = "starting"
	StateRunning  ProcessState = "running"
	StateDraining ProcessState = "draining"
	StateStopping ProcessState = "stopping"
	StateFailed   ProcessState = "failed"
	StateStopped  ProcessState = "stopped"
)

// Snapshot 是不包含配置和凭据的并发安全运行诊断。
type Snapshot struct {
	State        ProcessState
	Ready        bool
	Since        time.Time
	LastError    string
	PendingUnits []string
}

// UnexpectedCompletionError 标识长期任务在没有终止意图时提前成功返回。
type UnexpectedCompletionError struct{ Task string }

func (e *UnexpectedCompletionError) Error() string {
	return fmt.Sprintf("supervisor task %s completed unexpectedly", e.Task)
}

// Supervisor 管理一组进程级参与者和长期任务。
type Supervisor struct {
	participants []Participant
	tasks        []Task
	timeout      time.Duration

	mu          sync.Mutex
	state       supervisorState
	diagnostics Snapshot
	ready       chan struct{}
	readyOnce   sync.Once
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
		diagnostics:  Snapshot{State: StateNew, Since: time.Now()},
		ready:        make(chan struct{}),
	}
}

// AddTask 注册长期任务。必须在 Run 前完成注册。
func (s *Supervisor) AddTask(name string, run func(context.Context) error) error {
	return s.AddRunner(Task{Name: name, Run: run})
}

// AddRunner 注册带可选运行确认的长期任务。必须在 Run 前完成注册。
func (s *Supervisor) AddRunner(task Task) error {
	name := task.Name
	run := task.Run
	if run == nil {
		return fmt.Errorf("supervisor task %q run function is nil", name)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != supervisorCreated {
		return fmt.Errorf("supervisor already run")
	}
	if name == "" {
		return fmt.Errorf("supervisor task name is required")
	}
	for _, task := range s.tasks {
		if task.Name == name {
			return fmt.Errorf("supervisor task %q is duplicated", name)
		}
	}
	s.tasks = append(s.tasks, task)
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

	if err := validateParticipants(s.participants); err != nil {
		return err
	}
	if err := validateTaskOwnership(s.participants, s.tasks); err != nil {
		return err
	}
	s.mu.Lock()
	if s.state != supervisorCreated {
		s.mu.Unlock()
		return fmt.Errorf("supervisor already run")
	}
	s.state = supervisorRunning
	s.setDiagnosticsLocked(StateStarting, false, nil, nil)
	participants := append([]Participant(nil), s.participants...)
	tasks := append([]Task(nil), s.tasks...)
	s.mu.Unlock()

	started := make([]Participant, 0, len(participants))
	for _, participant := range participants {
		if participant == nil {
			continue
		}
		if err := participant.Start(ctx); err != nil {
			wrapped := fmt.Errorf("start participant %s: %w", participant.Name(), err)
			s.setDiagnostics(StateDraining, false, wrapped, nil)
			shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.timeout)
			stopErr := s.stopStarted(shutdownCtx, started)
			cancel()
			finalErr := errors.Join(normalizeCancellation(ctx, wrapped), stopErr)
			if finalErr != nil {
				s.setDiagnostics(StateFailed, false, finalErr, nil)
			}
			s.finish()
			return finalErr
		}
		started = append(started, participant)
	}

	runnerCtx, cancelRunners := context.WithCancel(ctx)
	results := startTasks(runnerCtx, tasks)
	readyErr, completed := waitForReadiness(ctx, tasks, results)
	var runErr error
	if readyErr != nil {
		runErr = readyErr
	} else {
		s.setDiagnostics(StateRunning, true, nil, nil)
		runErr, completed = waitForTermination(ctx, tasks, results, completed)
	}
	s.setDiagnostics(StateDraining, false, runErr, nil)
	cancelRunners()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.WithoutCancel(ctx), s.timeout)
	s.setDiagnostics(StateStopping, false, runErr, nil)
	stopErr := s.stopStarted(shutdownCtx, started)
	waitErr, pending := waitTasks(shutdownCtx, tasks, results, completed)
	cancelShutdown()
	finalErr := errors.Join(normalizeCancellation(ctx, runErr), stopErr, waitErr)
	if finalErr != nil {
		s.setDiagnostics(StateFailed, false, finalErr, pending)
	}
	s.finish()
	return finalErr
}

// RunOperation 正序启动参与者、同步执行一次操作，再在同一总期限内反序停止。
//
// one-shot operation 与长期 Task 语义互斥：正常返回表示操作完成，不会被解释为
// UnexpectedCompletionError。RunOperation 只能调用一次，并完整保留操作与清理错误。
func (s *Supervisor) RunOperation(ctx context.Context, operation func(context.Context) error) error {
	if ctx == nil {
		return fmt.Errorf("supervisor context is nil")
	}
	if operation == nil {
		return fmt.Errorf("supervisor operation is nil")
	}
	if err := validateParticipants(s.participants); err != nil {
		return err
	}
	s.mu.Lock()
	if len(s.tasks) != 0 {
		s.mu.Unlock()
		return fmt.Errorf("supervisor operation does not accept long-running tasks")
	}
	if s.state != supervisorCreated {
		s.mu.Unlock()
		return fmt.Errorf("supervisor already run")
	}
	s.state = supervisorRunning
	s.setDiagnosticsLocked(StateStarting, false, nil, nil)
	participants := append([]Participant(nil), s.participants...)
	s.mu.Unlock()

	started := make([]Participant, 0, len(participants))
	for _, participant := range participants {
		if err := participant.Start(ctx); err != nil {
			wrapped := fmt.Errorf("start participant %s: %w", participant.Name(), err)
			s.setDiagnostics(StateDraining, false, wrapped, nil)
			shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.timeout)
			stopErr := s.stopStarted(shutdownCtx, started)
			cancel()
			finalErr := errors.Join(wrapped, stopErr)
			s.setDiagnostics(StateFailed, false, finalErr, nil)
			s.finish()
			return finalErr
		}
		started = append(started, participant)
	}

	s.setDiagnostics(StateRunning, true, nil, nil)
	operationErr := operation(ctx)
	s.setDiagnostics(StateDraining, false, operationErr, nil)
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.timeout)
	s.setDiagnostics(StateStopping, false, operationErr, nil)
	stopErr := s.stopStarted(shutdownCtx, started)
	cancel()
	finalErr := errors.Join(operationErr, stopErr)
	if finalErr != nil {
		s.setDiagnostics(StateFailed, false, finalErr, nil)
	}
	s.finish()
	return finalErr
}

type taskResult struct {
	name string
	err  error
}

func startTasks(ctx context.Context, tasks []Task) <-chan taskResult {
	results := make(chan taskResult, len(tasks))
	for _, task := range tasks {
		task := task
		go func() {
			err := task.Run(ctx)
			if err != nil {
				err = fmt.Errorf("run task %s: %w", task.Name, err)
			}
			results <- taskResult{name: task.Name, err: err}
		}()
	}
	return results
}

func waitForReadiness(ctx context.Context, tasks []Task, results <-chan taskResult) (error, map[string]struct{}) {
	completed := make(map[string]struct{})
	for _, task := range tasks {
		if task.Ready == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err(), completed
		case result := <-results:
			completed[result.name] = struct{}{}
			if result.err != nil {
				return result.err, completed
			}
			return &UnexpectedCompletionError{Task: result.name}, completed
		case <-task.Ready:
		}
	}
	return nil, completed
}

func waitForTermination(ctx context.Context, tasks []Task, results <-chan taskResult, completed map[string]struct{}) (error, map[string]struct{}) {
	if len(tasks) == 0 {
		<-ctx.Done()
		return ctx.Err(), completed
	}
	select {
	case <-ctx.Done():
		return ctx.Err(), completed
	case result := <-results:
		completed[result.name] = struct{}{}
		if result.err != nil {
			return result.err, completed
		}
		if ctx.Err() != nil {
			return ctx.Err(), completed
		}
		return &UnexpectedCompletionError{Task: result.name}, completed
	}
}

func (s *Supervisor) stopStarted(stopCtx context.Context, participants []Participant) error {
	var joined error
	for index := len(participants) - 1; index >= 0; index-- {
		participant := participants[index]
		result := make(chan error, 1)
		go func() { result <- participant.Stop(stopCtx) }()
		select {
		case err := <-result:
			if err != nil {
				joined = errors.Join(joined, fmt.Errorf("stop participant %s: %w", participant.Name(), err))
			}
		case <-stopCtx.Done():
			joined = errors.Join(joined, fmt.Errorf("stop participant %s: %w", participant.Name(), stopCtx.Err()))
		}
	}
	return joined
}

func waitTasks(ctx context.Context, tasks []Task, results <-chan taskResult, completed map[string]struct{}) (error, []string) {
	var joined error
	for len(completed) < len(tasks) {
		select {
		case result := <-results:
			completed[result.name] = struct{}{}
			if result.err != nil && !isOnlyCancellation(result.err) {
				joined = errors.Join(joined, result.err)
			}
		case <-ctx.Done():
			pending := make([]string, 0, len(tasks)-len(completed))
			for _, task := range tasks {
				if _, done := completed[task.Name]; !done {
					pending = append(pending, task.Name)
				}
			}
			return errors.Join(joined, fmt.Errorf("wait supervisor tasks %v: %w", pending, ctx.Err())), pending
		}
	}
	return joined, nil
}

func validateParticipants(participants []Participant) error {
	seen := make(map[string]struct{}, len(participants))
	for index, participant := range participants {
		if participant == nil {
			return fmt.Errorf("supervisor participant %d is nil", index)
		}
		name := participant.Name()
		if name == "" {
			return fmt.Errorf("supervisor participant %d name is required", index)
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("supervisor participant %q is duplicated", name)
		}
		seen[name] = struct{}{}
	}
	return nil
}

func validateTaskOwnership(participants []Participant, tasks []Task) error {
	seen := make(map[string]struct{}, len(participants)+len(tasks))
	for _, participant := range participants {
		seen[participant.Name()] = struct{}{}
	}
	for index, task := range tasks {
		if task.Name == "" {
			return fmt.Errorf("supervisor task %d name is required", index)
		}
		if task.Run == nil {
			return fmt.Errorf("supervisor task %q run function is nil", task.Name)
		}
		if _, exists := seen[task.Name]; exists {
			return fmt.Errorf("supervisor owner %q is duplicated", task.Name)
		}
		seen[task.Name] = struct{}{}
	}
	return nil
}

func (s *Supervisor) finish() {
	s.mu.Lock()
	s.state = supervisorFinished
	if s.diagnostics.State != StateFailed {
		s.setDiagnosticsLocked(StateStopped, false, nil, nil)
	}
	s.mu.Unlock()
}

// Snapshot 返回当前进程监督状态的独立副本。
func (s *Supervisor) Snapshot() Snapshot {
	if s == nil {
		return Snapshot{State: StateFailed, LastError: "supervisor is nil"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := s.diagnostics
	copy.PendingUnits = append([]string(nil), copy.PendingUnits...)
	return copy
}

// Ready 在全部参与者启动且 runner 完成运行确认后关闭。
func (s *Supervisor) Ready() <-chan struct{} {
	if s == nil {
		closed := make(chan struct{})
		close(closed)
		return closed
	}
	return s.ready
}

func (s *Supervisor) setDiagnostics(state ProcessState, ready bool, err error, pending []string) {
	s.mu.Lock()
	s.setDiagnosticsLocked(state, ready, err, pending)
	s.mu.Unlock()
}

func (s *Supervisor) setDiagnosticsLocked(state ProcessState, ready bool, err error, pending []string) {
	s.diagnostics.State = state
	s.diagnostics.Ready = ready
	s.diagnostics.Since = time.Now()
	s.diagnostics.PendingUnits = append([]string(nil), pending...)
	if err != nil {
		s.diagnostics.LastError = fmt.Sprintf("%T", err)
	}
	if state == StateRunning && ready {
		s.readyOnce.Do(func() { close(s.ready) })
	}
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
