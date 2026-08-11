package logger

import (
	"fmt"
	"strings"

	"go.uber.org/zap/zapcore"
)

func resolveConfig(cfg *Config) (resolvedConfig, error) {
	environment := DefaultEnvironment
	if cfg != nil && cfg.Environment != "" {
		environment = cfg.Environment
	}

	environment, err := normalizeEnvironment(environment)
	if err != nil {
		return resolvedConfig{}, err
	}

	resolved := defaultConfigFor(environment)
	if cfg == nil {
		return resolved, nil
	}

	if cfg.Level != "" {
		level, err := normalizeLevel(cfg.Level)
		if err != nil {
			return resolvedConfig{}, err
		}
		resolved.Level = level
	}

	if cfg.Encoding != "" {
		encoding, err := normalizeEncoding(cfg.Encoding)
		if err != nil {
			return resolvedConfig{}, err
		}
		resolved.Encoding = encoding
	}

	if len(cfg.OutputPaths) > 0 {
		resolved.OutputPaths = cloneStrings(cfg.OutputPaths)
	}
	if len(cfg.ErrorOutputPaths) > 0 {
		resolved.ErrorOutputPaths = cloneStrings(cfg.ErrorOutputPaths)
	}
	if cfg.AddCaller != nil {
		resolved.AddCaller = *cfg.AddCaller
	}
	if cfg.AddStacktrace != nil {
		resolved.AddStacktrace = *cfg.AddStacktrace
	}
	if err := validateOutputPaths(resolved.OutputPaths); err != nil {
		return resolvedConfig{}, fmt.Errorf("validate logger output paths: %w", err)
	}
	if err := validateOutputPaths(resolved.ErrorOutputPaths); err != nil {
		return resolvedConfig{}, fmt.Errorf("validate logger error output paths: %w", err)
	}

	return resolved, nil
}

func buildEncoderConfig(cfg resolvedConfig) zapcore.EncoderConfig {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        encoderTimeKey,
		LevelKey:       encoderLevelKey,
		NameKey:        encoderLoggerNameKey,
		CallerKey:      encoderCallerKey,
		MessageKey:     encoderMessageKey,
		StacktraceKey:  encoderStacktraceKey,
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	if cfg.Encoding == EncodingConsole {
		encoderConfig.ConsoleSeparator = "\t"
	}

	return encoderConfig
}

func normalizeEnvironment(environment Environment) (Environment, error) {
	switch Environment(strings.ToLower(string(environment))) {
	case EnvironmentDevelopment:
		return EnvironmentDevelopment, nil
	case EnvironmentProduction:
		return EnvironmentProduction, nil
	default:
		return "", fmt.Errorf("unsupported log environment %q", environment)
	}
}

func normalizeEncoding(encoding Encoding) (Encoding, error) {
	switch Encoding(strings.ToLower(string(encoding))) {
	case EncodingConsole:
		return EncodingConsole, nil
	case EncodingJSON:
		return EncodingJSON, nil
	default:
		return "", fmt.Errorf("unsupported log encoding %q", encoding)
	}
}

func cloneStrings(values []string) []string {
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func validateOutputPaths(paths []string) error {
	for index, path := range paths {
		if strings.TrimSpace(path) == "" {
			return fmt.Errorf("output path %d is empty", index)
		}
		if strings.Contains(path, "://") {
			return fmt.Errorf("output path %q uses an unsupported sink scheme", path)
		}
	}
	return nil
}
