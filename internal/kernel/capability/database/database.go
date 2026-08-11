// Package database 定义由 Kernel 托管的 Database 能力。
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/rin721/go-scaffold2/internal/kernel"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkgdatabase "github.com/rin721/go-scaffold2/pkg/database"
)

const (
	// ID 是 Database capability 在 Kernel 内的稳定能力标识。
	ID kernel.ID = "database"
	// ConfigPath 是 Database capability 使用的顶层配置路径。
	ConfigPath = "database"
)

// Access 是业务构造函数接收的稳定数据库能力入口。
//
// Client、Rows、Row 和事务对象均不得逃逸 Use 回调；回调返回即代表本次使用结束。
type Access interface {
	kernel.Access[pkgdatabase.Client]
}

// Config 是 Database capability 的 typed 配置契约。
//
// Config 归属于 Kernel 能力边界，pkg/database 因此不依赖 mapstructure 或配置路径。
type Config struct {
	Engine      pkgdatabase.Engine `mapstructure:"engine"`
	Driver      pkgdatabase.Driver `mapstructure:"driver"`
	DSN         string             `mapstructure:"dsn"`
	Pool        PoolConfig         `mapstructure:"pool"`
	PingTimeout time.Duration      `mapstructure:"pingTimeout"`
}

// PoolConfig 是 Database capability 的连接池配置契约。
type PoolConfig struct {
	MaxOpenConns    int           `mapstructure:"maxOpenConns"`
	MaxIdleConns    int           `mapstructure:"maxIdleConns"`
	ConnMaxLifetime time.Duration `mapstructure:"connMaxLifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"connMaxIdleTime"`
}

// Definition 返回无注册副作用的 Database 能力定义。
//
// 调用方必须在 composition 中显式登记返回值；本包不持有 Kernel 或运行中实例。
func Definition() kernel.Definition[Config, pkgdatabase.Client] {
	implementation := capability{}
	return kernel.Definition[Config, pkgdatabase.Client]{
		ID:         ID,
		ConfigPath: ConfigPath,
		Decode:     decode,
		Builder:    implementation,
		Hooks:      implementation,
	}
}

type capability struct{}

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

func (capability) Build(ctx context.Context, cfg Config) (pkgdatabase.Client, error) {
	packageConfig := cfg.packageConfig()
	client, err := pkgdatabase.New(ctx, &packageConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (capability) Start(ctx context.Context, client pkgdatabase.Client) error {
	if err := client.Ping(ctx); err != nil {
		return fmt.Errorf("verify database readiness: %w", err)
	}
	return nil
}

func (capability) Stop(_ context.Context, client pkgdatabase.Client) error {
	return client.Close()
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

var _ kernel.Builder[Config, pkgdatabase.Client] = capability{}
var _ kernel.InstanceHooks[pkgdatabase.Client] = capability{}
