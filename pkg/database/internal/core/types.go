package core

import (
	"context"
	"database/sql"
	"time"
)

// Engine 表示数据库能力的底层实现路线。
type Engine string

const (
	// EngineGORM 表示使用 GORM 作为底层 ORM 实现。
	EngineGORM Engine = "gorm"
	// EngineSQL 表示使用 database/sql + sqlx 作为底层 SQL 实现。
	EngineSQL Engine = "sql"
)

// Driver 表示数据库协议驱动。
type Driver string

const (
	// DriverPostgres 表示 PostgreSQL 数据库。
	DriverPostgres Driver = "postgres"
	// DriverMySQL 表示 MySQL 数据库。
	DriverMySQL Driver = "mysql"
)

// PoolConfig 定义连接池参数。
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// Config 定义数据库客户端构造参数。
type Config struct {
	Engine      Engine
	Driver      Driver
	DSN         string
	Pool        PoolConfig
	PingTimeout time.Duration
}

// Result 表示 SQL 执行结果。
type Result interface {
	LastInsertId() (int64, error)
	RowsAffected() (int64, error)
}

// Executor 定义数据库查询和执行能力。
type Executor interface {
	Exec(ctx context.Context, query string, args ...any) (Result, error)
	Query(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) *sql.Row
	Get(ctx context.Context, dest any, query string, args ...any) error
	Select(ctx context.Context, dest any, query string, args ...any) error
}

// Tx 表示事务中的执行能力。
type Tx interface {
	Executor
}

// Transactor 定义事务边界能力。
type Transactor interface {
	WithinTx(ctx context.Context, opts *sql.TxOptions, fn func(context.Context, Tx) error) error
}

// HealthChecker 定义数据库健康检查能力。
type HealthChecker interface {
	Ping(ctx context.Context) error
}

// StatsProvider 定义连接池统计能力。
type StatsProvider interface {
	Stats() sql.DBStats
}

// Closer 定义资源释放能力。
type Closer interface {
	Close() error
}

// Client 定义业务代码使用的统一数据库能力。
type Client interface {
	Executor
	Transactor
	HealthChecker
	StatsProvider
	Closer
}

type ResolvedConfig struct {
	Engine      Engine
	Driver      Driver
	DSN         string
	Pool        PoolConfig
	PingTimeout time.Duration
}
