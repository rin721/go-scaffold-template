package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDoRetriesRetryableError(t *testing.T) {
	fail := errors.New("temporary")
	attempts := 0
	err := Do(context.Background(), RetryPolicy{MaxAttempts: 3, InitialWait: time.Nanosecond}, func(context.Context) error {
		attempts++
		if attempts < 3 {
			return fail
		}
		return nil
	})
	if err != nil || attempts != 3 {
		t.Fatalf("Do() err=%v attempts=%d", err, attempts)
	}
}

func TestBreakerOpensAfterThreshold(t *testing.T) {
	breaker := NewBreaker(1)
	_ = breaker.Do(func() error { return errors.New("down") })
	if err := breaker.Do(func() error { return nil }); err == nil {
		t.Fatal("breaker accepted call after opening")
	}
}
