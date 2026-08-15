package app

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestTerminalFinalizerCachesFailureAndRetainsSafeSnapshot(t *testing.T) {
	cause := errors.New("secret dsn must not appear")
	attempts := 0
	slot := &instanceSlot[*int]{
		generation: 7,
		instance:   new(int),
		phase:      FinalizationPhaseRetired,
		state:      FinalizationPending,
	}
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
	if snapshot.State != FinalizationTerminalFailed || snapshot.Attempts != 1 || snapshot.InstanceGeneration != 7 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if strings.Contains(snapshot.LastErrorType, cause.Error()) {
		t.Fatalf("snapshot leaked error text: %#v", snapshot)
	}
}

func TestNoFinalizationCompletesWithoutRawAttempt(t *testing.T) {
	slot := &instanceSlot[int]{generation: 1, instance: 42, phase: FinalizationPhaseCurrent, state: FinalizationPending}
	if err := finalizeSlot(t.Context(), slot, nil); err != nil {
		t.Fatalf("finalizeSlot() error = %v", err)
	}
	if slot.state != FinalizationSucceeded || slot.attempts != 0 {
		t.Fatalf("slot = %#v", slot)
	}
}

func TestWithTerminalFinalizerRejectsNilAndDuplicateDeclarations(t *testing.T) {
	var lifecycle lifecycle[int]
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
		lease: newLease[*int](),
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
