package execution

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rin721/go-scaffold-template/pkg/concurrency"
	"github.com/rin721/go-scaffold-template/pkg/fault"
	"github.com/rin721/go-scaffold-template/pkg/resilience"
)

// executor 组合幂等占用、失败重试与执行记录，输出稳定 OperationExecutor 契约。
type executor struct {
	store Store
	sf    concurrency.SingleFlight
	now   func() time.Time
}

var _ OperationExecutor = (*executor)(nil)

// NewExecutor 返回基于给定 Store 的操作执行器（默认 Store 供测试用 MemoryStore）。
func NewExecutor(store Store) OperationExecutor {
	return &executor{store: store, now: time.Now}
}

// Execute 执行一次受幂等 / 重试 / 执行记录托管的操作。
func (e *executor) Execute(ctx context.Context, exec Execution) (Result, error) {
	if err := validateExecution(exec); err != nil {
		return Result{}, err
	}
	// 幂等：已完成则直接返回重复，不再执行。
	done, err := e.store.IsCompleted(ctx, exec.Key)
	if err != nil {
		return Result{}, WrapBackend(err)
	}
	if done {
		return Result{Status: StatusCompleted, Duplicate: true}, nil
	}
	// 同进程同 key 并发合并：仅一个 goroutine 执行，其余复用结果。
	value, err, _ := e.sf.Do(string(exec.Key), func() (any, error) {
		return e.run(ctx, exec)
	})
	if err != nil {
		if res, ok := value.(Result); ok {
			return res, err
		}
		return Result{}, err
	}
	res, _ := value.(Result)
	return res, nil
}

// run 在占用保护下执行操作：占用 -> 带策略重试 -> 成功/失败记录。
func (e *executor) run(ctx context.Context, exec Execution) (any, error) {
	now := e.now()
	claimed, err := e.store.Claim(ctx, exec.Key, exec.LeaseTTL, now)
	if err != nil {
		return Result{}, WrapBackend(err)
	}
	if !claimed {
		return Result{Status: StatusRunning}, ErrAlreadyRunning
	}
	started := e.now()
	// operation 是原始业务执行体；call 是最终传给重试引擎的操作（可能包一层超时）。
	operation := func(c context.Context) error {
		_, err := exec.Operation(c)
		return err
	}
	call := operation

	// 单次操作超时（0 = 不额外超时）。超时包装的闭包必须引用原始的 operation，
	// 而不是被重新赋值的 call，否则会无限自递归（复用 Timer 的 WithTimeout 层层再包）。
	if exec.Timeout > 0 {
		call = func(c context.Context) error {
			return resilience.WithTimeout(c, resilience.TimeoutPolicy{Timeout: exec.Timeout}, operation)
		}
	}
	// 重试仅对可重试错误使用 pkg/resilience.Do。
	policy := exec.Policy
	if policy.Retryable == nil {
		policy.Retryable = fault.Retryable
	}
	runErr := resilience.Do(ctx, policy, call)

	rec := Record{
		Key: exec.Key, Status: StatusFailed,
		Trigger: exec.Trigger, Trace: TraceFrom(ctx),
		Duration: e.now().Sub(started), CreatedAt: e.now(),
	}
	if runErr == nil {
		rec.Status = StatusCompleted
		if err := e.store.Complete(ctx, exec.Key, exec.RetentionTTL, rec); err != nil {
			return Result{Status: StatusCompleted}, WrapBackend(err)
		}
		return Result{Status: StatusCompleted}, nil
	}
	rec.Error = runErr.Error()
	releaseErr := e.store.Release(ctx, exec.Key)
	recordErr := e.store.Record(ctx, exec.Key, rec)
	return Result{Status: StatusFailed}, errors.Join(
		WrapRetryExhausted(runErr),
		WrapBackend(releaseErr),
		WrapBackend(recordErr),
	)
}

func validateExecution(exec Execution) error {
	if exec.Key == "" {
		return ErrEmptyKey
	}
	if exec.Operation == nil {
		return ErrNilOperation
	}
	if exec.LeaseTTL < 0 {
		return fmt.Errorf("execution: lease ttl must be non-negative")
	}
	if exec.RetentionTTL < 0 {
		return fmt.Errorf("execution: retention ttl must be non-negative")
	}
	return nil
}
