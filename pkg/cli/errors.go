package cli

import (
	"context"
	"errors"
	"fmt"
)

// UsageError 表示命令行参数或调用方式错误。
type UsageError struct {
	Command string
	Message string
}

func (e *UsageError) Error() string {
	if e.Command != "" {
		return fmt.Sprintf("%s: %s\nRun '%s --help' for usage", e.Command, e.Message, e.Command)
	}
	return fmt.Sprintf("%s\nRun '--help' for usage", e.Message)
}

func (e *UsageError) ExitCode() int {
	return ExitUsage
}

// CommandError 包装命令执行阶段的错误。
type CommandError struct {
	Command string
	Message string
	Cause   error
}

// ConfigError 标识配置契约、绑定、校验或安全发布失败。
type ConfigError struct {
	Command string
	Stage   string
	Cause   error
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("%s: configuration %s failed: %v", e.Command, e.Stage, e.Cause)
}

func (e *ConfigError) Unwrap() error { return e.Cause }

func (e *ConfigError) ExitCode() int { return ExitConfig }

func (e *CommandError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Command, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Command, e.Message)
}

func (e *CommandError) Unwrap() error {
	return e.Cause
}

func (e *CommandError) ExitCode() int {
	return ExitError
}

// CancelledError 表示用户主动取消操作。
type CancelledError struct {
	Message string
}

func (e *CancelledError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return ErrMsgCancelled
}

func (e *CancelledError) ExitCode() int {
	return ExitInterrupted
}

// ExitCoder 由可以提供进程退出码的错误实现。
type ExitCoder interface {
	error
	ExitCode() int
}

// GetExitCode 返回 err 对应的稳定进程退出码。
func GetExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}
	if errors.Is(err, context.Canceled) {
		return ExitInterrupted
	}
	var exitCoder ExitCoder
	if errors.As(err, &exitCoder) {
		return exitCoder.ExitCode()
	}
	return ExitError
}
