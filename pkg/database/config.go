package database

import (
	"fmt"
	"strings"
	"time"
)

// Driver 表示数据库协议和方言。
type Driver string

const (
	// DriverSQLite 表示进程内 SQLite 数据库。
	DriverSQLite Driver = "sqlite"
	// DriverPostgres 表示 PostgreSQL 数据库。
	DriverPostgres Driver = "postgres"
	// DriverMySQL 表示 MySQL 数据库。
	DriverMySQL Driver = "mysql"
)

const (
	// DefaultSQLitePath 是本地 SQLite 的默认相对路径。
	DefaultSQLitePath = ".data/app.db"
	// DefaultMaxOpenConns 是连接池默认最大打开连接数。
	DefaultMaxOpenConns = 25
	// DefaultMaxIdleConns 是连接池默认最大空闲连接数。
	DefaultMaxIdleConns = 5
	// DefaultConnMaxLifetime 是单连接默认最大生命周期。
	DefaultConnMaxLifetime = 30 * time.Minute
	// DefaultConnMaxIdleTime 是空闲连接默认最大保留时间。
	DefaultConnMaxIdleTime = 5 * time.Minute
	// DefaultPingTimeout 是连接就绪检查默认超时。
	DefaultPingTimeout = 5 * time.Second
)

// PoolConfig 定义连接池参数。
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// Config 定义数据库资源构造参数。GORM 由构造代码选择，不属于运行时配置。
type Config struct {
	Driver      Driver
	DSN         string
	Pool        PoolConfig
	PingTimeout time.Duration
}

type resolvedConfig struct {
	Driver      Driver
	DSN         string
	Pool        PoolConfig
	PingTimeout time.Duration
}

// DefaultConfig 返回可直接启动本地 SQLite 的默认配置。
func DefaultConfig() Config {
	return Config{
		Driver: DriverSQLite,
		DSN:    DefaultSQLitePath,
		Pool: PoolConfig{
			MaxOpenConns:    DefaultMaxOpenConns,
			MaxIdleConns:    DefaultMaxIdleConns,
			ConnMaxLifetime: DefaultConnMaxLifetime,
			ConnMaxIdleTime: DefaultConnMaxIdleTime,
		},
		PingTimeout: DefaultPingTimeout,
	}
}

// ValidateConfig 校验配置并应用与 NewGORM 相同的默认值语义，但不创建连接。
func ValidateConfig(cfg *Config) error {
	_, err := resolveConfig(cfg)
	return err
}

func resolveConfig(cfg *Config) (resolvedConfig, error) {
	if cfg == nil {
		return resolvedConfig{}, ErrNilConfig
	}
	defaults := DefaultConfig()
	resolved := resolvedConfig{
		Driver: cfg.Driver, DSN: strings.TrimSpace(cfg.DSN),
		Pool: defaults.Pool, PingTimeout: defaults.PingTimeout,
	}
	if resolved.Driver == "" {
		resolved.Driver = defaults.Driver
	}
	if resolved.DSN == "" && resolved.Driver == DriverSQLite {
		resolved.DSN = defaults.DSN
	}
	if cfg.Pool.MaxOpenConns != 0 {
		resolved.Pool.MaxOpenConns = cfg.Pool.MaxOpenConns
	}
	if cfg.Pool.MaxIdleConns != 0 {
		resolved.Pool.MaxIdleConns = cfg.Pool.MaxIdleConns
	}
	if cfg.Pool.ConnMaxLifetime != 0 {
		resolved.Pool.ConnMaxLifetime = cfg.Pool.ConnMaxLifetime
	}
	if cfg.Pool.ConnMaxIdleTime != 0 {
		resolved.Pool.ConnMaxIdleTime = cfg.Pool.ConnMaxIdleTime
	}
	if cfg.PingTimeout != 0 {
		resolved.PingTimeout = cfg.PingTimeout
	}

	switch resolved.Driver {
	case DriverSQLite, DriverPostgres, DriverMySQL:
	default:
		return resolvedConfig{}, fmt.Errorf("%w: %q", ErrInvalidDriver, resolved.Driver)
	}
	if resolved.DSN == "" {
		return resolvedConfig{}, ErrEmptyDSN
	}
	if resolved.Pool.MaxOpenConns < 0 || resolved.Pool.MaxIdleConns < 0 ||
		resolved.Pool.ConnMaxLifetime < 0 || resolved.Pool.ConnMaxIdleTime < 0 || resolved.PingTimeout < 0 {
		return resolvedConfig{}, ErrInvalidPoolConfig
	}
	if resolved.Pool.MaxOpenConns > 0 && resolved.Pool.MaxIdleConns > resolved.Pool.MaxOpenConns {
		return resolvedConfig{}, fmt.Errorf("%w: max idle conns exceeds max open conns", ErrInvalidPoolConfig)
	}
	return resolved, nil
}
