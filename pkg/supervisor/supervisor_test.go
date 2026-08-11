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
