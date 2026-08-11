// Package logger 定义由 Kernel 托管的 Logger 能力。
package logger

import (
	"context"
	"fmt"

	"github.com/rin721/go-scaffold2/internal/kernel"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	kernellogging "github.com/rin721/go-scaffold2/internal/kernel/logging"
	pkglogger "github.com/rin721/go-scaffold2/pkg/logger"
)

const (
	// ID 是 Logger capability 在 Kernel 内的稳定能力标识。
	ID kernel.ID = "logger"
	// ConfigPath 是 Logger capability 使用的顶层配置路径。
	ConfigPath = "logger"
)

// Access 是业务构造函数接收的稳定日志能力入口。
//
// 回调只获得 Logger，不会获得由 Capability 独占的 Resource.Close。
type Access interface {
	Use(context.Context, func(pkglogger.Logger) error) error
}

// Config 是 Logger capability 的 typed 配置契约。
type Config struct {
	Environment      pkglogger.Environment `mapstructure:"environment"`
	Level            pkglogger.Level       `mapstructure:"level"`
	Encoding         pkglogger.Encoding    `mapstructure:"encoding"`
	OutputPaths      []string              `mapstructure:"outputPaths"`
	ErrorOutputPaths []string              `mapstructure:"errorOutputPaths"`
	AddCaller        *bool                 `mapstructure:"addCaller"`
	AddStacktrace    *bool                 `mapstructure:"addStacktrace"`
}

// Instance 保存单代配置化 logger 及其资源所有权。
//
// 该类型只为 Kernel 泛型 Definition 提供稳定实例类型；业务侧不能取得它。
type Instance struct {
	resource pkglogger.Resource
}

// Definition 返回无注册副作用的 Logger 能力定义。
func Definition(manager *kernellogging.Manager) kernel.Definition[Config, *Instance] {
	implementation := capability{manager: manager}
	return kernel.Definition[Config, *Instance]{
		ID:         ID,
		ConfigPath: ConfigPath,
		Decode:     decode,
		Defaults:   implementation,
		Builder:    implementation,
		Hooks:      implementation,
		Activation: implementation,
	}
}

// NewAccess 把 Kernel 的内部 Instance 租约收敛为业务 Logger 回调。
func NewAccess(delegate kernel.Access[*Instance]) (Access, error) {
	if delegate == nil {
		return nil, fmt.Errorf("logger access delegate is nil")
	}
	return &access{delegate: delegate}, nil
}

type access struct {
	delegate kernel.Access[*Instance]
}

func (a *access) Use(ctx context.Context, use func(pkglogger.Logger) error) error {
	if use == nil {
		return fmt.Errorf("logger access callback is nil")
	}
	return a.delegate.Use(ctx, func(instance *Instance) error {
		if instance == nil || instance.resource == nil {
			return fmt.Errorf("logger capability instance is nil")
		}
		return use(instance.resource)
	})
}

type capability struct {
	manager *kernellogging.Manager
}

func (capability) Defaults(ctx context.Context) (config.Object, config.Control, error) {
	if ctx == nil {
		return nil, config.Continue, fmt.Errorf("logger defaults context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, config.Continue, err
	}
	defaults := pkglogger.DefaultConfig()
	return config.Object{
		config.FieldOf("environment", config.String(string(defaults.Environment))),
		config.FieldOf("level", config.String(string(defaults.Level))),
		config.FieldOf("outputPaths", stringList(defaults.OutputPaths)),
		config.FieldOf("errorOutputPaths", stringList(defaults.ErrorOutputPaths)),
	}, config.Continue, nil
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

func (c capability) Build(ctx context.Context, cfg Config) (*Instance, error) {
	if ctx == nil {
		return nil, fmt.Errorf("logger build context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.manager == nil {
		return nil, fmt.Errorf("kernel logging manager is nil")
	}
	resource, err := pkglogger.New(pointerTo(cfg.packageConfig()))
	if err != nil {
		return nil, err
	}
	return &Instance{resource: resource}, nil
}

func (capability) Start(ctx context.Context, instance *Instance) error {
	if ctx == nil {
		return fmt.Errorf("logger start context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if instance == nil || instance.resource == nil {
		return fmt.Errorf("logger instance is nil")
	}
	return nil
}

func (capability) Stop(_ context.Context, instance *Instance) error {
	if instance == nil || instance.resource == nil {
		return fmt.Errorf("logger instance is nil")
	}
	return instance.resource.Close()
}

func (c capability) Activate(instance *Instance) {
	c.manager.Replace(instance.resource)
}

func (c capability) Deactivate(*Instance) {
	c.manager.Restore()
}

func defaultConfig() Config {
	defaults := pkglogger.DefaultConfig()
	return Config{
		Environment:      defaults.Environment,
		Level:            defaults.Level,
		Encoding:         defaults.Encoding,
		OutputPaths:      append([]string(nil), defaults.OutputPaths...),
		ErrorOutputPaths: append([]string(nil), defaults.ErrorOutputPaths...),
		AddCaller:        defaults.AddCaller,
		AddStacktrace:    defaults.AddStacktrace,
	}
}

func (c Config) packageConfig() pkglogger.Config {
	return pkglogger.Config{
		Environment:      c.Environment,
		Level:            c.Level,
		Encoding:         c.Encoding,
		OutputPaths:      append([]string(nil), c.OutputPaths...),
		ErrorOutputPaths: append([]string(nil), c.ErrorOutputPaths...),
		AddCaller:        c.AddCaller,
		AddStacktrace:    c.AddStacktrace,
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

var _ kernel.Builder[Config, *Instance] = capability{}
var _ kernel.InstanceHooks[*Instance] = capability{}
var _ kernel.ActivationHooks[*Instance] = capability{}
var _ config.DefaultContract = capability{}
var _ Access = (*access)(nil)
