// Package logger 定义由 Kernel 治理的 Logger App 组件。
package logger

import (
	"context"
	"fmt"

	"github.com/rin721/go-scaffold2/internal/kernel/app"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	kernellogging "github.com/rin721/go-scaffold2/internal/kernel/logging"
	pkglogger "github.com/rin721/go-scaffold2/pkg/logger"
)

const (
	ID         app.ID = "logger"
	ConfigPath        = "logger"
)

// Access 是调用方接收的稳定日志租约入口。
type Access interface {
	Use(context.Context, func(pkglogger.Logger) error) error
}

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
type dependencies struct{ manager *kernellogging.Manager }

// Definition 返回无安装副作用的 Logger 组件声明。
func Definition(manager *kernellogging.Manager) (app.Definition[Access], error) {
	if manager == nil {
		return app.Definition[Access]{}, fmt.Errorf("kernel logging manager is nil")
	}
	source, err := app.Configured(ConfigPath, decode, defaults{})
	if err != nil {
		return app.Definition[Access]{}, err
	}
	return app.ManagedConfigured(
		ID,
		source,
		app.FixedDependencies(dependencies{manager: manager}),
		build,
		app.Leased(newAccess),
		app.KernelInstanceSwap,
		app.WithStop(stop),
		app.WithActivation(activate(manager), deactivate(manager)),
	)
}

type access struct{ delegate app.Lease[*instance] }

func newAccess(delegate app.Lease[*instance]) (Access, error) {
	if delegate == nil {
		return nil, fmt.Errorf("logger lease is nil")
	}
	return &access{delegate: delegate}, nil
}

func (a *access) Use(ctx context.Context, use func(pkglogger.Logger) error) error {
	if use == nil {
		return fmt.Errorf("logger access callback is nil")
	}
	return a.delegate.Use(ctx, func(current *instance) error {
		if current == nil || current.resource == nil {
			return fmt.Errorf("logger instance is nil")
		}
		return use(current.resource)
	})
}

func build(ctx context.Context, cfg Config, deps dependencies) (*instance, error) {
	if ctx == nil {
		return nil, app.ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if deps.manager == nil {
		return nil, fmt.Errorf("kernel logging manager is nil")
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

func activate(manager *kernellogging.Manager) func(*instance) {
	return func(current *instance) { manager.Replace(current.resource) }
}

func deactivate(manager *kernellogging.Manager) func(*instance) {
	return func(*instance) { manager.Restore() }
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

var _ Access = (*access)(nil)
var _ config.DefaultContract = defaults{}
