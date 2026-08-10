package logger

import (
	"context"
	"sync"
)

// Noop 返回丢弃所有日志的实现。
func Noop() Logger {
	return noopLogger{}
}

type noopLogger struct{}

func (noopLogger) Debug(string, ...Field) {}
func (noopLogger) Info(string, ...Field)  {}
func (noopLogger) Warn(string, ...Field)  {}
func (noopLogger) Error(string, ...Field) {}
func (l noopLogger) With(...Field) Logger { return l }
func (noopLogger) Sync() error            { return nil }

// Entry 是测试日志条目。
type Entry struct {
	Level   string
	Message string
	Fields  []Field
}

// TestLogger 记录内存日志，供测试断言。
type TestLogger struct {
	mu      sync.Mutex
	entries []Entry
}

func NewTestLogger() *TestLogger {
	return &TestLogger{}
}

func (l *TestLogger) Debug(msg string, fields ...Field) { l.add("debug", msg, fields...) }
func (l *TestLogger) Info(msg string, fields ...Field)  { l.add("info", msg, fields...) }
func (l *TestLogger) Warn(msg string, fields ...Field)  { l.add("warn", msg, fields...) }
func (l *TestLogger) Error(msg string, fields ...Field) { l.add("error", msg, fields...) }
func (l *TestLogger) With(fields ...Field) Logger {
	child := NewTestLogger()
	child.entries = append(child.entries, Entry{Level: "with", Fields: append([]Field(nil), fields...)})
	return child
}
func (l *TestLogger) Sync() error { return nil }

func (l *TestLogger) Entries() []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]Entry(nil), l.entries...)
}

func (l *TestLogger) add(level string, msg string, fields ...Field) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, Entry{Level: level, Message: msg, Fields: append([]Field(nil), fields...)})
}

// WithContext 合并调用方从 context 提取出的日志字段。
func WithContext(ctx context.Context, base Logger, fields ...Field) Logger {
	if base == nil {
		base = Noop()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return base.With(fields...)
}

// AuditField 标记审计日志字段。
func AuditField(key string, value string) Field {
	return String("audit."+key, value)
}
