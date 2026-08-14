package supervisor

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestSupervisorStartsAndStopsParticipantsInOrder(t *testing.T) {
	var events []string
	ctx, cancel := context.WithCancel(t.Context())
	supervisor := New(Config{},
		&recordParticipant{name: "database", events: &events},
		&recordParticipant{name: "server", events: &events},
	)
	if err := supervisor.AddTask("shutdown", func(context.Context) error {
		cancel()
		return nil
	}); err != nil {
		t.Fatalf("AddTask() error = %v", err)
	}
	if err := supervisor.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := []string{"start:database", "start:server", "stop:server", "stop:database"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
}

func TestSupervisorStopsStartedParticipantsAfterStartFailure(t *testing.T) {
	startErr := errors.New("server start failed")
	stopErr := errors.New("database stop failed")
	var events []string
	supervisor := New(Config{},
		&recordParticipant{name: "database", events: &events, stopErr: stopErr},
		&recordParticipant{name: "server", events: &events, startErr: startErr},
	)

	err := supervisor.Run(t.Context())
	if !errors.Is(err, startErr) || !errors.Is(err, stopErr) {
		t.Fatalf("Run() error = %v, want start and cleanup errors", err)
	}
	want := []string{"start:database", "start:server", "stop:database"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
}

func TestSupervisorTaskFailureCancelsSiblingAndStopsParticipants(t *testing.T) {
	taskErr := errors.New("consumer failed")
	siblingStarted := make(chan struct{})
	siblingCanceled := make(chan struct{})
	participant := &recordParticipant{name: "database"}
	supervisor := New(Config{}, participant)
	if err := supervisor.AddTask("sibling", func(ctx context.Context) error {
		close(siblingStarted)
		<-ctx.Done()
		close(siblingCanceled)
		return nil
	}); err != nil {
		t.Fatalf("AddTask(sibling) error = %v", err)
	}
	if err := supervisor.AddTask("consumer", func(context.Context) error {
		<-siblingStarted
		return taskErr
	}); err != nil {
		t.Fatalf("AddTask(consumer) error = %v", err)
	}

	err := supervisor.Run(t.Context())
	if !errors.Is(err, taskErr) {
		t.Fatalf("Run() error = %v, want task error", err)
	}
	select {
	case <-siblingCanceled:
	default:
		t.Fatal("sibling task was not canceled")
	}
	if participant.stops != 1 {
		t.Fatalf("participant Stop count = %d, want 1", participant.stops)
	}
}

func TestSupervisorTreatsEarlyNilCompletionAsFailure(t *testing.T) {
	process := New(Config{})
	if err := process.AddTask("server", func(context.Context) error { return nil }); err != nil {
		t.Fatalf("AddTask() error = %v", err)
	}
	err := process.Run(t.Context())
	var unexpected *UnexpectedCompletionError
	if !errors.As(err, &unexpected) || unexpected.Task != "server" {
		t.Fatalf("Run() error = %v, want unexpected server completion", err)
	}
	if snapshot := process.Snapshot(); snapshot.Ready || snapshot.State != StateFailed {
		t.Fatalf("Snapshot() = %#v, want failed and not ready", snapshot)
	}
}

func TestSupervisorBoundsUncooperativeRunnerAndReportsOwner(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())
	process := New(Config{ShutdownTimeout: 20 * time.Millisecond})
	if err := process.AddTask("stuck", func(context.Context) error {
		close(started)
		<-release
		return nil
	}); err != nil {
		t.Fatalf("AddTask() error = %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- process.Run(ctx) }()
	<-started
	cancel()
	err := <-done
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v, want shutdown deadline", err)
	}
	snapshot := process.Snapshot()
	if snapshot.State != StateFailed || !reflect.DeepEqual(snapshot.PendingUnits, []string{"stuck"}) {
		t.Fatalf("Snapshot() = %#v, want failed stuck owner", snapshot)
	}
	close(release)
}

func TestSupervisorWaitsForRunnerReadyAcknowledgement(t *testing.T) {
	ready := make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())
	process := New(Config{})
	if err := process.AddRunner(Task{
		Name:  "server",
		Ready: ready,
		Run: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}); err != nil {
		t.Fatalf("AddRunner() error = %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- process.Run(ctx) }()
	if snapshot := process.Snapshot(); snapshot.Ready {
		t.Fatalf("Snapshot() ready before runner acknowledgement: %#v", snapshot)
	}
	close(ready)
	deadline := time.After(time.Second)
	for !process.Snapshot().Ready {
		select {
		case <-deadline:
			t.Fatal("supervisor did not become ready")
		default:
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestSupervisorWithoutTasksWaitsForCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	started := make(chan struct{})
	participant := &recordParticipant{name: "database", onStart: func() { close(started) }}
	supervisor := New(Config{}, participant)
	done := make(chan error, 1)
	go func() { done <- supervisor.Run(ctx) }()

	<-started
	select {
	case err := <-done:
		t.Fatalf("Run() returned before cancellation: %v", err)
	default:
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not return after cancellation")
	}
}

func TestSupervisorRunOperationStartsExecutesAndStopsInOrder(t *testing.T) {
	var events []string
	process := New(Config{},
		&recordParticipant{name: "database", events: &events},
		&recordParticipant{name: "schema", events: &events},
	)
	err := process.RunOperation(t.Context(), func(context.Context) error {
		events = append(events, "operation")
		return nil
	})
	if err != nil {
		t.Fatalf("RunOperation() error = %v", err)
	}
	want := []string{"start:database", "start:schema", "operation", "stop:schema", "stop:database"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
	if snapshot := process.Snapshot(); snapshot.State != StateStopped || snapshot.Ready {
		t.Fatalf("Snapshot() = %#v, want stopped and not ready", snapshot)
	}
}

func TestSupervisorRunOperationJoinsOperationAndStopErrors(t *testing.T) {
	operationErr := errors.New("operation failed")
	stopErr := errors.New("stop failed")
	process := New(Config{}, &recordParticipant{name: "database", stopErr: stopErr})
	err := process.RunOperation(t.Context(), func(context.Context) error { return operationErr })
	if !errors.Is(err, operationErr) || !errors.Is(err, stopErr) {
		t.Fatalf("RunOperation() error = %v, want operation and stop errors", err)
	}
	if snapshot := process.Snapshot(); snapshot.State != StateFailed || snapshot.Ready {
		t.Fatalf("Snapshot() = %#v, want failed", snapshot)
	}
}

func TestSupervisorRunOperationStopsStartedParticipantsAfterStartFailure(t *testing.T) {
	startErr := errors.New("schema start failed")
	var events []string
	process := New(Config{},
		&recordParticipant{name: "database", events: &events},
		&recordParticipant{name: "schema", events: &events, startErr: startErr},
	)
	err := process.RunOperation(t.Context(), func(context.Context) error {
		t.Fatal("operation ran after startup failure")
		return nil
	})
	if !errors.Is(err, startErr) {
		t.Fatalf("RunOperation() error = %v, want start error", err)
	}
	want := []string{"start:database", "start:schema", "stop:database"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
}

func TestSupervisorRunOperationRejectsInvalidUse(t *testing.T) {
	if err := New(Config{}).RunOperation(nil, func(context.Context) error { return nil }); err == nil {
		t.Fatal("RunOperation(nil context) error = nil")
	}
	if err := New(Config{}).RunOperation(t.Context(), nil); err == nil {
		t.Fatal("RunOperation(nil operation) error = nil")
	}
	withTask := New(Config{})
	if err := withTask.AddTask("runner", func(context.Context) error { return nil }); err != nil {
		t.Fatalf("AddTask() error = %v", err)
	}
	if err := withTask.RunOperation(t.Context(), func(context.Context) error { return nil }); err == nil {
		t.Fatal("RunOperation(with task) error = nil")
	}
	process := New(Config{})
	if err := process.RunOperation(t.Context(), func(context.Context) error { return nil }); err != nil {
		t.Fatalf("first RunOperation() error = %v", err)
	}
	if err := process.RunOperation(t.Context(), func(context.Context) error { return nil }); err == nil {
		t.Fatal("second RunOperation() error = nil")
	}
}

func TestSupervisorRunOperationBoundsStop(t *testing.T) {
	release := make(chan struct{})
	process := New(Config{ShutdownTimeout: 20 * time.Millisecond}, blockingStopParticipant{release: release})
	err := process.RunOperation(t.Context(), func(context.Context) error { return nil })
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("RunOperation() error = %v, want shutdown deadline", err)
	}
	close(release)
}

func TestSupervisorJoinsParticipantStopErrors(t *testing.T) {
	firstErr := errors.New("first stop failed")
	secondErr := errors.New("second stop failed")
	ctx, cancel := context.WithCancel(t.Context())
	supervisor := New(Config{ShutdownTimeout: time.Second},
		&recordParticipant{name: "first", stopErr: firstErr},
		&recordParticipant{name: "second", stopErr: secondErr},
	)
	if err := supervisor.AddTask("shutdown", func(context.Context) error {
		cancel()
		return nil
	}); err != nil {
		t.Fatalf("AddTask() error = %v", err)
	}

	err := supervisor.Run(ctx)
	if !errors.Is(err, firstErr) || !errors.Is(err, secondErr) {
		t.Fatalf("Run() error = %v, want both stop errors", err)
	}
}

func TestSupervisorPreservesTaskErrorJoinedWithIntentionalCancellation(t *testing.T) {
	taskErr := errors.New("task cleanup failed")
	ctx, cancel := context.WithCancel(t.Context())
	process := New(Config{})
	if err := process.AddTask("consumer", func(ctx context.Context) error {
		cancel()
		<-ctx.Done()
		return errors.Join(ctx.Err(), taskErr)
	}); err != nil {
		t.Fatalf("AddTask() error = %v", err)
	}

	err := process.Run(ctx)
	if !errors.Is(err, taskErr) {
		t.Fatalf("Run() error = %v, want task cleanup error", err)
	}
}

func TestSupervisorKeepsFailedDiagnosticsAfterRuntimeError(t *testing.T) {
	runtimeFailure := errors.New("runtime failure")
	s := New(Config{}, &recordParticipant{name: "application"})
	if err := s.AddTask("runner", func(context.Context) error { return runtimeFailure }); err != nil {
		t.Fatalf("add task: %v", err)
	}

	err := s.Run(t.Context())
	if !errors.Is(err, runtimeFailure) {
		t.Fatalf("run error = %v, want runtime failure", err)
	}
	snapshot := s.Snapshot()
	if snapshot.State != StateFailed || snapshot.Ready {
		t.Fatalf("snapshot = %+v, want failed and not ready", snapshot)
	}
}

func TestSupervisorRejectsNilContextAndSecondRun(t *testing.T) {
	supervisor := New(Config{})
	if err := supervisor.Run(nil); err == nil {
		t.Fatal("Run(nil) error = nil")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := supervisor.Run(ctx); err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	if err := supervisor.Run(ctx); err == nil {
		t.Fatal("second Run() error = nil")
	}
	if err := supervisor.AddTask("late", func(context.Context) error { return nil }); err == nil {
		t.Fatal("AddTask() after Run error = nil")
	}
}

func TestSupervisorUsesDefaultShutdownTimeoutForNonPositiveValues(t *testing.T) {
	for _, timeout := range []time.Duration{0, -time.Second} {
		process := New(Config{ShutdownTimeout: timeout})
		if process.timeout != defaultShutdownTimeout {
			t.Fatalf("timeout = %s, want %s", process.timeout, defaultShutdownTimeout)
		}
	}
}

type recordParticipant struct {
	name     string
	events   *[]string
	startErr error
	stopErr  error
	onStart  func()
	starts   int
	stops    int
}

type blockingStopParticipant struct{ release <-chan struct{} }

func (blockingStopParticipant) Name() string                { return "blocking" }
func (blockingStopParticipant) Start(context.Context) error { return nil }
func (p blockingStopParticipant) Stop(context.Context) error {
	<-p.release
	return nil
}

func (p *recordParticipant) Name() string { return p.name }

func (p *recordParticipant) Start(context.Context) error {
	p.starts++
	if p.events != nil {
		*p.events = append(*p.events, "start:"+p.name)
	}
	if p.onStart != nil {
		p.onStart()
	}
	return p.startErr
}

func (p *recordParticipant) Stop(context.Context) error {
	p.stops++
	if p.events != nil {
		*p.events = append(*p.events, "stop:"+p.name)
	}
	return p.stopErr
}
