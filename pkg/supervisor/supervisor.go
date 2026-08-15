// Package supervisor 负责进程级参与者、长期任务、信号和有界退出监督。
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

const (
	defaultShutdownTimeout = 10 * time.Second
	defaultForceTimeout    = time.Second
)

// Participant 表示由 Supervisor 统一启动和停止的参与者。
type Participant interface {
	Name() string
	Start(context.Context) error
	Stop(context.Context) error
}

// ForceStopper 表示协议明确支持有损终止的 Participant。
// 普通 Participant 不会因为 Stop 失败而被推断为支持 force。
type ForceStopper interface {
	ForceStop(context.Context) error
}

// Task 表示一个由 Supervisor 拥有并受 context 控制的长期任务。
type Task struct {
	Name string
	Run  func(context.Context) error
	// Ready 在 runner 已经取得其长期运行责任后关闭；nil 表示启动 goroutine 即可。
	Ready <-chan struct{}
}

// Config 配置进程监督器共享的总关闭预算和 force 预留。
type Config struct {
	ShutdownTimeout time.Duration
	ForceTimeout    time.Duration
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
	State               ProcessState
	Ready               bool
	Since               time.Time
	LastError           string
	PendingParticipants []string
	PendingTasks        []string
	ForcedParticipants  []string
}

// UnexpectedCompletionError 标识长期任务在没有终止意图时提前成功返回。
type UnexpectedCompletionError struct{ Task string }

func (e *UnexpectedCompletionError) Error() string {
	return fmt.Sprintf("supervisor task %s completed unexpectedly", e.Task)
}

// Supervisor 管理一组进程级参与者和长期任务。
type Supervisor struct {
	participants    []Participant
	tasks           []Task
	shutdownTimeout time.Duration
	forceTimeout    time.Duration

	mu          sync.Mutex
	state       supervisorState
	diagnostics Snapshot
	ready       chan struct{}
	readyOnce   sync.Once
}

// New 创建尚未运行的 Supervisor，并拒绝无法保留 graceful 阶段的预算。
func New(cfg Config, participants ...Participant) (*Supervisor, error) {
	shutdownTimeout := cfg.ShutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = defaultShutdownTimeout
	}
	forceTimeout := cfg.ForceTimeout
	if forceTimeout <= 0 {
		forceTimeout = defaultForceTimeout
	}
	if forceTimeout >= shutdownTimeout {
		return nil, fmt.Errorf("supervisor force timeout must be less than shutdown timeout")
	}
	return &Supervisor{
		participants:    append([]Participant(nil), participants...),
		shutdownTimeout: shutdownTimeout,
		forceTimeout:    forceTimeout,
		diagnostics:     Snapshot{State: StateNew, Since: time.Now()},
		ready:           make(chan struct{}),
	}, nil
}

// AddTask 注册长期任务。必须在 Run 前完成注册。
func (s *Supervisor) AddTask(name string, run func(context.Context) error) error {
	return s.AddRunner(Task{Name: name, Run: run})
}

// AddRunner 注册带可选运行确认的长期任务。必须在 Run 前完成注册。
func (s *Supervisor) AddRunner(task Task) error {
	if s == nil {
		return fmt.Errorf("supervisor is nil")
	}
	if task.Run == nil {
		return fmt.Errorf("supervisor task %q run function is nil", task.Name)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != supervisorCreated {
		return fmt.Errorf("supervisor already run")
	}
	if task.Name == "" {
		return fmt.Errorf("supervisor task name is required")
	}
	for _, existing := range s.tasks {
		if existing.Name == task.Name {
			return fmt.Errorf("supervisor task %q is duplicated", task.Name)
		}
	}
	s.tasks = append(s.tasks, task)
	return nil
}

// Run 完整执行参与者启动、任务等待和反向停止。
func (s *Supervisor) Run(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("supervisor context is nil")
	}
	participants, tasks, err := s.beginRun(true)
	if err != nil {
		return err
	}
	started := make([]Participant, 0, len(participants))
	for _, participant := range participants {
		if err := participant.Start(ctx); err != nil {
			wrapped := fmt.Errorf("start participant %s: %w", participant.Name(), err)
			s.setDiagnostics(StateDraining, false, wrapped, nil, nil, nil)
			stopErr := s.shutdown(started, nil, nil, nil)
			return s.complete(errors.Join(normalizeCancellation(ctx, wrapped), stopErr))
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
		s.setDiagnostics(StateRunning, true, nil, nil, nil, nil)
		runErr, completed = waitForTermination(ctx, tasks, results, completed)
	}
	s.setDiagnostics(StateDraining, false, runErr, nil, nil, nil)
	cancelRunners()
	stopErr := s.shutdown(started, tasks, results, completed)
	return s.complete(errors.Join(normalizeCancellation(ctx, runErr), stopErr))
}

// RunOperation 正序启动参与者、同步执行一次操作，再在同一总期限内反序停止。
func (s *Supervisor) RunOperation(ctx context.Context, operation func(context.Context) error) error {
	if ctx == nil {
		return fmt.Errorf("supervisor context is nil")
	}
	if operation == nil {
		return fmt.Errorf("supervisor operation is nil")
	}
	participants, _, err := s.beginRun(false)
	if err != nil {
		return err
	}
	started := make([]Participant, 0, len(participants))
	for _, participant := range participants {
		if err := participant.Start(ctx); err != nil {
			wrapped := fmt.Errorf("start participant %s: %w", participant.Name(), err)
			s.setDiagnostics(StateDraining, false, wrapped, nil, nil, nil)
			return s.complete(errors.Join(wrapped, s.shutdown(started, nil, nil, nil)))
		}
		started = append(started, participant)
	}
	s.setDiagnostics(StateRunning, true, nil, nil, nil, nil)
	operationErr := operation(ctx)
	s.setDiagnostics(StateDraining, false, operationErr, nil, nil, nil)
	return s.complete(errors.Join(operationErr, s.shutdown(started, nil, nil, nil)))
}

func (s *Supervisor) beginRun(allowTasks bool) ([]Participant, []Task, error) {
	if s == nil {
		return nil, nil, fmt.Errorf("supervisor is nil")
	}
	if err := validateParticipants(s.participants); err != nil {
		return nil, nil, err
	}
	if err := validateTaskOwnership(s.participants, s.tasks); err != nil {
		return nil, nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !allowTasks && len(s.tasks) != 0 {
		return nil, nil, fmt.Errorf("supervisor operation does not accept long-running tasks")
	}
	if s.state != supervisorCreated {
		return nil, nil, fmt.Errorf("supervisor already run")
	}
	s.state = supervisorRunning
	s.setDiagnosticsLocked(StateStarting, false, nil, nil, nil, nil)
	return append([]Participant(nil), s.participants...), append([]Task(nil), s.tasks...), nil
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

type participantStop struct {
	participant Participant
	result      <-chan error
}

func (s *Supervisor) shutdown(participants []Participant, tasks []Task, results <-chan taskResult, completed map[string]struct{}) error {
	s.setDiagnostics(StateStopping, false, nil, nil, nil, nil)
	startedAt := time.Now()
	gracefulDeadline := startedAt.Add(s.shutdownTimeout - s.forceTimeout)
	finalDeadline := startedAt.Add(s.shutdownTimeout)
	gracefulCtx, cancelGraceful := context.WithDeadline(context.Background(), gracefulDeadline)
	defer cancelGraceful()

	var joined error
	pending := make(map[string]participantStop)
	order := make([]string, 0, len(participants))
	for index := len(participants) - 1; index >= 0; index-- {
		participant := participants[index]
		result := make(chan error, 1)
		go func() { result <- participant.Stop(gracefulCtx) }()
		select {
		case err := <-result:
			if err != nil {
				joined = errors.Join(joined, fmt.Errorf("stop participant %s: %w", participant.Name(), err))
				pending[participant.Name()] = participantStop{participant: participant, result: result}
				order = append(order, participant.Name())
			}
		case <-gracefulCtx.Done():
			joined = errors.Join(joined, fmt.Errorf("stop participant %s: %w", participant.Name(), gracefulCtx.Err()))
			pending[participant.Name()] = participantStop{participant: participant, result: result}
			order = append(order, participant.Name())
		}
	}

	forceCtx, cancelForce := context.WithDeadline(context.Background(), finalDeadline)
	defer cancelForce()
	forced := make([]string, 0)
	for _, name := range order {
		entry, exists := pending[name]
		if !exists {
			continue
		}
		force, ok := entry.participant.(ForceStopper)
		if !ok {
			continue
		}
		result := make(chan error, 1)
		go func() { result <- force.ForceStop(forceCtx) }()
		select {
		case err := <-result:
			if err != nil {
				joined = errors.Join(joined, fmt.Errorf("force stop participant %s: %w", name, err))
				continue
			}
			forced = append(forced, name)
			delete(pending, name)
			joined = errors.Join(joined, fmt.Errorf("participant %s required force stop", name))
		case <-forceCtx.Done():
			joined = errors.Join(joined, fmt.Errorf("force stop participant %s: %w", name, forceCtx.Err()))
		}
	}

	for name, entry := range pending {
		select {
		case err := <-entry.result:
			if err == nil {
				delete(pending, name)
			}
		default:
		}
	}
	pendingTasks, waitErr := waitTasksUntil(forceCtx, tasks, results, completed)
	joined = errors.Join(joined, waitErr)
	pendingParticipants := orderedPending(order, pending)
	s.setDiagnostics(StateStopping, false, joined, pendingParticipants, pendingTasks, forced)
	return joined
}

func waitTasksUntil(ctx context.Context, tasks []Task, results <-chan taskResult, completed map[string]struct{}) ([]string, error) {
	if len(tasks) == 0 {
		return nil, nil
	}
	if completed == nil {
		completed = make(map[string]struct{})
	}
	var joined error
	for len(completed) < len(tasks) {
		select {
		case result := <-results:
			completed[result.name] = struct{}{}
			if result.err != nil && !isOnlyCancellation(result.err) {
				joined = errors.Join(joined, result.err)
			}
		case <-ctx.Done():
			pending := pendingTaskNames(tasks, completed)
			return pending, errors.Join(joined, fmt.Errorf("wait supervisor tasks %v: %w", pending, ctx.Err()))
		}
	}
	return nil, joined
}

func pendingTaskNames(tasks []Task, completed map[string]struct{}) []string {
	pending := make([]string, 0, len(tasks)-len(completed))
	for _, task := range tasks {
		if _, done := completed[task.Name]; !done {
			pending = append(pending, task.Name)
		}
	}
	return pending
}

func orderedPending(order []string, pending map[string]participantStop) []string {
	result := make([]string, 0, len(pending))
	for _, name := range order {
		if _, exists := pending[name]; exists {
			result = append(result, name)
		}
	}
	return result
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

func (s *Supervisor) complete(err error) error {
	s.mu.Lock()
	s.state = supervisorFinished
	if err != nil {
		s.setDiagnosticsLocked(StateFailed, false, err, s.diagnostics.PendingParticipants, s.diagnostics.PendingTasks, s.diagnostics.ForcedParticipants)
	} else {
		s.setDiagnosticsLocked(StateStopped, false, nil, nil, nil, nil)
	}
	s.mu.Unlock()
	return err
}

// Snapshot 返回当前进程监督状态的独立副本。
func (s *Supervisor) Snapshot() Snapshot {
	if s == nil {
		return Snapshot{State: StateFailed, LastError: "supervisor is nil"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := s.diagnostics
	copy.PendingParticipants = append([]string(nil), copy.PendingParticipants...)
	copy.PendingTasks = append([]string(nil), copy.PendingTasks...)
	copy.ForcedParticipants = append([]string(nil), copy.ForcedParticipants...)
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

func (s *Supervisor) setDiagnostics(state ProcessState, ready bool, err error, participants, tasks, forced []string) {
	s.mu.Lock()
	s.setDiagnosticsLocked(state, ready, err, participants, tasks, forced)
	s.mu.Unlock()
}

func (s *Supervisor) setDiagnosticsLocked(state ProcessState, ready bool, err error, participants, tasks, forced []string) {
	s.diagnostics.State = state
	s.diagnostics.Ready = ready
	s.diagnostics.Since = time.Now()
	s.diagnostics.PendingParticipants = append([]string(nil), participants...)
	s.diagnostics.PendingTasks = append([]string(nil), tasks...)
	s.diagnostics.ForcedParticipants = append([]string(nil), forced...)
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
