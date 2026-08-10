package resilience

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// RetryPolicy 控制重试次数和退避时间。
type RetryPolicy struct {
	MaxAttempts int
	InitialWait time.Duration
	MaxWait     time.Duration
	Retryable   func(error) bool
}

// Do 按策略执行重试。
func Do(ctx context.Context, policy RetryPolicy, operation func(context.Context) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if operation == nil {
		return fmt.Errorf("resilience operation is nil")
	}
	attempts := policy.MaxAttempts
	if attempts <= 0 {
		attempts = 1
	}
	wait := policy.InitialWait
	if wait <= 0 {
		wait = 50 * time.Millisecond
	}
	maxWait := policy.MaxWait
	if maxWait <= 0 {
		maxWait = wait
	}
	var last error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := operation(ctx); err != nil {
			last = err
			if attempt == attempts || !isRetryable(policy, err) {
				return err
			}
			if err := sleep(ctx, boundedWait(wait, maxWait)); err != nil {
				return errors.Join(last, err)
			}
			wait *= 2
			continue
		}
		return nil
	}
	return last
}

func isRetryable(policy RetryPolicy, err error) bool {
	if policy.Retryable == nil {
		return true
	}
	return policy.Retryable(err)
}

func boundedWait(wait time.Duration, maxWait time.Duration) time.Duration {
	if wait > maxWait {
		return maxWait
	}
	return wait
}

func sleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// TimeoutPolicy 控制单次操作超时。
type TimeoutPolicy struct {
	Timeout time.Duration
}

// WithTimeout 在受控超时内执行操作。
func WithTimeout(ctx context.Context, policy TimeoutPolicy, operation func(context.Context) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if policy.Timeout <= 0 {
		return operation(ctx)
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, policy.Timeout)
	defer cancel()
	return operation(timeoutCtx)
}

// Breaker 是轻量熔断器。
type Breaker struct {
	threshold int
	mu        sync.Mutex
	failures  int
	open      bool
}

// NewBreaker 创建按连续失败次数打开的熔断器。
func NewBreaker(threshold int) *Breaker {
	if threshold <= 0 {
		threshold = 3
	}
	return &Breaker{threshold: threshold}
}

func (b *Breaker) Do(operation func() error) error {
	b.mu.Lock()
	if b.open {
		b.mu.Unlock()
		return fmt.Errorf("circuit breaker is open")
	}
	b.mu.Unlock()
	err := operation()
	b.mu.Lock()
	defer b.mu.Unlock()
	if err != nil {
		b.failures++
		if b.failures >= b.threshold {
			b.open = true
		}
		return err
	}
	b.failures = 0
	return nil
}

func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.open = false
}
