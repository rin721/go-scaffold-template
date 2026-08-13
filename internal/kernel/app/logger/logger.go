// Package logger 定义可替换主槽位或作为独立实例运行的 Logger App 组件。
package logger

import (
	"context"
	"fmt"

	"github.com/rin721/go-scaffold2/internal/kernel/app"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkglogger "github.com/rin721/go-scaffold2/pkg/logger"
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

// Replacement 创建只用于显式替换 Logger Role 的运行期声明。
func Replacement(spec app.Spec) (app.ReplacementDefinition[pkglogger.Logger], error) {
	source, err := source(spec)
	if err != nil {
		return app.ReplacementDefinition[pkglogger.Logger]{}, err
	}
	return app.ManagedReplacement(
		spec, source, app.FixedDependencies(struct{}{}), build,
		func(resource pkglogger.Resource) (pkglogger.Logger, error) { return resource, nil },
		app.WithStop(stop),
	)
}

// Instance 创建拥有独立 Binding 与生命周期的 Logger 实例声明。
func Instance(spec app.Spec) (app.Definition[pkglogger.Access], error) {
	source, err := source(spec)
	if err != nil {
		return app.Definition[pkglogger.Access]{}, err
	}
	return app.ManagedConfigured(
		spec.ID, source, app.FixedDependencies(struct{}{}), build,
		app.Leased(newAccess), app.KernelInstanceSwap, app.WithStop(stop),
	)
}

func source(spec app.Spec) (app.ConfiguredSource[Config], error) {
	if err := spec.ValidateConfigured(); err != nil {
		return app.ConfiguredSource[Config]{}, err
	}
	return app.Configured(spec.ConfigPath, decoder(spec.ConfigPath), defaults{})
}

func build(ctx context.Context, cfg Config, _ struct{}) (pkglogger.Resource, error) {
	if ctx == nil {
		return nil, app.ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	packageConfig := cfg.packageConfig()
	return pkglogger.New(&packageConfig)
}

func stop(_ context.Context, current pkglogger.Resource) error {
	if current == nil {
		return fmt.Errorf("logger resource is nil")
	}
	return current.Close()
}

type access struct{ delegate app.Lease[pkglogger.Resource] }

func newAccess(delegate app.Lease[pkglogger.Resource]) (pkglogger.Access, error) {
	if delegate == nil {
		return nil, fmt.Errorf("logger lease is nil")
	}
	return &access{delegate: delegate}, nil
}

func (a *access) Use(ctx context.Context, use func(pkglogger.Logger) error) error {
	if ctx == nil {
		return app.ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if use == nil {
		return fmt.Errorf("logger access callback is nil")
	}
	return a.delegate.Use(ctx, func(current pkglogger.Resource) error {
		if current == nil {
			return fmt.Errorf("logger instance is nil")
		}
		return use(current)
	})
}

func decoder(path string) app.Decoder[Config] {
	return func(snapshot config.Snapshot) (Config, error) {
		cfg := defaultConfig()
		if err := snapshot.DecodeSection(path, &cfg); err != nil {
			return Config{}, err
		}
		packageConfig := cfg.packageConfig()
		if err := pkglogger.ValidateConfig(&packageConfig); err != nil {
			return Config{}, err
		}
		return cfg, nil
	}
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

var _ pkglogger.Access = (*access)(nil)
var _ config.DefaultContract = defaults{}
