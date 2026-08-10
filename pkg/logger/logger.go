package logger

import (
	"fmt"

	"go.uber.org/zap"
)

// Logger 定义业务代码使用的日志能力契约。
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	With(fields ...Field) Logger
	Sync() error
}

type zapLogger struct {
	logger *zap.Logger
}

// New 根据配置创建 logger。
func New(cfg *Config) (Logger, error) {
	resolved, err := resolveConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("resolve logger config: %w", err)
	}

	zapConfig, err := buildZapConfig(resolved)
	if err != nil {
		return nil, fmt.Errorf("build zap config: %w", err)
	}

	underlying, err := zapConfig.Build()
	if err != nil {
		return nil, fmt.Errorf("create logger: %w", err)
	}

	return &zapLogger{logger: underlying}, nil
}

func (l *zapLogger) Debug(msg string, fields ...Field) {
	l.logger.Debug(msg, toZapFields(fields)...)
}

func (l *zapLogger) Info(msg string, fields ...Field) {
	l.logger.Info(msg, toZapFields(fields)...)
}

func (l *zapLogger) Warn(msg string, fields ...Field) {
	l.logger.Warn(msg, toZapFields(fields)...)
}

func (l *zapLogger) Error(msg string, fields ...Field) {
	l.logger.Error(msg, toZapFields(fields)...)
}

func (l *zapLogger) With(fields ...Field) Logger {
	return &zapLogger{logger: l.logger.With(toZapFields(fields)...)}
}

func (l *zapLogger) Sync() error {
	return ignoreSyncError(l.logger.Sync())
}
