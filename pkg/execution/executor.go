package execution

import (
	"context"
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
	claimed, err := e.store.Claim(ctx, exec.Key, claimTTL(exec), now)
	if err != nil {
		return Result{}, WrapBackend(err)
	}
	if !claimed {
		return Result{Status: StatusRunning}, ErrAlreadyRunning
	}
	started := e.now()
	call := func(c context.Context) error {
		_, err := exec.Operation(c)
		return err
	}

	// 单次操作超时（0 = 不额外超时）。
	if exec.Timeout > 0 {
		call = func(c context.Context) error {
			return resilience.WithTimeout(c, resilience.TimeoutPolicy{Timeout: exec.Timeout}, call)
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
		if err := e.store.Complete(ctx, exec.Key, rec); err != nil {
			return Result{Status: StatusCompleted}, WrapBackend(err)
		}
		return Result{Status: StatusCompleted}, nil
	}
	rec.Error = runErr.Error()
	if err := e.store.Record(ctx, exec.Key, rec); err != nil {
		return Result{Status: StatusFailed}, WrapBackend(err)
	}
	return Result{Status: StatusFailed}, WrapRetryExhausted(runErr)
}

// claimTTL 返回占用 TTL；未配置时使用 0（不设过期，完成时保留）。
func claimTTL(exec Execution) time.Duration {
	if t, ok := execTTL(exec); ok {
		return t
	}
	return 0
}

// execTTL 从 Execution 提取可配置占用 TTL（Design 预留字段；当前键无内置 TTL 配置时返回 false）。
// 这是 035 计划中的占位约定：TTL 语义由 kernel/app/execution 的配置注入；此处保持 0 即"不设过期"。
func execTTL(Execution) (time.Duration, bool) { return 0, false }

func validateExecution(exec Execution) error {
	if exec.Key == "" {
		return ErrEmptyKey
	}
	if exec.Operation == nil {
		return ErrNilOperation
	}
	return nil
}
