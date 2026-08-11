// Package database 封装 pkg/database 的配置、构造和生命周期钩子。
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
	// ID 是 Database adapter 在 kernel 内的稳定能力标识。
	ID kernel.ID = "database"
	// ConfigPath 是 Database adapter 使用的顶层配置路径。
	ConfigPath = "database"
)

// Access 是业务构造函数接收的稳定数据库能力入口。
//
// Client、Rows、Row 和事务对象均不得逃逸 Use 回调；回调返回即代表本次使用结束。
type Access interface {
	kernel.Access[pkgdatabase.Client]
}

// Adapter 封装 Database 能力的配置解码、实例构造和生命周期钩子。
//
// Adapter 不登记 Kernel，也不持有运行中实例；具体启用位置由 Kernel assembly 决定。
type Adapter struct{}

// New 创建无状态的 Database Adapter。
func New() *Adapter {
	return &Adapter{}
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

// Decode 从 Kernel 配置快照中解码并校验 Database 配置。
func (a *Adapter) Decode(snapshot config.Snapshot) (Config, error) {
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

// Build 根据已校验配置创建新的 Database Client。
func (a *Adapter) Build(ctx context.Context, cfg Config) (pkgdatabase.Client, error) {
	packageConfig := cfg.packageConfig()
	client, err := pkgdatabase.New(ctx, &packageConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// Start 在实例发布前检查 Database 是否就绪。
func (a *Adapter) Start(ctx context.Context, client pkgdatabase.Client) error {
	if err := client.Ping(ctx); err != nil {
		return fmt.Errorf("verify database readiness: %w", err)
	}
	return nil
}

// Stop 释放 Database Client 拥有的资源。
func (a *Adapter) Stop(_ context.Context, client pkgdatabase.Client) error {
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

var _ kernel.Builder[Config, pkgdatabase.Client] = (*Adapter)(nil)
var _ kernel.Lifecycle[pkgdatabase.Client] = (*Adapter)(nil)
