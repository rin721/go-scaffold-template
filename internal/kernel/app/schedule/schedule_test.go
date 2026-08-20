package schedule

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pkgclock "github.com/rin721/go-scaffold-template/pkg/clock"
	"github.com/rin721/go-scaffold-template/pkg/coordination"
	pkgexecution "github.com/rin721/go-scaffold-template/pkg/execution"
	"github.com/rin721/go-scaffold-template/pkg/logger"
	pkgobservability "github.com/rin721/go-scaffold-template/pkg/observability"
	pkgschedule "github.com/rin721/go-scaffold-template/pkg/schedule"
)

func TestResourcePrepareDoesNotStartAndActivateRunsThroughGovernance(t *testing.T) {
	engine := newFakeEngine()
	executor := &recordingExecutor{}
	telemetry := &recordingTelemetry{}
	runs := make(chan context.Context, 1)
	binding := fixedDelayBinding(t, "billing.reconcile", pkgschedule.Local(), func(ctx context.Context) error {
		runs <- ctx
		return nil
	})
	current := buildTestResource(t, engine, coordination.Unavailable(), executor, telemetry, binding)
	defer stopTestResource(t, current)

	if engine.started.Load() {
		t.Fatal("Prepare 阶段不得启动 trigger engine")
	}
	if got := current.diagnostics().Tasks[0].State; got != pkgschedule.StatePrepared {
		t.Fatalf("Prepare state=%q want %q", got, pkgschedule.StatePrepared)
	}
	if err := current.activate(); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if !engine.started.Load() {
		t.Fatal("Commit 准入后应启动 trigger engine")
	}
	if err := engine.trigger(binding.ID()); err != nil {
		t.Fatalf("trigger: %v", err)
	}
	select {
	case runCtx := <-runs:
		if got := pkgexecution.TraceFrom(runCtx); got != "trace-scheduled" {
			t.Fatalf("execution trace=%q want trace-scheduled", got)
		}
	case <-time.After(time.Second):
		t.Fatal("任务未进入业务函数")
	}
	if got := executor.last(); got.LeaseTTL != current.config.OccurrenceRetention || got.RetentionTTL != current.config.OccurrenceRetention || got.Trigger != "schedule.fixedDelay" {
		t.Fatalf("execution mapping=%+v", got)
	}
	if telemetry.calls.Load() != 1 {
		t.Fatalf("telemetry calls=%d want 1", telemetry.calls.Load())
	}
}

func TestBuildRejectsUnknownTaskOverride(t *testing.T) {
	cfg := testConfig()
	cfg.Tasks = map[string]TaskConfig{"unknown.task": {}}
	_, err := build(context.Background(), cfg, Dependencies{
		ApplicationName: "test-application", Generation: 1, Logger: logger.Noop(), Clock: pkgclock.System(),
		Execution: &recordingExecutor{}, Coordination: coordination.Unavailable(), Telemetry: &recordingTelemetry{},
		ReportFailure: func(error) {}, engineFactory: func(Config, pkgclock.Clock) (triggerEngine, error) { return newFakeEngine(), nil },
	})
	if err == nil {
		t.Fatal("unknown task override should fail before activation")
	}
}

func TestOccurrenceIdentityUsesInjectedProjectClock(t *testing.T) {
	engine := newFakeEngine()
	executor := &recordingExecutor{}
	scheduledAt := time.Date(2026, time.August, 20, 8, 9, 10, 123, time.UTC)
	binding := fixedDelayBinding(t, "clock.task", pkgschedule.Local(), func(context.Context) error { return nil })
	current, err := build(context.Background(), testConfig(), Dependencies{
		ApplicationName: "test-application", Generation: 1, Logger: logger.Noop(), Clock: pkgclock.Fixed(scheduledAt),
		Execution: executor, Coordination: coordination.Unavailable(), Telemetry: &recordingTelemetry{},
		Bindings: []pkgschedule.Binding{binding}, ReportFailure: func(error) {},
		engineFactory: func(Config, pkgclock.Clock) (triggerEngine, error) { return engine, nil },
	})
	if err != nil {
		t.Fatalf("build resource: %v", err)
	}
	defer stopTestResource(t, current)
	if err := current.activate(); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if err := engine.trigger(binding.ID()); err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if got, want := string(executor.last().Key), "schedule:clock.task:20260820T080910.000000123Z"; got != want {
		t.Fatalf("occurrence key=%q want %q", got, want)
	}
}

func TestGlobalConcurrencySkipsIndependentTaskWhenCapacityIsFull(t *testing.T) {
	engine := newFakeEngine()
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	first := fixedDelayBinding(t, "first.task", pkgschedule.Local(), func(context.Context) error {
		entered <- struct{}{}
		<-release
		return nil
	})
	var secondRuns atomic.Int32
	second := fixedDelayBinding(t, "second.task", pkgschedule.Local(), func(context.Context) error {
		secondRuns.Add(1)
		return nil
	})
	cfg := testConfig()
	cfg.MaxConcurrency = 1
	current := buildTestResourceWithConfig(t, cfg, engine, coordination.Unavailable(), &recordingExecutor{}, &recordingTelemetry{}, first, second)
	defer stopTestResource(t, current)
	if err := current.activate(); err != nil {
		t.Fatalf("activate: %v", err)
	}
	firstDone := make(chan error, 1)
	go func() { firstDone <- engine.trigger(first.ID()) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first task did not enter")
	}
	if err := engine.trigger(second.ID()); err != nil {
		t.Fatalf("trigger second: %v", err)
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("trigger first: %v", err)
	}
	if secondRuns.Load() != 0 {
		t.Fatalf("second task runs=%d want 0", secondRuns.Load())
	}
	diagnostics := current.diagnostics()
	for _, task := range diagnostics.Tasks {
		if task.ID == second.ID() && task.Skipped != 1 {
			t.Fatalf("second task snapshot=%+v want one skipped occurrence", task)
		}
	}
}

func TestStrictTaskPausesAndAutomaticallyRecovers(t *testing.T) {
	manager := newSharedManager(false)
	engine := newFakeEngine()
	policy, err := pkgschedule.DistributedSingleton(true, pkgschedule.UnavailablePause)
	if err != nil {
		t.Fatalf("coordination policy: %v", err)
	}
	var runs atomic.Int32
	binding := fixedDelayBinding(t, "inventory.refresh", policy, func(context.Context) error {
		runs.Add(1)
		return nil
	})
	current := buildTestResource(t, engine, manager, &recordingExecutor{}, &recordingTelemetry{}, binding)
	defer stopTestResource(t, current)
	if err := current.activate(); err != nil {
		t.Fatalf("activate: %v", err)
	}
	waitTaskState(t, current, binding.ID(), pkgschedule.StatePaused)
	if engine.jobCount() != 0 {
		t.Fatalf("paused strict task jobs=%d want 0", engine.jobCount())
	}
	if current.health().Status != "fail" {
		t.Fatalf("paused health=%q want fail", current.health().Status)
	}

	manager.setAvailable(true)
	waitTaskState(t, current, binding.ID(), pkgschedule.StateLeader)
	if engine.jobCount() != 1 {
		t.Fatalf("recovered leader jobs=%d want 1", engine.jobCount())
	}
	if err := engine.trigger(binding.ID()); err != nil {
		t.Fatalf("trigger recovered task: %v", err)
	}
	if runs.Load() != 1 {
		t.Fatalf("runs=%d want 1", runs.Load())
	}
	manager.setAvailable(false)
	waitTaskState(t, current, binding.ID(), pkgschedule.StatePaused)
	if engine.jobCount() != 0 {
		t.Fatalf("lost leader jobs=%d want 0", engine.jobCount())
	}
	manager.setAvailable(true)
	waitTaskState(t, current, binding.ID(), pkgschedule.StateLeader)
}

func TestStrictSkipKeepsReadinessAndClosesAdmission(t *testing.T) {
	policy, err := pkgschedule.DistributedSingleton(true, pkgschedule.UnavailableSkip)
	if err != nil {
		t.Fatalf("coordination policy: %v", err)
	}
	engine := newFakeEngine()
	binding := fixedDelayBinding(t, "notifications.digest", policy, func(context.Context) error { return nil })
	current := buildTestResource(t, engine, newSharedManager(false), &recordingExecutor{}, &recordingTelemetry{}, binding)
	defer stopTestResource(t, current)
	if err := current.activate(); err != nil {
		t.Fatalf("activate: %v", err)
	}
	waitTaskState(t, current, binding.ID(), pkgschedule.StateDegraded)
	diagnostics := current.diagnostics()
	if !diagnostics.Ready || !diagnostics.Degraded || engine.jobCount() != 0 {
		t.Fatalf("skip diagnostics=%+v jobs=%d", diagnostics, engine.jobCount())
	}
	if current.health().Status != "warn" {
		t.Fatalf("skip health=%q want warn", current.health().Status)
	}
}

func TestStrictFailUsesExistingFatalChannel(t *testing.T) {
	policy, err := pkgschedule.DistributedSingleton(true, pkgschedule.UnavailableFail)
	if err != nil {
		t.Fatalf("coordination policy: %v", err)
	}
	engine := newFakeEngine()
	binding := fixedDelayBinding(t, "settlement.close", policy, func(context.Context) error { return nil })
	failures := make(chan error, 1)
	current, err := build(context.Background(), testConfig(), Dependencies{
		ApplicationName: "test-application", Generation: 1, Logger: logger.Noop(), Clock: pkgclock.System(),
		Execution: &recordingExecutor{}, Coordination: newSharedManager(false), Telemetry: &recordingTelemetry{},
		Bindings: []pkgschedule.Binding{binding}, ReportFailure: func(err error) { failures <- err },
		engineFactory: func(Config, pkgclock.Clock) (triggerEngine, error) { return engine, nil },
	})
	if err != nil {
		t.Fatalf("build resource: %v", err)
	}
	defer stopTestResource(t, current)
	if err := current.activate(); err != nil {
		t.Fatalf("activate: %v", err)
	}
	select {
	case failure := <-failures:
		if !errors.Is(failure, coordination.ErrUnavailable) {
			t.Fatalf("failure=%v should preserve coordination cause", failure)
		}
	case <-time.After(time.Second):
		t.Fatal("fatal policy did not report into generation failure channel")
	}
	waitTaskState(t, current, binding.ID(), pkgschedule.StateFailed)
}

func TestBestEffortLocalFallbackIsExplicitAndRecoversToLeader(t *testing.T) {
	policy, err := pkgschedule.DistributedSingleton(false, pkgschedule.UnavailableLocal)
	if err != nil {
		t.Fatalf("coordination policy: %v", err)
	}
	manager := newSharedManager(false)
	engine := newFakeEngine()
	binding := fixedDelayBinding(t, "catalog.refresh", policy, func(context.Context) error { return nil })
	current := buildTestResource(t, engine, manager, &recordingExecutor{}, &recordingTelemetry{}, binding)
	defer stopTestResource(t, current)
	if err := current.activate(); err != nil {
		t.Fatalf("activate: %v", err)
	}
	waitTaskState(t, current, binding.ID(), pkgschedule.StateWeakened)
	if engine.jobCount() != 1 || !current.diagnostics().Degraded {
		t.Fatalf("weakened diagnostics=%+v jobs=%d", current.diagnostics(), engine.jobCount())
	}
	manager.setAvailable(true)
	waitTaskState(t, current, binding.ID(), pkgschedule.StateLeader)
	if engine.jobCount() != 1 {
		t.Fatalf("leader jobs=%d want 1 after replacing fallback admission", engine.jobCount())
	}
}

func TestBestEffortStandbyClosesLocalFallbackAfterCoordinationRecovers(t *testing.T) {
	policy, err := pkgschedule.DistributedSingleton(false, pkgschedule.UnavailableLocal)
	if err != nil {
		t.Fatalf("coordination policy: %v", err)
	}
	manager := newSharedManager(false)
	binding := fixedDelayBinding(t, "search.refresh", policy, func(context.Context) error { return nil })
	firstEngine, secondEngine := newFakeEngine(), newFakeEngine()
	first := buildTestResource(t, firstEngine, manager, &recordingExecutor{}, &recordingTelemetry{}, binding)
	second := buildTestResource(t, secondEngine, manager, &recordingExecutor{}, &recordingTelemetry{}, binding)
	defer stopTestResource(t, second)
	defer stopTestResource(t, first)
	if err := first.activate(); err != nil {
		t.Fatalf("activate first: %v", err)
	}
	if err := second.activate(); err != nil {
		t.Fatalf("activate second: %v", err)
	}
	waitTaskState(t, first, binding.ID(), pkgschedule.StateWeakened)
	waitTaskState(t, second, binding.ID(), pkgschedule.StateWeakened)
	manager.setAvailable(true)
	waitForCondition(t, func() bool {
		states := []pkgschedule.RuntimeState{
			first.diagnostics().Tasks[0].State,
			second.diagnostics().Tasks[0].State,
		}
		return (states[0] == pkgschedule.StateLeader && states[1] == pkgschedule.StateStandby) ||
			(states[1] == pkgschedule.StateLeader && states[0] == pkgschedule.StateStandby)
	}, "one best-effort leader and one standby")
	if jobs := firstEngine.jobCount() + secondEngine.jobCount(); jobs != 1 {
		t.Fatalf("active jobs after coordination recovery=%d want 1", jobs)
	}
}

func TestTwoLogicalInstancesInstallOnlyOneStrictTask(t *testing.T) {
	manager := newSharedManager(true)
	policy, err := pkgschedule.DistributedSingleton(true, pkgschedule.UnavailableSkip)
	if err != nil {
		t.Fatalf("coordination policy: %v", err)
	}
	var runs atomic.Int32
	binding := fixedDelayBinding(t, "reports.rollup", policy, func(context.Context) error {
		runs.Add(1)
		return nil
	})
	firstEngine, secondEngine := newFakeEngine(), newFakeEngine()
	first := buildTestResource(t, firstEngine, manager, &recordingExecutor{}, &recordingTelemetry{}, binding)
	second := buildTestResource(t, secondEngine, manager, &recordingExecutor{}, &recordingTelemetry{}, binding)
	defer stopTestResource(t, second)
	defer stopTestResource(t, first)
	if err := first.activate(); err != nil {
		t.Fatalf("activate first: %v", err)
	}
	if err := second.activate(); err != nil {
		t.Fatalf("activate second: %v", err)
	}
	waitForCondition(t, func() bool {
		return firstEngine.jobCount()+secondEngine.jobCount() == 1
	}, "one logical instance to install the strict task")
	if firstEngine.jobCount() == 1 {
		if err := firstEngine.trigger(binding.ID()); err != nil {
			t.Fatalf("trigger first: %v", err)
		}
	} else if err := secondEngine.trigger(binding.ID()); err != nil {
		t.Fatalf("trigger second: %v", err)
	}
	if runs.Load() != 1 {
		t.Fatalf("runs=%d want 1", runs.Load())
	}
}

func TestHubSwitchPreventsOldGenerationFromExecuting(t *testing.T) {
	var oldRuns, newRuns atomic.Int32
	oldBinding := fixedDelayBinding(t, "generation.task", pkgschedule.Local(), func(context.Context) error {
		oldRuns.Add(1)
		return nil
	})
	newBinding := fixedDelayBinding(t, "generation.task", pkgschedule.Local(), func(context.Context) error {
		newRuns.Add(1)
		return nil
	})
	oldEngine, newEngine := newFakeEngine(), newFakeEngine()
	oldResource := buildTestResource(t, oldEngine, coordination.Unavailable(), &recordingExecutor{}, &recordingTelemetry{}, oldBinding)
	newResource := buildTestResource(t, newEngine, coordination.Unavailable(), &recordingExecutor{}, &recordingTelemetry{}, newBinding)
	defer stopTestResource(t, newResource)
	defer stopTestResource(t, oldResource)
	oldAccess := &access{delegate: &resourceLease{current: oldResource}}
	newAccess := &access{delegate: &resourceLease{current: newResource}}
	hub := NewHub()
	if err := hub.Commit(context.Background(), oldAccess); err != nil {
		t.Fatalf("commit old generation: %v", err)
	}
	if err := oldEngine.trigger(oldBinding.ID()); err != nil {
		t.Fatalf("trigger old generation: %v", err)
	}
	if err := hub.Commit(context.Background(), newAccess); err != nil {
		t.Fatalf("commit new generation: %v", err)
	}
	if err := oldEngine.trigger(oldBinding.ID()); err != nil {
		t.Fatalf("trigger drained old generation: %v", err)
	}
	if err := newEngine.trigger(newBinding.ID()); err != nil {
		t.Fatalf("trigger new generation: %v", err)
	}
	if oldRuns.Load() != 1 || newRuns.Load() != 1 {
		t.Fatalf("old runs=%d new runs=%d want 1 each", oldRuns.Load(), newRuns.Load())
	}
}

func TestTaskPanicIsContainedAndRecorded(t *testing.T) {
	engine := newFakeEngine()
	binding := fixedDelayBinding(t, "panic.task", pkgschedule.Local(), func(context.Context) error {
		panic("secret business payload")
	})
	current := buildTestResource(t, engine, coordination.Unavailable(), &recordingExecutor{}, &recordingTelemetry{}, binding)
	defer stopTestResource(t, current)
	if err := current.activate(); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if err := engine.trigger(binding.ID()); err == nil {
		t.Fatal("panic should be converted to a stable task error")
	}
	snapshot := current.diagnostics().Tasks[0]
	if snapshot.Active != 0 || snapshot.LastErrorType != "task_panic" {
		t.Fatalf("panic snapshot=%+v", snapshot)
	}
}

func fixedDelayBinding(t *testing.T, id pkgschedule.TaskID, policy pkgschedule.CoordinationPolicy, run pkgschedule.Task) pkgschedule.Binding {
	t.Helper()
	trigger, err := pkgschedule.FixedDelay(time.Hour, 0)
	if err != nil {
		t.Fatalf("fixed delay trigger: %v", err)
	}
	binding, err := pkgschedule.Bind(pkgschedule.Spec{
		ID: id, Trigger: trigger, Concurrency: pkgschedule.SerialSkip(), Coordination: policy,
	}, run)
	if err != nil {
		t.Fatalf("bind task: %v", err)
	}
	return binding
}

func testConfig() Config {
	cfg := defaultConfig()
	cfg.Enabled = true
	cfg.ShutdownTimeout = time.Second
	cfg.Coordination.LeaseTTL = 300 * time.Millisecond
	cfg.Coordination.RenewInterval = 50 * time.Millisecond
	cfg.Coordination.AcquireTimeout = 10 * time.Millisecond
	cfg.Coordination.RetryMin = 5 * time.Millisecond
	cfg.Coordination.RetryMax = 20 * time.Millisecond
	return cfg
}

func buildTestResource(
	t *testing.T,
	engine *fakeEngine,
	manager coordination.Manager,
	executor pkgexecution.OperationExecutor,
	telemetry pkgobservability.Telemetry,
	bindings ...pkgschedule.Binding,
) *resource {
	t.Helper()
	return buildTestResourceWithConfig(t, testConfig(), engine, manager, executor, telemetry, bindings...)
}

func buildTestResourceWithConfig(
	t *testing.T,
	cfg Config,
	engine *fakeEngine,
	manager coordination.Manager,
	executor pkgexecution.OperationExecutor,
	telemetry pkgobservability.Telemetry,
	bindings ...pkgschedule.Binding,
) *resource {
	t.Helper()
	current, err := build(context.Background(), cfg, Dependencies{
		ApplicationName: "test-application", Generation: 1, Logger: logger.Noop(), Clock: pkgclock.System(),
		Execution: executor, Coordination: manager, Telemetry: telemetry, Bindings: bindings,
		ReportFailure: func(error) {}, engineFactory: func(Config, pkgclock.Clock) (triggerEngine, error) { return engine, nil },
	})
	if err != nil {
		t.Fatalf("build resource: %v", err)
	}
	return current
}

func stopTestResource(t *testing.T, current *resource) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := stop(ctx, current); err != nil {
		t.Errorf("stop resource: %v", err)
	}
}

func waitTaskState(t *testing.T, current *resource, id pkgschedule.TaskID, want pkgschedule.RuntimeState) {
	t.Helper()
	waitForCondition(t, func() bool {
		for _, task := range current.diagnostics().Tasks {
			if task.ID == id {
				return task.State == want
			}
		}
		return false
	}, fmt.Sprintf("task %s state %s", id, want))
}

func waitForCondition(t *testing.T, condition func() bool, description string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", description)
}

type fakeEngineJob struct {
	taskID pkgschedule.TaskID
	ctx    context.Context
	run    func(context.Context) error
}

type fakeEngine struct {
	mu       sync.Mutex
	jobs     map[engineJobID]fakeEngineJob
	next     int
	started  atomic.Bool
	shutdown atomic.Bool
}

func newFakeEngine() *fakeEngine { return &fakeEngine{jobs: make(map[engineJobID]fakeEngineJob)} }

func (*fakeEngine) Validate(pkgschedule.Binding, string) error { return nil }

func (e *fakeEngine) Add(binding pkgschedule.Binding, _ string, ctx context.Context, run func(context.Context) error) (engineJobID, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.next++
	id := engineJobID(fmt.Sprintf("job-%d", e.next))
	e.jobs[id] = fakeEngineJob{taskID: binding.ID(), ctx: ctx, run: run}
	return id, nil
}

func (e *fakeEngine) Remove(id engineJobID) error {
	e.mu.Lock()
	delete(e.jobs, id)
	e.mu.Unlock()
	return nil
}

func (e *fakeEngine) Start() { e.started.Store(true) }

func (e *fakeEngine) Shutdown(context.Context) error {
	e.shutdown.Store(true)
	return nil
}

func (e *fakeEngine) trigger(taskID pkgschedule.TaskID) error {
	e.mu.Lock()
	var selected fakeEngineJob
	found := false
	for _, job := range e.jobs {
		if job.taskID == taskID {
			selected, found = job, true
			break
		}
	}
	e.mu.Unlock()
	if !found {
		return fmt.Errorf("task %s has no active job", taskID)
	}
	return selected.run(selected.ctx)
}

func (e *fakeEngine) jobCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.jobs)
}

type recordingExecutor struct {
	mu         sync.Mutex
	executions []pkgexecution.Execution
}

type resourceLease struct{ current *resource }

func (l *resourceLease) Use(ctx context.Context, use func(*resource) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return use(l.current)
}

func (e *recordingExecutor) Execute(ctx context.Context, execution pkgexecution.Execution) (pkgexecution.Result, error) {
	e.mu.Lock()
	e.executions = append(e.executions, execution)
	e.mu.Unlock()
	_, err := execution.Operation(ctx)
	if err != nil {
		return pkgexecution.Result{Status: pkgexecution.StatusFailed}, err
	}
	return pkgexecution.Result{Status: pkgexecution.StatusCompleted}, nil
}

func (e *recordingExecutor) last() pkgexecution.Execution {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.executions[len(e.executions)-1]
}

type recordingTelemetry struct{ calls atomic.Int32 }

func (*recordingTelemetry) HTTP([]pkgobservability.Operation) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler { return next }
}

func (t *recordingTelemetry) Observe(ctx context.Context, _ pkgobservability.Work, run pkgobservability.WorkFunc) error {
	t.calls.Add(1)
	return run(pkgobservability.WithTraceID(ctx, "trace-scheduled"))
}

func (*recordingTelemetry) Diagnostics(context.Context) (pkgobservability.Diagnostics, error) {
	return pkgobservability.Diagnostics{Ready: true}, nil
}

type sharedManager struct {
	mu        sync.Mutex
	available bool
	holder    *sharedLease
}

func newSharedManager(available bool) *sharedManager { return &sharedManager{available: available} }

func (m *sharedManager) setAvailable(available bool) {
	m.mu.Lock()
	m.available = available
	if !available && m.holder != nil {
		m.holder.valid = false
		m.holder = nil
	}
	m.mu.Unlock()
}

func (m *sharedManager) Acquire(ctx context.Context, key coordination.Key, options coordination.LeaseOptions) (coordination.Lease, error) {
	if err := coordination.ValidateAcquire(ctx, key, options); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.available {
		return nil, coordination.ErrUnavailable
	}
	if m.holder != nil && m.holder.valid {
		return nil, coordination.ErrNotAcquired
	}
	lease := &sharedLease{manager: m, valid: true}
	m.holder = lease
	return lease, nil
}

type sharedLease struct {
	manager *sharedManager
	valid   bool
}

func (l *sharedLease) Renew(ctx context.Context, options coordination.LeaseOptions) error {
	if err := coordination.ValidateAcquire(ctx, "renew", options); err != nil {
		return err
	}
	l.manager.mu.Lock()
	defer l.manager.mu.Unlock()
	if !l.manager.available {
		l.valid = false
		if l.manager.holder == l {
			l.manager.holder = nil
		}
		return coordination.ErrUnavailable
	}
	if !l.valid || l.manager.holder != l {
		return coordination.ErrLeaseLost
	}
	return nil
}

func (l *sharedLease) Release(ctx context.Context) error {
	if ctx == nil {
		return coordination.ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	l.manager.mu.Lock()
	defer l.manager.mu.Unlock()
	if !l.valid || l.manager.holder != l {
		return coordination.ErrLeaseLost
	}
	l.valid = false
	l.manager.holder = nil
	return nil
}

var _ pkgexecution.OperationExecutor = (*recordingExecutor)(nil)
var _ pkgobservability.Telemetry = (*recordingTelemetry)(nil)
var _ coordination.Manager = (*sharedManager)(nil)
var _ coordination.Lease = (*sharedLease)(nil)
