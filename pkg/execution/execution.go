// Package execution 提供幂等 / 失败重试 / 执行记录的稳定操作执行契约，供需要这些能力的
// 业务模块（如订单、支付、库存）消费。重试引擎复用 pkg/resilience；可重试判断复用 pkg/fault。
package execution

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rin721/go-scaffold-template/pkg/resilience"
)

// Key 标识一次业务操作的幂等单元（如订单创建、支付扣款、库存扣减）。
type Key string

// Status 描述一次执行的幂等/记录状态。
type Status string

const (
	// StatusCompleted 表示操作已成功完成并记录，幂等已完成。
	StatusCompleted Status = "completed"
	// StatusRunning 表示占用已建立、操作正在执行。
	StatusRunning Status = "running"
	// StatusFailed 表示操作最终失败（重试耗尽或不可重试）。
	StatusFailed Status = "failed"
)

// Result 描述一次 Execute 的结果。
type Result struct {
	Status    Status
	Duplicate bool // 重复提交且已完成时为 true，表示不再执行操作。
	RecordID  string
}

// Operation 是业务执行体，返回业务结果与错误。
type Operation func(ctx context.Context) (any, error)

// Execution 描述一次受托管执行。
type Execution struct {
	Key          Key
	Policy       resilience.RetryPolicy // 复用 pkg/resilience；Retryable 用 fault.Retryable
	Timeout      time.Duration          // 0 表示不额外超时
	LeaseTTL     time.Duration          // 运行占用期限；0 表示由 Store 保持不过期
	RetentionTTL time.Duration          // 成功完成后的去重窗口；0 表示由 Store 保持不过期
	Trigger      string                 // 低敏触发者/来源，用于执行记录
	Operation    Operation
	// PolicyName 是给装配方（Kernel App）使用的命令式策略选择标识：业务模块可通过
	// 独立配置声明命名策略并按此名引用，避免多个模块被绑定到同一套固定策略。
	// 装配方据此解析 Policy/Timeout；pkg 执行器本身不感知该字段。
	PolicyName string
}

// OperationExecutor 是业务模块消费的稳定契约。
type OperationExecutor interface {
	Execute(ctx context.Context, exec Execution) (Result, error)
}

// Record 描述一次执行记录（持久化 / 诊断用），保留结果与错误原因链文本。
type Record struct {
	Key       Key
	Status    Status
	Result    string
	Error     string
	Trigger   string
	Trace     string // 全链路追踪标识（低敏，如 trace/span ID），用于跨系统关联定位
	Duration  time.Duration
	CreatedAt time.Time
}

// Store 持久化幂等占用与执行记录。backend 实现决定是否跨进程可见（内存版仅进程内）。
type Store interface {
	// Claim 尽量为 key 建立运行占用；leaseTTL 只约束 running 状态，不定义完成保留窗口。
	// 返回 claimed=false 表示已有活动占用（同 key 进行中），调用方不应再执行。
	Claim(ctx context.Context, key Key, leaseTTL time.Duration, now time.Time) (claimed bool, err error)
	// IsCompleted 判断 key 是否已完成（幂等重复提交判定）。
	IsCompleted(ctx context.Context, key Key) (bool, error)
	// Complete 记录成功完成，并从 rec.CreatedAt 起建立 retentionTTL 去重窗口。
	Complete(ctx context.Context, key Key, retentionTTL time.Duration, rec Record) error
	// Release 释放当前 key 的 running 占用；completed 状态不得被释放。
	Release(ctx context.Context, key Key) error
	// Record 记录失败/过程记录（保留原因文本）。
	Record(ctx context.Context, key Key, rec Record) error
}

// 通用错误定义（AGENTS 3.3：可区分且保留原因）。
var (
	ErrNilContext     = errors.New("execution: nil context")
	ErrNilOperation   = errors.New("execution: nil operation")
	ErrEmptyKey       = errors.New("execution: empty key")
	ErrBackend        = errors.New("execution: backend unavailable")
	ErrAlreadyRunning = errors.New("execution: operation already running")
	ErrRetryExhausted = errors.New("execution: retry exhausted")
)

// WrapBackend 包装 backend 操作失败，保留原因链。
func WrapBackend(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrBackend, err)
}

// WrapRetryExhausted 包装重试耗尽，保留最后原因。
func WrapRetryExhausted(err error) error {
	if err == nil {
		return err
	}
	return fmt.Errorf("%w: %w", ErrRetryExhausted, err)
}
