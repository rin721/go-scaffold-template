// Package logger 定义由 Kernel 治理的 Logger App 组件。
package logger

import (
	"context"
	"fmt"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	kernellogging "github.com/rin721/go-scaffold-template/internal/kernel/logging"
	pkglogger "github.com/rin721/go-scaffold-template/pkg/logger"
)

const (
	ID         app.ID = "logger.configured"
	ConfigPath        = "logger"
)

// 应用层默认配置（032 集中声明）：本组件在配置中集中声明日志应用默认值，
// 不再直接复用 pkg/logger 的 DefaultConfig()；需要回退到通用库默认时由组件显式引用。
const (
	defaultEnvironment     = pkglogger.EnvironmentDevelopment
	defaultLevel           = pkglogger.LevelDebug
	defaultOutputPath      = "stdout"
	defaultErrorOutputPath = "stderr"
)

// Config 是 Logger App 的 typed 配置契约。
type Config struct {
	Environment      pkglogger.Environment `mapstructure:"environment"`
	Level            pkglogger.Level       `mapstructure:"level"`
	Encoding         pkglogger.Encoding    `mapstructure:"encoding"`
	OutputPaths      []string              `mapstructure:"outputPaths"`
	ErrorOutputPaths []string              `mapstructure:"errorOutputPaths"`
	AddCaller        *bool                 `mapstructure:"addCaller"`
	AddStacktrace    *bool                 `mapstructure:"addStacktrace"`
}

type instance struct{ resource pkglogger.Resource }

// Replacement 返回明确替换 Kernel 内置 Logger target 的配置化组件声明。
func Replacement() (app.ReplacementDefinition[kernellogging.Target], error) {
	source, err := app.Configured(ConfigPath, decode, defaults{})
	if err != nil {
		return app.ReplacementDefinition[kernellogging.Target]{}, err
	}
	return app.ManagedConfiguredReplacement(
		ID,
		source,
		build,
		activate,
		deactivate,
		app.WithTerminalFinalizer(stop),
	)
}

func build(ctx context.Context, cfg Config, target kernellogging.Target) (*instance, error) {
	if ctx == nil {
		return nil, app.ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if target == nil {
		return nil, fmt.Errorf("kernel logging target is nil")
	}
	resource, err := pkglogger.New(pointerTo(cfg.packageConfig()))
	if err != nil {
		return nil, err
	}
	return &instance{resource: resource}, nil
}

func stop(_ context.Context, current *instance) error {
	if current == nil || current.resource == nil {
		return fmt.Errorf("logger instance is nil")
	}
	return current.resource.Close()
}

func activate(target kernellogging.Target, current *instance) {
	target.Replace(current.resource)
}

func deactivate(target kernellogging.Target, _ *instance) {
	target.Restore()
}

func decode(snapshot config.Snapshot) (Config, error) {
	cfg := defaultConfig()
	if err := snapshot.DecodeSection(ConfigPath, &cfg); err != nil {
		return Config{}, err
	}
	if _, exists := snapshot.Value(ConfigPath + ".level"); !exists {
		// 缺失 level 继续由 Environment 决定；不能把 development 的默认值
		// 预填后误当成 production 的显式 debug 配置。
		cfg.Level = ""
	}
	packageConfig := cfg.packageConfig()
	if err := pkglogger.ValidateConfig(&packageConfig); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

type defaults struct{}

func (defaults) Defaults(ctx context.Context) (config.Object, config.Control, error) {
	if ctx == nil {
		return nil, config.Continue, app.ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return nil, config.Continue, err
	}
	cfg := defaultConfig()
	return config.Object{
		config.FieldOf("environment", config.String(string(cfg.Environment))),
		config.FieldOf("level", config.String(string(cfg.Level))),
		config.FieldOf("outputPaths", stringList(cfg.OutputPaths)),
		config.FieldOf("errorOutputPaths", stringList(cfg.ErrorOutputPaths)),
	}, config.Continue, nil
}

func defaultConfig() Config {
	return Config{
		Environment:      defaultEnvironment,
		Level:            defaultLevel,
		OutputPaths:      []string{defaultOutputPath},
		ErrorOutputPaths: []string{defaultErrorOutputPath},
	}
}

func (c Config) packageConfig() pkglogger.Config {
	return pkglogger.Config{
		Environment: c.Environment, Level: c.Level, Encoding: c.Encoding,
		OutputPaths:      append([]string(nil), c.OutputPaths...),
		ErrorOutputPaths: append([]string(nil), c.ErrorOutputPaths...),
		AddCaller:        c.AddCaller, AddStacktrace: c.AddStacktrace,
	}
}

func stringList(values []string) config.Value {
	elements := make([]config.Value, 0, len(values))
	for _, value := range values {
		elements = append(elements, config.String(value))
	}
	return config.List(elements...)
}

func pointerTo[T any](value T) *T { return &value }

var _ config.DefaultContract = defaults{}
