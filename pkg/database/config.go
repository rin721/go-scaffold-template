package database

import (
	"github.com/rin721/go-scaffold2/pkg/database/internal/core"
)

// Engine 表示数据库能力的底层实现路线。
type Engine = core.Engine

const (
	// EngineGORM 表示使用 GORM 作为底层 ORM 实现。
	EngineGORM = core.EngineGORM
	// EngineSQL 表示使用 database/sql + sqlx 作为底层 SQL 实现。
	EngineSQL = core.EngineSQL
)

// Driver 表示数据库协议驱动。
type Driver = core.Driver

const (
	// DriverPostgres 表示 PostgreSQL 数据库。
	DriverPostgres = core.DriverPostgres
	// DriverMySQL 表示 MySQL 数据库。
	DriverMySQL = core.DriverMySQL
)

// PoolConfig 定义连接池参数。
type PoolConfig = core.PoolConfig

// Config 定义数据库客户端构造参数。
type Config = core.Config

const (
	DefaultMaxOpenConns    = core.DefaultMaxOpenConns
	DefaultMaxIdleConns    = core.DefaultMaxIdleConns
	DefaultConnMaxLifetime = core.DefaultConnMaxLifetime
	DefaultConnMaxIdleTime = core.DefaultConnMaxIdleTime
	DefaultPingTimeout     = core.DefaultPingTimeout
)

// DefaultConfig 返回数据库默认配置骨架。
func DefaultConfig() Config {
	return core.DefaultConfig()
}

// ValidateConfig 校验数据库配置，并应用与 New 相同的默认值和约束语义。
//
// 本函数不创建连接，适合独立应用或装配层在资源切换前验证候选配置。
func ValidateConfig(cfg *Config) error {
	_, err := core.ResolveConfig(cfg)
	return err
}
