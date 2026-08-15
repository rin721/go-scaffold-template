// Package database 定义由 Kernel 治理的 Database App 组件。
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	pkgdatabase "github.com/rin721/go-scaffold-template/pkg/database"
)

const (
	ID         app.ID = "database"
	ConfigPath        = "database"
)

// Client 是调用方在租约内使用的数据库能力，不包含共享连接池关闭权。
type Client interface {
	pkgdatabase.Client
}

// Access 是调用方接收的稳定数据库租约入口。
type Access interface {
	Ping(context.Context) error
	Use(context.Context, func(Client) error) error
	WithinTx(context.Context, func(context.Context, Client, pkgdatabase.Tx) error) error
}

// Config 是 Database App 的 typed 配置契约。
type Config struct {
	Driver      pkgdatabase.Driver `mapstructure:"driver"`
	DSN         string             `mapstructure:"dsn"`
	Pool        PoolConfig         `mapstructure:"pool"`
	PingTimeout time.Duration      `mapstructure:"pingTimeout"`
}

// PoolConfig 描述数据库连接池边界。
type PoolConfig struct {
	MaxOpenConns    int           `mapstructure:"maxOpenConns"`
	MaxIdleConns    int           `mapstructure:"maxIdleConns"`
	ConnMaxLifetime time.Duration `mapstructure:"connMaxLifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"connMaxIdleTime"`
}

// Definition 返回无安装副作用的 Database 组件声明。
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
		app.WithTerminalFinalizer(stop),
	)
}

type access struct {
	delegate app.Lease[pkgdatabase.Resource]
}

func newAccess(delegate app.Lease[pkgdatabase.Resource]) (Access, error) {
	if delegate == nil {
		return nil, fmt.Errorf("database lease is nil")
	}
	return &access{delegate: delegate}, nil
}

// Ping 在当前资源租约内执行就绪检查，不向调用方暴露连接池所有权。
func (a *access) Ping(ctx context.Context) error {
	if ctx == nil {
		return pkgdatabase.ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return a.delegate.Use(ctx, func(resource pkgdatabase.Resource) error {
		if resource == nil {
			return pkgdatabase.ErrClientUnavailable
		}
		return resource.Ping(ctx)
	})
}

func (a *access) Use(ctx context.Context, use func(Client) error) error {
	if use == nil {
		return pkgdatabase.ErrNilClientFunc
	}
	return a.delegate.Use(ctx, func(client pkgdatabase.Resource) error {
		if client == nil {
			return fmt.Errorf("database instance is nil")
		}
		return pkgdatabase.Borrow(ctx, client.Client(), func(borrowed pkgdatabase.Client) error {
			return use(borrowed)
		})
	})
}

func (a *access) WithinTx(ctx context.Context, use func(context.Context, Client, pkgdatabase.Tx) error) error {
	if use == nil {
		return pkgdatabase.ErrNilTransactionFunc
	}
	return a.Use(ctx, func(client Client) error {
		return client.WithinTx(ctx, func(txCtx context.Context, tx pkgdatabase.Tx) error {
			return use(txCtx, client, tx)
		})
	})
}

func build(ctx context.Context, cfg Config, _ struct{}) (pkgdatabase.Resource, error) {
	packageConfig := cfg.packageConfig()
	return pkgdatabase.NewGORM(ctx, &packageConfig)
}

func ready(ctx context.Context, client pkgdatabase.Resource) error {
	if err := client.Ping(ctx); err != nil {
		return fmt.Errorf("verify database readiness: %w", err)
	}
	return nil
}

func stop(_ context.Context, client pkgdatabase.Resource) error { return client.Close() }

func decode(snapshot config.Snapshot) (Config, error) { return Decode(snapshot) }

// Decode 从完整配置快照读取 Database App 的项目自有配置。
func Decode(snapshot config.Snapshot) (Config, error) {
	cfg := defaultConfig()
	if err := snapshot.DecodeSection(ConfigPath, &cfg); err != nil {
		return Config{}, err
	}
	packageConfig := cfg.packageConfig()
	if err := pkgdatabase.ValidateConfig(&packageConfig); err != nil {
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
	values := pkgdatabase.DefaultConfig()
	maxOpen, err := config.Number(fmt.Sprint(values.Pool.MaxOpenConns))
	if err != nil {
		return nil, config.Continue, err
	}
	maxIdle, err := config.Number(fmt.Sprint(values.Pool.MaxIdleConns))
	if err != nil {
		return nil, config.Continue, err
	}
	return config.Object{
		config.FieldOf("driver", config.String(string(values.Driver))),
		config.FieldOf("dsn", config.String(values.DSN)),
		config.FieldOf("pool", config.ObjectValue(config.Object{
			config.FieldOf("maxOpenConns", maxOpen),
			config.FieldOf("maxIdleConns", maxIdle),
			config.FieldOf("connMaxLifetime", config.Duration(values.Pool.ConnMaxLifetime)),
			config.FieldOf("connMaxIdleTime", config.Duration(values.Pool.ConnMaxIdleTime)),
		})),
		config.FieldOf("pingTimeout", config.Duration(values.PingTimeout)),
	}, config.Continue, nil
}

func defaultConfig() Config {
	values := pkgdatabase.DefaultConfig()
	return Config{Driver: values.Driver, DSN: values.DSN, Pool: PoolConfig{
		MaxOpenConns: values.Pool.MaxOpenConns, MaxIdleConns: values.Pool.MaxIdleConns,
		ConnMaxLifetime: values.Pool.ConnMaxLifetime, ConnMaxIdleTime: values.Pool.ConnMaxIdleTime,
	}, PingTimeout: values.PingTimeout}
}

func (c Config) packageConfig() pkgdatabase.Config { return c.PackageConfig() }

// PackageConfig 返回跨模块 Adapter 可使用的稳定数据库配置副本。
func (c Config) PackageConfig() pkgdatabase.Config {
	return pkgdatabase.Config{
		Driver: c.Driver, DSN: c.DSN,
		Pool: pkgdatabase.PoolConfig{
			MaxOpenConns: c.Pool.MaxOpenConns, MaxIdleConns: c.Pool.MaxIdleConns,
			ConnMaxLifetime: c.Pool.ConnMaxLifetime, ConnMaxIdleTime: c.Pool.ConnMaxIdleTime,
		}, PingTimeout: c.PingTimeout,
	}
}

var _ Access = (*access)(nil)
var _ config.DefaultContract = defaults{}
