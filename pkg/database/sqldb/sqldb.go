package sqldb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/rin721/go-scaffold2/pkg/database/internal/core"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Config 是 SQL 实现使用的数据库配置。
type Config = core.Config

// Client 是基于 database/sql + sqlx 的数据库实现。
type Client struct {
	db  *sqlx.DB
	cfg core.ResolvedConfig
}

// New 创建 SQL 数据库客户端。
func New(ctx context.Context, cfg *Config) (*Client, error) {
	resolved, err := core.ResolveConfig(cfg)
	if err != nil {
		return nil, err
	}
	if resolved.Engine != core.EngineSQL {
		return nil, fmt.Errorf("%w: sqldb requires %q", core.ErrInvalidEngine, core.EngineSQL)
	}
	return Open(ctx, resolved)
}

// Open 使用已解析配置创建 SQL 数据库客户端。
func Open(ctx context.Context, cfg core.ResolvedConfig) (*Client, error) {
	if err := core.ValidateContext(ctx); err != nil {
		return nil, err
	}
	if err := core.ValidateResolvedConfig(cfg); err != nil {
		return nil, err
	}

	db, err := sqlx.Open(core.SQLDriverName(cfg.Driver), cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open sql driver %q: %w", cfg.Driver, err)
	}
	return newClient(ctx, cfg, db)
}

func newClient(ctx context.Context, cfg core.ResolvedConfig, db *sqlx.DB) (*Client, error) {
	core.ApplyPoolConfig(db.DB, cfg.Pool)
	if err := core.Ping(ctx, db.DB, cfg.PingTimeout); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Client{db: db, cfg: cfg}, nil
}

func (c *Client) Exec(ctx context.Context, query string, args ...any) (core.Result, error) {
	if err := core.ValidateContext(ctx); err != nil {
		return nil, err
	}
	return c.db.ExecContext(ctx, query, args...)
}

func (c *Client) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if err := core.ValidateContext(ctx); err != nil {
		return nil, err
	}
	return c.db.QueryContext(ctx, query, args...)
}

func (c *Client) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	if ctx == nil {
		ctx = context.Background()
	}
	return c.db.QueryRowContext(ctx, query, args...)
}

func (c *Client) Get(ctx context.Context, dest any, query string, args ...any) error {
	if err := core.ValidateContext(ctx); err != nil {
		return err
	}
	if err := c.db.GetContext(ctx, dest, query, args...); err != nil {
		return translateSQLError(err)
	}
	return nil
}

func (c *Client) Select(ctx context.Context, dest any, query string, args ...any) error {
	if err := core.ValidateContext(ctx); err != nil {
		return err
	}
	return c.db.SelectContext(ctx, dest, query, args...)
}

func (c *Client) WithinTx(ctx context.Context, opts *sql.TxOptions, fn func(context.Context, core.Tx) error) error {
	if err := core.ValidateContext(ctx); err != nil {
		return err
	}
	if fn == nil {
		return core.ErrNilTransactionFunc
	}

	tx, err := c.db.BeginTxx(ctx, opts)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if err := fn(ctx, &txClient{tx: tx}); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("transaction failed: %w; rollback failed: %v", err, rollbackErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (c *Client) Ping(ctx context.Context) error {
	return core.Ping(ctx, c.db.DB, c.cfg.PingTimeout)
}

func (c *Client) Stats() sql.DBStats {
	return c.db.Stats()
}

func (c *Client) Close() error {
	return c.db.Close()
}

type txClient struct {
	tx *sqlx.Tx
}

func (t *txClient) Exec(ctx context.Context, query string, args ...any) (core.Result, error) {
	if err := core.ValidateContext(ctx); err != nil {
		return nil, err
	}
	return t.tx.ExecContext(ctx, query, args...)
}

func (t *txClient) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if err := core.ValidateContext(ctx); err != nil {
		return nil, err
	}
	return t.tx.QueryContext(ctx, query, args...)
}

func (t *txClient) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	if ctx == nil {
		ctx = context.Background()
	}
	return t.tx.QueryRowContext(ctx, query, args...)
}

func (t *txClient) Get(ctx context.Context, dest any, query string, args ...any) error {
	if err := core.ValidateContext(ctx); err != nil {
		return err
	}
	if err := t.tx.GetContext(ctx, dest, query, args...); err != nil {
		return translateSQLError(err)
	}
	return nil
}

func (t *txClient) Select(ctx context.Context, dest any, query string, args ...any) error {
	if err := core.ValidateContext(ctx); err != nil {
		return err
	}
	return t.tx.SelectContext(ctx, dest, query, args...)
}

func translateSQLError(err error) error {
	if err == sql.ErrNoRows {
		return core.ErrNotFound
	}
	return err
}

var _ core.Client = (*Client)(nil)
var _ core.Tx = (*txClient)(nil)
