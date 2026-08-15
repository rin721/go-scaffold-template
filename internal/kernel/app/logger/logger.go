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
	values := pkglogger.DefaultConfig()
	return config.Object{
		config.FieldOf("environment", config.String(string(values.Environment))),
		config.FieldOf("level", config.String(string(values.Level))),
		config.FieldOf("outputPaths", stringList(values.OutputPaths)),
		config.FieldOf("errorOutputPaths", stringList(values.ErrorOutputPaths)),
	}, config.Continue, nil
}

func defaultConfig() Config {
	values := pkglogger.DefaultConfig()
	return Config{
		Environment: values.Environment, Level: values.Level, Encoding: values.Encoding,
		OutputPaths:      append([]string(nil), values.OutputPaths...),
		ErrorOutputPaths: append([]string(nil), values.ErrorOutputPaths...),
		AddCaller:        values.AddCaller, AddStacktrace: values.AddStacktrace,
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
