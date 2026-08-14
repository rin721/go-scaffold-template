package health

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRegistrySnapshotAggregatesStatusAndErrors(t *testing.T) {
	registry := New(time.Second)
	if err := registry.Register("db", func(context.Context) Result {
		return Result{Status: StatusPass}
	}); err != nil {
		t.Fatalf("Register(db) error = %v", err)
	}
	cause := errors.New("redis down")
	if err := registry.Register("cache", func(context.Context) Result {
		return Result{Error: cause}
	}); err != nil {
		t.Fatalf("Register(cache) error = %v", err)
	}
	snapshot := registry.Snapshot(context.Background())
	if snapshot.Status != StatusFail {
		t.Fatalf("Status = %q, want fail", snapshot.Status)
	}
	if len(snapshot.Results) != 2 || snapshot.Results[0].Name != "db" || snapshot.Results[1].Name != "cache" {
		t.Fatalf("Results order = %#v, want registration order", snapshot.Results)
	}
	if !errors.Is(snapshot.Error(), cause) {
		t.Fatalf("Snapshot.Error() = %v, want cause", snapshot.Error())
	}
}
