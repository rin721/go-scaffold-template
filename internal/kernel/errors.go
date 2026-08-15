package kernel

import (
	"errors"
	"fmt"
)

var (
	// ErrNilContext 表示调用方没有提供可控制取消的 context。
	ErrNilContext = errors.New("kernel context is nil")
	// ErrNotRunning 表示操作要求 kernel 已经完成启动。
	ErrNotRunning = errors.New("kernel is not running")
	// ErrStopped 表示 Kernel 已经永久停止。
	ErrStopped = errors.New("kernel is stopped")
)

// CommittedCleanupError 表示新实例已经发布，但旧实例清理失败。
//
// 收到此错误时不得回滚到旧配置；调用方应上报资源清理故障。
type CommittedCleanupError struct {
	Err error
}

// GenerationOperationError 保留 Application Generation 失败阶段、owner 与原始错误链。
// 日志只应输出这些结构化字段和错误类型，不直接输出可能含凭据的错误文本。
type GenerationOperationError struct {
	Phase      string
	Owner      string
	Generation uint64
	Err        error
}

func (e *GenerationOperationError) Error() string {
	if e == nil {
		return "application generation operation failed"
	}
	if e.Generation == 0 && e.Phase == "load" {
		return fmt.Sprintf("prepare application configuration: application generation load failed for %s: %v", e.Owner, e.Err)
	}
	if e.Generation == 0 {
		return fmt.Sprintf("application generation %s failed for %s: %v", e.Phase, e.Owner, e.Err)
	}
	return fmt.Sprintf("application generation %d %s failed for %s: %v", e.Generation, e.Phase, e.Owner, e.Err)
}

// Unwrap 保留 Load、Validate、Build、Ready、Commit 或 Retire 的原始错误链。
func (e *GenerationOperationError) Unwrap() error { return e.Err }

// DrainIncompleteError 表示终止排空尚未完成，owner 必须保留并允许后续继续 Stop。
type DrainIncompleteError struct {
	Err error
}

func (e *DrainIncompleteError) Error() string {
	return fmt.Sprintf("kernel terminal drain is incomplete: %v", e.Err)
}

// Unwrap 保留调用方 deadline 或取消原因。
func (e *DrainIncompleteError) Unwrap() error { return e.Err }

func (e *CommittedCleanupError) Error() string {
	return fmt.Sprintf("kernel reload committed but cleanup failed: %v", e.Err)
}

// Unwrap 保留旧实例清理错误链。
func (e *CommittedCleanupError) Unwrap() error {
	return e.Err
}
