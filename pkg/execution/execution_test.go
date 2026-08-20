package execution

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rin721/go-scaffold-template/pkg/fault"
	"github.com/rin721/go-scaffold-template/pkg/resilience"
)

func TestMemoryClaimAllowsFirstAndRejectsConcurrent(t *testing.T) {
	store := NewMemoryStore()
	now := time.Now()
	claimed, err := store.Claim(context.Background(), "k", 0, now)
	if err != nil || !claimed {
		t.Fatalf("first claim: claimed=%v err=%v", claimed, err)
	}
	claimed, err = store.Claim(context.Background(), "k", time.Minute, now)
	if err != nil || claimed {
		t.Fatalf("concurrent claim should reject: claimed=%v err=%v", claimed, err)
	}
}

func TestExecutorDuplicateDoesNotReRun(t *testing.T) {
	store := NewMemoryStore()
	executor := NewExecutor(store)
	var calls int32
	op := func(context.Context) (any, error) {
		atomic.AddInt32(&calls, 1)
		return "ok", nil
	}
	first, err := executor.Execute(context.Background(), Execution{Key: "order:1", Operation: op})
	if err != nil {
		t.Fatalf("first execute: %v", err)
	}
	if first.Status != StatusCompleted || first.Duplicate {
		t.Fatalf("first result unexpected: %+v", first)
	}
	second, err := executor.Execute(context.Background(), Execution{Key: "order:1", Operation: op})
	if err != nil {
		t.Fatalf("duplicate execute: %v", err)
	}
	if !second.Duplicate || second.Status != StatusCompleted {
		t.Fatalf("duplicate result unexpected: %+v", second)
	}
	if calls != 1 {
		t.Fatalf("operation ran %d times, want 1", calls)
	}
}

func TestExecutorRetryExhausted(t *testing.T) {
	store := NewMemoryStore()
	executor := NewExecutor(store)
	cause := errors.New("boom")
	bed := fault.Wrap(cause, fault.CodeUnavailable, "db", true)
	op := func(context.Context) (any, error) { return nil, bed }
	result, err := executor.Execute(context.Background(), Execution{
		Key:       "k",
		Operation: op,
		Policy: resilience.RetryPolicy{
			MaxAttempts: 3,
			InitialWait: time.Millisecond,
			MaxWait:     time.Millisecond,
			Retryable:   fault.Retryable,
		},
	})
	if err == nil {
		t.Fatalf("want error")
	}
	if !errors.Is(err, ErrRetryExhausted) {
		t.Fatalf("want ErrRetryExhausted, got %v", err)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("retry error lost original cause: %v", err)
	}
	if result.Status != StatusFailed {
		t.Fatalf("status=%s want failed", result.Status)
	}
	records, err := store.Records(context.Background(), "k")
	if err != nil || len(records) == 0 {
		t.Fatalf("want records, got %d err=%v", len(records), err)
	}
	if !strings.Contains(records[0].Error, "boom") {
		t.Fatalf("record missing cause text: %+v", records[0])
	}
}

func TestExecutorRetrySucceedsAfterTransient(t *testing.T) {
	store := NewMemoryStore()
	executor := NewExecutor(store)
	var attempts int32
	bed := fault.Wrap(errors.New("transient"), fault.CodeUnavailable, "db", true)
	op := func(context.Context) (any, error) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			return nil, bed
		}
		return "ok", nil
	}
	result, err := executor.Execute(context.Background(), Execution{
		Key:       "k2",
		Operation: op,
		Policy: resilience.RetryPolicy{
			MaxAttempts: 3,
			InitialWait: time.Millisecond,
			MaxWait:     time.Millisecond,
			Retryable:   fault.Retryable,
		},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("status=%s want completed", result.Status)
	}
	if attempts != 2 {
		t.Fatalf("attempts=%d want 2", attempts)
	}
}

func TestExecutorNonRetryableSingleAttempt(t *testing.T) {
	store := NewMemoryStore()
	executor := NewExecutor(store)
	bad := fault.Wrap(errors.New("invalid"), fault.CodeInvalidArgument, "validate", false)
	var attempts int32
	op := func(context.Context) (any, error) {
		atomic.AddInt32(&attempts, 1)
		return nil, bad
	}
	if _, err := executor.Execute(context.Background(), Execution{
		Key:       "k3",
		Operation: op,
		Policy: resilience.RetryPolicy{
			MaxAttempts: 5,
			InitialWait: time.Millisecond,
			MaxWait:     time.Millisecond,
			Retryable:   fault.Retryable,
		},
	}); err == nil {
		t.Fatalf("want error")
	}
	if attempts != 1 {
		t.Fatalf("attempts=%d want 1 for non-retryable", attempts)
	}
}

func TestExecutorValidation(t *testing.T) {
	executor := NewExecutor(NewMemoryStore())
	if _, err := executor.Execute(context.Background(), Execution{
		Operation: func(context.Context) (any, error) { return nil, nil },
	}); !errors.Is(err, ErrEmptyKey) {
		t.Fatalf("empty key: %v", err)
	}
	if _, err := executor.Execute(context.Background(), Execution{Key: "k"}); !errors.Is(err, ErrNilOperation) {
		t.Fatalf("nil operation: %v", err)
	}
	if _, err := executor.Execute(context.Background(), Execution{
		Key: "k", ClaimTTL: -time.Second, Operation: func(context.Context) (any, error) { return nil, nil },
	}); err == nil {
		t.Fatal("negative claim ttl error = nil")
	}
}

func TestExecutionClaimTTLIsExplicitAndBackendErrorsKeepCause(t *testing.T) {
	exec := Execution{ClaimTTL: 15 * time.Minute}
	if got := claimTTL(exec); got != exec.ClaimTTL {
		t.Fatalf("claimTTL() = %s, want %s", got, exec.ClaimTTL)
	}
	cause := errors.New("backend cause")
	err := WrapBackend(cause)
	if !errors.Is(err, ErrBackend) || !errors.Is(err, cause) {
		t.Fatalf("WrapBackend() = %v", err)
	}
}

// TestExecutorTimeoutDoesNotRecurse 回归验证 Timeout>0 时不会因超时包装闭包自递归而挂死。
func TestExecutorTimeoutDoesNotRecurse(t *testing.T) {
	store := NewMemoryStore()
	executor := NewExecutor(store)
	var calls int32
	res, err := executor.Execute(context.Background(), Execution{
		Key: "with-timeout", Timeout: time.Second,
		Operation: func(context.Context) (any, error) {
			atomic.AddInt32(&calls, 1)
			return "ok", nil
		},
	})
	if err != nil {
		t.Fatalf("execute with timeout: %v", err)
	}
	if res.Status != StatusCompleted {
		t.Fatalf("status=%s want completed", res.Status)
	}
	if calls != 1 {
		t.Fatalf("calls=%d want 1 (no recursive retry)", calls)
	}
}

// TestExecutorTimeoutExpiry 验证单次操作超时后返回可区分错误，而非无限递归或无限重试。
func TestExecutorTimeoutExpiry(t *testing.T) {
	executor := NewExecutor(NewMemoryStore())
	deadline := errors.New("deadline")
	_, err := executor.Execute(context.Background(), Execution{
		Key: "expired", Timeout: 20 * time.Millisecond,
		Policy: resilience.RetryPolicy{MaxAttempts: 2, InitialWait: time.Millisecond, MaxWait: time.Millisecond},
		Operation: func(ctx context.Context) (any, error) {
			<-ctx.Done()
			return nil, deadline
		},
	})
	if err == nil {
		t.Fatal("want timeout error")
	}
	if !errors.Is(err, ErrRetryExhausted) {
		t.Fatalf("error=%v want ErrRetryExhausted", err)
	}
}
