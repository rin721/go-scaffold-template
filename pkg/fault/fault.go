package fault

import (
	"context"
	"errors"
	"fmt"
)

// Code 表示跨基础能力复用的错误分类。
type Code string

const (
	CodeInvalidArgument Code = "invalid_argument"
	CodeNotFound        Code = "not_found"
	CodeConflict        Code = "conflict"
	CodeUnavailable     Code = "unavailable"
	CodeTimeout         Code = "timeout"
	CodeCanceled        Code = "canceled"
	CodeInternal        Code = "internal"
)

// Error 是保留错误链和项目语义的错误类型。
type Error struct {
	Code      Code
	Op        string
	Message   string
	Retryable bool
	Cause     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	prefix := string(e.Code)
	if e.Op != "" {
		prefix = e.Op + ": " + prefix
	}
	if e.Message != "" {
		prefix += ": " + e.Message
	}
	if e.Cause != nil {
		return prefix + ": " + e.Cause.Error()
	}
	return prefix
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// New 构造带分类的错误。
func New(code Code, message string) error {
	return &Error{Code: code, Message: message}
}

// Wrap 为底层错误增加操作、分类和可重试语义。
func Wrap(err error, code Code, op string, retryable bool) error {
	if err == nil {
		return nil
	}
	return &Error{Code: code, Op: op, Retryable: retryable, Cause: err}
}

// CodeOf 提取错误分类；没有项目分类时根据 context 错误做保守映射。
func CodeOf(err error) Code {
	if err == nil {
		return ""
	}
	var classified *Error
	if errors.As(err, &classified) {
		return classified.Code
	}
	switch {
	case errors.Is(err, context.Canceled):
		return CodeCanceled
	case errors.Is(err, context.DeadlineExceeded):
		return CodeTimeout
	default:
		return CodeInternal
	}
}

// Retryable 判断错误是否可由调用方安全重试。
func Retryable(err error) bool {
	var classified *Error
	return errors.As(err, &classified) && classified.Retryable
}

// JoinClose 保留主错误和资源关闭错误。
func JoinClose(primary error, label string, closeErr error) error {
	if closeErr == nil {
		return primary
	}
	wrappedClose := fmt.Errorf("%s close: %w", label, closeErr)
	return errors.Join(primary, wrappedClose)
}
