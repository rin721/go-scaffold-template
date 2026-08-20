// Package schedule 封装 generation-owned gocron 引擎、任务运行治理和分布式执行权状态机。
package schedule

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	pkgclock "github.com/rin721/go-scaffold-template/pkg/clock"
	"github.com/rin721/go-scaffold-template/pkg/coordination"
	pkgexecution "github.com/rin721/go-scaffold-template/pkg/execution"
	"github.com/rin721/go-scaffold-template/pkg/health"
	"github.com/rin721/go-scaffold-template/pkg/logger"
	pkgobservability "github.com/rin721/go-scaffold-template/pkg/observability"
	pkgschedule "github.com/rin721/go-scaffold-template/pkg/schedule"
)

const scheduledWorkKind = "scheduled-task"

// Dependencies 是 Scheduler Component 的显式项目能力输入。
type Dependencies struct {
	ApplicationName string
	Generation      uint64
	Logger          logger.Logger
	Clock           pkgclock.Clock
	Execution       pkgexecution.OperationExecutor
	Coordination    coordination.Manager
	Telemetry       pkgobservability.Telemetry
	Bindings        []pkgschedule.Binding
	ReportFailure   func(error)
	engineFactory   func(Config, pkgclock.Clock) (triggerEngine, error)
}

// Access 只向 Application Generation 暴露受控准入、停止和诊断，不向业务模块暴露。
type Access interface {
	Activate(context.Context) error
	Deactivate(context.Context) error
	Diagnostics(context.Context) (pkgschedule.Diagnostics, error)
	Health(context.Context) (health.Result, error)
}

type access struct{ delegate app.Lease[*resource] }

// Definition 构造由 Application Generation 持有的 Scheduler Component 声明。
func Definition(dependencies Dependencies) (app.Definition[Access], error) {
	source, err := app.Configured(ConfigPath, decode, defaults{})
	if err != nil {
		return app.Definition[Access]{}, err
	}
	return app.ManagedConfigured(
		ID, source, app.FixedDependencies(dependencies), build, app.Leased(newAccess), app.RestartRequired,
		app.WithReady(ready), app.WithTerminalFinalizer(stop),
	)
}

func newAccess(delegate app.Lease[*resource]) (Access, error) {
	if delegate == nil {
		return nil, fmt.Errorf("scheduler lease is nil")
	}
	return &access{delegate: delegate}, nil
}

func (a *access) Activate(ctx context.Context) error {
	return a.delegate.Use(ctx, func(current *resource) error { return current.activate() })
}

func (a *access) Deactivate(ctx context.Context) error {
	return a.delegate.Use(ctx, func(current *resource) error {
		current.deactivate()
		return nil
	})
}

func (a *access) Diagnostics(ctx context.Context) (pkgschedule.Diagnostics, error) {
	var diagnostics pkgschedule.Diagnostics
	err := a.delegate.Use(ctx, func(current *resource) error {
		diagnostics = current.diagnostics()
		return nil
	})
	return diagnostics, err
}

func (a *access) Health(ctx context.Context) (health.Result, error) {
	var result health.Result
	err := a.delegate.Use(ctx, func(current *resource) error {
		result = current.health()
		return nil
	})
	return result, err
}

type resource struct {
	config       Config
	dependencies Dependencies
	engine       triggerEngine
	ctx          context.Context
	cancel       context.CancelFunc

	mu          sync.Mutex
	active      bool
	deactivated bool
	tasks       []*taskRuntime
	globalSlots chan struct{}
	wg          sync.WaitGroup
}

type taskRuntime struct {
	owner        *resource
	binding      pkgschedule.Binding
	coordination pkgschedule.CoordinationPolicy
	enabled      bool
	slots        chan struct{}
	queue        chan struct{}
	fatalOnce    sync.Once

	mu        sync.Mutex
	jobID     engineJobID
	jobCancel context.CancelFunc
	snapshot  pkgschedule.TaskSnapshot
}

func build(ctx context.Context, cfg Config, dependencies Dependencies) (*resource, error) {
	if ctx == nil {
		return nil, app.ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(dependencies.ApplicationName) == "" || dependencies.Generation == 0 ||
		dependencies.Logger == nil || dependencies.Clock == nil || dependencies.Execution == nil || dependencies.Coordination == nil ||
		dependencies.Telemetry == nil || dependencies.ReportFailure == nil {
		return nil, fmt.Errorf("scheduler dependencies are incomplete")
	}
	bindings := append([]pkgschedule.Binding(nil), dependencies.Bindings...)
	sort.Slice(bindings, func(left, right int) bool { return bindings[left].ID() < bindings[right].ID() })
	seen := make(map[pkgschedule.TaskID]struct{}, len(bindings))
	for _, binding := range bindings {
		if err := binding.Validate(); err != nil {
			return nil, fmt.Errorf("validate schedule %s: %w", binding.ID(), err)
		}
		if _, exists := seen[binding.ID()]; exists {
			return nil, fmt.Errorf("schedule task %q is duplicated", binding.ID())
		}
		seen[binding.ID()] = struct{}{}
	}
	for taskID := range cfg.Tasks {
		if _, exists := seen[pkgschedule.TaskID(taskID)]; !exists {
			return nil, fmt.Errorf("scheduler task override %q has no module binding", taskID)
		}
	}
	engineFactory := dependencies.engineFactory
	if engineFactory == nil {
		engineFactory = newGocronEngine
	}
	engine, err := engineFactory(cfg, dependencies.Clock)
	if err != nil {
		return nil, err
	}
	ownedCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	result := &resource{
		config: cfg, dependencies: dependencies, engine: engine, ctx: ownedCtx, cancel: cancel,
		globalSlots: make(chan struct{}, cfg.MaxConcurrency),
	}
	abort := func(cause error) (*resource, error) {
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer shutdownCancel()
		return nil, errors.Join(cause, engine.Shutdown(shutdownCtx))
	}
	for _, binding := range bindings {
		coordinationPolicy, enabled, resolveErr := resolveTask(cfg, binding)
		if resolveErr != nil {
			return abort(resolveErr)
		}
		concurrency := binding.Concurrency()
		task := &taskRuntime{
			owner: result, binding: binding, coordination: coordinationPolicy, enabled: enabled,
			slots: make(chan struct{}, concurrency.MaxConcurrent()),
			snapshot: pkgschedule.TaskSnapshot{
				ID: binding.ID(), Trigger: binding.Trigger().Kind(), Coordination: coordinationPolicy.Mode(),
				Unavailable: coordinationPolicy.OnUnavailable(), State: pkgschedule.StateDisabled, Ready: true,
			},
		}
		if concurrency.Congestion() == pkgschedule.CongestionWait {
			task.queue = make(chan struct{}, concurrency.QueueLimit())
		}
		if err := engine.Validate(binding, cfg.Timezone); err != nil {
			return abort(fmt.Errorf("validate scheduler task %s: %w", binding.ID(), err))
		}
		if enabled {
			task.setState(pkgschedule.StatePrepared, "")
			if !coordinationPolicy.Distributed() {
				if err := task.installJob(ownedCtx); err != nil {
					return abort(err)
				}
			}
		}
		result.tasks = append(result.tasks, task)
	}
	return result, nil
}

func resolveTask(cfg Config, binding pkgschedule.Binding) (pkgschedule.CoordinationPolicy, bool, error) {
	enabled := cfg.Enabled
	policy := binding.Coordination()
	override, exists := cfg.Tasks[string(binding.ID())]
	if !exists {
		return policy, enabled, nil
	}
	if override.Enabled != nil {
		enabled = cfg.Enabled && *override.Enabled
	}
	if override.UnavailablePolicy == "" {
		return policy, enabled, nil
	}
	if !policy.Distributed() {
		return pkgschedule.CoordinationPolicy{}, false, fmt.Errorf("local scheduler task %q cannot override coordination unavailable policy", binding.ID())
	}
	strict := policy.Mode() == pkgschedule.CoordinationDistributedStrict
	resolved, err := pkgschedule.DistributedSingleton(strict, override.UnavailablePolicy)
	if err != nil {
		return pkgschedule.CoordinationPolicy{}, false, fmt.Errorf("resolve scheduler task %s coordination: %w", binding.ID(), err)
	}
	return resolved, enabled, nil
}

func ready(ctx context.Context, current *resource) error {
	if ctx == nil {
		return app.ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if current == nil || current.engine == nil || current.cancel == nil {
		return fmt.Errorf("scheduler resource is incomplete")
	}
	return nil
}

func (r *resource) activate() error {
	r.mu.Lock()
	if r.deactivated {
		r.mu.Unlock()
		return fmt.Errorf("scheduler generation %d is deactivated", r.dependencies.Generation)
	}
	if r.active {
		r.mu.Unlock()
		return nil
	}
	r.active = true
	r.mu.Unlock()
	if !r.config.Enabled {
		return nil
	}
	r.engine.Start()
	for _, task := range r.tasks {
		if !task.enabled {
			continue
		}
		if task.coordination.Distributed() {
			task.setState(pkgschedule.StateContending, "")
			r.wg.Add(1)
			// #nosec G118 -- resource 拥有 cancel 与 WaitGroup，terminal finalizer 有界等待退出。
			go func(current *taskRuntime) {
				defer r.wg.Done()
				r.coordinate(current)
			}(task)
			continue
		}
		task.setState(pkgschedule.StateLocal, "")
	}
	r.dependencies.Logger.Info("application scheduler started",
		logger.String("owner", "scheduler"), logger.String("phase", "active"),
		logger.Any("generation", r.dependencies.Generation), logger.Any("tasks", len(r.tasks)),
	)
	return nil
}

func (r *resource) deactivate() {
	r.mu.Lock()
	if r.deactivated {
		r.mu.Unlock()
		return
	}
	r.deactivated = true
	r.active = false
	r.cancel()
	r.mu.Unlock()
	for _, task := range r.tasks {
		if task.enabled {
			task.setState(pkgschedule.StateStopping, "")
		}
	}
	if r.config.Enabled {
		r.dependencies.Logger.Info("application scheduler draining",
			logger.String("owner", "scheduler"), logger.String("phase", "drain"),
			logger.Any("generation", r.dependencies.Generation),
		)
	}
}

func stop(ctx context.Context, current *resource) error {
	if current == nil {
		return nil
	}
	if ctx == nil {
		return app.ErrNilContext
	}
	current.deactivate()
	waitDone := make(chan struct{})
	go func() {
		current.wg.Wait()
		close(waitDone)
	}()
	var joined error
	select {
	case <-waitDone:
	case <-ctx.Done():
		joined = errors.Join(joined, ctx.Err())
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, current.config.ShutdownTimeout)
	defer cancel()
	joined = errors.Join(joined, current.engine.Shutdown(shutdownCtx))
	if joined == nil && current.config.Enabled {
		current.dependencies.Logger.Info("application scheduler stopped",
			logger.String("owner", "scheduler"), logger.String("phase", "stopped"),
			logger.Any("generation", current.dependencies.Generation),
		)
	}
	return joined
}

func (r *resource) coordinate(task *taskRuntime) {
	attempt := 0
	for {
		if err := r.ctx.Err(); err != nil {
			task.removeJob()
			return
		}
		acquireCtx, cancel := context.WithTimeout(r.ctx, r.config.Coordination.AcquireTimeout)
		lease, err := r.dependencies.Coordination.Acquire(acquireCtx, r.coordinationKey(task.binding.ID()), coordination.LeaseOptions{TTL: r.config.Coordination.LeaseTTL})
		cancel()
		if err == nil {
			attempt = 0
			if removeErr := task.removeJob(); removeErr != nil {
				task.fail(fmt.Errorf("remove weakened scheduler job: %w", removeErr))
				return
			}
			if installErr := task.installJob(r.ctx); installErr != nil {
				task.fail(installErr)
				return
			}
			task.setState(pkgschedule.StateLeader, "")
			renewErr := r.renew(task, lease)
			removeErr := task.removeJob()
			releaseCtx, releaseCancel := context.WithTimeout(context.WithoutCancel(r.ctx), r.config.Coordination.AcquireTimeout)
			releaseErr := lease.Release(releaseCtx)
			releaseCancel()
			if r.ctx.Err() != nil {
				return
			}
			if removeErr != nil {
				task.fail(fmt.Errorf("remove scheduler leader job: %w", removeErr))
				return
			}
			if releaseErr != nil && !errors.Is(releaseErr, coordination.ErrLeaseLost) {
				r.logCoordination("release", task, releaseErr)
			}
			if renewErr != nil {
				r.logCoordination("lost", task, renewErr)
			}
			task.setState(pkgschedule.StateContending, errorType(renewErr))
			if !waitFor(r.ctx, retryDelay(task.binding.ID(), attempt, r.config.Coordination)) {
				return
			}
			attempt++
			continue
		}
		if errors.Is(err, coordination.ErrNotAcquired) {
			attempt = 0
			if removeErr := task.removeJob(); removeErr != nil {
				task.fail(fmt.Errorf("remove weakened scheduler job while standing by: %w", removeErr))
				return
			}
			task.setState(pkgschedule.StateStandby, "")
			if !waitFor(r.ctx, r.config.Coordination.RetryMin) {
				return
			}
			continue
		}
		if r.ctx.Err() != nil {
			return
		}
		if !r.handleUnavailable(task, err) {
			return
		}
		if !waitFor(r.ctx, retryDelay(task.binding.ID(), attempt, r.config.Coordination)) {
			return
		}
		attempt++
	}
}

func (r *resource) renew(task *taskRuntime, lease coordination.Lease) error {
	ticker := time.NewTicker(r.config.Coordination.RenewInterval)
	defer ticker.Stop()
	for {
		select {
		case <-r.ctx.Done():
			return r.ctx.Err()
		case <-ticker.C:
			renewCtx, cancel := context.WithTimeout(r.ctx, r.config.Coordination.AcquireTimeout)
			err := lease.Renew(renewCtx, coordination.LeaseOptions{TTL: r.config.Coordination.LeaseTTL})
			cancel()
			if err != nil {
				return err
			}
			task.setState(pkgschedule.StateLeader, "")
		}
	}
}

func (r *resource) handleUnavailable(task *taskRuntime, err error) bool {
	errorName := errorType(err)
	switch task.coordination.OnUnavailable() {
	case pkgschedule.UnavailableSkip:
		if removeErr := task.removeJob(); removeErr != nil {
			task.fail(fmt.Errorf("remove scheduler job while coordination is unavailable: %w", removeErr))
			return false
		}
		task.setState(pkgschedule.StateDegraded, errorName)
		r.logCoordination("skip", task, err)
		return true
	case pkgschedule.UnavailablePause:
		if removeErr := task.removeJob(); removeErr != nil {
			task.fail(fmt.Errorf("remove scheduler job while coordination is unavailable: %w", removeErr))
			return false
		}
		task.setState(pkgschedule.StatePaused, errorName)
		r.logCoordination("pause", task, err)
		return true
	case pkgschedule.UnavailableFail:
		if removeErr := task.removeJob(); removeErr != nil {
			err = errors.Join(err, removeErr)
		}
		task.fail(fmt.Errorf("scheduler coordination unavailable: %w", err))
		return false
	case pkgschedule.UnavailableLocal:
		if installErr := task.installJob(r.ctx); installErr != nil {
			task.fail(installErr)
			return false
		}
		task.setState(pkgschedule.StateWeakened, errorName)
		r.logCoordination("weaken", task, err)
		return true
	default:
		task.fail(fmt.Errorf("unsupported unavailable policy %q", task.coordination.OnUnavailable()))
		return false
	}
}

func (r *resource) coordinationKey(taskID pkgschedule.TaskID) coordination.Key {
	application := strings.NewReplacer(":", "-", " ", "-").Replace(strings.TrimSpace(r.dependencies.ApplicationName))
	return coordination.Key(r.config.Coordination.Namespace + ":" + application + ":" + string(taskID))
}

func (r *resource) logCoordination(phase string, task *taskRuntime, err error) {
	if err == nil {
		return
	}
	r.dependencies.Logger.Warn("scheduled task coordination state changed",
		logger.String("owner", "scheduler"), logger.String("phase", phase),
		logger.String("task", string(task.binding.ID())), logger.Any("generation", r.dependencies.Generation),
		logger.String("error_type", errorType(err)),
	)
}

func (r *resource) diagnostics() pkgschedule.Diagnostics {
	result := pkgschedule.Diagnostics{
		Enabled: r.config.Enabled, Ready: true, Generation: r.dependencies.Generation,
		Tasks: make([]pkgschedule.TaskSnapshot, 0, len(r.tasks)),
	}
	for _, task := range r.tasks {
		snapshot := task.copySnapshot()
		result.Tasks = append(result.Tasks, snapshot)
		if !snapshot.Ready {
			result.Ready = false
		}
		if snapshot.State == pkgschedule.StateDegraded || snapshot.State == pkgschedule.StateWeakened {
			result.Degraded = true
		}
	}
	return result
}

func (r *resource) health() health.Result {
	diagnostics := r.diagnostics()
	result := health.Result{Name: string(ID), Kind: health.KindReadiness, Status: health.StatusPass, Message: "scheduler ready"}
	if !diagnostics.Ready {
		result.Status = health.StatusFail
		result.Message = "scheduler task policy paused or failed"
		return result
	}
	if diagnostics.Degraded {
		result.Status = health.StatusWarn
		result.Message = "scheduler running with degraded task policy"
	}
	return result
}

func (t *taskRuntime) installJob(parent context.Context) error {
	t.mu.Lock()
	if t.jobID != "" {
		t.mu.Unlock()
		return nil
	}
	jobCtx, cancel := context.WithCancel(parent)
	id, err := t.owner.engine.Add(t.binding, t.owner.config.Timezone, jobCtx, t.invoke)
	if err != nil {
		cancel()
		t.mu.Unlock()
		return fmt.Errorf("install scheduler task %s: %w", t.binding.ID(), err)
	}
	t.jobID = id
	t.jobCancel = cancel
	t.mu.Unlock()
	return nil
}

func (t *taskRuntime) removeJob() error {
	t.mu.Lock()
	id, cancel := t.jobID, t.jobCancel
	t.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if id == "" {
		return nil
	}
	if err := t.owner.engine.Remove(id); err != nil {
		return err
	}
	t.mu.Lock()
	if t.jobID == id {
		t.jobID, t.jobCancel = "", nil
	}
	t.mu.Unlock()
	return nil
}

func (t *taskRuntime) invoke(ctx context.Context) (runErr error) {
	if ctx == nil || ctx.Err() != nil || t.owner.ctx.Err() != nil {
		return nil
	}
	now := t.owner.dependencies.Clock.Now()
	t.mu.Lock()
	t.snapshot.LastScheduledAt = now
	t.mu.Unlock()
	release, admitted := t.acquire(ctx)
	if !admitted {
		t.mu.Lock()
		t.snapshot.Skipped++
		t.mu.Unlock()
		return nil
	}
	defer release()
	t.mu.Lock()
	t.snapshot.Active++
	t.snapshot.Runs++
	t.snapshot.LastStartedAt = now
	t.mu.Unlock()
	defer func() {
		if recover() != nil {
			runErr = taskPanicError{}
		}
		t.mu.Lock()
		t.snapshot.Active--
		t.snapshot.LastCompletedAt = t.owner.dependencies.Clock.Now()
		t.snapshot.LastErrorType = errorType(runErr)
		t.mu.Unlock()
	}()
	t.owner.dependencies.Logger.Debug("scheduled task started",
		logger.String("owner", "scheduler"), logger.String("phase", "execute"),
		logger.String("task", string(t.binding.ID())), logger.Any("generation", t.owner.dependencies.Generation),
	)
	runErr = t.execute(ctx, now)
	if runErr != nil && !errors.Is(runErr, context.Canceled) && !errors.Is(runErr, context.DeadlineExceeded) {
		t.owner.dependencies.Logger.Warn("scheduled task failed",
			logger.String("owner", "scheduler"), logger.String("phase", "execute"),
			logger.String("task", string(t.binding.ID())), logger.Any("generation", t.owner.dependencies.Generation),
			logger.String("error_type", errorType(runErr)),
		)
	} else if runErr == nil {
		t.owner.dependencies.Logger.Debug("scheduled task completed",
			logger.String("owner", "scheduler"), logger.String("phase", "execute"),
			logger.String("task", string(t.binding.ID())), logger.Any("generation", t.owner.dependencies.Generation),
		)
	}
	return runErr
}

func (t *taskRuntime) execute(ctx context.Context, scheduledAt time.Time) (runErr error) {
	defer func() {
		if recover() != nil {
			runErr = taskPanicError{}
		}
	}()
	return t.owner.dependencies.Telemetry.Observe(ctx, pkgobservability.Work{
		Name: "schedule." + string(t.binding.ID()), Kind: scheduledWorkKind,
	}, func(observed context.Context) error {
		observed = pkgexecution.WithTrace(observed, pkgobservability.TraceIDFrom(observed))
		_, err := t.owner.dependencies.Execution.Execute(observed, pkgexecution.Execution{
			Key:          pkgexecution.Key(occurrenceKey(t.binding, scheduledAt)),
			LeaseTTL:     t.owner.config.OccurrenceRetention,
			RetentionTTL: t.owner.config.OccurrenceRetention,
			Trigger:      "schedule." + string(t.binding.Trigger().Kind()), PolicyName: t.binding.ExecutionPolicy(),
			Operation: func(operationCtx context.Context) (any, error) {
				return nil, t.binding.Run(operationCtx)
			},
		})
		return err
	})
}

func (t *taskRuntime) acquire(ctx context.Context) (func(), bool) {
	acquireImmediately := func() (func(), bool) {
		select {
		case t.slots <- struct{}{}:
		default:
			return func() {}, false
		}
		select {
		case t.owner.globalSlots <- struct{}{}:
			return func() {
				<-t.owner.globalSlots
				<-t.slots
			}, true
		default:
			<-t.slots
			return func() {}, false
		}
	}
	if release, acquired := acquireImmediately(); acquired {
		return release, true
	}
	if t.binding.Concurrency().Congestion() == pkgschedule.CongestionSkip {
		return func() {}, false
	}
	select {
	case t.queue <- struct{}{}:
		t.mu.Lock()
		t.snapshot.Queued++
		t.mu.Unlock()
	default:
		return func() {}, false
	}
	dequeue := func() {
		<-t.queue
		t.mu.Lock()
		t.snapshot.Queued--
		t.mu.Unlock()
	}
	select {
	case t.slots <- struct{}{}:
	case <-ctx.Done():
		dequeue()
		return func() {}, false
	case <-t.owner.ctx.Done():
		dequeue()
		return func() {}, false
	}
	select {
	case t.owner.globalSlots <- struct{}{}:
		dequeue()
		return func() {
			<-t.owner.globalSlots
			<-t.slots
		}, true
	case <-ctx.Done():
		<-t.slots
		dequeue()
		return func() {}, false
	case <-t.owner.ctx.Done():
		<-t.slots
		dequeue()
		return func() {}, false
	}
}

func (t *taskRuntime) setState(state pkgschedule.RuntimeState, errorName string) {
	t.mu.Lock()
	t.snapshot.State = state
	t.snapshot.LastErrorType = errorName
	switch state {
	case pkgschedule.StatePaused, pkgschedule.StateFailed, pkgschedule.StateStopping, pkgschedule.StatePrepared:
		t.snapshot.Ready = false
	default:
		t.snapshot.Ready = true
	}
	t.mu.Unlock()
}

func (t *taskRuntime) fail(err error) {
	t.fatalOnce.Do(func() {
		t.setState(pkgschedule.StateFailed, errorType(err))
		t.owner.dependencies.Logger.Error("scheduled task entered fatal state",
			logger.String("owner", "scheduler"), logger.String("phase", "fatal"),
			logger.String("task", string(t.binding.ID())), logger.Any("generation", t.owner.dependencies.Generation),
			logger.String("error_type", errorType(err)),
		)
		t.owner.dependencies.ReportFailure(fmt.Errorf("scheduler task %s: %w", t.binding.ID(), err))
	})
}

func (t *taskRuntime) copySnapshot() pkgschedule.TaskSnapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.snapshot
}

func occurrenceKey(binding pkgschedule.Binding, started time.Time) string {
	resolved := started.UTC()
	if binding.Trigger().Kind() == pkgschedule.TriggerCron {
		_, _, withSeconds, _ := binding.Trigger().CronValues()
		if withSeconds {
			resolved = resolved.Truncate(time.Second)
		} else {
			resolved = resolved.Truncate(time.Minute)
		}
	}
	return "schedule:" + string(binding.ID()) + ":" + resolved.Format("20060102T150405.000000000Z")
}

func retryDelay(taskID pkgschedule.TaskID, attempt int, cfg CoordinationConfig) time.Duration {
	delay := cfg.RetryMin
	for index := 0; index < attempt && delay < cfg.RetryMax/2; index++ {
		delay *= 2
	}
	if delay > cfg.RetryMax {
		delay = cfg.RetryMax
	}
	window := delay / 5
	if window <= 0 {
		return delay
	}
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(fmt.Sprintf("%s:%d", taskID, attempt)))
	// 在基准值的 90%-110% 内产生稳定抖动，避免实例同频争抢且便于测试。
	offset := time.Duration(hash.Sum64()%uint64(window*2+1)) - window
	resolved := delay + offset/2
	if resolved > cfg.RetryMax {
		return cfg.RetryMax
	}
	if resolved <= 0 {
		return cfg.RetryMin
	}
	return resolved
}

func waitFor(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func errorType(err error) string {
	if err == nil {
		return ""
	}
	classifications := []struct {
		target error
		name   string
	}{
		{context.Canceled, "context_canceled"},
		{context.DeadlineExceeded, "context_deadline_exceeded"},
		{coordination.ErrNotAcquired, "coordination_not_acquired"},
		{coordination.ErrUnavailable, "coordination_unavailable"},
		{coordination.ErrLeaseLost, "coordination_lease_lost"},
		{pkgexecution.ErrBackend, "execution_backend_unavailable"},
		{pkgexecution.ErrAlreadyRunning, "execution_already_running"},
		{pkgexecution.ErrRetryExhausted, "execution_retry_exhausted"},
	}
	for _, classification := range classifications {
		if errors.Is(err, classification.target) {
			return classification.name
		}
	}
	var panicErr taskPanicError
	if errors.As(err, &panicErr) {
		return "task_panic"
	}
	return reflect.TypeOf(err).String()
}

type taskPanicError struct{}

func (taskPanicError) Error() string { return "scheduled task panicked" }

var _ Access = (*access)(nil)
