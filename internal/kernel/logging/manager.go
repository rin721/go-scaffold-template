// Package logging 提供 Kernel 启动前后始终稳定的日志委托入口。
package logging

import (
	"fmt"
	"sync"

	pkglogger "github.com/rin721/go-scaffold2/pkg/logger"
)

// Target 是 composition 用来声明 Kernel 内置 Logger 替换关系的 typed target。
// 普通消费者只接收 pkglogger.Logger，不应取得 Replace 或 Restore 权限。
type Target interface {
	Logger() pkglogger.Logger
	Replace(pkglogger.Logger)
	Restore()
}

// Manager 在基线 logger 与当前配置化 logger 之间提供并发安全的稳定入口。
//
// Manager 不拥有任何 logger 的关闭责任；基线由应用入口关闭，配置化实例由
// Logger Replacement App 关闭。
type Manager struct {
	mu       sync.RWMutex
	baseline pkglogger.Logger
	current  pkglogger.Logger
	view     pkglogger.Logger
}

// New 创建使用必填基线 logger 的 Manager。
func New(baseline pkglogger.Logger) (*Manager, error) {
	if baseline == nil {
		return nil, fmt.Errorf("kernel baseline logger is nil")
	}
	manager := &Manager{baseline: baseline, current: baseline}
	manager.view = &boundLogger{manager: manager}
	return manager, nil
}

// Logger 返回不带替换权的稳定只读 view。
func (m *Manager) Logger() pkglogger.Logger {
	if m == nil {
		return nil
	}
	return m.view
}

// Replace 在不可失败的 Kernel 提交区切换到已完成构造的 logger。
func (m *Manager) Replace(next pkglogger.Logger) {
	if next == nil {
		panic("kernel logging replacement is nil")
	}
	m.mu.Lock()
	m.current = next
	m.mu.Unlock()
}

// Restore 把委托目标恢复为启动前基线 logger。
func (m *Manager) Restore() {
	m.mu.Lock()
	m.current = m.baseline
	m.mu.Unlock()
}

func (m *Manager) Debug(message string, fields ...pkglogger.Field) {
	m.write(func(current pkglogger.Logger) { current.Debug(message, fields...) })
}

func (m *Manager) Info(message string, fields ...pkglogger.Field) {
	m.write(func(current pkglogger.Logger) { current.Info(message, fields...) })
}

func (m *Manager) Warn(message string, fields ...pkglogger.Field) {
	m.write(func(current pkglogger.Logger) { current.Warn(message, fields...) })
}

func (m *Manager) Error(message string, fields ...pkglogger.Field) {
	m.write(func(current pkglogger.Logger) { current.Error(message, fields...) })
}

func (m *Manager) With(fields ...pkglogger.Field) pkglogger.Logger {
	return &boundLogger{manager: m, fields: append([]pkglogger.Field(nil), fields...)}
}

func (m *Manager) write(write func(pkglogger.Logger)) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	write(m.current)
}

type boundLogger struct {
	manager *Manager
	fields  []pkglogger.Field
}

func (l *boundLogger) Debug(message string, fields ...pkglogger.Field) {
	l.write(func(current pkglogger.Logger, merged []pkglogger.Field) { current.Debug(message, merged...) }, fields)
}

func (l *boundLogger) Info(message string, fields ...pkglogger.Field) {
	l.write(func(current pkglogger.Logger, merged []pkglogger.Field) { current.Info(message, merged...) }, fields)
}

func (l *boundLogger) Warn(message string, fields ...pkglogger.Field) {
	l.write(func(current pkglogger.Logger, merged []pkglogger.Field) { current.Warn(message, merged...) }, fields)
}

func (l *boundLogger) Error(message string, fields ...pkglogger.Field) {
	l.write(func(current pkglogger.Logger, merged []pkglogger.Field) { current.Error(message, merged...) }, fields)
}

func (l *boundLogger) With(fields ...pkglogger.Field) pkglogger.Logger {
	merged := make([]pkglogger.Field, 0, len(l.fields)+len(fields))
	merged = append(merged, l.fields...)
	merged = append(merged, fields...)
	return &boundLogger{manager: l.manager, fields: merged}
}

func (l *boundLogger) write(
	write func(pkglogger.Logger, []pkglogger.Field),
	fields []pkglogger.Field,
) {
	merged := make([]pkglogger.Field, 0, len(l.fields)+len(fields))
	merged = append(merged, l.fields...)
	merged = append(merged, fields...)
	l.manager.write(func(current pkglogger.Logger) { write(current, merged) })
}

var _ pkglogger.Logger = (*Manager)(nil)
var _ Target = (*Manager)(nil)
var _ pkglogger.Logger = (*boundLogger)(nil)
