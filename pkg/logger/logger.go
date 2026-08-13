package logger

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Access 为调用方提供当前 Logger 代际的受控租约。
// callback 返回后不得继续保存或使用其中收到的 Logger。
type Access interface {
	Use(context.Context, func(Logger) error) error
}

// Logger 定义业务代码使用的日志能力契约。
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	With(fields ...Field) Logger
}

// Resource 表示由构造方独占生命周期的 logger 资源。
//
// 业务调用方只应依赖 Logger；创建 Resource 的入口或 Capability 负责关闭。
type Resource interface {
	Logger
	Sync() error
	Close() error
}

type zapLogger struct {
	logger *zap.Logger
	state  *resourceState
}

type resourceState struct {
	syncers   []zapcore.WriteSyncer
	closers   []io.Closer
	closeOnce sync.Once
	closeErr  error
}

// ValidateConfig 在不创建 logger 或打开输出文件的前提下校验配置。
func ValidateConfig(cfg *Config) error {
	if _, err := resolveConfig(cfg); err != nil {
		return fmt.Errorf("resolve logger config: %w", err)
	}
	return nil
}

// New 根据配置创建由调用方负责关闭的 logger Resource。
func New(cfg *Config) (Resource, error) {
	resolved, err := resolveConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("resolve logger config: %w", err)
	}

	level, err := toZapLevel(resolved.Level)
	if err != nil {
		return nil, fmt.Errorf("resolve logger level: %w", err)
	}
	sinks := newSinkSet()
	output, err := sinks.open(resolved.OutputPaths)
	if err != nil {
		return nil, fmt.Errorf("open logger output: %w", errors.Join(err, sinks.close()))
	}
	errorOutput, err := sinks.open(resolved.ErrorOutputPaths)
	if err != nil {
		return nil, fmt.Errorf("open logger error output: %w", errors.Join(err, sinks.close()))
	}

	encoderConfig := buildEncoderConfig(resolved)
	var encoder zapcore.Encoder
	if resolved.Encoding == EncodingJSON {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}
	options := []zap.Option{zap.ErrorOutput(errorOutput)}
	if resolved.Environment == EnvironmentDevelopment {
		options = append(options, zap.Development())
	}
	if resolved.AddCaller {
		options = append(options, zap.AddCaller())
	}
	if resolved.AddStacktrace {
		stacktraceLevel := zapcore.ErrorLevel
		if resolved.Environment == EnvironmentDevelopment {
			stacktraceLevel = zapcore.WarnLevel
		}
		options = append(options, zap.AddStacktrace(stacktraceLevel))
	}

	underlying := zap.New(zapcore.NewCore(encoder, output, level), options...)
	state := &resourceState{syncers: sinks.syncers, closers: sinks.closers}
	return &zapLogger{logger: underlying, state: state}, nil
}

type sinkSet struct {
	writers map[string]zapcore.WriteSyncer
	syncers []zapcore.WriteSyncer
	closers []io.Closer
}

func newSinkSet() *sinkSet {
	return &sinkSet{writers: make(map[string]zapcore.WriteSyncer)}
}

func (s *sinkSet) open(paths []string) (zapcore.WriteSyncer, error) {
	writers := make([]zapcore.WriteSyncer, 0, len(paths))
	for _, path := range paths {
		writer, exists := s.writers[path]
		if !exists {
			opened, closer, err := openSink(path)
			if err != nil {
				return nil, err
			}
			writer = zapcore.Lock(opened)
			s.writers[path] = writer
			s.syncers = append(s.syncers, writer)
			if closer != nil {
				s.closers = append(s.closers, closer)
			}
		}
		writers = append(writers, writer)
	}
	return zapcore.NewMultiWriteSyncer(writers...), nil
}

func (s *sinkSet) close() error {
	var joined error
	for index := len(s.closers) - 1; index >= 0; index-- {
		joined = errors.Join(joined, s.closers[index].Close())
	}
	return joined
}

func openSink(path string) (zapcore.WriteSyncer, io.Closer, error) {
	switch path {
	case outputPathStdout:
		return nonClosingWriteSyncer{Writer: os.Stdout}, nil, nil
	case outputPathStderr:
		return nonClosingWriteSyncer{Writer: os.Stderr}, nil, nil
	default:
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, defaultLogFileMode)
		if err != nil {
			return nil, nil, fmt.Errorf("open log file %q: %w", path, err)
		}
		return zapcore.AddSync(file), file, nil
	}
}

type nonClosingWriteSyncer struct {
	io.Writer
}

func (nonClosingWriteSyncer) Sync() error { return nil }

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
	return &zapLogger{logger: l.logger.With(toZapFields(fields)...), state: l.state}
}

func (l *zapLogger) Sync() error {
	return syncAll(l.state.syncers)
}

func (l *zapLogger) Close() error {
	l.state.closeOnce.Do(func() {
		l.state.closeErr = errors.Join(syncAll(l.state.syncers), closeAll(l.state.closers))
	})
	return l.state.closeErr
}

func syncAll(syncers []zapcore.WriteSyncer) error {
	var joined error
	for _, syncer := range syncers {
		joined = errors.Join(joined, syncer.Sync())
	}
	return joined
}

func closeAll(closers []io.Closer) error {
	var joined error
	for index := len(closers) - 1; index >= 0; index-- {
		joined = errors.Join(joined, closers[index].Close())
	}
	return joined
}
