package resource

import (
	"errors"
	"reflect"
	"testing"
)

func TestRegistryClosesInReverseOrderAndJoinsErrors(t *testing.T) {
	var events []string
	firstErr := errors.New("first")
	registry := NewRegistry()
	if err := registry.Add(Handle{Name: "first", Close: func() error {
		events = append(events, "first")
		return firstErr
	}}); err != nil {
		t.Fatalf("Add(first) error = %v", err)
	}
	if err := registry.Add(Handle{Name: "shared", Shared: true, Close: func() error {
		events = append(events, "shared")
		return nil
	}}); err != nil {
		t.Fatalf("Add(shared) error = %v", err)
	}
	if err := registry.Add(Handle{Name: "second", Close: func() error {
		events = append(events, "second")
		return nil
	}}); err != nil {
		t.Fatalf("Add(second) error = %v", err)
	}
	err := registry.Close()
	if !errors.Is(err, firstErr) {
		t.Fatalf("Close() = %v, want firstErr", err)
	}
	if !reflect.DeepEqual(events, []string{"second", "first"}) {
		t.Fatalf("events = %#v", events)
	}
}
