package concurrency

import (
	"context"
	"sync/atomic"
	"testing"
)

func TestPoolRunsTasks(t *testing.T) {
	var count int64
	pool := NewPool(2)
	err := pool.Run(context.Background(), []func(context.Context) error{
		func(context.Context) error { atomic.AddInt64(&count, 1); return nil },
		func(context.Context) error { atomic.AddInt64(&count, 1); return nil },
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
}

func TestSingleFlightSharesResult(t *testing.T) {
	var calls int
	var sf SingleFlight
	value, err, _ := sf.Do("k", func() (any, error) {
		calls++
		return "ok", nil
	})
	if err != nil || value != "ok" || calls != 1 {
		t.Fatalf("Do() value=%v err=%v calls=%d", value, err, calls)
	}
}
