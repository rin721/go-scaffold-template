package logger

import (
	"fmt"
	"strings"

	"go.uber.org/zap/zapcore"
)

// Level 表示 logger 支持的日志级别。
type Level string

const (
	// LevelDebug 输出 debug 及以上级别日志。
	LevelDebug Level = "debug"
	// LevelInfo 输出 info 及以上级别日志。
	LevelInfo Level = "info"
	// LevelWarn 输出 warn 及以上级别日志。
	LevelWarn Level = "warn"
	// LevelError 输出 error 级别日志。
	LevelError Level = "error"
)

func normalizeLevel(level Level) (Level, error) {
	switch Level(strings.ToLower(string(level))) {
	case LevelDebug:
		return LevelDebug, nil
	case LevelInfo:
		return LevelInfo, nil
	case LevelWarn:
		return LevelWarn, nil
	case LevelError:
		return LevelError, nil
	default:
		return "", fmt.Errorf("unsupported log level %q", level)
	}
}

func toZapLevel(level Level) (zapcore.Level, error) {
	switch level {
	case LevelDebug:
		return zapcore.DebugLevel, nil
	case LevelInfo:
		return zapcore.InfoLevel, nil
	case LevelWarn:
		return zapcore.WarnLevel, nil
	case LevelError:
		return zapcore.ErrorLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("unsupported log level %q", level)
	}
}
