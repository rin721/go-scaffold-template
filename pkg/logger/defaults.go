package logger

const (
	// DefaultEnvironment 是未显式配置环境时使用的默认环境。
	DefaultEnvironment Environment = EnvironmentDevelopment
	// DefaultLevel 是未显式配置级别时使用的默认日志级别。
	DefaultLevel Level = LevelInfo
)

// DefaultConfig 返回一份可修改的默认配置副本。
//
// Encoding、AddCaller 和 AddStacktrace 会在 New 中按 Environment 补全，
// 这样调用方修改 Environment 后仍能获得对应环境的默认行为。
func DefaultConfig() Config {
	return Config{
		Environment:      DefaultEnvironment,
		Level:            DefaultLevel,
		OutputPaths:      []string{outputPathStdout},
		ErrorOutputPaths: []string{outputPathStderr},
	}
}

func defaultConfigFor(environment Environment) resolvedConfig {
	cfg := resolvedConfig{
		Environment:      environment,
		Level:            DefaultLevel,
		OutputPaths:      []string{outputPathStdout},
		ErrorOutputPaths: []string{outputPathStderr},
		AddCaller:        true,
		AddStacktrace:    false,
	}

	if environment == EnvironmentProduction {
		cfg.Encoding = EncodingJSON
		cfg.AddStacktrace = true
		return cfg
	}

	cfg.Encoding = EncodingConsole
	return cfg
}
