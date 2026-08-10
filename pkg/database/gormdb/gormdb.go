package gormdb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rin721/go-scaffold2/pkg/database/internal/core"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Config 是 GORM 实现使用的数据库配置。
type Config = core.Config

// Client 是基于 GORM 的数据库实现。
type Client struct {
	db    *gorm.DB
	sqlDB *sql.DB
	cfg   core.ResolvedConfig
}

// New 创建 GORM 数据库客户端。
func New(ctx context.Context, cfg *Config) (*Client, error) {
	resolved, err := core.ResolveConfig(cfg)
	if err != nil {
		return nil, err
	}
	if resolved.Engine != core.EngineGORM {
		return nil, fmt.Errorf("%w: gormdb requires %q", core.ErrInvalidEngine, core.EngineGORM)
	}
	return Open(ctx, resolved)
}

// Open 使用已解析配置创建 GORM 数据库客户端。
func Open(ctx context.Context, cfg core.ResolvedConfig) (*Client, error) {
	if err := core.ValidateContext(ctx); err != nil {
		return nil, err
	}
	if err := core.ValidateResolvedConfig(cfg); err != nil {
		return nil, err
	}

	db, err := gorm.Open(dialector(cfg), &gorm.Config{
		SkipDefaultTransaction: true,
		DisableAutomaticPing:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("open gorm driver %q: %w", cfg.Driver, err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("load gorm sql db: %w", err)
	}

	return newClient(ctx, cfg, db, sqlDB)
}

func newClient(ctx context.Context, cfg core.ResolvedConfig, db *gorm.DB, sqlDB *sql.DB) (*Client, error) {
	core.ApplyPoolConfig(sqlDB, cfg.Pool)
	if err := core.Ping(ctx, sqlDB, cfg.PingTimeout); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &Client{db: db, sqlDB: sqlDB, cfg: cfg}, nil
}

func dialector(cfg core.ResolvedConfig) gorm.Dialector {
	switch cfg.Driver {
	case core.DriverPostgres:
		return postgres.Open(cfg.DSN)
	case core.DriverMySQL:
		return mysql.Open(cfg.DSN)
	default:
		return postgres.Open(cfg.DSN)
	}
}

// DB 返回带有上下文的 GORM 实例，仅供明确选择 ORM 的基础设施层使用。
func (c *Client) DB(ctx context.Context) *gorm.DB {
	if ctx == nil {
		ctx = context.Background()
	}
	return c.db.WithContext(ctx)
}

// SQLDB 返回 GORM 底层共享连接池。
func (c *Client) SQLDB() *sql.DB {
	return c.sqlDB
}

func (c *Client) Exec(ctx context.Context, query string, args ...any) (core.Result, error) {
	if err := core.ValidateContext(ctx); err != nil {
		return nil, err
	}
	result := c.db.WithContext(ctx).Exec(query, args...)
	if result.Error != nil {
		return nil, result.Error
	}
	return core.RowsAffectedResult{Count: result.RowsAffected}, nil
}

func (c *Client) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if err := core.ValidateContext(ctx); err != nil {
		return nil, err
	}
	return c.db.WithContext(ctx).Raw(query, args...).Rows()
}

func (c *Client) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	if ctx == nil {
		ctx = context.Background()
	}
	return c.db.WithContext(ctx).Raw(query, args...).Row()
}

func (c *Client) Get(ctx context.Context, dest any, query string, args ...any) error {
	if err := core.ValidateContext(ctx); err != nil {
		return err
	}
	tx := c.db.WithContext(ctx).Raw(query, args...).Scan(dest)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return core.ErrNotFound
	}
	return nil
}

func (c *Client) Select(ctx context.Context, dest any, query string, args ...any) error {
	if err := core.ValidateContext(ctx); err != nil {
		return err
	}
	return c.db.WithContext(ctx).Raw(query, args...).Scan(dest).Error
}

func (c *Client) WithinTx(ctx context.Context, opts *sql.TxOptions, fn func(context.Context, core.Tx) error) error {
	if err := core.ValidateContext(ctx); err != nil {
		return err
	}
	if fn == nil {
		return core.ErrNilTransactionFunc
	}

	return c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(ctx, &txClient{db: tx})
	}, opts)
}

func (c *Client) Ping(ctx context.Context) error {
	return core.Ping(ctx, c.sqlDB, c.cfg.PingTimeout)
}

func (c *Client) Stats() sql.DBStats {
	return c.sqlDB.Stats()
}

func (c *Client) Close() error {
	return c.sqlDB.Close()
}

type txClient struct {
	db *gorm.DB
}

func (t *txClient) Exec(ctx context.Context, query string, args ...any) (core.Result, error) {
	if err := core.ValidateContext(ctx); err != nil {
		return nil, err
	}
	result := t.db.WithContext(ctx).Exec(query, args...)
	if result.Error != nil {
		return nil, result.Error
	}
	return core.RowsAffectedResult{Count: result.RowsAffected}, nil
}

func (t *txClient) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if err := core.ValidateContext(ctx); err != nil {
		return nil, err
	}
	return t.db.WithContext(ctx).Raw(query, args...).Rows()
}

func (t *txClient) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	if ctx == nil {
		ctx = context.Background()
	}
	return t.db.WithContext(ctx).Raw(query, args...).Row()
}

func (t *txClient) Get(ctx context.Context, dest any, query string, args ...any) error {
	if err := core.ValidateContext(ctx); err != nil {
		return err
	}
	tx := t.db.WithContext(ctx).Raw(query, args...).Scan(dest)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return core.ErrNotFound
	}
	return nil
}

func (t *txClient) Select(ctx context.Context, dest any, query string, args ...any) error {
	if err := core.ValidateContext(ctx); err != nil {
		return err
	}
	return t.db.WithContext(ctx).Raw(query, args...).Scan(dest).Error
}

var _ core.Client = (*Client)(nil)
var _ core.Tx = (*txClient)(nil)
