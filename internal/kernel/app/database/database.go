// Package database 定义由 Kernel 治理的具名 Database App 组件。
package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rin721/go-scaffold2/internal/kernel/app"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkgdatabase "github.com/rin721/go-scaffold2/pkg/database"
	pkglogger "github.com/rin721/go-scaffold2/pkg/logger"
)

// Client 是调用方在租约内使用的数据库能力，不包含共享连接池关闭权。
type Client interface {
	pkgdatabase.Executor
	pkgdatabase.Transactor
	pkgdatabase.HealthChecker
	pkgdatabase.StatsProvider
}

// Access 是调用方接收的稳定数据库租约入口。
type Access interface {
	Use(context.Context, func(Client) error) error
}

// Config 是 Database App 的 typed 配置契约。
type Config struct {
	Engine      pkgdatabase.Engine `mapstructure:"engine"`
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

type dependencies struct{ logging pkglogger.Access }

// Definition 创建使用显式 Logger Binding 的具名 Database 声明。
func Definition(spec app.Spec, logging app.Input[pkglogger.Access]) (app.Definition[Access], error) {
	if err := spec.ValidateConfigured(); err != nil {
		return app.Definition[Access]{}, err
	}
	source, err := app.Configured(spec.ConfigPath, decoder(spec.ConfigPath), defaults{})
	if err != nil {
		return app.Definition[Access]{}, err
	}
	deps, err := app.DependencySet(func(values app.Values) (dependencies, error) {
		access, err := app.Resolve(values, logging)
		if err != nil {
			return dependencies{}, err
		}
		if access == nil {
			return dependencies{}, fmt.Errorf("database logger access is nil")
		}
		return dependencies{logging: access}, nil
	}, logging)
	if err != nil {
		return app.Definition[Access]{}, err
	}
	return app.ManagedConfigured(spec.ID, source, deps, build, app.Leased(newAccess), app.KernelInstanceSwap, app.WithReady(ready), app.WithStop(stop))
}

type access struct{ delegate app.Lease[pkgdatabase.Client] }

func newAccess(delegate app.Lease[pkgdatabase.Client]) (Access, error) {
	if delegate == nil {
		return nil, fmt.Errorf("database lease is nil")
	}
	return &access{delegate: delegate}, nil
}
func (a *access) Use(ctx context.Context, use func(Client) error) error {
	if use == nil {
		return fmt.Errorf("database access callback is nil")
	}
	return a.delegate.Use(ctx, func(client pkgdatabase.Client) error {
		if client == nil {
			return fmt.Errorf("database instance is nil")
		}
		return use(client)
	})
}

func build(ctx context.Context, cfg Config, deps dependencies) (pkgdatabase.Client, error) {
	packageConfig := cfg.packageConfig()
	client, err := pkgdatabase.New(ctx, &packageConfig)
	if err != nil {
		return nil, err
	}
	if err := deps.logging.Use(ctx, func(log pkglogger.Logger) error { log.Debug("database candidate built"); return nil }); err != nil {
		return nil, errors.Join(fmt.Errorf("write database build log: %w", err), client.Close())
	}
	return client, nil
}
func ready(ctx context.Context, client pkgdatabase.Client) error {
	if err := client.Ping(ctx); err != nil {
		return fmt.Errorf("verify database readiness: %w", err)
	}
	return nil
}
func stop(_ context.Context, client pkgdatabase.Client) error { return client.Close() }

func decoder(path string) app.Decoder[Config] {
	return func(snapshot config.Snapshot) (Config, error) {
		cfg := defaultConfig()
		if err := snapshot.DecodeSection(path, &cfg); err != nil {
			return Config{}, err
		}
		packageConfig := cfg.packageConfig()
		if err := pkgdatabase.ValidateConfig(&packageConfig); err != nil {
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
	values := pkgdatabase.DefaultConfig()
	maxOpen, err := config.Number(fmt.Sprint(values.Pool.MaxOpenConns))
	if err != nil {
		return nil, config.Continue, err
	}
	maxIdle, err := config.Number(fmt.Sprint(values.Pool.MaxIdleConns))
	if err != nil {
		return nil, config.Continue, err
	}
	return config.Object{config.FieldOf("engine", config.String("")), config.FieldOf("driver", config.String("")), config.FieldOf("dsn", config.String("")), config.FieldOf("pool", config.ObjectValue(config.Object{config.FieldOf("maxOpenConns", maxOpen), config.FieldOf("maxIdleConns", maxIdle), config.FieldOf("connMaxLifetime", config.Duration(values.Pool.ConnMaxLifetime)), config.FieldOf("connMaxIdleTime", config.Duration(values.Pool.ConnMaxIdleTime))})), config.FieldOf("pingTimeout", config.Duration(values.PingTimeout))}, config.Continue, nil
}
func defaultConfig() Config {
	values := pkgdatabase.DefaultConfig()
	return Config{Pool: PoolConfig{MaxOpenConns: values.Pool.MaxOpenConns, MaxIdleConns: values.Pool.MaxIdleConns, ConnMaxLifetime: values.Pool.ConnMaxLifetime, ConnMaxIdleTime: values.Pool.ConnMaxIdleTime}, PingTimeout: values.PingTimeout}
}
func (c Config) packageConfig() pkgdatabase.Config {
	return pkgdatabase.Config{Engine: c.Engine, Driver: c.Driver, DSN: c.DSN, Pool: pkgdatabase.PoolConfig{MaxOpenConns: c.Pool.MaxOpenConns, MaxIdleConns: c.Pool.MaxIdleConns, ConnMaxLifetime: c.Pool.ConnMaxLifetime, ConnMaxIdleTime: c.Pool.ConnMaxIdleTime}, PingTimeout: c.PingTimeout}
}

var _ Access = (*access)(nil)
var _ config.DefaultContract = defaults{}
