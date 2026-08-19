// Package execution 定义由 Kernel 治理的后台任务执行能力 App 组件。
// 后端默认形态：内存 backend（幂等占用 + 执行记录），通过组件开关可在 memory 与 disabled 间切换。
package execution

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	pkgexecution "github.com/rin721/go-scaffold-template/pkg/execution"
	"github.com/rin721/go-scaffold-template/pkg/resilience"
)

const (
	ID         app.ID = "execution"
	ConfigPath        = "execution"
)

// Driver 表示 Execution backend 的明确选择。
type Driver string

const (
	// DriverDisabled 表示当前进程不启用后台任务执行能力。
	DriverDisabled Driver = "disabled"
	// DriverMemory 表示当前进程使用内存 backend（进程内幂等 + 记录，单实例）。
	DriverMemory Driver = "memory"
)

// 应用层默认配置（032 集中声明）：不依赖 pkg/execution 的默认值。
const (
	defaultDriver        = DriverMemory
	defaultMaxAttempts   = 3
	defaultInitialWaitMs = 50
	defaultMaxWaitMs     = 500
)

// Config 是 Execution App 的 typed 配置契约。
type Config struct {
	Driver           Driver `mapstructure:"driver"`
	RetryMaxAttempts int    `mapstructure:"retryMaxAttempts"`
	RetryInitialWait int    `mapstructure:"retryInitialWaitMs"`
	RetryMaxWait     int    `mapstructure:"retryMaxWaitMs"`
}

// Access 是业务模块消费的稳定执行入口。
type Access interface {
	Execute(context.Context, pkgexecution.Execution) (pkgexecution.Result, error)
}

type resource struct {
	driver        Driver
	executor      pkgexecution.OperationExecutor
	defaultPolicy resilience.RetryPolicy
}

type access struct {
	delegate app.Lease[*resource]
}

// Definition 返回无安装副作用的 Execution 组件声明。
func Definition() (app.Definition[Access], error) {
	source, err := app.Configured(ConfigPath, decode, defaults{})
	if err != nil {
		return app.Definition[Access]{}, err
	}
	return app.ManagedConfigured(
		ID,
		source,
		app.FixedDependencies(struct{}{}),
		build,
		app.Leased(newAccess),
		app.KernelInstanceSwap,
		app.WithReady(ready),
	)
}

func newAccess(delegate app.Lease[*resource]) (Access, error) {
	if delegate == nil {
		return nil, fmt.Errorf("execution lease is nil")
	}
	return &access{delegate: delegate}, nil
}

func (a *access) Execute(ctx context.Context, exec pkgexecution.Execution) (pkgexecution.Result, error) {
	if ctx == nil {
		return pkgexecution.Result{}, pkgexecution.ErrNilContext
	}
	var result pkgexecution.Result
	err := a.delegate.Use(ctx, func(current *resource) error {
		if current == nil {
			return fmt.Errorf("execution instance is nil")
		}
		if current.driver == DriverDisabled || current.executor == nil {
			return fmt.Errorf("execution backend is disabled")
		}
		// 未显式配置重试策略时应用本组件集中声明的默认策略。
		if exec.Policy.MaxAttempts == 0 {
			exec.Policy = current.defaultPolicy
		}
		var err error
		result, err = current.executor.Execute(ctx, exec)
		return err
	})
	return result, err
}

func build(ctx context.Context, cfg Config, _ struct{}) (*resource, error) {
	if ctx == nil {
		return nil, app.ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch cfg.Driver {
	case DriverDisabled:
		return &resource{driver: DriverDisabled}, nil
	case DriverMemory:
		policy := resilience.RetryPolicy{
			MaxAttempts: cfg.RetryMaxAttempts,
			InitialWait: time.Duration(cfg.RetryInitialWait) * time.Millisecond,
			MaxWait:     time.Duration(cfg.RetryMaxWait) * time.Millisecond,
		}
		store := pkgexecution.NewMemoryStore()
		return &resource{
			driver:        DriverMemory,
			executor:      pkgexecution.NewExecutor(store),
			defaultPolicy: policy,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported execution driver %q", cfg.Driver)
	}
}

func ready(ctx context.Context, current *resource) error {
	if current == nil {
		return fmt.Errorf("execution instance is nil")
	}
	// 内存 backend 无外部就绪依赖；disabled 也视为就绪（组件存在但关闭）。
	return nil
}

func decode(snapshot config.Snapshot) (Config, error) {
	cfg := defaultConfig()
	if err := snapshot.DecodeSection(ConfigPath, &cfg); err != nil {
		return Config{}, err
	}
	cfg.Driver = Driver(strings.ToLower(strings.TrimSpace(string(cfg.Driver))))
	switch cfg.Driver {
	case DriverDisabled, DriverMemory:
	default:
		return Config{}, fmt.Errorf("unsupported execution driver %q", cfg.Driver)
	}
	if cfg.RetryMaxAttempts < 0 || cfg.RetryInitialWait < 0 || cfg.RetryMaxWait < 0 {
		return Config{}, fmt.Errorf("execution retry policy values must be non-negative")
	}
	return cfg, nil
}

type defaults struct{}

func (defaults) Defaults(ctx context.Context) (config.Object, config.Control, error) {
	if ctx == nil {
		return nil, config.Continue, app.ErrNilContext
	}
	cfg := defaultConfig()
	attempts, err := config.Number(fmt.Sprint(cfg.RetryMaxAttempts))
	if err != nil {
		return nil, config.Continue, err
	}
	initial, err := config.Number(fmt.Sprint(cfg.RetryInitialWait))
	if err != nil {
		return nil, config.Continue, err
	}
	max, err := config.Number(fmt.Sprint(cfg.RetryMaxWait))
	if err != nil {
		return nil, config.Continue, err
	}
	return config.Object{
		config.FieldOf("driver", config.String(string(cfg.Driver))),
		config.FieldOf("retryMaxAttempts", attempts),
		config.FieldOf("retryInitialWaitMs", initial),
		config.FieldOf("retryMaxWaitMs", max),
	}, config.Continue, nil
}

func defaultConfig() Config {
	return Config{
		Driver:           defaultDriver,
		RetryMaxAttempts: defaultMaxAttempts,
		RetryInitialWait: defaultInitialWaitMs,
		RetryMaxWait:     defaultMaxWaitMs,
	}
}

var _ Access = (*access)(nil)
var _ config.DefaultContract = defaults{}
