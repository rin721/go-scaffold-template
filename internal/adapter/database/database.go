// Package database 把 pkg/database 适配为可由 kernel 管理和注入的基础能力。
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/rin721/go-scaffold2/internal/adapter"
	"github.com/rin721/go-scaffold2/internal/kernel"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkgdatabase "github.com/rin721/go-scaffold2/pkg/database"
)

const (
	// ID 是 Database adapter 在 kernel 内的稳定能力标识。
	ID kernel.ID = "database"
	// ConfigPath 是 Database adapter 使用的顶层配置路径。
	ConfigPath = "database"
)

// Access 是业务构造函数接收的稳定数据库能力入口。
//
// Client、Rows、Row 和事务对象均不得逃逸 Use 回调；回调返回即代表本次使用结束。
type Access interface {
	adapter.Access[pkgdatabase.Client]
}

// Config 是 Database adapter 的 typed 配置 DTO。
//
// DTO 归属于装配边界，pkg/database 因此不依赖 mapstructure 或 kernel 配置路径。
type Config struct {
	Engine      pkgdatabase.Engine `mapstructure:"engine"`
	Driver      pkgdatabase.Driver `mapstructure:"driver"`
	DSN         string             `mapstructure:"dsn"`
	Pool        PoolConfig         `mapstructure:"pool"`
	PingTimeout time.Duration      `mapstructure:"pingTimeout"`
}

// PoolConfig 是 Database adapter 的连接池配置 DTO。
type PoolConfig struct {
	MaxOpenConns    int           `mapstructure:"maxOpenConns"`
	MaxIdleConns    int           `mapstructure:"maxIdleConns"`
	ConnMaxLifetime time.Duration `mapstructure:"connMaxLifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"connMaxIdleTime"`
}

// Register 把 Database adapter 登记到尚未启动的 kernel。
func Register(runtime *kernel.Kernel) (Access, error) {
	return register(runtime, adapter.BuilderFunc[Config, pkgdatabase.Client](build))
}

func register(runtime *kernel.Kernel, builder adapter.Builder[Config, pkgdatabase.Client]) (Access, error) {
	handle, err := kernel.Register(runtime, kernel.Definition[Config, pkgdatabase.Client]{
		ID:         ID,
		ConfigPath: ConfigPath,
		Decode:     decode,
		Builder:    builder,
		Lifecycle: adapter.LifecycleFuncs[pkgdatabase.Client]{
			OnStart: func(ctx context.Context, client pkgdatabase.Client) error {
				if err := client.Ping(ctx); err != nil {
					return fmt.Errorf("verify database readiness: %w", err)
				}
				return nil
			},
			OnStop: func(_ context.Context, client pkgdatabase.Client) error {
				return client.Close()
			},
		},
	})
	if err != nil {
		return nil, err
	}
	return &databaseAccess{handle: handle}, nil
}

type databaseAccess struct {
	handle *kernel.Handle[pkgdatabase.Client]
}

func (a *databaseAccess) Use(ctx context.Context, use func(pkgdatabase.Client) error) error {
	return a.handle.Use(ctx, use)
}

func decode(snapshot config.Snapshot) (Config, error) {
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

func defaultConfig() Config {
	defaults := pkgdatabase.DefaultConfig()
	return Config{
		Pool: PoolConfig{
			MaxOpenConns:    defaults.Pool.MaxOpenConns,
			MaxIdleConns:    defaults.Pool.MaxIdleConns,
			ConnMaxLifetime: defaults.Pool.ConnMaxLifetime,
			ConnMaxIdleTime: defaults.Pool.ConnMaxIdleTime,
		},
		PingTimeout: defaults.PingTimeout,
	}
}

func (c Config) packageConfig() pkgdatabase.Config {
	return pkgdatabase.Config{
		Engine: c.Engine,
		Driver: c.Driver,
		DSN:    c.DSN,
		Pool: pkgdatabase.PoolConfig{
			MaxOpenConns:    c.Pool.MaxOpenConns,
			MaxIdleConns:    c.Pool.MaxIdleConns,
			ConnMaxLifetime: c.Pool.ConnMaxLifetime,
			ConnMaxIdleTime: c.Pool.ConnMaxIdleTime,
		},
		PingTimeout: c.PingTimeout,
	}
}

func build(ctx context.Context, cfg Config) (pkgdatabase.Client, error) {
	packageConfig := cfg.packageConfig()
	client, err := pkgdatabase.New(ctx, &packageConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
}
