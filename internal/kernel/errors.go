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
