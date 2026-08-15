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
	State         ProcessState
	Ready         bool
	Since         time.Time
	LastErrorType string
	Budget        ShutdownBudgetSnapshot
	Units         []UnitSnapshot
}

// UnexpectedCompletionError 标识长期任务在没有终止意图时提前成功返回。
type UnexpectedCompletionError struct{ Task string }

func (e *UnexpectedCompletionError) Error() string {
	return fmt.Sprintf("supervisor task %s completed unexpectedly", e.Task)
}

// IncompleteShutdownError 表示仍有非 clean terminal 责任时禁止返回成功。
type IncompleteShutdownError struct{ Owners []string }

func (e *IncompleteShutdownError) Error() string {
	return fmt.Sprintf("supervisor shutdown has incomplete owners %v", e.Owners)
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
	unitIndexes map[string]int
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
		diagnostics: Snapshot{
			State: StateNew, Since: time.Now(),
			Budget: ShutdownBudgetSnapshot{Phase: ShutdownNotStarted},
		},
		ready: make(chan struct{}),
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != supervisorCreated {
		return fmt.Errorf("supervisor already run")
	}
	if task.Name == "" {
		return fmt.Errorf("supervisor task name is required")
	}
	if err := validateOwnerName(task.Name); err != nil {
		return err
	}
	if task.Run == nil {
		return fmt.Errorf("supervisor task %q run function is nil", task.Name)
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
		s.transitionUnit(participant.Name(), UnitPhaseStart, UnitRunning, true, nil)
		if err := participant.Start(ctx); err != nil {
			wrapped := fmt.Errorf("start participant %s: %w", participant.Name(), err)
			s.transitionUnit(participant.Name(), UnitPhaseStart, UnitFailed, false, err)
			s.settleUnstartedUnits()
			s.setDiagnostics(StateDraining, false, wrapped)
			stopErr := s.shutdown(started, nil, nil, nil)
			return s.complete(errors.Join(normalizeCancellation(ctx, wrapped), stopErr))
		}
		started = append(started, participant)
		s.transitionUnit(participant.Name(), UnitPhaseReady, UnitReady, false, nil)
	}

	runnerCtx, cancelRunners := context.WithCancel(ctx)
	results := s.startTasks(runnerCtx, tasks)
	readyErr, completed := s.waitForReadiness(ctx, tasks, results)
	var runErr error
	if readyErr != nil {
		runErr = readyErr
	} else {
		s.setDiagnostics(StateRunning, true, nil)
		runErr, completed = s.waitForTermination(ctx, tasks, results, completed)
	}
	normalizedRunErr := normalizeCancellation(ctx, runErr)
	s.setDiagnostics(StateDraining, false, normalizedRunErr)
	cancelRunners()
	stopErr := s.shutdown(started, tasks, results, completed)
	return s.complete(errors.Join(normalizedRunErr, stopErr))
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
		s.transitionUnit(participant.Name(), UnitPhaseStart, UnitRunning, true, nil)
		if err := participant.Start(ctx); err != nil {
			wrapped := fmt.Errorf("start participant %s: %w", participant.Name(), err)
			s.transitionUnit(participant.Name(), UnitPhaseStart, UnitFailed, false, err)
			s.settleUnstartedUnits()
			s.setDiagnostics(StateDraining, false, wrapped)
			return s.complete(errors.Join(wrapped, s.shutdown(started, nil, nil, nil)))
		}
		started = append(started, participant)
		s.transitionUnit(participant.Name(), UnitPhaseReady, UnitReady, false, nil)
	}
	s.setDiagnostics(StateRunning, true, nil)
	operationErr := operation(ctx)
	s.setDiagnostics(StateDraining, false, operationErr)
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
	s.initializeUnitsLocked(s.participants, s.tasks)
	s.setDiagnosticsLocked(StateStarting, false, nil)
	return append([]Participant(nil), s.participants...), append([]Task(nil), s.tasks...), nil
}

type taskResult struct {
	name string
	err  error
}

func (s *Supervisor) startTasks(ctx context.Context, tasks []Task) <-chan taskResult {
	results := make(chan taskResult, len(tasks))
	for _, task := range tasks {
		task := task
		s.transitionUnit(task.Name, UnitPhaseRun, UnitRunning, true, nil)
		go func() {
			results <- taskResult{name: task.Name, err: task.Run(ctx)}
		}()
	}
	return results
}

func (s *Supervisor) waitForReadiness(ctx context.Context, tasks []Task, results <-chan taskResult) (error, map[string]struct{}) {
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
			if ctx.Err() != nil && (result.err == nil || isOnlyCancellation(result.err)) {
				s.transitionUnit(result.name, UnitPhaseRun, UnitStopped, false, nil)
				return ctx.Err(), completed
			}
			if result.err != nil {
				s.transitionUnit(result.name, UnitPhaseRun, UnitFailed, false, result.err)
				return fmt.Errorf("run task %s: %w", result.name, result.err), completed
			}
			unexpected := &UnexpectedCompletionError{Task: result.name}
			s.transitionUnit(result.name, UnitPhaseRun, UnitFailed, false, unexpected)
			return unexpected, completed
		case <-task.Ready:
			s.transitionUnit(task.Name, UnitPhaseRun, UnitReady, false, nil)
		}
	}
	return nil, completed
}

func (s *Supervisor) waitForTermination(ctx context.Context, tasks []Task, results <-chan taskResult, completed map[string]struct{}) (error, map[string]struct{}) {
	if len(tasks) == 0 {
		<-ctx.Done()
		return ctx.Err(), completed
	}
	select {
	case <-ctx.Done():
		return ctx.Err(), completed
	case result := <-results:
		completed[result.name] = struct{}{}
		if ctx.Err() != nil && (result.err == nil || isOnlyCancellation(result.err)) {
			s.transitionUnit(result.name, UnitPhaseRun, UnitStopped, false, nil)
			return ctx.Err(), completed
		}
		if result.err != nil {
			s.transitionUnit(result.name, UnitPhaseRun, UnitFailed, false, result.err)
			return fmt.Errorf("run task %s: %w", result.name, result.err), completed
		}
		unexpected := &UnexpectedCompletionError{Task: result.name}
		s.transitionUnit(result.name, UnitPhaseRun, UnitFailed, false, unexpected)
		return unexpected, completed
	}
}

type participantStop struct {
	participant Participant
	result      <-chan error
	phase       UnitPhase
}

func (s *Supervisor) shutdown(participants []Participant, tasks []Task, results <-chan taskResult, completed map[string]struct{}) error {
	s.setDiagnostics(StateStopping, false, nil)
	startedAt := time.Now()
	gracefulDeadline := startedAt.Add(s.shutdownTimeout - s.forceTimeout)
	finalDeadline := startedAt.Add(s.shutdownTimeout)
	s.mu.Lock()
	s.diagnostics.Budget = ShutdownBudgetSnapshot{
		Phase: ShutdownGraceful, StartedAt: startedAt,
		GracefulDeadline: gracefulDeadline, FinalDeadline: finalDeadline,
	}
	s.mu.Unlock()
	gracefulCtx, cancelGraceful := context.WithDeadline(context.Background(), gracefulDeadline)
	defer cancelGraceful()

	var joined error
	pending := make(map[string]participantStop)
	order := make([]string, 0, len(participants))
	forceCandidates := make(map[string]Participant)
	for index := len(participants) - 1; index >= 0; index-- {
		participant := participants[index]
		name := participant.Name()
		order = append(order, name)
		s.transitionUnit(name, UnitPhaseStop, UnitRunning, true, nil)
		result := make(chan error, 1)
		go func() { result <- participant.Stop(gracefulCtx) }()
		select {
		case err := <-result:
			if err != nil {
				wrapped := fmt.Errorf("stop participant %s: %w", name, err)
				joined = errors.Join(joined, wrapped)
				s.transitionUnit(name, UnitPhaseStop, UnitFailed, false, err)
				if _, ok := participant.(ForceStopper); ok {
					forceCandidates[name] = participant
				}
			} else {
				s.transitionUnit(name, UnitPhaseStop, UnitStopped, false, nil)
			}
		case <-gracefulCtx.Done():
			wrapped := fmt.Errorf("stop participant %s: %w", name, gracefulCtx.Err())
			joined = errors.Join(joined, wrapped)
			s.transitionUnit(name, UnitPhaseStop, UnitPending, false, gracefulCtx.Err())
			pending[name] = participantStop{participant: participant, result: result, phase: UnitPhaseStop}
			if _, ok := participant.(ForceStopper); ok {
				forceCandidates[name] = participant
			}
		}
	}

	forceCtx, cancelForce := context.WithDeadline(context.Background(), finalDeadline)
	defer cancelForce()
	s.mu.Lock()
	s.diagnostics.Budget.Phase = ShutdownForce
	s.mu.Unlock()
	for _, name := range order {
		participant, exists := forceCandidates[name]
		if !exists {
			continue
		}
		if entry, isPending := pending[name]; isPending {
			select {
			case err := <-entry.result:
				delete(pending, name)
				if err == nil {
					s.transitionUnit(name, UnitPhaseStop, UnitStopped, false, nil)
					continue
				}
				wrapped := fmt.Errorf("stop participant %s: %w", name, err)
				joined = errors.Join(joined, wrapped)
				s.transitionUnit(name, UnitPhaseStop, UnitFailed, false, err)
			default:
			}
		}
		force := participant.(ForceStopper)
		s.transitionUnit(name, UnitPhaseForce, UnitRunning, true, nil)
		result := make(chan error, 1)
		go func() { result <- force.ForceStop(forceCtx) }()
		select {
		case err := <-result:
			if err != nil {
				wrapped := fmt.Errorf("force stop participant %s: %w", name, err)
				joined = errors.Join(joined, wrapped)
				s.transitionUnit(name, UnitPhaseForce, UnitFailed, false, err)
				delete(pending, name)
				continue
			}
			s.transitionUnit(name, UnitPhaseForce, UnitForced, false, nil)
			delete(pending, name)
			joined = errors.Join(joined, fmt.Errorf("participant %s required force stop", name))
		case <-forceCtx.Done():
			wrapped := fmt.Errorf("force stop participant %s: %w", name, forceCtx.Err())
			joined = errors.Join(joined, wrapped)
			s.transitionUnit(name, UnitPhaseForce, UnitPending, false, forceCtx.Err())
			pending[name] = participantStop{participant: participant, result: result, phase: UnitPhaseForce}
		}
	}

	for _, name := range order {
		entry, exists := pending[name]
		if !exists {
			continue
		}
		select {
		case err := <-entry.result:
			delete(pending, name)
			if err == nil {
				state := UnitStopped
				if entry.phase == UnitPhaseForce {
					state = UnitForced
				}
				s.transitionUnit(name, entry.phase, state, false, nil)
			} else {
				wrapped := fmt.Errorf("%s participant %s: %w", entry.phase, name, err)
				joined = errors.Join(joined, wrapped)
				s.transitionUnit(name, entry.phase, UnitFailed, false, err)
			}
		case <-forceCtx.Done():
		}
	}
	waitErr := s.waitTasksUntil(forceCtx, tasks, results, completed)
	joined = errors.Join(joined, waitErr)
	s.mu.Lock()
	s.diagnostics.Budget.Phase = ShutdownComplete
	s.diagnostics.Budget.Exhausted = errors.Is(forceCtx.Err(), context.DeadlineExceeded)
	s.mu.Unlock()
	s.setDiagnostics(StateStopping, false, joined)
	return joined
}

func (s *Supervisor) waitTasksUntil(ctx context.Context, tasks []Task, results <-chan taskResult, completed map[string]struct{}) error {
	if len(tasks) == 0 {
		return nil
	}
	if completed == nil {
		completed = make(map[string]struct{})
	}
	for _, task := range tasks {
		if _, done := completed[task.Name]; !done {
			s.transitionUnit(task.Name, UnitPhaseStop, UnitPending, false, nil)
		}
	}
	var joined error
	for len(completed) < len(tasks) {
		select {
		case result := <-results:
			completed[result.name] = struct{}{}
			if result.err != nil && !isOnlyCancellation(result.err) {
				joined = errors.Join(joined, fmt.Errorf("run task %s: %w", result.name, result.err))
				s.transitionUnit(result.name, UnitPhaseStop, UnitFailed, false, result.err)
			} else {
				s.transitionUnit(result.name, UnitPhaseStop, UnitStopped, false, nil)
			}
		case <-ctx.Done():
			pending := pendingTaskNames(tasks, completed)
			for _, name := range pending {
				s.transitionUnit(name, UnitPhaseStop, UnitPending, false, ctx.Err())
			}
			return errors.Join(joined, fmt.Errorf("wait supervisor tasks %v: %w", pending, ctx.Err()))
		}
	}
	return joined
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

func validateParticipants(participants []Participant) error {
	seen := make(map[string]struct{}, len(participants))
	for index, participant := range participants {
		if participant == nil {
			return fmt.Errorf("supervisor participant %d is nil", index)
		}
		name := participant.Name()
		if err := validateOwnerName(name); err != nil {
			return fmt.Errorf("supervisor participant %d: %w", index, err)
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
		if err := validateOwnerName(task.Name); err != nil {
			return fmt.Errorf("supervisor task %d: %w", index, err)
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

func validateOwnerName(name string) error {
	if name == "" {
		return fmt.Errorf("supervisor owner name is required")
	}
	if len(name) > 128 {
		return fmt.Errorf("supervisor owner name exceeds 128 bytes")
	}
	for index := 0; index < len(name); index++ {
		character := name[index]
		valid := character >= 'a' && character <= 'z' || character >= '0' && character <= '9'
		if index > 0 {
			valid = valid || character == '.' || character == '_' || character == '-'
		}
		if !valid {
			return fmt.Errorf("supervisor owner name must use lowercase letters, digits, dot, underscore or hyphen")
		}
	}
	return nil
}

func (s *Supervisor) complete(err error) error {
	s.mu.Lock()
	s.state = supervisorFinished
	if err == nil && !unitsClean(s.diagnostics.Units) {
		owners := make([]string, 0, len(s.diagnostics.Units))
		for _, unit := range s.diagnostics.Units {
			if unit.State != UnitStopped {
				owners = append(owners, unit.Owner)
			}
		}
		err = &IncompleteShutdownError{Owners: owners}
	}
	if err != nil {
		s.setDiagnosticsLocked(StateFailed, false, err)
	} else {
		s.setDiagnosticsLocked(StateStopped, false, nil)
	}
	s.mu.Unlock()
	return err
}

// Snapshot 返回当前进程监督状态的独立副本。
func (s *Supervisor) Snapshot() Snapshot {
	if s == nil {
		return Snapshot{State: StateFailed, Budget: ShutdownBudgetSnapshot{Phase: ShutdownNotStarted}}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := s.diagnostics
	copy.Units = cloneUnits(copy.Units)
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

func (s *Supervisor) setDiagnostics(state ProcessState, ready bool, err error) {
	s.mu.Lock()
	s.setDiagnosticsLocked(state, ready, err)
	s.mu.Unlock()
}

func (s *Supervisor) setDiagnosticsLocked(state ProcessState, ready bool, err error) {
	s.diagnostics.State = state
	s.diagnostics.Ready = ready
	s.diagnostics.Since = time.Now()
	if err != nil {
		s.diagnostics.LastErrorType = fmt.Sprintf("%T", err)
	} else if state == StateRunning || state == StateStopped {
		s.diagnostics.LastErrorType = ""
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
