package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestTerminalFinalizerCachesFailureAndRetainsSafeSnapshot(t *testing.T) {
	cause := errors.New("secret dsn must not appear")
	attempts := 0
	slot := newInstanceSlot(7, new(int), FinalizationPhaseRetired, DrainThenTerminalClose)
	finalizer := func(context.Context, *int) error {
		attempts++
		return cause
	}
	if err := finalizeSlot(t.Context(), slot, finalizer); !errors.Is(err, cause) {
		t.Fatalf("first finalizeSlot() error = %v", err)
	}
	if err := finalizeSlot(t.Context(), slot, finalizer); !errors.Is(err, cause) {
		t.Fatalf("second finalizeSlot() error = %v", err)
	}
	if attempts != 1 || slot.instance == nil {
		t.Fatalf("attempts = %d, instance retained = %v", attempts, slot.instance != nil)
	}
	snapshot := slot.snapshot("database")
	if snapshot.State != OwnershipTerminalFailed || snapshot.Attempt != 1 || snapshot.InstanceGeneration != 7 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if strings.Contains(snapshot.LastErrorType, cause.Error()) {
		t.Fatalf("snapshot leaked error text: %#v", snapshot)
	}
}

func TestNoFinalizationCompletesWithoutRawAttempt(t *testing.T) {
	slot := newInstanceSlot(1, 42, FinalizationPhaseCurrent, NoFinalization)
	if err := finalizeSlot(t.Context(), slot, nil); err != nil {
		t.Fatalf("finalizeSlot() error = %v", err)
	}
	snapshot := slot.snapshot("clock")
	if snapshot.State != OwnershipFinalized || snapshot.Attempt != 0 || snapshot.Verification != VerificationNotRequired {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}

func TestInstanceSlotRejectsZeroFinalizationPolicy(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("newInstanceSlot(zero policy) did not panic")
		}
	}()
	newInstanceSlot(1, 42, FinalizationPhaseCurrent, FinalizationPolicy(""))
}

func TestWithTerminalFinalizerRejectsNilAndDuplicateDeclarations(t *testing.T) {
	lifecycle := newLifecycle[int]()
	if err := WithTerminalFinalizer[int](nil)(&lifecycle); err == nil {
		t.Fatal("nil terminal finalizer error = nil")
	}
	option := WithTerminalFinalizer(func(context.Context, int) error { return nil })
	if err := option(&lifecycle); err != nil {
		t.Fatalf("first option error = %v", err)
	}
	if err := option(&lifecycle); err == nil {
		t.Fatal("duplicate terminal finalizer error = nil")
	}
}

func TestManagedComponentAssignsMonotonicInstanceGeneration(t *testing.T) {
	value := 0
	component := &managedComponent[int, struct{}, *int]{
		componentID:         "clock",
		resolveDependencies: func() (struct{}, error) { return struct{}{}, nil },
		build: func(context.Context, int, struct{}) (*int, error) {
			value++
			instance := value
			return &instance, nil
		},
		lifecycle: newLifecycle[*int](),
		lease:     newLease[*int](),
	}
	if err := component.Build(t.Context()); err != nil {
		t.Fatalf("first Build() error = %v", err)
	}
	if component.candidate.generation != 1 {
		t.Fatalf("first generation = %d", component.candidate.generation)
	}
	component.PublishInitial()
	if err := component.Build(t.Context()); err != nil {
		t.Fatalf("second Build() error = %v", err)
	}
	if component.candidate.generation != 2 {
		t.Fatalf("second generation = %d", component.candidate.generation)
	}
	drained, err := component.BeginDrain()
	if err != nil {
		t.Fatalf("BeginDrain() error = %v", err)
	}
	<-drained
	component.Commit()
	component.Resume()
	if err := component.StopPrevious(t.Context()); err != nil {
		t.Fatalf("StopPrevious() error = %v", err)
	}
}

func TestManagedComponentRetainsBoundedNoFinalizationTombstone(t *testing.T) {
	component := &managedComponent[int, struct{}, *int]{
		componentID:         "clock",
		resolveDependencies: func() (struct{}, error) { return struct{}{}, nil },
		build: func(context.Context, int, struct{}) (*int, error) {
			value := 42
			return &value, nil
		},
		lifecycle: newLifecycle[*int](),
		lease:     newLease[*int](),
	}
	if err := component.Build(t.Context()); err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	component.PublishInitial()
	serving := component.Ownerships()
	if len(serving) != 1 || serving[0].State != OwnershipServing || serving[0].Policy != NoFinalization {
		t.Fatalf("serving ownerships = %#v", serving)
	}
	drained, err := component.BeginTerminalDrain()
	if err != nil {
		t.Fatalf("BeginTerminalDrain() error = %v", err)
	}
	<-drained
	component.PrepareStop()
	if err := component.StopCurrent(t.Context()); err != nil {
		t.Fatalf("StopCurrent() error = %v", err)
	}
	terminal := component.Ownerships()
	if len(terminal) != 1 || terminal[0].State != OwnershipFinalized || terminal[0].Attempt != 0 || terminal[0].Verification != VerificationNotRequired {
		t.Fatalf("terminal ownerships = %#v", terminal)
	}
}

func TestOwnershipSnapshotIsReadableDuringDrainAndFinalization(t *testing.T) {
	useStarted := make(chan struct{})
	releaseUse := make(chan struct{})
	finalizerStarted := make(chan struct{})
	releaseFinalizer := make(chan struct{})
	component := &managedComponent[int, struct{}, *int]{
		componentID:         "database",
		resolveDependencies: func() (struct{}, error) { return struct{}{}, nil },
		build: func(context.Context, int, struct{}) (*int, error) {
			value := 7
			return &value, nil
		},
		lifecycle: lifecycle[*int]{
			finalizationPolicy: DrainThenTerminalClose,
			terminalFinalizer: func(context.Context, *int) error {
				close(finalizerStarted)
				<-releaseFinalizer
				return nil
			},
		},
		lease: newLease[*int](),
	}
	if err := component.Build(t.Context()); err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	component.PublishInitial()
	useDone := make(chan error, 1)
	go func() {
		useDone <- component.lease.Use(t.Context(), func(*int) error {
			close(useStarted)
			<-releaseUse
			return nil
		})
	}()
	<-useStarted
	drained, err := component.BeginTerminalDrain()
	if err != nil {
		t.Fatalf("BeginTerminalDrain() error = %v", err)
	}
	assertOwnershipState(t, component, OwnershipWaitingForDrain)
	close(releaseUse)
	<-drained
	if err := <-useDone; err != nil {
		t.Fatalf("Use() error = %v", err)
	}
	component.PrepareStop()
	stopDone := make(chan error, 1)
	go func() { stopDone <- component.StopCurrent(t.Context()) }()
	<-finalizerStarted
	assertOwnershipState(t, component, OwnershipFinalizing)
	close(releaseFinalizer)
	if err := <-stopDone; err != nil {
		t.Fatalf("StopCurrent() error = %v", err)
	}
	terminal := component.Ownerships()
	if len(terminal) != 1 || terminal[0].State != OwnershipFinalized || terminal[0].Verification != VerificationNotProven {
		t.Fatalf("terminal ownerships = %#v", terminal)
	}
}

func assertOwnershipState(t *testing.T, component *managedComponent[int, struct{}, *int], want OwnershipState) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		for _, snapshot := range component.Ownerships() {
			if snapshot.State == want {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("ownerships = %#v, want state %s", component.Ownerships(), want)
		}
	}
}
